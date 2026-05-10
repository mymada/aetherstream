# AetherStream Troubleshooting Guide

Common issues, symptoms, causes, and fixes.

---

## Startup

### Server fails to start with "failed to load config"

**Symptom:**
```
FATAL failed to load config: open config.yaml: no such file or directory
```

**Cause:** `config.yaml` is missing and no environment variables override the defaults.

**Fix:**
- Create `config.yaml` in the working directory, OR
- Set all required values via environment variables (`AETHERSTREAM_AUTH_SECRET`, etc.)

---

### "failed to open database"

**Symptom:**
```
FATAL failed to open database: open ./data/aetherstream.db: permission denied
```

**Cause:** The process user lacks write permission to the database directory.

**Fix:**
```bash
mkdir -p ./data
chmod 755 ./data
chown $(whoami) ./data
```

In Docker, ensure the volume mount is writable by UID 1000 (the `aetherstream` user).

---

### "auth secret must be at least 32 characters"

**Symptom:**
```
FATAL failed to init auth: auth secret must be at least 32 characters
```

**Cause:** `AETHERSTREAM_AUTH_SECRET` is missing or too short.

**Fix:**
```bash
export AETHERSTREAM_AUTH_SECRET=$(openssl rand -base64 32)
```

---

## Authentication

### 401 Unauthorized on all /api/* routes

**Symptom:** Every API call returns `401`.

**Causes & Fixes:**

| Cause | Fix |
|-------|-----|
| Missing `Authorization` header | Add `Authorization: Bearer <token>` |
| Token expired | Re-login via `POST /auth/login` |
| Wrong token format | Ensure `Bearer ` prefix is present |
| Clock skew | Sync server and client clocks (JWT is time-sensitive) |

---

### 429 Too Many Requests on login

**Symptom:** `POST /auth/login` returns `429`.

**Cause:** Brute-force protection triggered after repeated failed logins.

**Fix:** Wait for the exponential backoff to expire (doubles with each failure, max 30 minutes). On success, the counter resets automatically.

---

### 403 Forbidden on admin endpoints

**Symptom:** `POST /api/users` or `POST /api/libraries` returns `403`.

**Cause:** Authenticated user does not have `admin` role.

**Fix:** Check JWT claims. Create an admin user directly in the database if locked out:

```sql
INSERT INTO users(id, username, password_hash, role)
VALUES ('admin-2', 'admin2', '<bcrypt_hash>', 'admin');
```

---

## Streaming

### HLS playlist returns "Waiting for transcode..."

**Symptom:** `GET /videos/:id/hls/master.m3u8` returns a placeholder playlist.

**Cause:** Transcode has not completed yet. Background transcode is triggered on first request.

**Fix:**
- Wait 10–60 seconds (depends on file size and CPU)
- Check FFmpeg is installed and in PATH
- Check `AETHERSTREAM_FFMPEG_MAX_JOBS` is not saturated
- Check disk space in `./media/transcodes/`

---

### Direct play returns "file not found on disk"

**Symptom:** `GET /videos/:id/stream` returns `404`.

**Cause:** The file path stored in the database no longer exists, or the media root changed.

**Fix:**
- Verify the file exists at the stored path
- Re-scan the library: `POST /api/libraries/:id/scan`
- Ensure the media volume is mounted correctly in Docker

---

### Transcode jobs fail with "transcode error"

**Symptom:** Logs show FFmpeg output with errors.

**Common causes & fixes:**

| Cause | Fix |
|-------|-----|
| Missing FFmpeg | Install FFmpeg 5.x+ |
| GPU driver missing | Install NVIDIA/Intel/AMD drivers on host |
| Wrong hwaccel mode | Set `AETHERSTREAM_FFMPEG_HWACCEL=none` to test software fallback |
| Insufficient disk space | Clean `./media/transcodes/` or expand volume |
| Corrupt source file | Re-encode or replace the file |

---

### Hardware acceleration not detected

**Symptom:** `GET /system/hwaccel` returns `"none"` despite having a GPU.

**Fix:**

