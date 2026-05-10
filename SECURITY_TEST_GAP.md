# AetherStream Security Test Gap Analysis

**Date:** 2026-05-10
**Scope:** 129 Go files, 59 test files, 51 packages
**Methodology:** Static code review, OWASP ASVS mapping, gosec report analysis, test coverage gap identification

---

## Executive Summary

AetherStream has 50 packages with tests but significant security test gaps remain. Only 2 fuzz tests exist (`naming`, `probe`). No dedicated security penetration tests, no auth bypass tests, no rate-limiting stress tests, no input validation fuzzing for web handlers. This document maps every identified vulnerability to a concrete test recommendation.

---

## 1. SQL Injection

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 1.1 | `pkg/db/db.go` | 576-628 | **Medium** | `SearchItemsFTS` uses `MATCH ?` with fallback to `LIKE "%"+query+"%"`. The `query` parameter is passed directly to SQLite FTS5 `MATCH` operator. While parameterized, FTS5 query syntax allows injection of boolean operators (`AND`, `OR`, `NOT`, `NEAR`). A malicious query like `foo OR 1=1` could alter search semantics or cause DoS via complex queries. | **Add `TestSearchFTSInjection`** in `pkg/search/search_test.go`: fuzz FTS5 queries with `OR`, `AND`, `NEAR`, `*`, `"`, `^`, `$`, `-`, `+` and assert no panic / no unexpected rows returned. Validate query length limit (max 200 chars). |
| 1.2 | `pkg/db/db.go` | 587,598,609,620 | **Medium** | Dynamic query building for `mediaType` filter uses `?` placeholders — safe. However, `query` string is concatenated into `LIKE "%"+query+"%"` in fallback path (lines 598-628). While the `?` placeholder prevents direct SQL injection, the `query` value is user-controlled and could contain `%` or `_` wildcards causing information leakage or performance degradation. | **Add `TestSearchLikeWildcardLeakage`** in `pkg/search/search_test.go`: inject queries with `%`, `_`, `\%`, `\_` and assert results are bounded and no cross-library leakage. |
| 1.3 | `pkg/smartplaylists/smartplaylists.go` | 127 | **High** | `BuildQuery` constructs dynamic SQL from JSON `rules` field. The `rules` are deserialized and translated to SQL. If any field value is concatenated without parameterization, this is a direct SQLi vector. | **Add `TestSmartPlaylistSQLInjection`** in `pkg/smartplaylists/smartplaylists_test.go`: fuzz `rules` JSON with `" OR 1=1 --`, `'; DROP TABLE items; --`, field names containing backticks/quotes. Assert parameterized queries only. |
| 1.4 | `pkg/autocollections/autocollections.go` | 165 | **Medium** | `SELECT id FROM items WHERE media_type = ?` — safe. But `group_by` and `value` parameters used in `DELETE` and `SELECT` should be validated against allowlist. | **Add `TestAutoCollectionsGroupByValidation`** in `pkg/autocollections/autocollections_test.go`: test `group_by` with SQL keywords, special chars; assert rejection or safe escaping. |

---

## 2. Cross-Site Scripting (XSS)

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 2.1 | `pkg/api/api.go` | 134-138 | **Medium** | `handleSystemInfo` returns JSON with hardcoded version. No issue here, but any endpoint returning user-controlled strings (e.g., `item.Name`, `user.Username`) as JSON does not escape HTML. If the frontend renders these values in HTML without escaping, stored XSS is possible. | **Add `TestJSONResponseXSSPayload`** in `pkg/api/api_test.go`: create item/user with `<script>alert(1)</script>` in name, fetch via API, assert response body contains literal string (not escaped — frontend responsibility), and document frontend must use `textContent` not `innerHTML`. |
| 2.2 | `pkg/dlna/server.go` | 261,451,514,639,654,668,688 | **High** | DLNA SOAP responses and device description XML are built via string concatenation (`fmt.Sprintf`) with user-controlled values (`friendlyName`, `item.Name`, `item.Path`). No XML escaping is applied. An item name like `</name><script>alert(1)</script>` would inject into XML responses. | **Add `TestDLNAXMLEscaping`** in `pkg/dlna/server_test.go`: create item with XML special chars in name (`<`, `>`, `&`, `"`, `'`). Fetch `description.xml` and `Browse` response, assert all values are XML-escaped. |
| 2.3 | `pkg/stream/stream.go` | 95,186 | **Low** | HLS/DASH waiting playlists/manifests contain hardcoded text. No XSS risk unless user-controlled values are injected into playlist strings. | **Add `TestPlaylistNoUserContent`** in `pkg/stream/stream_test.go`: assert waiting playlist/manifest contains no user-controlled strings. |
| 2.4 | `pkg/oauth/oauth.go` | 264,268 | **Medium** | OAuth error messages (`provider not enabled`, `err.Error()`) are returned as JSON. If `err.Error()` contains user input, it could leak paths or internal state. | **Add `TestOAuthErrorSanitization`** in `pkg/oauth/oauth_test.go`: trigger errors with malicious provider names, assert error messages do not contain stack traces or file paths. |

