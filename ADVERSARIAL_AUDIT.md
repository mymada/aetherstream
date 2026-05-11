# AetherStream Adversarial Audit Report

**Date:** 2026-05-11
**Scope:** pkg/api/, pkg/stream/, pkg/db/, pkg/oauth/, pkg/encoder/
**Methodology:** Static code review + adversarial reasoning (assume everything is broken)

---

## CRITICAL — Data Races & Concurrency Bugs

### C1. OAuth State Map Cleanup Goroutine Leak + Double-Close Panic
**File:** `pkg/oauth/oauth.go`
**Lines:** 60-64, 283-311
**Severity:** CRITICAL

`NewService` starts `go s.cleanupLoop()` unconditionally. `cleanupLoop` runs a `time.NewTicker(5 * time.Minute)` with `defer ticker.Stop()`. The `Stop()` method closes `s.stopClean` (line 310). If `Stop()` is called twice (e.g., shutdown + test cleanup), the second `close(s.stopClean)` will **panic** with "close of closed channel". There is no `sync.Once` guard.

Additionally, if `NewService` is called many times (e.g., in tests), each instance leaks a goroutine because nothing stops the ticker until `Stop()` is explicitly called. In long-running processes, this is acceptable, but in tests or dynamic reconfiguration scenarios, it leaks.

**Fix:**
```go
var stopOnce sync.Once
func (s *Service) Stop() {
    stopOnce.Do(func() { close(s.stopClean) })
}
```

---

### C2. Transcoder Jobs Map — Race on Concurrent Transcode Start
**File:** `pkg/stream/stream.go`
**Lines:** 376-426
**Severity:** CRITICAL

`Transcoder.Transcode` uses a classic check-then-act pattern with `RLock`/`Lock`:

```go
t.mu.RLock()
running := t.jobs[itemID]
t.mu.RUnlock()
if running { return ... }
// ... DB call ...
t.mu.Lock()
t.jobs[itemID] = true
t.mu.Unlock()
```

Between the `RUnlock` and the `Lock`, another goroutine can enter, see `running == false`, and also start a transcode for the same `itemID`. The test `TestTranscoderConcurrency` (line 84-107) only validates that after `wg.Wait()` the map is empty — it does **not** verify that only one transcode actually launched. This race can spawn multiple FFmpeg processes for the same item, causing disk corruption or resource exhaustion.

**Fix:** Move the entire check-and-set into a single `Lock` critical section, or use `sync.Map`/`atomic` with a compare-and-swap pattern.

---

### C3. WebSocket Hub — Global Singleton Race + Connection Leak
**File:** `pkg/ws/hub.go`
**Lines:** 34-36, 71-81, 105-129
**Severity:** CRITICAL

`globalHub` is a package-level singleton initialized with `clients: make(map[string]*Client)`. There is **no synchronization** during initialization — if multiple goroutines call `HandleWebSocket` before the package init completes (unlikely in Go but possible with `go test -count=N`), the map could be reallocated.

More importantly, `HandleWebSocket` defers a cleanup that deletes the client from `globalHub.clients`, but the `writePump` goroutine also calls `c.conn.Close()` on exit. If the reader loop exits first, the deferred cleanup runs and closes the connection. Then `writePump` tries `c.conn.WriteMessage(...)` on a closed connection — `gorilla/websocket` returns an error, but the second `Close()` is benign. However, if `writePump` exits first (e.g., write timeout), it closes the connection, and then the reader loop's deferred cleanup also calls `Close()` — double-close is safe in `gorilla/websocket`, but the real issue is:

The `client.send` channel is never closed. If `Broadcast` or `BroadcastToUser` tries to send to a client whose `writePump` has exited but whose reader loop hasn't yet run the deferred cleanup, the `select` in `Broadcast` uses `default:` to skip — this is fine. But if `writePump` is blocked on `c.send` and the reader loop hasn't started draining, the channel can fill up (buffered 256). This is a memory pressure issue under high broadcast load.

