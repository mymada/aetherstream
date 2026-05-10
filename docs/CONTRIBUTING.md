# AetherStream Contributing Guide

Thank you for contributing to AetherStream! This guide covers the development workflow, coding standards, and how to submit changes.

---

## Development Setup

### Prerequisites

- Go 1.25+
- FFmpeg 5.x + ffprobe
- SQLite 3.35+ (with FTS5 support for full-text search)
- Docker (optional, for integration tests)

### Clone and Build

```bash
git clone https://github.com/mymada/aetherstream.git
cd aetherstream

# Download dependencies
go mod download

# Build
CGO_ENABLED=1 go build -o aetherstream ./cmd/aetherstream

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

---

## Project Structure

```
aetherstream/
├── cmd/aetherstream/      # Main entry point
├── pkg/
│   ├── api/               # REST API controllers + middleware
│   ├── auth/              # JWT token management
│   ├── bwadapter/         # Bandwidth adaptation
│   ├── cache/             # LRU cache
│   ├── cast/              # AirPlay + Chromecast
│   ├── cluster/           # Distributed clustering
│   ├── config/            # YAML + env configuration
│   ├── dash/              # DASH streaming
│   ├── db/                # SQLite database + migrations
│   ├── dlna/              # UPnP/DLNA server
│   ├── docs/              # Swagger/OpenAPI registration
│   ├── encoder/           # FFmpeg command builder + profiles
│   ├── hls/               # HLS playlist generator
│   ├── library/           # Library CRUD + metadata fetch
│   ├── livetv/            # Live TV / DVR manager
│   ├── metrics/           # Prometheus metrics + pprof
│   ├── models/            # Domain structs (shared types)
│   ├── naming/            # Movie/TV/music filename parser
│   ├── oauth/             # OAuth provider integration
│   ├── profiles/          # Per-device encode profiles
│   ├── probe/             # ffprobe JSON parser
│   ├── scanner/           # File system library scanner
│   ├── search/            # SQLite FTS5 search
│   ├── securestore/       # AES-256-GCM encrypted store
│   ├── sessionsync/       # Cross-device session sync
│   ├── stream/            # HTTP streaming handlers
│   ├── swiftflow/         # SwiftFlow API client
│   ├── tasks/             # Background task runner
│   ├── thumbnail/         # Thumbnail generation
│   ├── transcode/         # Transcode job manager
│   ├── webrtc/            # WebRTC signaling
│   └── ws/                # WebSocket hub
├── web/                   # React frontend (built separately)
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

---

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Run `go fmt` and `go vet` before committing
- Use `golangci-lint` for comprehensive linting
- Keep functions focused; prefer small, testable units

### Naming

- Exported identifiers: `PascalCase`
- Unexported identifiers: `camelCase`
- Constants: `PascalCase` or `ALL_CAPS` for package-level
- Test files: `*_test.go`
- Fuzz tests: `fuzz_test.go`

### Error Handling

- Wrap errors with context: `fmt.Errorf("action: %w", err)`
- Do not swallow errors; log or return them
- Use `echo.NewHTTPError(status, message)` in HTTP handlers

### Security

- All user input must be validated
- File paths must be sanitized (`filepath.Clean`, prefix checks)
- Passwords must be hashed with `bcrypt`
- JWT secrets must be >= 32 characters
- Use `subtle.ConstantTimeCompare` for token comparisons

---

## Testing

### Unit Tests

```bash
go test ./...
```

### Race Detector

```bash
go test -race ./...
```

### Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Benchmarks

```bash
go test -bench=. ./pkg/benchmark
```

### Fuzzing

```bash
go test -fuzz=FuzzParseFilename ./pkg/naming
```

---

## Pull Request Workflow

1. **Fork** the repository
2. **Create a branch:** `git checkout -b feature/my-feature`
3. **Make changes** with clear, atomic commits
4. **Add tests** for new functionality
5. **Run the full test suite:** `go test ./...`
6. **Update documentation** if behavior changes
7. **Push** and open a Pull Request against `main`

### PR Checklist

- [ ] Tests pass (`go test ./...`)
- [ ] No race conditions (`go test -race ./...`)
- [ ] Lint passes (`golangci-lint run`)
- [ ] Security scan passes (`gosec ./...`)
- [ ] Documentation updated (README, API.md, CONFIG.md, etc.)
- [ ] Commit messages are descriptive

---

## Commit Message Format

```
type(scope): subject

body (optional)

footer (optional)
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `security`

Examples:
```
feat(api): add OAuth callback handler
fix(stream): prevent path traversal in segment serving
docs(readme): update Docker instructions
security(auth): enforce minimum password length
```

---

## Code Review

All PRs require at least one review before merging. Reviewers check for:

- Correctness and edge cases
- Test coverage
- Performance implications
- Security considerations
- Documentation completeness

---

## Release Process

1. Update version in `pkg/api/items.go` (`handleSystemInfo`)
2. Update version strings in `docs/API.md`
3. Update `CHANGELOG.md` (if present)
4. Update `README.md` version badge and feature list if needed
5. Tag: `git tag -a v1.x.y -m "Release v1.x.y"`
6. Push tag: `git push origin v1.x.y`
7. CI builds artifacts and Docker image

---

## Areas Needing Help

- **WebRTC playback** — improve signaling stability
- **DASH streaming** — complete DASH manifest generation
- **Plugin system** — external plugin sandboxing
- **Clustering** — replication consistency improvements
- **Live TV** — EPG parsing and scheduling UI
- **Documentation** — translations and deployment guides

---

## Community

- Issues: https://github.com/mymada/aetherstream/issues
- Discussions: https://github.com/mymada/aetherstream/discussions

Be respectful, constructive, and patient. We review PRs as quickly as possible.