---

## 3. Cross-Site Request Forgery (CSRF)

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 3.1 | `pkg/api/security.go` | 35-72 | **Medium** | CSRF middleware validates cookie against header/form token. However, `c.FormValue("csrf_token")` is checked for state-changing methods. If an attacker can set the `csrf_token` cookie via a subdomain or XSS, they can bypass CSRF. The cookie is `HttpOnly` and `Secure` but `SameSite=Strict` helps. Missing test for cookie fixation scenario. | **Add `TestCSRFCookieFixationResistance`** in `pkg/api/security_test.go`: simulate attacker setting csrf cookie from different origin, assert POST is rejected. Test that cookie is regenerated on each GET if missing. |
| 3.2 | `pkg/api/security.go` | 243 | **Medium** | `username := c.FormValue("username")` in brute-force middleware only reads form data, not JSON body. Login endpoint accepts JSON (`c.Bind`), so brute-force by username is ineffective for JSON logins. | **Add `TestBruteForceJSONBody`** in `pkg/api/security_test.go`: send JSON login requests, assert username-based rate limiting still applies (read from body or fallback to IP-only). |
| 3.3 | `pkg/api/api.go` | 72-73 | **Low** | `/auth/login` and `/auth/callback` have `RateLimitByIP(10)` but no CSRF token requirement. Login forms are not protected by CSRF (not needed for login typically, but password change should be). | **Add `TestLoginCSRFNotRequired`** in `pkg/api/api_test.go`: document that login does not require CSRF (by design) but password change endpoint (if added) MUST require it. |

---

