# AetherStream

A modern media server rewritten from Jellyfin in Go, optimized for adaptive transcoding, WiFi captive portal integration via SwiftFlow, and low-latency WebRTC streaming.

**Version:** v1.3.0  
**Packages:** 57 | **Production LoC:** ~15,482  
**Repository:** https://github.com/mymada/aetherstream

---

## Features

- **Media Library Management** — Scan, organize, and browse movies, TV shows, and music with automatic metadata fetching (TMDb, MusicBrainz)
- **Adaptive Streaming** — HLS/DASH with per-device encode profiles and bandwidth adaptation
- **Real-time Transcoding** — FFmpeg orchestration with hardware acceleration support (VAAPI, NVENC, QSV, AMF)
- **SwiftFlow Integration** — QoS-aware streaming and captive portal authentication for WiFi deployments
- **Web UI** — React-based responsive interface served from `/app/web`
- **WebSocket Realtime** — Live activity feed and session sync
- **DLNA / UPnP** — Network discovery and direct play for Smart TVs, consoles, and media players
- **Live TV / DVR** — Stream and record live television via M3U/XMLTV
- **Plugin System** — Extensible external-process plugin architecture (JSON-RPC)
- **Collections & Playlists** — User-managed media groupings with smart playlists
- **Subtitles** — Automatic extraction, on-the-fly serving, and chapter support
- **Full-text Search** — SQLite FTS5 powered search across library items
- **Secure Authentication** — JWT tokens, bcrypt password hashing, role-based access control
- **Secure Store** — AES-256-GCM encrypted secret storage
- **Prometheus Metrics** — Built-in `/metrics` and pprof endpoints
- **WebRTC** — Low-latency streaming for live and real-time use cases
- **Chromecast / AirPlay** — Cast playback to external devices
- **Docker Ready** — Multi-stage build with health checks
- **Clustering** — Lightweight gossip-based node discovery for multi-server deployments

---

## Installation

### Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/mymada/aetherstream.git
cd aetherstream

# Build and start with Docker Compose
docker compose up --build -d
```

The server will be available at `http://localhost:8080`.

**Required environment variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `AETHERSTREAM_AUTH_SECRET` | JWT signing secret (change in production!) | — |
| `AETHERSTREAM_MASTER_KEY` | AES-256-GCM master key for secure store | — |
| `AETHERSTREAM_DATABASE_PATH` | SQLite database file path | `./data/aetherstream.db` |
| `AETHERSTREAM_SERVER_PORT` | HTTP API port | `8096` |
| `AETHERSTREAM_SERVER_HOST` | Bind address | `0.0.0.0` |
| `AETHERSTREAM_FFMPEG_PATH` | FFmpeg binary path | `ffmpeg` |
| `AETHERSTREAM_FFMPEG_MAX_JOBS` | Max concurrent transcodes | `4` |
| `AETHERSTREAM_FFMPEG_HWACCEL` | Hardware acceleration mode | `auto` |
| `AETHERSTREAM_SWIFTFLOW_URL` | SwiftFlow API base URL | — |
| `AETHERSTREAM_SWIFTFLOW_KEY` | SwiftFlow API key | — |
| `AETHERSTREAM_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `AETHERSTREAM_LOG_PRETTY` | Pretty-print logs (human-readable) | `false` |

### Binary

**Prerequisites:** Go 1.25+, FFmpeg, ffprobe

```bash
# Clone
git clone https://github.com/mymada/aetherstream.git
cd aetherstream

# Build (CGO required for SQLite)
CGO_ENABLED=1 go build -ldflags='-w -s' -o aetherstream ./cmd/aetherstream

# Run
./aetherstream
```

Server starts on `:8096` by default.

---

## Configuration

Configuration is loaded in this priority order:

1. **Defaults** (see `pkg/config/config.go`)
2. **YAML file** — `config.yaml` in working directory
3. **Environment variables** — prefixed with `AETHERSTREAM_`

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

---

## API Endpoints

### System

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/system/info` | No | Server version and health status |
| GET | `/metrics` | No | Prometheus metrics |
| GET | `/debug/pprof` | No | Go runtime profiles |

### Authentication

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/auth/login` | No | Username/password login, returns JWT |
| POST | `/auth/callback` | No | OAuth callback placeholder |

### Users

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/users` | Yes | List all users |
| GET | `/api/users/:id` | Yes | Get user by ID |
| POST | `/api/users` | Admin | Create user |
| PUT | `/api/users/:id` | Admin | Update user role |
| DELETE | `/api/users/:id` | Admin | Delete user |