**Fix:** Close `client.send` when the reader loop exits, and ensure `writePump` handles channel closure gracefully.

---

### C4. Brute-Force Limiter — Unbounded Map Growth + Goroutine Leak
**File:** `pkg/api/security.go`
**Lines:** 180-237
**Severity:** HIGH

`newBruteForceLimiter` starts `go b.cleanup()` which runs a `time.NewTicker(10 * time.Minute)`. The ticker is **never stopped**. If `BruteForceProtection()` middleware is re-initialized (e.g., in tests or hot reload), each call creates a new `bruteForceLimiter` with its own immortal cleanup goroutine and an unbounded `attempts` map. Even in the singleton `globalBruteForce`, the ticker goroutine leaks for the lifetime of the process.

The `attempts` map grows without bound for every unique IP/username that ever hits a login endpoint. An attacker can flood login with random usernames to cause memory exhaustion (DoS).

**Fix:**
1. Stop the ticker when the limiter is no longer needed.
2. Cap the map size (LRU eviction).
3. Use a single global instance with proper lifecycle management.

---

### C5. IP Rate Limiter — Unbounded Map Growth + Goroutine Leak
**File:** `pkg/api/security.go`
**Lines:** 364-416
**Severity:** HIGH

`newIPRateLimiter` starts `go rl.cleanup()` with a `time.NewTicker(5 * time.Minute)` that is **never stopped**. The `buckets` map is cleaned only after 10 minutes of inactivity, but under a DDoS or scan from thousands of IPs, the map grows unbounded. Each `RateLimitByIP` call creates a **new** limiter instance (line 420), so every route with a different `requestsPerMin` spawns a new goroutine and a new map. For 20 routes with rate limits, that's 20 immortal cleanup goroutines and 20 unbounded maps.

**Fix:** Use a single global rate limiter pool, or at least share limiters by capacity. Stop tickers on shutdown. Cap bucket map size.

---

### C6. SessionTimeout — DB Query on Every Request (Hot Path Inefficiency)
**File:** `pkg/api/security.go`
**Lines:** 308-346
**Severity:** HIGH

`SessionTimeout` middleware performs **two DB round-trips on every single request**:
1. `database.GetSessionLastSeen(sessionID)` (line 326)
2. `database.UpdateSessionLastSeen(sessionID)` (line 338)

With SQLite single-writer (`SetMaxOpenConns(1)`), this serializes all protected API requests through the single DB connection. Under load, this becomes a massive bottleneck. The `remaining` header is also computed incorrectly — it always returns `int(idleDuration.Seconds())` (1800 for 30min), not the actual remaining time.

**Fix:** Cache session last-seen in memory (e.g., `sync.Map` or Redis) with periodic DB flush, or at least batch updates.

---

## HIGH — Goroutine Leaks & Resource Exhaustion

### H1. LiveTV Recording — Goroutine + File Descriptor Leak
**File:** `pkg/livetv/manager.go`
**Lines:** 260-314
**Severity:** HIGH

`StartRecording` launches `go m.recordStream(ch, rec)` (line 276). `recordStream` opens an `http.Get` (line 283) and an `os.OpenFile` (line 291). It then starts another goroutine (line 301-303) that waits on a `time.After` timer to close `resp.Body`. However:

- If `io.Copy` blocks indefinitely (e.g., source stream never ends), the goroutine and file descriptors leak.
- The `done` timer is created with `time.After(rec.StopTime.Sub(time.Now()))`. If `rec.StopTime` is in the past (e.g., zero-duration recording), the timer fires immediately, closing `resp.Body`. `io.Copy` then returns, but the goroutine that closed the body has already exited — no leak there. However, if the recording is long-running, the goroutine leaks until the source stream ends or the timer fires.
- `rec.Status` is mutated from the goroutine without synchronization. The `Recording` struct is shared with the caller, creating a data race on `rec.Status` and `rec.FilePath`.

**Fix:** Use `context.WithTimeout` for `http.Get`, and synchronize access to `rec.Status` with a mutex or channel.