## 4. Path Traversal

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 4.1 | `pkg/api/api.go` | 333-358 | **High** | `handleGetSubtitle` validates `lang` against `..`, `/`, `\` but uses `strings.Contains` which is insufficient. `lang` like `en..srt` or `en%2f..` could bypass. `probe.ExtractSubtitleToFile` writes to `/tmp/aetherstream_sub_` + `lang` + `.srt` — if `lang` contains path separators or null bytes, file is written outside tmp. The `subPath` validation checks `os.TempDir()` prefix but `os.TempDir()` can be manipulated via `TMPDIR` env. | **Add `TestSubtitlePathTraversal`** in `pkg/api/api_test.go`: fuzz `lang` with `../etc/passwd`, `..\\windows\\system.ini`, `en\x00`, `en%2f..`, `en..srt`. Assert all return 400/403 and no file written outside temp dir. Use `os.Setenv("TMPDIR", controlledDir)` during test. |
| 4.2 | `pkg/api/api.go` | 533-566 | **High** | `handleGetThumbnail` constructs `thumbPath` from `itemID` and `thumbType`. `thumbSvc.Path(itemID, t)` likely uses `filepath.Join(s.outputDir, fmt.Sprintf("%s_%s.jpg", itemID, t))`. If `itemID` is user-controlled (path param), path traversal is possible unless validated. `thumbType` is enum-checked but `itemID` is not. | **Add `TestThumbnailPathTraversal`** in `pkg/api/api_test.go`: request thumbnail with `itemID=../../../etc/passwd` and `itemID=..%2f..%2fetc%2fpasswd`. Assert 400/403 and no file access outside thumbnail dir. |
| 4.3 | `pkg/stream/stream.go` | 53-80 | **Medium** | `handleDirectStream` uses `filepath.Clean(path)` + `strings.HasPrefix(cleanPath, cleanRoot+sep)`. This is vulnerable to prefix bypass if `cleanRoot` is `/media` and attacker uses `/media2/file` (mitigated by `+sep`). However, symlinks are not followed — if `item.Path` in DB is a symlink to `/etc/passwd`, `os.Stat` follows it and `c.File` serves it. | **Add `TestDirectStreamSymlinkEscape`** in `pkg/stream/stream_test.go`: create symlink in mediaRoot pointing outside, insert item with symlink path, request stream, assert 403. Test `filepath.Rel` validation as alternative. |
| 4.4 | `pkg/stream/stream.go` | 110-130 | **Medium** | `handleHLSVariant` reads `playlist.m3u8` from `filepath.Join(s.mediaRoot, "transcodes", itemID, profile, "playlist.m3u8")`. `profile` param is not validated against allowlist. A profile like `../../..` could traverse if `filepath.Clean` + `HasPrefix` is bypassed. | **Add `TestHLSVariantPathTraversal`** in `pkg/stream/stream_test.go`: request variant with `profile=../../../etc`. Assert 403. Fuzz profile param with traversal sequences. |
| 4.5 | `pkg/stream/stream.go` | 132-148 | **Medium** | `handleHLSSegment` validates `segment` param but only checks `..`, `/`, `\` via `strings.Contains`. `segment` like `segment..%2fts` or with null byte could bypass. | **Add `TestHLSSegmentPathTraversal`** in `pkg/stream/stream_test.go`: fuzz segment names with traversal sequences, null bytes, URL encoding. Assert 403. |
| 4.6 | `pkg/stream/stream.go` | 230-256 | **Medium** | `handleDASHSegment` has same validation as HLS segment but `file` param is checked. `profile` is not checked for traversal. | **Add `TestDASHSegmentPathTraversal`** in `pkg/stream/stream_test.go`: fuzz `profile` and `file` params with traversal sequences. Assert 403. |
| 4.7 | `pkg/stream/burnin.go` | 131-159 | **Medium** | `BurnIn` uses `extractSubtitleForBurnIn` which writes to `os.TempDir()`. `lang` is not sanitized. `outPath` is deterministic based on hash but inside `mediaRoot/transcodes/burnin` — safe. | **Add `TestBurnInSubtitleExtractionTraversal`** in `pkg/stream/burnin_test.go`: fuzz `lang` with path traversal, assert no file written outside temp. |
| 4.8 | `pkg/backup/backup.go` | 68,143 | **Medium** | `os.Open(srcPath)` and `os.Create(tmpPath)` have `#nosec G304` comments with note "caller must validate". No validation is visible in the reviewed code. | **Add `TestBackupPathValidation`** in `pkg/backup/backup_test.go`: attempt backup/restore with paths outside allowed dirs, assert rejection. |
| 4.9 | `pkg/nfo/nfo.go` | 97,130 | **Low** | `os.ReadFile(path)` with `#nosec G304` — caller must validate. If `path` comes from library scan, it should be safe, but manual NFO loading could be abused. | **Add `TestNFOPathValidation`** in `pkg/nfo/nfo_test.go`: attempt to load NFO outside library dir, assert rejection. |
| 4.10 | `pkg/livetv/manager.go` | 158,252,271,291,348 | **Medium** | Multiple file operations with `#nosec G304`. `rec.FilePath` is constructed from `uuid.New().String()+".ts"` — safe. But `path` in `loadEPG` comes from config. | **Add `TestLiveTVPathValidation`** in `pkg/livetv/manager_test.go`: set config paths outside expected dirs, assert EPG load rejected or sandboxed. |
| 4.11 | `pkg/integrations/sonarr.go` | 184-194 | **Medium** | `cleanPath` is constructed from `p` (Sonarr payload). If `p` is absolute, it's used directly; if relative, joined with `MediaRoot`. No validation that final path is within `MediaRoot`. | **Add `TestSonarrPathValidation`** in `pkg/integrations/sonarr_test.go`: send Sonarr webhook with `path="../../../etc/passwd"`, assert path is sanitized to within `MediaRoot`. |

---

