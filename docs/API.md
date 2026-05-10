# AetherStream API Reference

This document describes all public HTTP endpoints, request/response formats, and authentication requirements for AetherStream v1.3.0.

---

## Base URL

| Environment | Base URL |
|-------------|----------|
| Local dev   | `http://localhost:8096` |
| Docker      | `http://localhost:8080` |

---

## Authentication

Most endpoints require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Obtain a token via `POST /auth/login`. Tokens expire after the configured TTL (default: 24 hours).

### Roles

- `admin` — full access (users, libraries, system)
- `user`  — read-only for most resources, can manage own collections

---

## Common Response Formats

### Success

```json
{
  "id": "uuid",
  "name": "..."
}
```

### Error

```json
{
  "message": "error description"
}
```

HTTP status codes used:

| Code | Meaning |
|------|---------|
| 200  | OK |
| 201  | Created |
| 204  | No Content |
| 400  | Bad Request |
| 401  | Unauthorized (missing/invalid token) |
| 403  | Forbidden (insufficient role) |
| 404  | Not Found |
| 429  | Too Many Requests (rate limit or brute-force) |
| 500  | Internal Server Error |

---

## System

### GET /system/info

**Auth:** None  
**Rate limit:** 1000 req/min per IP

Returns server version and health status.

**Response:**
```json
{
  "name": "AetherStream",
  "version": "1.3.0",
  "status": "ok"
}
```

---

### GET /api/system/hardware

**Auth:** None  
**Rate limit:** 1000 req/min per IP

Returns detected hardware acceleration capabilities.

**Response:**
```json
{
  "nvenc": true,
  "qsv": false,
  "vaapi": false,
  "gpus": [
    {
      "vendor": "NVIDIA",
      "model": "RTX 4090",
      "driver": "550.78",
      "backend": "nvenc"
    }
  ],
  "active": "nvenc"
}
```

---

### GET /metrics

**Auth:** None

Prometheus exposition format. Includes request counts, latencies, and Go runtime metrics.

---

### GET /debug/pprof

**Auth:** None

Go runtime profiling endpoints (pprof). Available when enabled via `pkg/metrics`.

---

## Authentication

### POST /auth/login

**Auth:** None  
**Rate limit:** 10 req/min per IP  
**Brute-force protection:** exponential backoff per IP/username

Login with username and password.