---

### H2. Transcode Goroutine — No Cancellation / Timeout
**File:** `pkg/stream/stream.go`
**Lines:** 400-423
**Severity:** HIGH

The transcode goroutine runs `exec.Command("ffmpeg", args...)` with `cmd.CombinedOutput()`. There is **no context, no timeout, no cancellation**. If FFmpeg hangs (e.g., on a corrupt file), the goroutine leaks forever, and the `jobs` map entry is never deleted (because the goroutine never returns). This permanently blocks future transcodes for that `itemID` until process restart.

**Fix:** Use `context.WithTimeout` and `cmd.Start()` + `cmd.Wait()` with a timeout goroutine that calls `cmd.Process.Kill()`.

---

### H3. WebSocket Upgrade — Leaked HTTP Response After Upgrade
**File:** `pkg/ws/hub.go`
**Lines:** 39-54
**Severity:** HIGH

`HandleWebSocket` calls `upgrader.Upgrade(c.Response(), c.Request(), nil)` (line 40). If the upgrade succeeds but then `c.QueryParam("token") != ""` (line 47), it returns `echo.NewHTTPError(...)`. At this point, the WebSocket connection has already been hijacked from the HTTP response writer. Returning an HTTP error after a successful upgrade is undefined behavior — the client may receive malformed data, or the connection may be left in a half-open state.

**Fix:** Perform all validation (including token query param check) **before** calling `upgrader.Upgrade`.

---

### H4. OAuth Exchange — No HTTP Client Timeout
**File:** `pkg/oauth/oauth.go`
**Lines:** 137-158, 160-188, 190-230
**Severity:** HIGH

`Exchange` uses `p.Client(ctx, token)` (line 148) and then `client.Get(...)` (line 161, 191, 233). The default `oauth2.Config.Client` uses `http.DefaultClient`, which has **no timeout**. If the Google/GitHub userinfo endpoint hangs, the goroutine blocks forever. This is a goroutine leak on every OAuth callback under network partition.

**Fix:** Provide a custom `http.Client` with timeouts in `oauth2.Config`.

---

### H5. SecureStore / SecureCookie — Potential Header Flushing Bug
**File:** `pkg/api/security.go`
**Lines:** 111-164
**Severity:** MEDIUM

`SecureCookieMiddleware` wraps `c.Response().Writer` with `secureCookieResponseWriter`. In `WriteHeader`, it calls `w.secureCookies()` before delegating to the original writer. However, if the inner handler calls `c.Response().WriteHeader(code)` explicitly, and then later `c.Response().Header().Set(...)` is called, the header modifications happen **after** `WriteHeader` was already called, which is a no-op in `net/http`. The middleware assumes all `Set-Cookie` headers are finalized before `WriteHeader`, but Echo's cookie setting often happens inside the handler body, which may call `Write` (which auto-calls `WriteHeader(http.StatusOK)`) before cookies are set.

This is a subtle ordering bug: cookies set after the first `Write` will not get `SameSite=Strict`, `HttpOnly`, or `Secure` appended.

**Fix:** Use Echo's native `Before` hook or a custom response writer that intercepts `Header()` and rewrites cookies on every access, not just at `WriteHeader` time.

---

## MEDIUM — TOCTOU, Path Traversal, Logic Bugs

### M1. Direct Stream — TOCTOU Path Validation Race
**File:** `pkg/stream/stream.go`
**Lines:** 56-96
**Severity:** MEDIUM

`handleDirectStream` validates `cleanPath` against `cleanRoot` (line 85), then calls `os.Stat(cleanPath)` (line 90), then `c.File(cleanPath)` (line 95). Between `os.Stat` and `c.File`, an attacker with local filesystem access can replace the file with a symlink to `/etc/passwd` (TOCTOU). Echo's `c.File` follows symlinks.

**Fix:** Open the file with `O_NOFOLLOW` and serve via `http.ServeContent` with the opened `*os.File`.

---

