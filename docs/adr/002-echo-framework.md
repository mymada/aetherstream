# ADR-002: Echo Framework for HTTP API

## Status

Accepted

## Context

We needed an HTTP router/middleware framework for the REST API, WebSocket, and static file serving.

## Decision

Use **Echo v4** (`labstack/echo/v4`) as the HTTP framework.

## Rationale

- Minimal API surface, high performance.
- Built-in middleware for CORS, rate limiting, logging, recovery.
- Easy JWT middleware integration.
- Good WebSocket support via `gorilla/websocket` with Echo context.

## Consequences

- Positive: Rapid development of secure endpoints with middleware chaining.
- Positive: Large community and stable API.
- Negative: Tied to Echo's context model; switching frameworks later would require refactoring handlers.

## Alternatives Considered

- **Gin** — similar performance, but Echo's middleware API was preferred.
- **stdlib net/http** — rejected due to repetitive middleware boilerplate.
- **FastHTTP** — rejected due to compatibility concerns with some libraries.