## 5. Race Conditions

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 5.1 | `pkg/api/security.go` | 229,261-274 | **High** | `globalBruteForce` is a package-level variable. `record()` and `reset()` are called from multiple goroutines (HTTP handlers). `record()` uses `b.mu.Lock()` — safe. But `cleanup()` goroutine iterates map while `record()` may resize it. In Go, map iteration + write without external synchronization is a data race. The `cleanup()` holds `b.mu.Lock()` during iteration — safe. However, `allowed()` uses `b.mu.RLock()` and reads `a.lockedUntil` — if `record()` writes `lockedUntil` concurrently, this is a race on the `loginAttempt` struct fields. | **Add `TestBruteForceRaceDetection`** in `pkg/api/security_test.go`: run `go test -race` with concurrent login attempts from multiple goroutines. The existing test does not run with `-race` flag by default. |
| 5.2 | `pkg/oauth/oauth.go` | 46,98-114 | **High** | `states` map is accessed concurrently by `GenerateState` and `ValidateState`. `GenerateState` uses `s.stateMu.Lock()` — safe. But `ValidateState` also uses `s.stateMu.Lock()`. However, `fetchGitHubUser` / `fetchGoogleUser` are called from `Exchange` which is called from HTTP handler goroutines. `states` map cleanup is missing — memory leak, not race. But `Service` struct has no mutex on `providers` map (read-only after init). | **Add `TestOAuthStateRaceDetection`** in `pkg/oauth/oauth_test.go`: run `go test -race` with concurrent `GenerateState` and `ValidateState` calls. |
| 5.3 | `pkg/ws/hub.go` | 14-18,37-93 | **Medium** | `upgrader` is a package-level variable with `CheckOrigin: func(r *http.Request) bool { return true }`. This allows any origin to connect via WebSocket. Combined with no auth on `/ws`, any website can open a WebSocket to AetherStream and receive broadcast messages. | **Add `TestWebSocketCORSOriginValidation`** in `pkg/ws/hub_test.go`: attempt WebSocket upgrade from `evil.com`, assert rejection. Test that `CheckOrigin` validates against allowlist. |
| 5.4 | `pkg/ws/hub.go` | 61-70 | **Medium** | `globalHub` is package-level. `HandleWebSocket` writes to `globalHub.clients` while `Broadcast` / `BroadcastToUser` read from it. Both use `globalHub.mu` — safe. But `client.writePump()` runs in a separate goroutine and writes to `c.conn` while `HandleWebSocket` may close the connection. `c.conn.Close()` is not synchronized with `writePump`. | **Add `TestWebSocketConcurrentClose`** in `pkg/ws/hub_test.go`: simulate rapid connect/disconnect from multiple goroutines, run with `-race`. |
| 5.5 | `pkg/library/manager.go` | 45-48 | **Medium** | `scanWorker` and `watchWorker` run concurrently. `processFile` calls `m.db.CreateItem` and `m.cache.Set`. `cache` is an LRU cache — if not thread-safe, race. | **Add `TestLibraryScanRaceDetection`** in `pkg/library/manager_test.go`: run concurrent scans, use `go test -race`. |
| 5.6 | `pkg/stream/stream.go` | 90-93,184-185 | **Medium** | `go s.transcoder.Transcode(...)` is called without checking if a transcode is already in progress. Multiple requests for the same item could spawn multiple FFmpeg processes. `Transcoder` has a `jobs` map with mutex — should be safe, but the `go` call happens before checking the map. | **Add `TestTranscodeDuplicateJobPrevention`** in `pkg/stream/stream_test.go`: send 10 concurrent requests for same item HLS master, assert only one transcode job started. |

---

## 6. Authentication Bypass

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 6.1 | `pkg/stream/stream.go` | 37-50 | **Critical** | `RegisterRoutes` accepts `authMiddleware` parameter but stream routes in `pkg/api/api.go:125-127` pass `s.auth.Middleware()`. However, `stream.RegisterAdaptiveRoutes` at line 127 also adds `/videos/:id/adaptive.m3u8` which is protected. But `streamSrv.RegisterRoutes(e, s.auth.Middleware())` at line 126 registers `/videos/:id/stream` etc. The `authMiddleware` is applied via `g.Use(authMiddleware)`. If `authMiddleware` is `nil`, routes are unprotected. In `stream_test.go`, `RegisterRoutes(e, nil)` is used — tests confirm unprotected access is possible. | **Add `TestStreamRoutesRequireAuth`** in `pkg/stream/stream_test.go`: register routes with auth middleware, assert 401 without token. Test that `nil` middleware is rejected at server startup. |
| 6.2 | `pkg/api/api.go` | 122 | **Critical** | `e.GET("/ws", s.handleWebSocket, s.auth.Middleware())` — WebSocket has auth middleware. But `handleWebSocket` in `pkg/ws/hub.go` ignores auth context and reads `user_id` from query param. An attacker can pass any `user_id` and `device_id` via query string. The JWT middleware validates the token but `hub.go` does not extract user from context — it uses query params. | **Add `TestWebSocketAuthBypass`** in `pkg/ws/hub_test.go`: connect with valid token but query param `user_id=admin`, assert connection uses token claims, not query param. Reject connection if query param mismatches token. |
| 6.3 | `pkg/api/api.go` | 72 | **High** | `/auth/login` has `RateLimitByIP(10)` but no account lockout after repeated failures. Brute-force middleware records attempts but does not block the account permanently. | **Add `TestAccountLockoutAfterFailures`** in `pkg/api/api_test.go`: attempt 20 failed logins, assert account locked for N minutes. Test unlock after timeout or admin action. |
| 6.4 | `pkg/auth/auth.go` | 61-76 | **Medium** | `ValidateToken` checks `token.Method.(*jwt.SigningMethodHMAC)` but does not explicitly whitelist `HS256`. An `alg: none` attack is mitigated by jwt/v5 library, but explicit check is defense in depth. | **Add `TestJWTAlgNoneAttack`** in `pkg/auth/auth_test.go`: craft token with `alg: "none"`, `alg: "HS384"`, `alg: "HS512"`, assert all rejected. |
| 6.5 | `pkg/auth/auth.go` | 104-117 | **Medium** | `RequireRole` allows `admin` role to bypass any role check. This is intentional but should be documented. No test verifies non-admin user cannot access admin endpoints. | **Add `TestRequireRoleAdminBypassDocumented`** in `pkg/auth/rbac_test.go`: test that admin can access user-only endpoints, and user cannot access admin endpoints. Document the bypass behavior. |
| 6.6 | `pkg/apikeys/apikeys.go` | 96-133 | **Medium** | `Validate` checks prefix via `rawKey[:7]` and then full key via bcrypt. But `rawKey` length is not checked before slicing — a key shorter than 7 chars causes panic. | **Add `TestAPIKeyShortInput`** in `pkg/apikeys/apikeys_test.go`: pass key `"ak_"` (3 chars), assert graceful error, no panic. |
| 6.7 | `pkg/device/device.go` | 201-229 | **Medium** | Device trust middleware checks device but does not bind device to user session. A stolen device token could be reused across users. | **Add `TestDeviceTokenUserBinding`** in `pkg/device/device_test.go`: create device for user A, attempt to use it for user B, assert rejection. |