**NVIDIA:**
```bash
nvidia-smi
# If missing, install drivers and NVIDIA Container Toolkit (Docker)
```

**Intel:**
```bash
vainfo
# If missing, install `intel-media-va-driver` or `intel-media-driver`
```

**AMD:**
```bash
vainfo
# If missing, install `mesa-va-drivers` or `libva-mesa-driver`
```

---

## Database

### SQLite "database is locked"

**Symptom:** Requests intermittently fail with `database is locked`.

**Cause:** Multiple writers or a long-running transaction.

**Fix:**
- Ensure only one AetherStream process accesses the database file
- Do not open the `.db` file with external tools while the server is running
- The server already sets WAL mode and busy timeout; if the issue persists, restart the server

---

### FTS5 search returns no results

**Symptom:** `GET /api/search?q=...` always returns empty.

**Cause:** SQLite was compiled without FTS5 support.

**Fix:**
- Rebuild with `CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"`
- The server falls back to a plain table if FTS5 is unavailable, but full-text ranking will be basic

---

## WebSocket

### WebSocket disconnects immediately

**Symptom:** Client connects to `/ws` then drops.

**Causes & Fixes:**

| Cause | Fix |
|-------|-----|
| Missing JWT | Pass token in query param or header |
| Reverse proxy blocks upgrade | Enable `proxy_http_version 1.1` and `Upgrade` headers in nginx |
| Firewall blocks port | Open the API port for WebSocket (same as HTTP) |

---

## Docker-Specific

### Container exits immediately

**Symptom:** `docker compose up` starts then stops.

**Fix:** Check logs:
```bash
docker logs aetherstream
```
Common causes: missing `AETHERSTREAM_AUTH_SECRET`, port conflict, or permission error on `/data`.

---

### Healthcheck fails

**Symptom:** Container status is `unhealthy`.

**Fix:**
```bash
docker exec aetherstream wget -qO- http://localhost:8080/system/info
```
If this fails, the server may not have started. Check logs.

---

### Media files not visible

**Symptom:** Libraries scan but find zero items.

**Cause:** Volume mount path mismatch. The container sees `/media`, but the library path in the database points to a host path.

**Fix:** Ensure the library `path` matches the mounted path inside the container. Example:
- Host: `/srv/media/movies` mounted to `/media/movies`
- Library path in DB: `/media/movies`

---

## Performance

### High CPU during streaming

**Symptom:** CPU usage spikes when playing videos.

**Cause:** Software transcoding because hardware acceleration is not active.

**Fix:**
- Verify `GET /api/system/hardware` detects your GPU
- Set `AETHERSTREAM_FFMPEG_HWACCEL` to the correct backend
- Reduce `AETHERSTREAM_FFMPEG_MAX_JOBS` if CPU is oversubscribed

---

### Slow library scan

**Symptom:** `POST /api/libraries/:id/scan` takes hours.

**Causes & Fixes:**

| Cause | Fix |
|-------|-----|
| Large library on slow disk (HDD) | Move to SSD or scan in batches |
| Metadata fetching slow (TMDb) | Check internet connection; TMDb API key |
| Many small files | Exclude non-media directories from the library path |

---

## Logs

AetherStream uses zerolog with Unix timestamps. To enable pretty-printed logs during development:

```bash
export AETHERSTREAM_LOG_PRETTY=true
./aetherstream
```

For JSON log parsing:
```bash
./aetherstream | jq -R 'fromjson?'
```

Log levels (default: info):
- `debug` — verbose (WebSocket messages, FFmpeg output)
- `info` — startup, requests, scans
- `warn` — recoverable issues (DLNA fail, missing GPU)
- `error` — crashes, DB errors

Set log level via environment variable:
```bash
export AETHERSTREAM_LOG_LEVEL=debug
```

---

## Getting Help

If an issue is not covered here:

1. Check logs: `docker logs aetherstream` or `journalctl -u aetherstream`
2. Verify config: `GET /system/info` and `GET /api/system/hardware`
3. Run tests: `go test ./...`
4. Open an issue with: version, OS, FFmpeg version, and relevant log snippets
