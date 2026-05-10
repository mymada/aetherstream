# ADR-012: React Web UI with Vite Build

## Status

Accepted

## Context

AetherStream needs a modern, responsive web interface for browsing libraries, managing collections, and controlling playback. The UI must be lightweight, fast to build, and easy to maintain.

## Decision

Build the web UI with **React 18**, bundled with **Vite**, served from `/app` by the Go backend.

## Rationale

- React is the most widely used frontend framework; large ecosystem of media player and UI components.
- Vite provides fast HMR and optimized production builds (ESM + rollup).
- Serving from `/app` avoids conflicts with REST API routes (`/api/*`).
- Static build output is embedded in the Docker image and served by Echo's static file middleware.

## Consequences

- Positive: Modern component-based architecture, reusable UI elements.
- Positive: Fast development cycle with Vite HMR.
- Negative: Requires Node.js toolchain for building (not at runtime).
- Negative: SPA routing needs `BrowserRouter` with `basename=/app` to work behind reverse proxy.
- Mitigation: Build is done in Docker multi-stage; runtime is pure Go binary.

## Alternatives Considered

- **Vue / Svelte** — both excellent; React chosen for broader contributor familiarity.
- **HTMX + server-rendered HTML** — considered for simplicity but rejected for rich media UI needs.
- **Next.js** — overkill for a self-hosted media server; no SSR needed.