---

## 7. Session Fixation

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 7.1 | `pkg/api/security.go` | 291-324 | **High** | `SessionTimeout` middleware stores `sessions` in a package-level map keyed by `userID`. There is no session ID cookie — the middleware tracks by user ID from JWT claims. This means all requests from the same user share the same timeout state. If a user logs in from two devices, activity on one resets the timeout on the other. More critically, there is no session fixation protection: an attacker who knows the JWT can extend the session indefinitely by making requests before timeout. | **Add `TestSessionTimeoutPerDevice`** in `pkg/api/security_test.go`: simulate two devices for same user, assert timeouts are independent. Add session ID generation and rotation on login. |
| 7.2 | `pkg/api/security.go` | 40-55 | **Medium** | CSRF cookie is set on GET requests with `MaxAge: 86400`. If a user visits a malicious site before visiting AetherStream, the attacker cannot read the cookie (HttpOnly) but can fixate the CSRF token by making a cross-site GET request to obtain the cookie, then use it in a CSRF attack. However, `SameSite=Strict` prevents cross-site cookie sending. | **Add `TestCSRFSameSiteStrict`** in `pkg/api/security_test.go`: simulate cross-site GET, assert cookie is not sent (or request is rejected). |
| 7.3 | `pkg/oauth/oauth.go` | 99-108 | **Medium** | `GenerateState` creates state and stores it with 10-minute expiry. But there is no session binding — any client can use any valid state. While state is single-use (deleted on validation), there is no binding to the user's session cookie. | **Add `TestOAuthStateSessionBinding`** in `pkg/oauth/oauth_test.go`: generate state in session A, attempt to validate in session B, assert rejection. |

---

