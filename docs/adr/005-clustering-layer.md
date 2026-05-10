# ADR-005: Clustering via Custom Replication Layer

## Status

Proposed

## Context

As AetherStream may be deployed in multi-node environments (family homes with multiple servers, small ISPs), we need a clustering strategy for load balancing and failover.

## Decision

Build a **custom lightweight clustering layer** in `pkg/cluster` instead of using heavy orchestrators like Kubernetes as a hard dependency.

## Rationale

- Home and small-business users may not run Kubernetes.
- AetherStream should be able to cluster with minimal configuration (gossip-based discovery).
- The replication layer handles SQLite read-replicas and distributed locking.

## Consequences

- Positive: Works without Kubernetes; simple node addition.
- Positive: Custom load balancer can factor in GPU availability and bandwidth.
- Negative: More code to maintain compared to off-the-shelf solutions.
- Negative: Eventual consistency for SQLite replication; not suitable for strongly consistent multi-writer workloads.

## Alternatives Considered

- **Kubernetes + StatefulSets** — supported as a deployment target, but not a runtime dependency.
- **etcd / Consul** — rejected to avoid external coordination service.
- **CockroachDB / TiDB** — rejected to keep the single-binary deployment model.
