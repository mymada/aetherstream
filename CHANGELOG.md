# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] — 2026-05-10

### Added
- Production-ready Docker multi-stage build with health checks
- GitHub Actions CI/CD workflow (build, test, lint)
- React Web UI with static file serving
- DLNA / UPnP server for network device discovery
- Live TV / DVR support with stream recording
- Plugin system for extensible integrations
- MusicBrainz metadata provider for music libraries
- Benchmark suite (`pkg/benchmark`)
- Prometheus metrics and pprof endpoints
- Secure store with AES-256-GCM encryption
- Audit log middleware
- Rate limiting and security headers middleware
- WebSocket realtime activity feed
- Adaptive bitrate streaming (`/videos/:id/adaptive.m3u8`)
- Collections and playlists management
- Subtitle extraction and serving
- Full-text search with SQLite FTS5
- Thumbnail generation service
- Session sync for multi-device playback
- Task scheduler for background jobs

### Changed
- Migrated license from Apache 2.0 to MIT
- Improved hardware acceleration auto-detection
- Enhanced path traversal protection

### Security
- Password hashing upgraded to bcrypt
- JWT secrets enforced via environment variable
- Secure store master key isolation

## [0.1.0] — 2026-05-09

### Added
- Initial project scaffold in Go
- Echo HTTP framework with structured logging (zerolog)
- SQLite database with migrations (`pkg/db`)
- JWT authentication and role-based access control (`pkg/auth`)
- YAML configuration with environment overrides (`pkg/config`)
- Library scanner with naming parser (`pkg/scanner`, `pkg/naming`)
- TMDb metadata fetcher (`pkg/metadata`)
- FFmpeg probe and encoder wrappers (`pkg/probe`, `pkg/encoder`)
- HLS playlist generator and stream server (`pkg/hls`, `pkg/stream`)
- Transcode job manager (`pkg/transcode`)
- Per-device encode profiles (`pkg/profiles`)
- SwiftFlow API client and bandwidth adapter (`pkg/swiftflow`, `pkg/bwadapter`)
- Basic REST API: system info, auth, users, libraries, items
- WebSocket handler stub (`pkg/ws`)
- Search stub (`pkg/search`)
- Image processing utilities (`pkg/images`)
- Captive portal integration stub (`pkg/captive`)
- DASH streaming stub (`pkg/dash`)

[Unreleased]: https://github.com/mymada/aetherstream/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/mymada/aetherstream/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/mymada/aetherstream/releases/tag/v0.1.0