## 8. Denial of Service (DoS)

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 8.1 | `pkg/api/api.go` | 487-512 | **Medium** | `handleSearch` parses `limit` from query param with `maxLimit = 100`. But `q` (query string) has no length limit. A 10MB query string could cause memory exhaustion in SQLite FTS5 or the fallback LIKE query. | **Add `TestSearchQueryLengthLimit`** in `pkg/api/api_test.go`: send query of 1MB, assert 400 Bad Request. Test max length enforcement (e.g., 500 chars). |
| 8.2 | `pkg/api/api.go` | 238-262 | **Medium** | `handleCreateLibrary` accepts `Path` from JSON. No validation that path exists, is readable, or is within allowed filesystem boundaries. An attacker could create a library pointing to `/proc` or `/sys`, causing scanner to read infinite pseudo-files. | **Add `TestCreateLibraryPathValidation`** in `pkg/api/api_test.go`: attempt to create library at `/proc`, `/sys`, `/dev/zero`, assert rejection or sandboxing. |
| 8.3 | `pkg/stream/stream.go` | 90-93 | **Medium** | `go s.transcoder.Transcode(...)` spawns FFmpeg without job queue size limit. If 1000 clients request HLS for different items, 1000 FFmpeg processes start concurrently, exhausting CPU/RAM. | **Add `TestTranscodeJobQueueLimit`** in `pkg/stream/stream_test.go`: request 1000 concurrent transcodes, assert queue rejects or limits active jobs to `cfg.FFmpeg.MaxJobs`. |
| 8.4 | `pkg/stream/stream.go` | 53-80 | **Low** | `handleDirectStream` serves file via `c.File(cleanPath)` which supports HTTP Range requests. A malicious client could request many small ranges (e.g., 1-byte ranges) to cause excessive disk I/O. | **Add `TestDirectStreamRangeFlood`** in `pkg/stream/stream_test.go`: send 1000 Range requests for 1 byte each, assert rate limiting or connection throttling. |
| 8.5 | `pkg/ws/hub.go` | 54-59 | **Medium** | `client.send` channel has buffer size 256. `Broadcast` sends to all clients without checking channel capacity. If a client is slow, `Broadcast` blocks on `client.send <- msg` because `select` with `default` only skips if channel is full — but `Broadcast` uses `select { case client.send <- msg: default: }` which is non-blocking. However, if 10000 clients connect, `Broadcast` iterates over all clients under `RLock`, causing latency spikes. | **Add `TestWebSocketBroadcastScalability`** in `pkg/ws/hub_test.go`: connect 10000 mock clients, broadcast message, assert completion within N milliseconds. |
| 8.6 | `pkg/dlna/server.go` | 59-86 | **Medium** | DLNA HTTP server has no request size limit or timeout on body read. A malicious SSDP M-SEARCH or HTTP POST could cause memory exhaustion. | **Add `TestDLNARequestSizeLimit`** in `pkg/dlna/server_test.go`: send 100MB POST to `/ContentDirectory/control`, assert 413 or connection closed. |
| 8.7 | `pkg/backup/backup.go` | 147 | **Medium** | `io.Copy(df, rc)` during zip extraction has no size limit. A malicious backup zip with a compressed bomb (e.g., 1KB -> 10GB) could exhaust disk space. | **Add `TestBackupZipBomb`** in `pkg/backup/backup_test.go`: create zip bomb, attempt restore, assert size limit enforced (e.g., max 1GB). |
| 8.8 | `pkg/api/security.go` | 396-408 | **Low** | `RateLimitByIP` uses in-memory map. An attacker can exhaust memory by rotating source IPs (e.g., IPv6 addresses). No IP whitelist or bucket size limit. | **Add `TestRateLimitIPExhaustion`** in `pkg/api/security_test.go`: simulate 1 million distinct IPs, assert memory stays bounded (cleanup works). |

---

## 9. CORS Misconfiguration

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 9.1 | `pkg/api/security.go` | 476-485 | **Critical** | `CORSMiddleware` allows origins `http://localhost:5173`, `http://localhost:3000`, `http://localhost:8080`, `https://localhost:5173`, `https://localhost:8080`. `AllowCredentials: true` is set. Any attacker running a local server on these ports can make authenticated cross-origin requests. In production, localhost should never be in the allowlist. | **Add `TestCORSProductionBlockLocalhost`** in `pkg/api/security_test.go`: assert that in production mode (env var), localhost origins are rejected. Test dynamic origin validation from config. |
| 9.2 | `pkg/api/security.go` | 476-485 | **High** | `AllowHeaders` includes `Authorization`. `AllowMethods` includes `PUT`, `DELETE`, `PATCH`. With credentials enabled, a malicious site on an allowed origin can perform state-changing operations. | **Add `TestCORSStateChangingMethods`** in `pkg/api/security_test.go`: send preflight `OPTIONS` for `DELETE` from allowed origin, assert response. But also test that actual `DELETE` request with credentials is blocked if origin not in list. |
| 9.3 | `pkg/stream/stream.go` | 226,254 | **Medium** | DASH manifest and segment handlers set `Access-Control-Allow-Origin: *` manually. This overrides the global CORS middleware and allows any origin to stream content without credentials check. | **Add `TestDASHCORSWildcard`** in `pkg/stream/stream_test.go`: request DASH manifest/segment with `Origin: evil.com`, assert no `Access-Control-Allow-Origin: *` in response. |

---