### Libraries

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/libraries` | Yes | List libraries |
| POST | `/api/libraries` | Admin | Create library |
| POST | `/api/libraries/:id/scan` | Admin | Trigger library scan |

### Items

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/items` | Yes | List items (query: `library_id`) |
| GET | `/api/items/:id` | Yes | Get item details |
| GET | `/api/items/:id/subtitles` | Yes | List subtitle tracks |
| GET | `/api/items/:id/subtitles/:lang` | Yes | Download subtitle file |
| GET | `/api/items/:id/thumbnails/:type` | Yes | Get thumbnail image |

### Search

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/search?q=...&type=...&limit=...` | Yes | Full-text search |

### Collections & Playlists

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/collections` | Yes | List user's collections |
| POST | `/api/collections` | Yes | Create collection/playlist |
| GET | `/api/collections/:id` | Yes | Get collection with items |
| POST | `/api/collections/:id/items` | Yes | Add item to collection |
| DELETE | `/api/collections/:id/items/:item_id` | Yes | Remove item from collection |

### Activity

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/activity` | Yes | Recent activity log |
| GET | `/api/session` | Yes | Current session info |

### Streaming

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/videos/:id/stream` | Yes | Direct play (original file) |
| GET | `/videos/:id/hls/master.m3u8` | Yes | HLS master playlist |
| GET | `/videos/:id/hls/:profile/playlist.m3u8` | Yes | HLS variant playlist |
| GET | `/videos/:id/hls/:profile/:segment` | Yes | HLS `.ts` segment |
| GET | `/videos/:id/probe` | Yes | ffprobe JSON metadata |
| GET | `/videos/:id/adaptive.m3u8` | Yes | Adaptive bitrate playlist |
| GET | `/system/hwaccel` | Yes | Hardware acceleration status |

### WebSocket

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/ws` | Yes | Realtime activity WebSocket |

### Webhooks

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/webhooks/swiftflow` | No | SwiftFlow captive portal callback |

---

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Build for production
CGO_ENABLED=1 go build -ldflags='-w -s -extldflags "-static"' -o aetherstream ./cmd/aetherstream

# Lint
go vet ./...
```

### Project Structure

| Package | Responsibility |
|---------|---------------|
| `pkg/api` | REST API controllers + middleware |
| `pkg/auth` | JWT token management |
| `pkg/bwadapter` | Bandwidth adaptation |
| `pkg/cache` | LRU cache |
| `pkg/cast` | AirPlay + Chromecast |
| `pkg/cluster` | Distributed clustering |
| `pkg/config` | YAML + env configuration |
| `pkg/dash` | DASH streaming |
| `pkg/db` | SQLite database + migrations |
| `pkg/dlna` | UPnP/DLNA server |
| `pkg/encoder` | FFmpeg command builder + profiles |
| `pkg/hls` | HLS playlist generator |
| `pkg/library` | Library CRUD + metadata fetch |
| `pkg/livetv` | Live TV / DVR manager |
| `pkg/metadata` | TMDb, MusicBrainz clients |
| `pkg/metrics` | Prometheus metrics + pprof |
| `pkg/models` | Domain structs |
| `pkg/naming` | Movie/TV/music naming parser |
| `pkg/probe` | FFmpeg ffprobe parser |
| `pkg/profiles` | Per-device encode profiles |
| `pkg/scanner` | File system library scanner |
| `pkg/search` | SQLite FTS5 search |
| `pkg/securestore` | AES-256-GCM encrypted store |
| `pkg/stream` | HTTP streaming handlers |
| `pkg/swiftflow` | SwiftFlow API client |
| `pkg/tasks` | Background task runner |
| `pkg/thumbnail` | Thumbnail generation |
| `pkg/transcode` | Transcode job manager |
| `pkg/webrtc` | WebRTC signaling |
| `pkg/ws` | WebSocket hub |

---

## Documentation

| Document | Description |
|----------|-------------|
| [API Reference](docs/API.md) | Complete endpoint reference, request/response schemas, auth, rate limits |
| [Deployment Guide](docs/DEPLOYMENT.md) | Docker, systemd, Kubernetes, reverse proxy, GPU setup |
| [Configuration Reference](docs/CONFIG.md) | Environment variables, YAML options, security checklist |
| [Troubleshooting Guide](docs/TROUBLESHOOTING.md) | Common issues, symptoms, and fixes |
| [Contributing Guide](docs/CONTRIBUTING.md) | Development setup, coding standards, PR workflow |
| [Architecture Decision Records](docs/adr/) | ADRs for key design choices |

---

## License

MIT License — see [LICENSE](LICENSE) for details.