### M2. DASH Segment — Path Traversal via `filepath.Clean` Bypass
**File:** `pkg/stream/stream.go`
**Lines:** 247-272
**Severity:** MEDIUM

`handleDASHSegment` checks `strings.Contains(file, "..")` (line 252), then joins with `filepath.Join` and validates with `strings.HasPrefix(cleanSegment, cleanRoot+string(filepath.Separator))`. On Windows, `filepath.Clean` can normalize `..\..\secret` to an absolute path that still escapes `cleanRoot`. The check `strings.Contains(file, "..")` is also insufficient — it doesn't catch encoded traversal or absolute paths.

**Fix:** Reject any `file` parameter containing path separators after `filepath.Base` extraction, and ensure the final path is strictly under `cleanRoot` using `filepath.Rel` + `!strings.HasPrefix(rel, "..")`.

---

### M3. BurnIn — Command Injection via `lang` Parameter
**File:** `pkg/stream/burnin.go`
**Lines:** 162-175
**Severity:** MEDIUM

`extractSubtitleForBurnIn` constructs:
```go
cmd := exec.Command("ffmpeg", "-i", path, "-map", "0:s:m:language:"+lang, outPath, "-y")
```

`lang` is validated in `handleWebVTT` (line 329: `strings.Contains(lang, "..")`), but **not** in `handleBurnIn` (line 275-303). If `BurnIn` is called directly or via a future API, `lang` can contain shell metacharacters (e.g., `en"; rm -rf /; "`). While `exec.Command` passes arguments directly to the kernel (no shell injection), FFmpeg's `-map` option may interpret special characters. More critically, `lang` is used in the temp file path:
```go
outPath := filepath.Join(os.TempDir(), "aetherstream_burnin_"+lang+".srt")
```
If `lang` contains path separators, the file is written outside `/tmp`.

**Fix:** Sanitize `lang` with a strict regex (ISO 639-1/2) before using it in paths or command arguments.

---

### M4. Subtitle Extraction — Fixed Temp Path Collision
**File:** `pkg/probe/probe.go`
**Lines:** 219-226
**Severity:** MEDIUM

`ExtractSubtitleToFile` uses a fixed temp path:
```go
outPath := "/tmp/aetherstream_sub_" + lang + ".srt"
```

This is a **hardcoded absolute path** (not using `os.TempDir()` or `ioutil.TempFile`). Concurrent requests for the same language will collide, causing race conditions where one request overwrites another's file, or one request reads incomplete data. Also, `/tmp` is world-writable — an attacker can pre-create a symlink at this path to redirect writes to arbitrary files (TOCTOU).

**Fix:** Use `os.CreateTemp("", "aetherstream_sub_*.srt")` and clean up after serving.

---

### M5. API Key Prefix Collision — False Positive Validation
**File:** `pkg/apikeys/apikeys.go`
**Lines:** 96-133
**Severity:** MEDIUM

`Validate` extracts `prefix := rawKey[:7]` (line 100). It then queries by prefix and validates the full key with bcrypt. The prefix is only 4 random hex chars after `ak_`. With a large key database, prefix collisions are likely (birthday paradox). An attacker who knows a valid prefix can brute-force the remaining characters offline (bcrypt is slow, but the check is online). More importantly, the `last_used` update (line 131) is fire-and-forget (`_, _ = s.db.Exec(...)`), so failed updates are silently ignored.

**Fix:** Use a longer prefix or store a fast hash (e.g., SHA-256) for lookup, then bcrypt only for verification. Handle `last_used` update errors.

---

### M6. Mobile Dashboard — Unbounded Memory Growth
**File:** `pkg/api/mobile.go`
**Lines:** 48-86
**Severity:** MEDIUM

`handleMobileDashboard` iterates all libraries, then all items in each library, appending to `recent` until it reaches 20 items. If there are 10,000 libraries or 1M items per library, this loads everything into memory just to return 20. There is no pagination or early abort at the DB level.

**Fix:** Add `LIMIT 20` to the DB query, or implement pagination in `ListItemsByLibrary`.