## 10. Insecure Deserialization

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 10.1 | `pkg/api/api.go` | 151,244,282,368,394,434,467,581,614 | **Medium** | `c.Bind(&req)` uses Echo's default binder which binds JSON/XML/form data. Echo's binder does not enforce strict field validation — unknown fields are ignored. If a struct has an interface{} field or uses `json.RawMessage`, arbitrary JSON could be deserialized. No `json.Unmarshal` with `DisallowUnknownFields` is used. | **Add `TestBindUnknownFieldsRejection`** in `pkg/api/api_test.go`: send JSON with extra fields to login/create-user endpoints, assert 400 if strict validation enabled. |
| 10.2 | `pkg/oauth/oauth.go` | 166-183,196-210 | **Medium** | `json.NewDecoder(resp.Body).Decode(&data)` decodes OAuth provider responses into structs. If provider returns malicious JSON (e.g., very large numbers, nested objects), it could cause memory exhaustion. No `Decoder.UseNumber()` or size limit. | **Add `TestOAuthJSONDecodeLimit`** in `pkg/oauth/oauth_test.go`: mock provider returning 100MB JSON, assert decoder rejects or limits. |
| 10.3 | `pkg/cluster/server.go` | 57-61 | **Medium** | `c.Bind(&req)` for cluster join requests. If cluster endpoint is exposed, an attacker could send malformed JSON to crash the decoder. | **Add `TestClusterBindMalformedJSON`** in `pkg/cluster/cluster_test.go`: send invalid JSON to `/cluster/join`, assert 400, no panic. |

---

## 11. Information Leakage

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 11.1 | `pkg/api/api.go` | 155-176 | **Medium** | `handleLogin` returns `echo.NewHTTPError(401, "invalid credentials")` for both missing user and wrong password. This is correct (no user enumeration). But `RecordFailedLogin` is called with `ip` and `username`. If an attacker probes usernames, the timing of bcrypt comparison could leak existence (constant-time comparison is used for hash, but DB lookup time may vary). | **Add `TestLoginTimingSideChannel`** in `pkg/api/api_test.go`: measure response time for existing vs non-existing username, assert difference is within noise margin (use `bcrypt.CompareHashAndPassword` which is designed to be constant-time). |
| 11.2 | `pkg/api/api.go` | 141-144 | **Low** | `handleSystemHardware` returns `encoder.DetectHardwareCapabilities()` which may expose GPU model, driver version, and hardware acceleration support. This aids attackers in crafting exploits. | **Add `TestSystemHardwareInfoLeakage`** in `pkg/api/api_test.go`: assert hardware endpoint requires admin role or is disabled in production. |
| 11.3 | `pkg/stream/stream.go` | 150-169 | **Low** | `handleProbe` returns ffprobe output including file path, codec details, bitrate. File path may reveal directory structure. | **Add `TestProbePathRedaction`** in `pkg/stream/stream_test.go`: assert `path` field is redacted or removed from probe response. |
| 11.4 | `pkg/api/api.go` | 195,204,212,216,225,233,etc | **Low** | Many handlers return `echo.NewHTTPError(http.StatusInternalServerError, "an internal error occurred")` — good, generic message. But some return `err.Error()` (e.g., `stream.go:166`, `oauth.go:268`). | **Add `TestErrorMessageSanitization`** across all packages: grep for `err.Error()` in HTTP responses, create table-driven test ensuring no file paths, SQL errors, or stack traces leak. |
| 11.5 | `pkg/db/db.go` | 289-305 | **Low** | `GetUserByID` returns `createdAt` timestamp. Combined with login timing, this could aid reconnaissance. | **Add `TestUserResponseFieldAllowlist`** in `pkg/db/db_test.go`: assert only whitelisted fields are returned in API responses (no internal IDs, timestamps unless needed). |
| 11.6 | `pkg/metrics/pprof.go` | 1-50 | **High** | `pprof` endpoints are registered. If exposed without auth, they leak goroutine stacks, heap profiles, and memory addresses. | **Add `TestPprofAuthRequired`** in `pkg/metrics/metrics_test.go`: request `/debug/pprof/` without auth, assert 401. Test with auth, assert 200. |

---

## 12. Cryptographic Issues

| # | File | Line | Severity | Finding | Test Gap / Recommendation |
|---|------|------|----------|---------|---------------------------|
| 12.1 | `pkg/config/config.go` | 85-91 | **Critical** | `generateRandomSecret(32)` is called if `AETHERSTREAM_AUTH_SECRET` is not set. This generates a random secret at startup but it is NOT persisted. Every restart invalidates all tokens. In a container environment, this means tokens break on every redeploy. More critically, if the fallback is removed without persistence, the app may fail to start in stateless environments. | **Add `TestSecretPersistence`** in `pkg/config/config_test.go`: start app without env secret, assert secret is generated AND persisted to file with 0600 permissions. Assert subsequent loads read from file. |
| 12.2 | `pkg/securestore/securestore.go` | 21-28 | **Medium** | `NewStore` truncates key to 32 bytes without key stretching. If key is a password, brute-force is feasible. | **Add `TestSecureStoreKeyDerivation`** in `pkg/securestore/securestore_test.go`: test that weak password (< 32 bytes) is stretched via PBKDF2/Argon2 before AES use. Test key length in bytes, not runes. |
| 12.3 | `pkg/apikeys/apikeys.go` | 176-179 | **Medium** | `hashKey` uses `bcrypt.GenerateFromPassword` with `bcrypt.DefaultCost` (10). This is slow (~100ms) which is good for security but may cause DoS on API key validation if many requests arrive. | **Add `TestAPIKeyHashPerformance`** in `pkg/apikeys/apikeys_test.go`: benchmark `Validate` with 1000 concurrent requests, assert throughput > N req/sec or implement caching. |