**Request body:**
```json
{
  "username": "admin",
  "password": "secret"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Errors:**
- 401 — invalid credentials
- 429 — too many failed attempts

---

### POST /auth/callback

**Auth:** None  
**Rate limit:** 10 req/min per IP

OAuth callback placeholder. Returns:
```json
{
  "status": "not implemented"
}
```

---

## Users

### GET /api/users

**Auth:** Required (any role)

List all users (id, username, role, created_at).

**Response:**
```json
[
  {
    "id": "admin-1",
    "username": "admin",
    "role": "admin",
    "created_at": "2025-01-01T00:00:00Z"
  }
]
```

---

### GET /api/users/:id

**Auth:** Required

Get a single user by ID.

**Response:** same shape as list item.

---

### POST /api/users

**Auth:** Required + `admin` role

Create a new user.

**Request body:**
```json
{
  "username": "alice",
  "password": "changeme",
  "role": "user"
}
```

**Response:**
```json
{
  "id": "uuid",
  "username": "alice",
  "role": "user"
}
```

---

### PUT /api/users/:id

**Auth:** Required + `admin` role

Update user role.

**Request body:**
```json
{
  "role": "admin"
}
```

**Response:**
```json
{
  "id": "uuid",
  "role": "admin"
}
```

---

### DELETE /api/users/:id

**Auth:** Required + `admin` role

Delete a user. Returns `204 No Content` on success.

---

## Libraries

### GET /api/libraries

**Auth:** Required

List all libraries.

**Response:**
```json
[
  {
    "id": "lib-1",
    "name": "Movies",
    "path": "/media/movies",
    "media_type": "movie",
    "created_at": "2025-01-01T00:00:00Z"
  }
]
```

---

### POST /api/libraries

**Auth:** Required + `admin` role

Create a library.

**Request body:**
```json
{
  "name": "Movies",
  "path": "/media/movies",
  "media_type": "movie"
}
```

**Response:**
```json
{
  "id": "lib-1",
  "name": "Movies",
  "path": "/media/movies"
}
```

---

### POST /api/libraries/:id/scan

**Auth:** Required + `admin` role

Trigger a library scan. Returns immediately; scan runs in background.

**Response:**
```json
{
  "status": "scanning",
  "library_id": "lib-1"
}
```

---

## Items

### GET /api/items

**Auth:** Required

List items in a library.

**Query params:**

| Param      | Required | Description |
|------------|----------|-------------|
| library_id | Yes      | Library UUID |

**Response:**
```json
[
  {
    "id": "item-1",
    "library_id": "lib-1",
    "path": "/media/movies/film.mkv",
    "name": "film",
    "media_type": "movie",
    "container": "mkv",
    "size_bytes": 2147483648,
    "duration_seconds": 7200,
    "width": 1920,
    "height": 1080,
    "video_codec": "h264",
    "audio_codec": "aac"
  }
]
```

---

### GET /api/items/:id

**Auth:** Required

Get single item details.

**Response:** same shape as list item.

---

### GET /api/items/:id/subtitles

**Auth:** Required

List embedded/external subtitle tracks for an item.

**Response:**
```json
[
  {
    "language": "eng",
    "codec": "subrip",
    "index": 2
  }
]
```

---

### GET /api/items/:id/subtitles/:lang

**Auth:** Required

Download subtitle file for a language. Returns the subtitle file directly (e.g., `.srt` or `.vtt`).

---

### GET /api/items/:id/thumbnails/:type

**Auth:** Required

Get thumbnail image. `type` must be `poster` or `backdrop`. Generated on-demand if missing.

---

## Search

### GET /api/search

**Auth:** Required

Full-text search across library items (SQLite FTS5).

**Query params:**

| Param | Required | Default | Description |
|-------|----------|---------|-------------|
| q     | Yes      | —       | Search query |
| type  | No       | —       | Filter by media_type |
| limit | No       | 20      | Max results |

**Response:**
```json
{
  "query": "inception",
  "type": "movie",
  "limit": 20,
  "results": [
    {
      "id": "item-1",
      "name": "Inception",
      "media_type": "movie"
    }
  ]
}
```

---

## Collections & Playlists

### GET /api/collections

**Auth:** Required

List collections for the authenticated user.

**Response:**
```json
[
  {
    "id": "col-1",
    "user_id": "user-1",
    "name": "Favorites",
    "collection_type": "collection",
    "created_at": "2025-01-01T00:00:00Z"
  }
]
```

---

### POST /api/collections

**Auth:** Required

Create a collection or playlist.

**Request body:**
```json
{
  "name": "Favorites",
  "type": "collection"
}
```

`type` can be `collection` or `playlist`. Defaults to `collection`.

**Response:**
```json
{
  "id": "col-1",
  "name": "Favorites",
  "type": "collection"
}
```

---

### GET /api/collections/:id

**Auth:** Required

Get collection with its items.

**Response:**
```json
{
  "collection": {
    "id": "col-1",
    "name": "Favorites",
    "collection_type": "collection"
  },
  "items": [
    {
      "id": "item-1",
      "name": "Inception"
    }
  ]
}
```

---

### POST /api/collections/:id/items

**Auth:** Required

Add an item to a collection.

**Request body:**
```json
{
  "item_id": "item-1"
}
```

Returns `204 No Content` on success.

---

### DELETE /api/collections/:id/items/:item_id

**Auth:** Required

Remove an item from a collection. Returns `204 No Content`.

---

## Activity

### GET /api/activity

**Auth:** Required

Recent activity log (last 50 entries).

**Response:**
```json
[
  {
    "id": 1,
    "user_id": "user-1",
    "action": "play",
    "details": "item-1",
    "created_at": "2025-01-01T00:00:00Z"
  }
]
```

---

### GET /api/session

**Auth:** Required

Current session info extracted from JWT claims.

**Response:**
```json
{
  "user_id": "user-1",
  "device_id": "device-1",
  "bandwidth_kbps": 5000,
  "timestamp": 1735689600
}
```

---

## Streaming

### GET /videos/:id/stream

**Auth:** Required

Direct play — serves the original media file with HTTP range support.

---

### GET /videos/:id/hls/master.m3u8

**Auth:** Required

HLS master playlist. If transcode is not ready, returns a waiting playlist and triggers background transcode for default profiles (`mobile`, `tablet`).

---

### GET /videos/:id/hls/:profile/playlist.m3u8

**Auth:** Required

HLS variant playlist for a specific profile (`mobile`, `tablet`, `tv`, `tv_4k`, etc.).

---

### GET /videos/:id/hls/:profile/:segment

**Auth:** Required

HLS `.ts` segment file.

---

### GET /videos/:id/probe

**Auth:** Required

Returns ffprobe JSON metadata for the item.

**Response:**
```json
{
  "format": "matroska",
  "duration": 7200.5,
  "streams": [
    {
      "codec_type": "video",
      "codec_name": "h264",
      "width": 1920,
      "height": 1080
    }
  ]
}
```

---

### GET /videos/:id/adaptive.m3u8

**Auth:** Required

Adaptive bitrate playlist. Automatically selects the best profile based on `bandwidth` and `device` query parameters.

**Query params:**

| Param     | Required | Default | Description |
|-----------|----------|---------|-------------|
| bandwidth | No       | 5000    | Client bandwidth in kbps |
| device    | No       | auto    | `mobile`, `tablet`, `tv` |

---

### GET /system/hwaccel

**Auth:** Required

Returns the currently active hardware acceleration backend.

**Response:**
```json
{
  "hwaccel": "nvenc",
  "status": "ok"
}
```

---

## WebSocket

### GET /ws

**Auth:** Required (JWT via query param or header)

Real-time WebSocket connection for activity feed and session sync.

**Query params:**

| Param     | Required | Default     | Description |
|-----------|----------|-------------|-------------|
| user_id   | No       | anonymous   | User identifier |
| device_id | No       | unknown     | Device identifier |

**Messages:**

Server sends:
```json
{"type":"connected","server":"AetherStream"}
```

Client can send playback progress / heartbeat JSON messages.

---

## Webhooks

### POST /webhooks/swiftflow

**Auth:** None (validated by webhook secret if configured)  
**Rate limit:** 100 req/min per IP

SwiftFlow captive portal callback. Creates a session and returns a JWT token.

**Request body:**
```json
{
  "user_id": "user-1",
  "device_id": "dev-1",
  "ip_address": "192.168.1.100",
  "mac_address": "aa:bb:cc:dd:ee:ff",
  "client_type": "ios",
  "bandwidth_kbps": 5000,
  "token": "swiftflow-token"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "device_id": "dev-1",
  "bandwidth_kbps": 5000,
  "client_type": "ios"
}
```

---

## Middleware Summary

All routes are protected by the following middleware stack (in order):

1. **SecurityHeaders** — OWASP recommended headers (CSP, HSTS, etc.)
2. **SecureCookieMiddleware** — enforces `Secure`, `HttpOnly`, `SameSite=Strict`
3. **CSRFProtection** — token validation on state-changing methods
4. **BruteForceProtection** — exponential backoff on `/auth/*`
5. **CORS** — configured for localhost dev origins
6. **RateLimiter** — Echo memory store (20 req/s global)
7. **JWT Middleware** — on `/api/*` routes
8. **SessionTimeout** — 30-minute idle logout on protected routes

---

## Rate Limits

| Endpoint group | Limit |
|----------------|-------|
| `/system/info` | 1000/min |
| `/auth/login`  | 10/min |
| `/webhooks/swiftflow` | 100/min |
| All other public | 20/sec (Echo default) |

---

## Changelog

- v1.3.0 — Gap closure: subtitles, chapters, progress tracking, WebRTC, clustering, GPU transcoding, Chromecast/AirPlay, DLNA, plugin system, Live TV/DVR, secure store, audit logging, E2E tests
- v1.2.0 — Production hardening: TLS/CORS/rate-limit, Prometheus/pprof, cache LRU, thumbnails, FTS5 search, benchmarks, CI/CD, web UI
- v1.1.0 — Streaming engine: probe, encoder, HLS, DASH, adaptive streaming, bandwidth adapter
- v1.0.0 — Library engine: scanner, naming parser, metadata fetch (TMDb, MusicBrainz), library manager
- v0.1.0 — Initial API set
