# ADR-006: Plugin System with External Process Sandbox

## Status

Accepted

## Context

Users want custom integrations (metadata providers, notification channels, custom transcode pipelines). We need an extension mechanism that does not compromise server stability.

## Decision

Use an **external plugin system** where plugins run as separate processes, communicating via stdin/stdout JSON-RPC or HTTP callbacks.

## Rationale

- Isolates plugin crashes from the main server.
- Plugins can be written in any language.
- JSON-RPC is simple and language-agnostic.
- Registry + event bus in `pkg/plugin` manages lifecycle.

## Consequences

- Positive: Server stability protected from faulty plugins.
- Positive: Community can contribute plugins without learning Go.
- Negative: Higher latency for plugin-involved operations.
- Negative: Process management overhead.

## Alternatives Considered

- **Go plugins (plugin package)** — rejected due to platform limitations (Linux-only, fragile).
- **WASM** — considered for future; may complement external process model.
- **Embedded scripting (Lua/JS)** — rejected due to sandboxing complexity.
