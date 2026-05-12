# Changelog

## [1.0.0] - 2026-05-12

### Added
- **Direct Play** — Streaming adaptatif par User-Agent (codec detection)
- **Heartbeat** — Ping sessions HTTP `/api/sessions/:id/ping`
- **Transcode Lifecycle** — Cleanup auto 24h, graceful shutdown, queue
- **PostgreSQL** — Support production via lib/pq, DSN env vars
- **Health Checks** — `/health` (DB+FFmpeg), `/ready` (DB ping)
- **Chapters API** — `/items/:id/chapters`, `/items/:id/chapters/at`, scan
- **Trickplay** — Vignettes WebVTT pour seekbar
- **Continue Watching** — Playback position persistée
- **Speed Control** — 0.5x à 2x dans le player
- **Admin UI** — Dashboard, Users CRUD, Libraries CRUD, Activity logs
- **Métadonnées** — TVDb client, collections auto (genre/décennie)
- **Sécurité** — CSRF, Security Headers, Secure Cookies, Brute Force, Rate Limiting, Session Timeout, CORS, 2FA TOTP (structure), Audit logs

### Changed
- Frontend React — 25 composants, 4,230 LOC
- Backend Go — 163 fichiers, 28,025 LOC
- Couverture tests — 46.5%

### Fixed
- 50 commits depuis le début du projet
- Tous les blockers P0 des audits SF résolus

## [0.2.0] - 2026-05-01

### Added
- Core streaming HLS/DASH
- Auth JWT + sessions
- Library management
- Search + collections
- Web UI React
- Mobile API
- WebSocket playback
- TV/DLNA/Chromecast