---

## 13. Missing Security Tests Summary

| Category | Existing Tests | Missing Tests | Priority |
|----------|--------------|---------------|----------|
| Fuzz Tests | 2 (`naming`, `probe`) | 15+ (API inputs, file paths, SQL queries, JSON binders) | P0 |
| Auth Bypass | Basic login test | JWT alg none, token tampering, role bypass, stream auth | P0 |
| Rate Limiting | Basic brute-force | IP rotation, queue exhaustion, OAuth rate limiting | P0 |
| Input Validation | Some limit checks | Length limits, charset allowlists, path traversal fuzz | P0 |
| Path Traversal | None dedicated | Subtitle, thumbnail, stream, backup, DLNA | P0 |
| CORS | None | Production localhost block, wildcard rejection, preflight | P1 |
| Session Security | None | Timeout per device, fixation protection, rotation | P1 |
| DoS | None | Search query size, transcode queue, WS broadcast, zip bomb | P1 |
| Information Leakage | None | Timing side-channels, error sanitization, pprof auth | P1 |
| Cryptography | Basic token gen/val | Secret persistence, key derivation, API key perf | P1 |
| Race Conditions | Some (`transcode`) | Brute-force map, OAuth state, WS close, library scan | P1 |
| XSS / XML Injection | None | DLNA XML escaping, JSON payload reflection | P2 |
| SQL Injection | None | FTS5 injection, smart playlist rules, LIKE wildcard | P2 |

---

## 14. Recommended Test Implementation Order

### Phase 1 — Critical (P0)
1. `TestStreamRoutesRequireAuth` — ensure all `/videos/*` require valid JWT.
2. `TestWebSocketAuthBypass` — ensure WS uses token claims, not query params.
3. `TestSubtitlePathTraversal` + `TestThumbnailPathTraversal` — fuzz path params.
4. `TestJWTAlgNoneAttack` + `TestJWTTokenTampering` — auth bypass prevention.
5. `TestBruteForceRaceDetection` + `TestOAuthStateRaceDetection` — run with `-race`.
6. `TestSearchFTSInjection` + `TestSmartPlaylistSQLInjection` — SQL/FTS injection.
7. `TestBindUnknownFieldsRejection` — insecure deserialization.

### Phase 2 — High (P1)
8. `TestCORSProductionBlockLocalhost` + `TestDASHCORSWildcard` — CORS hardening.
9. `TestSessionTimeoutPerDevice` + `TestCSRFSameSiteStrict` — session security.
10. `TestTranscodeJobQueueLimit` + `TestDirectStreamRangeFlood` — DoS resilience.
11. `TestLoginTimingSideChannel` + `TestErrorMessageSanitization` — info leakage.
12. `TestSecretPersistence` + `TestSecureStoreKeyDerivation` — cryptography.
13. `TestDLNAXMLEscaping` + `TestJSONResponseXSSPayload` — XSS prevention.
14. `TestPprofAuthRequired` — endpoint exposure.

### Phase 3 — Medium (P2)
15. `TestBackupZipBomb` + `TestRateLimitIPExhaustion` — resource exhaustion.
16. `TestWebSocketBroadcastScalability` + `TestWebSocketConcurrentClose` — WS robustness.
17. `TestSonarrPathValidation` + `TestLiveTVPathValidation` — integration path safety.
18. `TestAPIKeyHashPerformance` + `TestAPIKeyShortInput` — API key robustness.

---

## 15. CI/CD Integration Recommendations

- **Add `go test -race ./...`** to CI pipeline (currently missing — race tests not run).
- **Add `gosec ./...`** with fail-on-medium threshold.
- **Add fuzz regression:** `go test -fuzz=FuzzParseFilename -fuzztime=30s` and `go test -fuzz=FuzzParseResult -fuzztime=30s`.
- **Add new fuzz targets:** `FuzzAPISearchQuery`, `FuzzPathParam`, `FuzzJSONBinder`.
- **Add penetration test suite:** Use `httptest` to simulate OWASP ZAP-style scans (path traversal payloads, XSS payloads, SQLi payloads).

---

*End of Security Test Gap Analysis*