---

### M7. Playback Reporting — Unbounded Result Sets
**File:** `pkg/db/db.go`
**Lines:** 878-924
**Severity:** MEDIUM

`GetPlaybackReporting` loads **all** playback progress and **all** watch history for a user into memory, with no limit. A user with years of history could have tens of thousands of rows, causing OOM.

**Fix:** Add a `limit` parameter and default cap (e.g., 1000).

---

### M8. CSRF Cookie — Missing Secure Flag in Production
**File:** `pkg/api/security.go`
**Lines:** 46-54
**Severity:** MEDIUM

The CSRF cookie is set with `Secure: false` (line 52). The comment says "intentionally lax for HTTP local dev", but there is no runtime check to enforce `Secure: true` in production. If the same code is deployed behind HTTPS without modification, the CSRF cookie is transmitted over HTTP if the connection is downgraded, enabling session hijacking.

**Fix:** Make `Secure` configurable based on `cfg.Server.HTTPS` or `X-Forwarded-Proto`.

---

## LOW — Code Smells, Inefficiencies, Minor Bugs

### L1. `isPathWithinAllowedDirs` — Incorrect Relative Path Check
**File:** `pkg/api/items.go`
**Lines:** 345-364
**Severity:** LOW

```go
rel, err := filepath.Rel(absDir, absPath)
if !strings.HasPrefix(rel, "..") && rel != ".." {
    return true
}
```

`filepath.Rel` returns an error if the paths are on different drives (Windows). The error is ignored (`continue`), so the function falls through to `return false`. Also, `rel` could be `"../foo"`, and `strings.HasPrefix(rel, "..")` catches it, but `".."` alone is also caught by `rel != ".."`. However, if `absPath == absDir`, `filepath.Rel` returns `"."`, which passes the check — this is correct. The logic is mostly sound but fragile.

**Fix:** Use `filepath.Rel` and explicitly check `!strings.HasPrefix(rel, "..") && rel != ".." && rel != "."` if you want to exclude the root itself, or use a proper path traversal library.

---

### L2. `extractValue` — Incorrect Parsing for Quoted Values
**File:** `pkg/encoder/hdr.go`
**Lines:** 127-135
**Severity:** LOW

```go
val := line[idx+len(key):]
val = strings.TrimSpace(val)
return strings.Trim(val, "\"")
```

If the value contains embedded quotes (e.g., `color_primaries="bt2020"`), this works. But if the format is `color_primaries=bt2020` (no quotes), it still works. However, if the line is `key="val"extra`, it returns `val"extra` because `Trim` only removes quotes from both ends. This is a minor parsing bug that could mis-detect HDR metadata.

**Fix:** Use a proper key-value parser or regex.

---

### L3. `BurnIn` — Deferred `os.Remove(subPath)` on Non-Existent File
**File:** `pkg/stream/burnin.go`
**Lines:** 133-159
**Severity:** LOW

`defer os.Remove(subPath)` is called immediately after `extractSubtitleForBurnIn`. If `extractSubtitleForBurnIn` returns an error, `subPath` may be empty or invalid, and `os.Remove("")` returns an error that is silently ignored. Not a security issue, but noisy.

**Fix:** Only defer removal if `subPath != ""`.

---

### L4. `handleBurnIn` — No Validation of `lang` Parameter
**File:** `pkg/stream/stream.go`
**Lines:** 274-303
**Severity:** LOW

`handleBurnIn` binds the request and calls `BurnIn(item.Path, req.Language, ...)`. It does not validate `req.Language` against the same whitelist used in `handleGetSubtitle` (`isValidLanguageCode`). This allows arbitrary strings to reach FFmpeg's `-map` option and the temp file path.

**Fix:** Call `isValidLanguageCode(req.Language)` before processing.

---

### L5. `GetHardwareCapabilities` — Caches Failure Forever
**File:** `pkg/encoder/encoder.go`
**Lines:** 313-361
**Severity:** LOW

