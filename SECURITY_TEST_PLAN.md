# AetherStream Security Test Plan

**Version:** 1.0  
**Date:** 2026-05-10  
**Scope:** Full AetherStream codebase (50 packages, 129 Go files)  
**Baseline:** gosec 0 HIGH / 0 MEDIUM / 0 LOW (target), current 0-0-0  
**Standards:** OWASP ASVS v4.0, OWASP Testing Guide v4.2, NIST SP 800-53 Rev 5, GDPR Art. 32

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Test Strategy & Priorities](#2-test-strategy--priorities)
3. [Unit Security Tests](#3-unit-security-tests)
4. [Integration Security Tests](#4-integration-security-tests)
5. [Fuzzing Tests](#5-fuzzing-tests)
6. [Performance Security Tests](#6-performance-security-tests)
7. [Compliance Tests](#7-compliance-tests)
8. [Recommended Tooling](#8-recommended-tooling)
9. [CI/CD Security Pipeline](#9-cicd-security-pipeline)
10. [Metrics & Reporting](#10-metrics--reporting)
11. [Appendix: Test Case Catalog](#11-appendix-test-case-catalog)

---

## 1. Executive Summary

AetherStream is a Go media server with 50 packages handling authentication, streaming, transcoding, WebSocket real-time communication, clustering, and DLNA. The security audit baseline (SECURITY_AUDIT_REPORT.md) identified 4 HIGH, 7 MEDIUM, and 12 LOW findings. The current gosec scan reports 0-0-0, indicating all static findings have been remediated.

This plan defines a **continuous, layered security testing program** covering:
- **Static & unit-level** validation of auth, crypto, and input handling
- **Integration-level** end-to-end auth flows and session lifecycle tests
- **Dynamic & fuzzing** coverage of API endpoints and file parsers
- **Performance & DoS** resistance validation
- **Compliance & privacy** verification (GDPR, audit logging)
- **Tooling & automation** integrated into CI/CD

### Risk-Based Priorities

| Priority | Focus Area | Rationale |
|----------|------------|-----------|
| **P0** | Authentication & Authorization | Hardcoded secrets, JWT validation, RBAC bypasses, unprotected stream routes |
| **P0** | Input Validation & Path Traversal | Media file serving, subtitle extraction, thumbnail paths — direct data exposure risk |
| **P1** | Session Management & OAuth | Broken session timeout, race conditions in OAuth state store |
| **P1** | Cryptography | Key derivation, API key hashing, AES-GCM implementation |
| **P2** | Network & CORS | Wildcard origins, HSTS misconfiguration, WebSocket auth |
| **P2** | Compliance & Audit | GDPR data privacy, audit log completeness, retention policies |
| **P3** | Dependency & Supply Chain | Vulnerable modules, container image scanning |

---

## 2. Test Strategy & Priorities

### 2.1 Testing Pyramid for Security

```
         /\
        /  \     Dynamic / Penetration (OWASP ZAP, manual)
       /____\    ─────────────────────────────────────────
      /      \   Integration (end-to-end auth, session, API)
     /________\  ─────────────────────────────────────────
    /          \ Unit (auth, crypto, validation, RBAC)
   /____________\─────────────────────────────────────────
  /              \ Static (gosec, nancy, trivy, semgrep)
 /________________\───────────────────────────────────────
```

### 2.2 Definition of Done (Security)

A release is security-approved when ALL of the following are true:

1. `gosec` reports **0 HIGH, 0 MEDIUM, ≤5 LOW** (with documented waivers)
2. `go test -race ./...` passes with **0 race conditions**
3. Unit security tests achieve **≥80% coverage** on `pkg/auth`, `pkg/apikeys`, `pkg/securestore`, `pkg/api/security.go`
4. Integration security tests pass (auth flow, session expiry, rate limiting)
5. Fuzzing corpus runs for **≥10 minutes per target** with **0 crashes**
6. Dependency scan (`nancy`, `trivy`) reports **0 CRITICAL, 0 HIGH** unpatched CVEs
7. Container scan (`trivy image`) reports **0 CRITICAL, 0 HIGH**
8. OWASP ZAP baseline scan reports **0 High / 0 Medium** alerts
9. GDPR compliance checklist is signed off
10. All P0 audit findings have regression tests

### 2.3 Test Environments

| Environment | Purpose | Data |
|-------------|---------|------|
| `dev` | Developer local runs, unit tests | Synthetic / mock |
| `ci` | Automated pipeline, fast feedback | Synthetic / mock |
| `staging` | Integration, fuzzing, ZAP scans | Anonymized production-like |
| `prod` | Continuous monitoring, audit log verification | Real (read-only tests) |

---

## 3. Unit Security Tests

### 3.1 Authentication Tests (`pkg/auth`, `pkg/apikeys`, `pkg/oauth`)

**Goal:** Validate token lifecycle, RBAC enforcement, and credential handling in isolation.

| ID | Test Case | Target | Expected Result | Priority |
|----|-----------|--------|-----------------|----------|
| UT-AUTH-01 | JWT secret must be ≥32 bytes; reject shorter secrets | `auth.NewService` | Error returned | P0 |
| UT-AUTH-02 | Token generation produces valid HS256 JWT with correct claims | `auth.GenerateToken` | Valid token, correct `sub`, `role`, `exp` | P0 |
| UT-AUTH-03 | Token validation rejects expired tokens | `auth.ValidateToken` | Error: token expired | P0 |
| UT-AUTH-04 | Token validation rejects tampered signature | `auth.ValidateToken` | Error: invalid signature | P0 |
| UT-AUTH-05 | Token validation rejects wrong signing method (e.g., `none`, `RS256`) | `auth.ValidateToken` | Error: unexpected signing method | P0 |
| UT-AUTH-06 | `RequireRole` middleware allows exact role match | `auth.RequireRole("user")` | 200 for user, 403 for mismatch | P0 |
| UT-AUTH-07 | `RequireRole` middleware allows admin bypass with documentation | `auth.RequireRole("user")` | 200 for admin, documented behavior | P0 |
| UT-AUTH-08 | Brute-force protection blocks after N failed attempts from same IP | `security.BruteForceProtection` | 429 after threshold | P0 |
| UT-AUTH-09 | Brute-force protection resets after successful login | `security.BruteForceProtection` | Counter reset, 200 OK | P1 |
| UT-AUTH-10 | Brute-force protection uses `RemoteAddr` fallback when not behind proxy | `security.BruteForceProtection` | Correct IP extraction | P1 |
| UT-AUTH-11 | API key generation produces high-entropy 32-byte random key | `apikeys.GenerateAPIKey` | 64 hex chars, crypto-rand | P0 |
| UT-AUTH-12 | API key validation rejects revoked keys | `apikeys.ValidateAPIKey` | Error: key revoked | P0 |
| UT-AUTH-13 | API key validation rejects non-existent keys | `apikeys.ValidateAPIKey` | Error: key not found | P0 |
| UT-AUTH-14 | API key hashing uses bcrypt/Argon2id (not raw SHA-256) | `apikeys.hashKey` | bcrypt cost ≥10 or Argon2id | P1 |
| UT-AUTH-15 | OAuth state generation produces random, time-bound state parameter | `oauth.GenerateState` | 32+ random bytes, expiry set | P1 |
| UT-AUTH-16 | OAuth state validation rejects expired/reused states | `oauth.ValidateState` | Error: state invalid/expired | P1 |
| UT-AUTH-17 | OAuth state store is concurrency-safe (no data races) | `oauth.Service` | `go test -race` clean | P1 |
| UT-AUTH-18 | Login endpoint uses actual DB `id` and `role` (not hardcoded `"admin-1"`) | `api.handleLogin` | Token claims match DB record | P0 |
| UT-AUTH-19 | Password verification uses bcrypt with correct cost factor | `db.VerifyPassword` or equivalent | bcrypt.CompareHashAndPassword success | P0 |
| UT-AUTH-20 | SSO/LDAP provider initialization validates required config fields | `auth.NewLDAPProvider` | Error if host/credentials missing | P2 |

**Implementation Notes:**
- Use `github.com/golang-jwt/jwt/v5` test helpers to forge malicious tokens.
- Mock the DB layer with an in-memory SQLite `:memory:` instance for login tests.
- Run `go test -race` on all auth packages in CI.

### 3.2 Input Validation Tests

**Goal:** Ensure all user-controlled inputs are sanitized, bounded, and reject malicious payloads.

| ID | Test Case | Target | Expected Result | Priority |
|----|-----------|--------|-----------------|----------|
| UT-INPUT-01 | `itemID` path parameter rejects path traversal (`../etc/passwd`) | `api.handleGetSubtitle` | 400/403, no file access | P0 |
| UT-INPUT-02 | `lang` parameter validated against allowlist `^[a-zA-Z]{2,3}(-[a-zA-Z]{2})?$` | `api.handleGetSubtitle` | 400 for invalid lang | P0 |
| UT-INPUT-03 | `itemID` for thumbnails validated as UUID or alphanumeric | `api.handleGetThumbnail` | 400 for non-UUID traversal | P0 |
| UT-INPUT-04 | `thumbSvc.Path` resolves strictly under configured thumbnail root | `thumbnail.Path` | Error if path escapes root | P0 |
| UT-INPUT-05 | Search `q` parameter bounded to ≤200 characters | `api.handleSearch` | Truncated or 400 if exceeded | P1 |
| UT-INPUT-06 | Search `limit` parameter bounded to 1–100 | `api.handleSearch` | Clamped to valid range | P1 |
| UT-INPUT-07 | Search `mediaType` validated against known enum values | `api.handleSearch` | 400 for unknown type | P1 |
| UT-INPUT-08 | File upload endpoints validate MIME type and extension allowlist | `api.handleUpload` (if exists) | 400 for disallowed types | P1 |
| UT-INPUT-09 | JSON request bodies reject unexpected fields (strict unmarshaling) | All API handlers | 400 for unknown fields | P2 |
| UT-INPUT-10 | All `echo.Context` path/query params are escaped before logging | Middleware | No log injection | P2 |
| UT-INPUT-11 | `probe.ExtractSubtitleToFile` returns path within known-safe temp dir | `probe.ExtractSubtitleToFile` | Error if path escapes temp | P0 |
| UT-INPUT-12 | Direct stream path validation uses `filepath.Rel` + `..` check | `stream.Server.handleDirectStream` | 403 for traversal | P0 |
| UT-INPUT-13 | `config.Load` rejects invalid port ranges, unreadable DB paths | `config.Load` | Error on bad config | P2 |

**Implementation Notes:**
- Create a `pkg/validation` package with reusable validators (UUID, language code, safe path).
- Use `afero` (virtual filesystem) for path traversal tests to avoid real disk access.
- Property-based testing (quick.Check) for input generators.

### 3.3 Encryption & Cryptography Tests

**Goal:** Validate correct use of cryptographic primitives and key management.

| ID | Test Case | Target | Expected Result | Priority |
|----|-----------|--------|-----------------|----------|
| UT-CRYPTO-01 | `securestore.NewStore` rejects keys < 32 bytes | `securestore.NewStore` | Error | P0 |
| UT-CRYPTO-02 | `securestore.Encrypt` produces AES-256-GCM ciphertext with nonce | `securestore.Encrypt` | Ciphertext format: nonce(12) + ciphertext + tag(16) | P0 |
| UT-CRYPTO-03 | `securestore.Decrypt` rejects tampered ciphertext (MAC failure) | `securestore.Decrypt` | Error: authentication failed | P0 |
| UT-CRYPTO-04 | `securestore.Decrypt` rejects ciphertext with wrong key | `securestore.Decrypt` | Error: authentication failed | P0 |
| UT-CRYPTO-05 | Secure compare (`subtle.ConstantTimeCompare`) used for key comparison | `securestore` or `apikeys` | No timing side-channel | P1 |
| UT-CRYPTO-06 | Key derivation uses PBKDF2 (≥100k iterations) or Argon2id from password input | `securestore.NewStore` (after fix) | Derived key ≠ raw password | P1 |
| UT-CRYPTO-07 | JWT `exp` claim is validated with explicit leeway (≤60s) | `auth.ValidateToken` | Accepts minor clock skew | P1 |
| UT-CRYPTO-08 | JWT `nbf` / `iat` claims validated if present | `auth.ValidateToken` | Rejects tokens used before issuance | P2 |
| UT-CRYPTO-09 | Master key `AETHERSTREAM_MASTER_KEY` refuses fallback to `cfg.Auth.Secret` in production | `cmd/main` startup | Fatal error if missing in prod mode | P0 |
| UT-CRYPTO-10 | Brotli compression middleware does not leak plaintext via compression side-channel | `performance.BrotliMiddleware` | No BREACH-like vulnerability | P2 |

**Implementation Notes:**
- Use `crypto/aes` and `crypto/cipher` test vectors where applicable.
- Verify GCM nonce uniqueness with statistical tests (never repeat nonce across 1M encryptions).

---

## 4. Integration Security Tests

### 4.1 End-to-End Authentication Flow

**Goal:** Verify complete auth lifecycle across HTTP handlers, middleware, and DB.

| ID | Test Case | Setup | Expected Result | Priority |
|----|-----------|-------|-----------------|----------|
| IT-AUTH-01 | Valid login returns JWT + sets secure cookie | `POST /api/login` | 200, `HttpOnly; Secure; SameSite=Strict` cookie | P0 |
| IT-AUTH-02 | Invalid password returns 401, increments brute-force counter | `POST /api/login` (bad pwd × 5) | 401, then 429 on 6th attempt | P0 |
| IT-AUTH-03 | Brute-force block expires after configured duration | Wait for cooldown | Login succeeds again | P1 |
| IT-AUTH-04 | Protected route rejects missing/invalid Authorization header | `GET /api/users` (no token) | 401 | P0 |
| IT-AUTH-05 | Protected route rejects expired token | Use expired JWT | 401 | P0 |
| IT-AUTH-06 | Protected route rejects token with forged role claim | Forge JWT with `role: admin` | 403 (signature invalid) | P0 |
| IT-AUTH-07 | Logout invalidates session/token (if blocklist implemented) | `POST /api/logout` | Subsequent requests 401 | P1 |
| IT-AUTH-08 | API key auth succeeds with valid `X-API-Key` header | `GET /api/items` (valid key) | 200 | P0 |
| IT-AUTH-09 | API key auth fails with revoked key | Use revoked key | 401 | P0 |
| IT-AUTH-10 | OAuth2 login flow completes with valid state + code exchange | Simulate Google/GitHub OAuth | 200, session cookie set | P1 |
| IT-AUTH-11 | OAuth2 login rejects CSRF (missing/invalid state) | Tamper state param | 403 | P1 |
| IT-AUTH-12 | OAuth2 login rejects replayed state | Reuse state token | 403 | P1 |
| IT-AUTH-13 | CORS preflight rejects origin not in allowlist | `OPTIONS` from `evil.com` | 403 / no CORS headers | P0 |
| IT-AUTH-14 | CORS allows configured origins with credentials | `OPTIONS` from `https://app.example.com` | 204, `Access-Control-Allow-Credentials: true` | P0 |
| IT-AUTH-15 | CORS rejects wildcard `*` when credentials enabled | Any origin with `*` config | No `Access-Control-Allow-Origin: *` + credentials | P0 |

**Implementation Notes:**
- Use `httptest.Server` + Echo router for in-memory integration tests.
- Spin up a real SQLite DB (`:memory:` or temp file) for persistence tests.
- Use `jar, _ := cookiejar.New(nil)` to test cookie behavior.

### 4.2 Session Management Tests

**Goal:** Verify session lifecycle, timeout, and fixation protection.

| ID | Test Case | Setup | Expected Result | Priority |
|----|-----------|-------|-----------------|----------|
| IT-SESS-01 | Session cookie has `Secure`, `HttpOnly`, `SameSite=Strict` | Login + inspect response | All flags present | P0 |
| IT-SESS-02 | Session ID is random (≥128 bits entropy) | Generate 1000 sessions | No collisions, uniform distribution | P0 |
| IT-SESS-03 | Idle timeout expires session after configured duration | Wait + make request | 401, session invalidated server-side | P0 |
| IT-SESS-04 | Absolute timeout expires session after max lifetime | Wait long duration | 401 regardless of activity | P1 |
| IT-SESS-05 | Session fixation protection: new session ID after login | Login, capture session ID | Different ID post-auth | P1 |
| IT-SESS-06 | Concurrent sessions from same user are tracked/limited | Login from 2 browsers | Configurable max sessions enforced | P2 |
| IT-SESS-07 | Session store survives server restart (if persistent) | Restart server, reuse cookie | Session still valid (if not expired) | P2 |
| IT-SESS-08 | WebSocket upgrade requires valid token/cookie | `GET /ws` (no auth) | 401 or connection rejected | P1 |
| IT-SESS-09 | WebSocket connection closes when session expires mid-flight | Expire session while WS open | Server closes connection | P2 |

**Implementation Notes:**
- Implement a `MemorySessionStore` and `DBSessionStore` behind a `SessionStore` interface for testability.
- Use `time.Sleep` in tests with `testing.Short()` guard for timeout tests.

### 4.3 Authorization & RBAC Integration

| ID | Test Case | Setup | Expected Result | Priority |
|----|-----------|-------|-----------------|----------|
| IT-RBAC-01 | Admin can access all admin-only endpoints | Admin JWT | 200 | P0 |
| IT-RBAC-02 | Non-admin user receives 403 on admin endpoints | User JWT | 403 | P0 |
| IT-RBAC-03 | Permission matrix evaluated correctly (e.g., `users:read` vs `users:write`) | Custom role with partial perms | 200 for allowed, 403 for denied | P1 |
| IT-RBAC-04 | Stream routes require authentication | `GET /videos/:id/stream` (no token) | 401 (after fix) | P0 |
| IT-RBAC-05 | HLS master playlist requires authentication | `GET /videos/:id/hls/master.m3u8` | 401 (after fix) | P0 |
| IT-RBAC-06 | DLNA endpoints respect device authorization (if implemented) | Unregistered device | 403 or ignored | P2 |

---

## 5. Fuzzing Tests

### 5.1 API Endpoint Fuzzing

**Goal:** Discover crashes, panics, and unexpected behavior from malformed inputs.

| ID | Target | Input Strategy | Duration | Priority |
|----|--------|---------------|----------|----------|
| FZ-API-01 | `POST /api/login` — JSON body | Random JSON structures, long strings, Unicode, nested objects | 10 min | P0 |
| FZ-API-02 | `GET /api/search?q=&type=&limit=` | Query param mutations: SQLi patterns, path traversal, null bytes | 10 min | P0 |
| FZ-API-03 | `GET /api/videos/:id/stream` — `id` param | UUID variants, path traversal, format strings, overlong strings | 10 min | P0 |
| FZ-API-04 | `GET /api/videos/:id/hls/master.m3u8` — `id` param | Same as FZ-API-03 | 10 min | P0 |
| FZ-API-05 | `GET /api/subtitles/:id/:lang` — both params | Lang code mutations, ID traversal | 10 min | P0 |
| FZ-API-06 | `GET /api/thumbnails/:id/:type` — both params | Type enum fuzzing, ID traversal | 10 min | P0 |
| FZ-API-07 | `POST /api/libraries` — JSON body | Library name/path fuzzing, path traversal in paths | 10 min | P1 |
| FZ-API-08 | `PUT /api/users/:id` — JSON body | Role escalation attempts, invalid fields | 10 min | P1 |
| FZ-API-09 | WebSocket message frames | Binary frames, oversized UTF-8, control frame injection | 10 min | P1 |
| FZ-API-10 | OAuth callback URL — `code` and `state` | Long strings, special chars, replayed values | 10 min | P1 |

**Implementation Notes:**
- Use Go 1.18+ native fuzzing: `func FuzzLogin(f *testing.F)`.
- Seed corpus: valid request + 10 known-bad variants (empty, max length, Unicode, null bytes).
- Run with `-race` during fuzzing to catch concurrency bugs.
- Integrate with `go-fuzz` or `fuzzing` package for continuous corpus evolution.

### 5.2 File Parsing Fuzzing

**Goal:** Ensure media metadata parsers and transcode probes handle malformed files safely.

| ID | Target | Input Strategy | Duration | Priority |
|----|--------|---------------|----------|----------|
| FZ-FILE-01 | `probe.ExtractSubtitleToFile` — input media file | Truncated MP4/MKV, malformed headers, oversized files | 15 min | P1 |
| FZ-FILE-02 | `metadata.ParseNFO` — NFO XML files | Malformed XML, XXE payloads, billion laughs | 10 min | P1 |
| FZ-FILE-03 | `m3u.ParsePlaylist` — M3U playlists | Path traversal in URLs, infinite loops, recursion | 10 min | P1 |
| FZ-FILE-04 | `scanner.ScanDirectory` — directory traversal | Symlinks, recursive symlinks, permission changes | 10 min | P1 |
| FZ-FILE-05 | `backup.Restore` — archive decompression | Zip/gzip bombs, truncated archives, path traversal | 10 min | P1 |
| FZ-FILE-06 | `thumbnail.GenerateThumbnails` — image input | Malformed JPEG/PNG, memory exhaustion attempts | 10 min | P2 |
| FZ-FILE-07 | `images.Resize` — image processing | Pixel flood, ICC profile exploits | 10 min | P2 |

**Implementation Notes:**
- Use `afero.MemMapFs` to avoid real disk writes during fuzzing.
- Set `ulimit` and memory limits on fuzzing processes.
- For FFmpeg probes, use mock subprocesses to avoid actual FFmpeg invocation in fuzz loops.

---

## 6. Performance Security Tests

### 6.1 Denial of Service Resistance

**Goal:** Verify the system remains stable under malicious load.

| ID | Test Case | Method | Threshold | Priority |
|----|-----------|--------|-----------|----------|
| PT-DOS-01 | Login endpoint — brute-force rate limit enforcement | `k6` or `vegeta` at 1000 req/s | ≤1% pass rate after block, 0% server crash | P0 |
| PT-DOS-02 | Search endpoint — large `limit` param does not OOM | `limit=999999999` | 400 response, <50MB memory spike | P1 |
| PT-DOS-03 | HLS master request — concurrent transcode storm | 100 concurrent requests for same item | Rate-limited, single transcode job, no OOM | P0 |
| PT-DOS-04 | WebSocket — connection exhaustion | 10k concurrent connections | Graceful rejection, no panic, memory stable | P1 |
| PT-DOS-05 | WebSocket — message flood from single client | 1M messages/sec | Rate-limited or disconnected, no hub deadlock | P1 |
| PT-DOS-06 | File upload — oversized file rejection | 10GB POST body | 413 early rejection, no temp disk exhaustion | P1 |
| PT-DOS-07 | Backup restore — zip/gzip bomb handling | 10MB zip expanding to 100GB | `io.CopyN` limit enforced, 400 error | P1 |
| PT-DOS-08 | Brotli compression — compression bomb | Highly compressible repeated data | Response size bounded, no memory spike | P2 |
| PT-DOS-09 | DLNA SSDP — broadcast storm handling | Flood SSDP M-SEARCH | No CPU exhaustion, responses rate-limited | P2 |
| PT-DOS-10 | API key validation — cache hit under load | 10k req/s with same key | <1ms p99 latency, no DB overload | P1 |

**Implementation Notes:**
- Use `k6` scripts in `tests/perf/` directory.
- Monitor with Prometheus metrics: `http_requests_total`, `go_memstats_heap_inuse_bytes`, `goroutines`.
- Set hard timeouts on all HTTP handlers (Echo `TimeoutMiddleware`).

### 6.2 Rate Limiting Tests

| ID | Test Case | Target | Expected Result | Priority |
|----|-----------|--------|-----------------|----------|
| PT-RATE-01 | IP-based rate limit on login (5/min) | `POST /api/login` | 429 after 5 failures, `Retry-After` header | P0 |
| PT-RATE-02 | IP-based rate limit on OAuth login | `GET /auth/oauth/:provider/login` | 429 after threshold | P1 |
| PT-RATE-03 | User-based rate limit on API key usage | Any API endpoint | 429 after threshold per key | P1 |
| PT-RATE-04 | Global rate limit on HLS segment requests | `GET /videos/:id/hls/*.ts` | 429 if global threshold exceeded | P2 |
| PT-RATE-05 | Burst allowance with token bucket | Short burst of 10 requests | First 10 pass, then throttled | P2 |

---

## 7. Compliance Tests

### 7.1 GDPR & Data Privacy

| ID | Test Case | Requirement | Expected Result | Priority |
|----|-----------|-------------|-----------------|----------|
| CP-GDPR-01 | User data export returns all personal data in machine-readable format | Art. 20 (Portability) | JSON/CSV with user profile, history, preferences | P1 |
| CP-GDPR-02 | User deletion (right to erasure) removes all PII from DB and FS | Art. 17 (Erasure) | No residual user records, thumbs/logs anonymized | P1 |
| CP-GDPR-03 | Consent log records timestamp and purpose for each consent | Art. 7 (Conditions) | Audit table entry with `user_id`, `purpose`, `timestamp` | P2 |
| CP-GDPR-04 | Data retention policy enforces automatic deletion after configured period | Art. 5(1)(e) | Cron job or background task deletes expired data | P2 |
| CP-GDPR-05 | PII is not logged in plain text (passwords, tokens, API keys) | Art. 32 (Security) | Logs mask sensitive fields | P0 |
| CP-GDPR-06 | Encryption at rest for sensitive DB columns (if applicable) | Art. 32 | SQLite DB or sensitive columns encrypted | P1 |
| CP-GDPR-07 | Privacy policy version tracked and user acceptance required on update | Art. 13/14 | DB tracks `privacy_policy_version` | P2 |
| CP-GDPR-08 | Cross-border data transfer safeguards (if cloud backup used) | Art. 44-49 | Configurable region, encryption in transit | P2 |

### 7.2 Audit Logging

| ID | Test Case | Event | Expected Log Fields | Priority |
|----|-----------|-------|---------------------|----------|
| CP-AUDIT-01 | Login success logged | `login.success` | `user_id`, `ip`, `timestamp`, `user_agent`, `method` | P0 |
| CP-AUDIT-02 | Login failure logged | `login.failure` | `username`, `ip`, `timestamp`, `reason` | P0 |
| CP-AUDIT-03 | Logout logged | `logout` | `user_id`, `ip`, `timestamp` | P1 |
| CP-AUDIT-04 | Password change logged | `password.change` | `user_id`, `ip`, `timestamp` | P1 |
| CP-AUDIT-05 | API key creation/revocation logged | `apikey.created`, `apikey.revoked` | `user_id`, `key_id`, `ip`, `timestamp` | P1 |
| CP-AUDIT-06 | Role/permission change logged | `rbac.change` | `admin_id`, `target_user_id`, `changes`, `timestamp` | P1 |
| CP-AUDIT-07 | Stream access logged | `stream.access` | `user_id`, `item_id`, `ip`, `timestamp`, `format` | P1 |
| CP-AUDIT-08 | Library deletion logged | `library.deleted` | `user_id`, `library_id`, `timestamp` | P1 |
| CP-AUDIT-09 | Audit logs are tamper-resistant (append-only, signed or WORM) | N/A | Logs cannot be modified by admin | P2 |
| CP-AUDIT-10 | Audit log retention policy enforced | N/A | Logs retained for configured period (e.g., 1 year) | P2 |

### 7.3 Data Integrity & Backup

| ID | Test Case | Expected Result | Priority |
|----|-----------|-----------------|----------|
| CP-BACKUP-01 | Backup encryption uses AES-256-GCM with unique key per backup | Encrypted file, decryptable only with key | P1 |
| CP-BACKUP-02 | Backup integrity verified via checksum (SHA-256 or BLAKE3) | Mismatch detected, restore rejected | P1 |
| CP-BACKUP-03 | Backup restore does not overwrite newer data without confirmation | Warning or confirmation required | P2 |

---

## 8. Recommended Tooling

### 8.1 Static Analysis & SAST

| Tool | Purpose | Integration | Frequency | Priority |
|------|---------|-------------|-----------|----------|
| **gosec** | Go security scanner (G101-G602) | `make security-scan` | Every commit | P0 |
| **semgrep** | Custom rule engine for Go security patterns | CI job | Every commit | P1 |
| **govulncheck** | Official Go vulnerability scanner | `go install golang.org/x/vuln/cmd/govulncheck@latest` | Every commit | P0 |
| **nancy** | Dependency vulnerability scanner (Sonatype) | `nancy sleuth -q go.sum` | Every commit | P0 |
| **trivy** | Dependency + container + filesystem scanner | `trivy fs .` + `trivy image aetherstream` | Every commit + nightly | P0 |
| **staticcheck** | Go static analysis (broader than security) | `staticcheck ./...` | Every commit | P1 |
| **errcheck** | Unhandled error detection | `errcheck ./...` | Every commit | P2 |

**gosec Configuration:**
```yaml
# .gosec.json
{
  "severity": "medium",
  "confidence": "medium",
  "exclude": ["G104"],
  "tests": true,
  "nosec": false,
  "sort": true
}
```
Run: `gosec -fmt=json -out=gosec-report.json ./...`

### 8.2 Dynamic Analysis & DAST

| Tool | Purpose | Integration | Frequency | Priority |
|------|---------|-------------|-----------|----------|
| **OWASP ZAP** | Web app vulnerability scanner | Docker container in CI | Weekly on staging | P1 |
| **Burp Suite Community** | Manual penetration testing | Local / staging | Quarterly | P2 |
| **ffuf** | Directory/file brute-forcing | `tests/dast/` scripts | Monthly | P2 |
| **sqlmap** | SQL injection testing | Staging only | Quarterly | P2 |

**ZAP Baseline Scan:**
```bash
docker run -t owasp/zap2docker-stable zap-baseline.py \
  -t http://aetherstream-staging:8080 \
  -r zap-report.html \
  -c zap-rules.conf
```

### 8.3 Fuzzing Tools

| Tool | Purpose | Integration | Frequency | Priority |
|------|---------|-------------|-----------|----------|
| **Go Native Fuzzing** (`testing.F`) | In-process fuzzing for Go functions | `go test -fuzz=FuzzLogin` | Nightly CI (30 min) | P1 |
| **go-fuzz** | Coverage-guided fuzzing (legacy but robust) | `tests/fuzz/` | Weekly | P2 |
| **AFL++** | Binary fuzzing for C dependencies (FFmpeg) | Separate pipeline | Quarterly | P2 |

### 8.4 Dependency & Supply Chain

| Tool | Purpose | Integration | Frequency | Priority |
|------|---------|-------------|-----------|----------|
| **nancy** | Go module vulnerability scan | CI | Every commit | P0 |
| **trivy** | Full-stack vulnerability scan | CI + nightly | Every commit | P0 |
| **Snyk** | Dependency + container monitoring | SaaS dashboard | Continuous | P2 |
| **Sigstore/cosign** | Container image signing | Build pipeline | Every release | P1 |

### 8.5 Performance & Load Testing

| Tool | Purpose | Integration | Frequency | Priority |
|------|---------|-------------|-----------|----------|
| **k6** | HTTP load / DoS simulation | `tests/perf/k6/` | Weekly on staging | P1 |
| **vegeta** | HTTP rate testing | `tests/perf/vegeta/` | Nightly | P2 |
| **Go benchmark** | Micro-benchmarks for hot paths | `*_test.go` (Benchmark*) | Every commit | P2 |

---

## 9. CI/CD Security Pipeline

### 9.1 Pipeline Stages

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   BUILD     │───▶│    TEST     │───▶│    SCAN     │───▶│   REPORT    │───▶│   DEPLOY    │
│             │    │             │    │             │    │             │    │             │
│ go build    │    │ go test     │    │ gosec       │    │ SARIF       │    │ cosign      │
│ go vet      │    │ -race       │    │ govulncheck │    │ upload      │    │ sign image  │
│ lint        │    │ coverage    │    │ nancy       │    │ GH Security │    │ deploy      │
│             │    │ fuzz (short)│    │ trivy fs    │    │ Slack alert │    │             │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

### 9.2 GitHub Actions Workflow (`.github/workflows/security.yml`)

```yaml
name: Security Pipeline

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # nightly at 02:00 UTC

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go build ./...
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  unit-tests:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go test -race -coverprofile=coverage.out ./...
      - run: go tool cover -func=coverage.out
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out

  security-scan:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: gosec
        uses: securego/gosec@master
        with:
          args: '-fmt sarif -out gosec.sarif ./...'

      - name: govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

      - name: nancy
        run: |
          go install github.com/sonatypecommunity/nancy@latest
          nancy sleuth -q go.sum

      - name: trivy filesystem
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          format: 'sarif'
          output: 'trivy-fs.sarif'

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: gosec.sarif

  container-scan:
    runs-on: ubuntu-latest
    needs: build
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - name: Build image
        run: docker build -t aetherstream:${{ github.sha }} .
      - name: Trivy image scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: 'aetherstream:${{ github.sha }}'
          format: 'sarif'
          output: 'trivy-image.sarif'
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: trivy-image.sarif

  fuzzing:
    runs-on: ubuntu-latest
    needs: unit-tests
    if: github.event_name == 'schedule'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Fuzz auth
        run: go test -fuzz=FuzzLogin -fuzztime=10m ./pkg/auth
      - name: Fuzz API inputs
        run: go test -fuzz=FuzzSearch -fuzztime=10m ./pkg/api
      - name: Fuzz file parsing
        run: go test -fuzz=FuzzM3U -fuzztime=10m ./pkg/m3u

  dast:
    runs-on: ubuntu-latest
    needs: container-scan
    if: github.event_name == 'schedule'
    steps:
      - name: Start staging
        run: docker compose -f deploy/docker-compose.staging.yml up -d
      - name: ZAP baseline scan
        run: |
          docker run -t owasp/zap2docker-stable zap-baseline.py \
            -t http://host.docker.internal:8080 \
            -r zap-report.html
      - name: Upload ZAP report
        uses: actions/upload-artifact@v4
        with:
          name: zap-report
          path: zap-report.html
```

### 9.3 Branch Protection & Gates

| Gate | Rule | Enforcement |
|------|------|-------------|
| PR requires `security-scan` job to pass | Block merge on HIGH/MEDIUM findings | GitHub branch protection |
| PR requires `unit-tests` with ≥70% coverage | Block merge if coverage drops | Codecov / GitHub |
| `main` requires `container-scan` clean | Block deploy on CRITICAL/HIGH CVEs | GitHub environments |
| Nightly fuzzing must report 0 crashes | Alert on Slack/Discord if crash found | GitHub Actions + webhook |
| Weekly ZAP scan must report 0 High/Medium | Create issue auto-assigned to security lead | GitHub Issues API |

### 9.4 Makefile Targets

```makefile
# Makefile additions
.PHONY: security-scan security-test fuzz dast

security-scan:
	gosec -fmt=json -out=gosec-report.json ./...
	govulncheck ./...
	nancy sleuth -q go.sum
	trivy fs --scanners vuln,secret,config .

security-test:
	go test -race -v -run 'Test(CSRF|BruteForce|SessionTimeout|SecureCookie|Encrypt|Decrypt|Hash|ValidateToken)' ./...

fuzz:
	go test -fuzz=FuzzLogin -fuzztime=5m ./pkg/auth
	go test -fuzz=FuzzSearch -fuzztime=5m ./pkg/api
	go test -fuzz=FuzzM3U -fuzztime=5m ./pkg/m3u

dast:
	docker run -t owasp/zap2docker-stable zap-baseline.py \
	  -t http://localhost:8080 -r zap-report.html
```

---

## 10. Metrics & Reporting

### 10.1 Key Security Metrics (KSMs)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Static scan findings (HIGH/MEDIUM) | 0 | gosec + trivy + govulncheck |
| Dependency CVEs (CRITICAL/HIGH) | 0 | nancy + trivy + Snyk |
| Unit test coverage (security-critical packages) | ≥80% | `go test -cover` |
| Race conditions | 0 | `go test -race` |
| Fuzzing crashes | 0 | Nightly CI |
| Mean time to patch (MTTP) CVEs | ≤7 days | Issue tracker |
| Audit log completeness | 100% of required events | Compliance audit |
| Penetration test findings (High/Medium) | 0 | Quarterly ZAP + manual |

### 10.2 Reporting Cadence

| Report | Audience | Frequency | Content |
|--------|----------|-----------|---------|
| Security Scan Summary | Engineering | Every commit | gosec, govulncheck, nancy results |
| Vulnerability Report | Security lead + Engineering | Weekly | New CVEs, patch status, risk rating |
| Compliance Dashboard | Legal + DPO | Monthly | GDPR metrics, audit log health, retention compliance |
| Penetration Test Report | Security lead + CTO | Quarterly | ZAP findings, manual test results, remediation plan |
| Annual Security Review | Executive | Yearly | KSM trends, incident summary, roadmap |

---

## 11. Appendix: Test Case Catalog

### 11.1 Quick Reference Matrix

| Category | P0 Cases | P1 Cases | P2 Cases | Total |
|----------|----------|----------|----------|-------|
| Unit — Auth | 11 | 6 | 3 | 20 |
| Unit — Input | 5 | 5 | 3 | 13 |
| Unit — Crypto | 4 | 3 | 3 | 10 |
| Integration — Auth | 8 | 4 | 1 | 13 |
| Integration — Session | 3 | 3 | 3 | 9 |
| Integration — RBAC | 3 | 1 | 1 | 5 |
| Fuzzing — API | 5 | 5 | 0 | 10 |
| Fuzzing — File | 0 | 4 | 2 | 6 |
| Performance — DoS | 3 | 5 | 2 | 10 |
| Performance — Rate Limit | 2 | 2 | 1 | 5 |
| Compliance — GDPR | 1 | 3 | 4 | 8 |
| Compliance — Audit | 2 | 4 | 4 | 10 |
| Compliance — Backup | 0 | 2 | 1 | 3 |
| **Total** | **47** | **47** | **28** | **122** |

### 11.2 File Structure for Tests

```
tests/
├── unit/
│   ├── auth/
│   │   ├── jwt_test.go
│   │   ├── rbac_test.go
│   │   └── brute_force_test.go
│   ├── input/
│   │   ├── path_traversal_test.go
│   │   └── validation_test.go
│   └── crypto/
│       ├── securestore_test.go
│       └── apikey_hash_test.go
├── integration/
│   ├── auth_flow_test.go
│   ├── session_test.go
│   └── rbac_integration_test.go
├── fuzz/
│   ├── fuzz_login.go
│   ├── fuzz_search.go
│   ├── fuzz_m3u.go
│   └── fuzz_probe.go
├── perf/
│   ├── k6/
│   │   ├── login_dos.js
│   │   ├── hls_storm.js
│   │   └── ws_flood.js
│   └── vegeta/
│       └── rate_limit.targets
├── dast/
│   ├── zap-rules.conf
│   └── ffuf-wordlist.txt
├── compliance/
│   ├── gdpr_export_test.go
│   ├── gdpr_delete_test.go
│   └── audit_log_test.go
└── fixtures/
    ├── malicious/
    │   ├── path_traversal_m3u.m3u
    │   ├── xxe_nfo.xml
    │   └── zip_bomb.zip
    └── valid/
        ├── sample_media.mp4
        └── sample_nfo.xml
```

### 11.3 OWASP ASVS Mapping

| ASVS Chapter | Coverage in This Plan | Test IDs |
|--------------|----------------------|----------|
| V1: Architecture | Architecture audit, DI interfaces | ARCH-01..05 |
| V2: Authentication | UT-AUTH-01..20, IT-AUTH-01..15 | Auth suite |
| V3: Session Management | IT-SESS-01..09 | Session suite |
| V4: Access Control | IT-RBAC-01..06, UT-AUTH-06..07 | RBAC suite |
| V5: Validation | UT-INPUT-01..13 | Input suite |
| V6: Cryptography | UT-CRYPTO-01..10 | Crypto suite |
| V7: Error Handling | UT-INPUT-10, static analysis | Logging suite |
| V8: Data Protection | CP-GDPR-01..08, CP-BACKUP-01..03 | Compliance suite |
| V9: Communication | IT-AUTH-13..15 (CORS), IT-SESS-01 (cookies) | Network suite |
| V10: Malicious Code | gosec, semgrep, dependency scans | Tooling |
| V11: Business Logic | IT-AUTH-18 (hardcoded admin), PT-DOS-03 | Integration suite |
| V12: File Handling | UT-INPUT-01..04, FZ-FILE-01..07 | File suite |
| V13: API | FZ-API-01..10, PT-DOS-01..10 | API suite |
| V14: Configuration | UT-INPUT-13, CP-AUDIT-09..10 | Config suite |

---

## Document Control

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-05-10 | Hermes Security Agent | Initial comprehensive security test plan |

---

*This plan is a living document. Review and update quarterly or after every major security incident, penetration test, or architecture change.*
