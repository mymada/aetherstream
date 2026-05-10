# ADR-004: JWT + bcrypt for Authentication

## Status

Accepted

## Context

AetherStream needs session authentication for REST API, WebSocket, and SwiftFlow captive portal integration.

## Decision

Use **JWT (JSON Web Tokens)** signed with HS256, plus **bcrypt** for password hashing.

## Rationale

- JWT is stateless; no session store required on the server.
- Easy to pass across WebSocket, HTTP headers, and SwiftFlow webhooks.
- `golang-jwt/jwt/v5` is well-maintained and supports modern JWT features.
- `bcrypt` is the standard for password hashing in Go.

## Consequences

- Positive: Stateless auth scales horizontally (no shared session store).
- Positive: Token can carry bandwidth/device claims from SwiftFlow.
- Negative: Token revocation is hard; we rely on short TTL (24h default) and session timeout middleware.
- Negative: Secret rotation requires all active tokens to expire.

## Alternatives Considered

- **OAuth2 only** — rejected; local deployments need username/password auth without external IdP.
- **Session cookies + Redis** — rejected to avoid external dependency; may be added later for admin revocation.
