# ADR-001: Go + SQLite as Core Stack

## Status

Accepted

## Context

AetherStream is a rewrite of Jellyfin, originally a .NET/SQL Server application. We needed to choose a language and database for the new implementation.

## Decision

Use **Go** as the primary language and **SQLite** as the embedded database.

## Rationale

- **Go** provides static typing, fast compilation, excellent concurrency (goroutines), and a rich ecosystem for media tooling (FFmpeg bindings, HTTP servers).
- **SQLite** eliminates the need for a separate database server, simplifies deployment (single binary), and supports FTS5 for full-text search.
- CGO is required for `mattn/go-sqlite3`, but the build complexity is manageable with Docker multi-stage builds.

## Consequences

- Positive: Simple deployment, single binary, no external DB dependency.
- Positive: WAL mode gives reasonable concurrency for a media server workload.
- Negative: SQLite is not ideal for multi-writer horizontal scaling; clustering requires replication layer (see ADR-005).
- Negative: CGO complicates cross-compilation.

## Alternatives Considered

- **PostgreSQL** — rejected for operational complexity; SQLite is sufficient for home/media server scale.
- **Rust** — rejected due to longer development time and smaller team familiarity.
- **Node.js** — rejected due to runtime overhead and type safety concerns.
