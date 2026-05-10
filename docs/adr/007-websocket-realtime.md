# ADR-007: WebSocket for Real-Time Activity Feed

## Status

Accepted

## Context

AetherStream needs real-time updates for playback progress, session sync, and live activity feed. We evaluated Server-Sent Events (SSE) vs WebSocket.

## Decision

Use **WebSocket** (`gorilla/websocket`) for bidirectional real-time communication.

## Rationale

- Bidirectional: clients can send playback progress and heartbeats.
- Single connection per client, efficient for frequent updates.
- Echo framework integrates cleanly with `gorilla/websocket`.

## Consequences

- Positive: Low-latency bidirectional messaging.
- Positive: Broadcast and user-targeted messaging implemented in `pkg/ws`.
- Negative: More complex than SSE (connection management, ping/pong).
- Negative: Reverse proxies must support WebSocket upgrade.

## Alternatives Considered

- **SSE** — rejected because we need client-to-server messages (playback progress).
- **gRPC streaming** — rejected for browser compatibility and complexity.