`hwCacheOnce.Do` caches the result of `DetectHardwareCapabilities` forever. If the first call fails to detect GPUs (e.g., `nvidia-smi` not installed at startup but installed later), the cache forever reports no hardware acceleration. A process restart is required to redetect.

**Fix:** Add a TTL to the cache or allow explicit cache invalidation.

---

### L6. `RegisterRoutes` — Stream Server Created Per-Route-Group
**File:** `pkg/api/api.go`
**Lines:** 118-127
**Severity:** LOW

`RegisterRoutes` creates `streamSrv := stream.NewServer(s.db, mediaRoot)` locally. This is fine because `RegisterRoutes` is called once at startup. However, `stream.NewServer` also creates a `Transcoder` internally. If `RegisterRoutes` is called multiple times (e.g., in tests), multiple `Transcoder` instances are created, each with its own `jobs` map. This can lead to inconsistent transcode state if tests share the same `mediaRoot`.

**Fix:** Make `streamSrv` a field of `Server` and initialize it once in `NewServer`.

---

### L7. `parseXMLTVTime` — Panic on Short Strings
**File:** `pkg/livetv/manager.go`
**Lines:** 213-225
**Severity:** LOW

```go
if len(s) < 14 {
    return time.Time{}, fmt.Errorf("invalid xmltv time")
}
```

If `s` is exactly 14 characters and `s[14]` is accessed (line 219), it panics with index out of range. The check should be `len(s) <= 14` or `len(s) > 14`.

**Fix:** Change line 219 to `if len(s) > 14 && s[14] == ' '` — actually the code already has `if len(s) > 14`, so this is safe. Wait, re-reading: line 214 checks `< 14`, line 219 checks `> 14 && s[14] == ' '`. If `len(s) == 14`, it falls through to line 223-224, which slices `s[:14]`. This is safe. No bug here — my mistake.

---

### L8. `handleDASHManifest` in `dash/handler.go` — Missing Transcode Trigger
**File:** `pkg/dash/handler.go`
**Lines:** 86-101
**Severity:** LOW

Unlike `stream.go`'s `handleDASHManifest` (line 189-244), `dash/handler.go`'s `handleDASHManifest` does **not** trigger a background transcode when the output dir is missing (line 98-100). It just returns a waiting manifest. This means the DASH endpoint in `dash/handler.go` will never actually start transcoding — clients will be stuck in "waiting" forever unless the HLS endpoint is hit first.

**Fix:** Add `go s.transcoder.Transcode(itemID, []string{"mobile", "tablet"})` or share a single transcode trigger mechanism.

---

### L9. `backup.Compress` — `gzip.Writer` Closed on Stack Buffer
**File:** `pkg/backup/backup.go`
**Lines:** 174-194
**Severity:** LOW

```go
var buf []byte
w := gzip.NewWriter(&bufWriter{&buf})
```

`gzip.Writer.Close()` flushes the footer to the underlying writer. The `bufWriter` appends to a slice. This works, but `Compress` returns `([]byte, error)` and the caller may hold the slice. However, `gzip.NewWriter` requires `io.Writer`, and `bufWriter` implements it. The issue is that `buf` starts as `nil`, and `append` to `nil` is fine. No real bug, but the pattern is unusual.

---

## Summary Table

