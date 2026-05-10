# ADR-009: Prometheus Metrics and pprof Profiling

## Status

Accepted

## Context

AetherStream needs observability for production deployments: request latency, error rates, resource usage, and runtime profiling for debugging performance issues.

## Decision

Expose **Prometheus metrics** on a separate HTTP server (`:9090` by default) and integrate **Go pprof** endpoints for runtime profiling.

## Rationale

- Prometheus is the de-facto standard for cloud-native monitoring.
- `github.com/prometheus/client_golang` is mature and integrates with Echo middleware.
- Separate metrics port avoids exposing profiling data on the public API port.
- pprof is built into Go; zero additional dependencies.

## Consequences

- Positive: Out-of-the-box Grafana dashboards possible.
- Positive: pprof helps diagnose memory leaks and goroutine leaks quickly.
- Negative: Metrics endpoint is unauthenticated by default (intended for internal network).
- Mitigation: Bind metrics to `127.0.0.1` or protect with reverse proxy auth.

## Alternatives Considered

- **StatsD / DataDog** — rejected to avoid vendor lock-in and external agents.
- **OpenTelemetry** — considered for future; Prometheus is the baseline.
- **Custom metrics endpoint** — rejected; Prometheus format is standard.
