# AetherStream Configuration Reference

Configuration is loaded in this priority order (highest wins):

1. **Defaults** (compiled into `pkg/config/config.go`)
2. **YAML file** — `config.yaml` in working directory
3. **Environment variables** — prefixed with `AETHERSTREAM_`

---

## YAML Configuration

Example `config.yaml`:

```yaml
server:
  port: 8096
  host: 0.0.0.0
  static_path: ./web/static

database:
  path: ./data/aetherstream.db

auth:
  secret: "change-me-in-production"
  token_ttl_hours: 24

ffmpeg:
  path: ffmpeg
  probe_path: ffprobe
  max_jobs: 4
  hwaccel: auto

swiftflow:
  base_url: ""
  api_key: ""
  webhook_secret: ""

logging:
  level: info
  pretty: false

metrics:
  port: 9090
  enabled: true
```

### Server

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.port` | int | `8096` | HTTP API port |
| `server.host` | string | `0.0.0.0` | Bind address |
| `server.static_path` | string | `./web/static` | Static files directory |

### Database

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `database.path` | string | `./data/aetherstream.db` | SQLite file path |

SQLite is opened with WAL mode (`_journal_mode=WAL`) and a 5-second busy timeout.

### Authentication

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `auth.secret` | string | `AETHERSTREAM_AUTH_SECRET` env | JWT signing secret (min 32 chars) |
| `auth.token_ttl_hours` | int | `24` | JWT expiration time |

### FFmpeg

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ffmpeg.path` | string | `ffmpeg` | FFmpeg binary path |
| `ffmpeg.probe_path` | string | `ffprobe` | ffprobe binary path |
| `ffmpeg.max_jobs` | int | `4` | Max concurrent transcode jobs |
| `ffmpeg.hwaccel` | string | `auto` | Hardware acceleration mode |

#### HWAccel modes

| Value | Behavior |
|-------|----------|
| `auto` | Detect best available (nvenc > qsv > vaapi > none) |
| `none` | Software encoding only |
| `vaapi` | Intel/AMD VAAPI |
| `nvenc` | NVIDIA NVENC |
| `qsv` | Intel QuickSync |
| `amf` | AMD AMF (planned) |

### SwiftFlow

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `swiftflow.base_url` | string | — | SwiftFlow API base URL |
| `swiftflow.api_key` | string | — | SwiftFlow API key |
| `swiftflow.webhook_secret` | string | — | Webhook signature validation secret |

### Logging

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `logging.pretty` | bool | `false` | Pretty-print logs (human-readable) |

---

## Environment Variables

All environment variables are prefixed with `AETHERSTREAM_`.

| Variable | Maps to | Required | Default |
|----------|---------|----------|---------|
| `AETHERSTREAM_AUTH_SECRET` | `auth.secret` | Yes | — |
| `AETHERSTREAM_AUTH_TOKEN_TTL` | `auth.token_ttl_hours` | No | `24` |
| `AETHERSTREAM_DATABASE_PATH` | `database.path` | No | `./data/aetherstream.db` |
| `AETHERSTREAM_SERVER_PORT` | `server.port` | No | `8096` |
| `AETHERSTREAM_SERVER_HOST` | `server.host` | No | `0.0.0.0` |
| `AETHERSTREAM_FFMPEG_PATH` | `ffmpeg.path` | No | `ffmpeg` |
| `AETHERSTREAM_FFMPEG_PROBE_PATH` | `ffmpeg.probe_path` | No | `ffprobe` |
| `AETHERSTREAM_FFMPEG_MAX_JOBS` | `ffmpeg.max_jobs` | No | `4` |
| `AETHERSTREAM_FFMPEG_HWACCEL` | `ffmpeg.hwaccel` | No | `auto` |
| `AETHERSTREAM_SWIFTFLOW_URL` | `swiftflow.base_url` | No | — |
| `AETHERSTREAM_SWIFTFLOW_KEY` | `swiftflow.api_key` | No | — |
| `AETHERSTREAM_SWIFTFLOW_WEBHOOK_SECRET` | `swiftflow.webhook_secret` | No | — |
| `AETHERSTREAM_MASTER_KEY` | secure store key | No | — |
| `AETHERSTREAM_LOG_LEVEL` | `logging.level` | No | `info` |
| `AETHERSTREAM_LOG_PRETTY` | `logging.pretty` | No | `false` |
| `AETHERSTREAM_METRICS_PORT` | `metrics.port` | No | `9090` |
| `AETHERSTREAM_METRICS_ENABLED` | `metrics.enabled` | No | `true` |

---

## Command-Line Flags

AetherStream currently does not expose CLI flags. All configuration is via YAML or environment variables.

To run with a custom config file, set the working directory or symlink `config.yaml`:

```bash
./aetherstream   # loads ./config.yaml
```

To use a custom config file path, symlink it into the working directory:

```bash
ln -s /etc/aetherstream/config.yaml ./config.yaml
./aetherstream
```

---

## Security Checklist

Before deploying to production:

- [ ] Change `auth.secret` to a random 32+ character string
- [ ] Set `AETHERSTREAM_MASTER_KEY` for encrypted secure store
- [ ] Run behind HTTPS (reverse proxy or AutoTLS)
- [ ] Restrict `server.host` to `127.0.0.1` if using reverse proxy
- [ ] Enable firewall rules for ports 8096 and 9090 (metrics)
- [ ] Rotate secrets periodically

---

## Metrics Port

Prometheus metrics are exposed on a separate HTTP server:

| Endpoint | Port | Path |
|----------|------|------|
| Metrics | `9090` | `/metrics` |

The metrics server bind address follows `server.host`:
- If `host` is `0.0.0.0`, metrics listen on `:9090`
- Otherwise, metrics listen on `<host>:9090`

---

## DLNA Port

The DLNA/UPnP server starts automatically on `server.port + 1`:

| Server | Port (default) |
|--------|----------------|
| HTTP API | `8096` |
| DLNA | `8097` |
| Metrics | `9090` |

---

## Database Tuning

SQLite is configured with:

- **WAL mode** — better concurrency for reads
- **Busy timeout 5000ms** — waits instead of returning `SQLITE_BUSY`
- **Max open connections = 1** — SQLite single-writer safety

For high-traffic deployments, consider:
- Running on fast SSD/NVMe
- Separating transcode temp directory from database directory
- Monitoring `database_path` disk usage (transcodes + thumbnails can grow large)