| ID | File | Line | Severity | Category | Description |
|----|------|------|----------|----------|-------------|
| C1 | `pkg/oauth/oauth.go` | 60-64, 283-311 | CRITICAL | Goroutine leak / panic | OAuth cleanup goroutine leak + double-close panic |
| C2 | `pkg/stream/stream.go` | 376-426 | CRITICAL | Data race | Transcoder jobs map check-then-act race |
| C3 | `pkg/ws/hub.go` | 34-36, 71-81, 105-129 | CRITICAL | Goroutine leak / race | WebSocket hub singleton race, channel leak |
| C4 | `pkg/api/security.go` | 180-237 | HIGH | Goroutine leak / DoS | Brute-force limiter unbounded map + ticker leak |
| C5 | `pkg/api/security.go` | 364-416 | HIGH | Goroutine leak / DoS | IP rate limiter unbounded map + ticker leak |
| C6 | `pkg/api/security.go` | 308-346 | HIGH | Hot path inefficiency | SessionTimeout 2 DB round-trips per request |
| H1 | `pkg/livetv/manager.go` | 260-314 | HIGH | Goroutine leak / race | Recording goroutine leak + unsynchronized `rec.Status` |
| H2 | `pkg/stream/stream.go` | 400-423 | HIGH | Goroutine leak | Transcode goroutine no timeout/cancellation |
| H3 | `pkg/ws/hub.go` | 39-54 | HIGH | Connection leak | WS upgrade before validation |
| H4 | `pkg/oauth/oauth.go` | 137-158 | HIGH | Goroutine leak | OAuth exchange no HTTP timeout |
| H5 | `pkg/api/security.go` | 111-164 | MEDIUM | Logic bug | SecureCookieMiddleware header flush ordering |
| M1 | `pkg/stream/stream.go` | 56-96 | MEDIUM | TOCTOU | Direct stream path validation race |
| M2 | `pkg/stream/stream.go` | 247-272 | MEDIUM | Path traversal | DASH segment path traversal bypass |
| M3 | `pkg/stream/burnin.go` | 162-175 | MEDIUM | Command injection | BurnIn `lang` param not sanitized |
| M4 | `pkg/probe/probe.go` | 219-226 | MEDIUM | Race condition | Fixed temp path collision + symlink attack |
| M5 | `pkg/apikeys/apikeys.go` | 96-133 | MEDIUM | Logic bug | Short prefix collision, silent last_used failure |
| M6 | `pkg/api/mobile.go` | 48-86 | MEDIUM | Hot path inefficiency | Unbounded memory growth in dashboard |
| M7 | `pkg/db/db.go` | 878-924 | MEDIUM | Hot path inefficiency | Unbounded playback reporting result set |
| M8 | `pkg/api/security.go` | 46-54 | MEDIUM | Security config | CSRF cookie Secure=false in production |
| L1 | `pkg/api/items.go` | 345-364 | LOW | Logic bug | Fragile path traversal check |
| L2 | `pkg/encoder/hdr.go` | 127-135 | LOW | Parsing bug | `extractValue` quote trimming bug |
| L3 | `pkg/stream/burnin.go` | 133-159 | LOW | Code smell | Deferred remove on empty path |
| L4 | `pkg/stream/stream.go` | 274-303 | LOW | Validation missing | BurnIn handler missing language validation |
| L5 | `pkg/encoder/encoder.go` | 313-361 | LOW | Caching bug | Hardware detection cached forever |
| L6 | `pkg/api/api.go` | 118-127 | LOW | Design | Stream server created inside RegisterRoutes |
| L8 | `pkg/dash/handler.go` | 86-101 | LOW | Functional bug | DASH handler never triggers transcode |

---

## Recommendations

1. **Audit all goroutine lifecycles.** Every `go func()` must have a matching stop/close/cancel mechanism. Use `context.Context` pervasively.
2. **Replace all unbounded maps with LRU caches.** The brute-force and rate limiter maps are trivial DoS vectors.
3. **Add timeouts to all external I/O.** FFmpeg, HTTP clients, OAuth exchanges, and WebSocket reads/writes must have deadlines.
4. **Use `filepath.Rel` + strict checks for all path traversal validation.** Never rely on `strings.HasPrefix` alone.
5. **Cache session activity in memory.** The DB round-trip per request is a critical scalability bottleneck.
6. **Run `go test -race` regularly.** The transcoder and WebSocket packages have obvious races that `-race` would catch.
7. **Add `context.WithTimeout` to `exec.Command` calls.** Use `cmd.Start()` + `cmd.Wait()` with a timeout goroutine.
8. **Validate all user inputs before using them in file paths or command arguments.** Language codes, item IDs, and file names need strict whitelists.

---

*End of report.*
