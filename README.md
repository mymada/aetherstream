# AetherStream

A modern media server rewritten from Jellyfin in Go, optimized for adaptive transcoding and WiFi captive portal integration via SwiftFlow.

## Architecture

```
SwiftFlow (WiFi/Captive Portal) → AetherStream (Media Server)
                                    ├── HTTP API (Echo)
                                    ├── Library Engine (scanner, naming, metadata)
                                    ├── Transcode Manager (FFmpeg orchestration)
                                    ├── HLS/DASH Stream Server
                                    └── SwiftFlow Integration (QoS, bandwidth adapter)
```

## Quick Start

```bash
cd /home/devuser/dev/aetherstream
go build ./...
./aetherstream
```

Server starts on `:8096`.

## Project Structure

| Package | Responsibility |
|---------|---------------|
| `pkg/api` | REST API controllers |
| `pkg/auth` | JWT token management |
| `pkg/config` | YAML configuration |
| `pkg/db` | SQLite database + migrations |
| `pkg/models` | Domain structs |
| `pkg/scanner` | File system library scanner |
| `pkg/naming` | Movie/TV/music naming parser |
| `pkg/library` | Library CRUD |
| `pkg/metadata` | TMDb, MusicBrainz clients |
| `pkg/probe` | FFmpeg ffprobe parser |
| `pkg/encoder` | FFmpeg command builder |
| `pkg/transcode` | Transcode job manager |
| `pkg/hls` | HLS playlist generator |
| `pkg/stream` | HTTP streaming handlers |
| `pkg/profiles` | Per-device encode profiles |
| `pkg/swiftflow` | SwiftFlow API client |
| `pkg/bwadapter` | Bandwidth adaptation |

## Design

See `aetherstream_DESIGN.md` for full architecture document.

## Health Check

See `HEALTH_CHECK.md` for latest quality assessment.

## License

Apache 2.0
