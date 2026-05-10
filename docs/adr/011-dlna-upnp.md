# ADR-011: DLNA/UPnP for Network Discovery

## Status

Accepted

## Context

Many Smart TVs, game consoles, and media players (Roku, Xbox, PS5, Samsung TVs) support DLNA/UPnP for network media discovery and direct play. AetherStream needs to integrate with these devices without requiring custom client apps.

## Decision

Implement a **DLNA/UPnP server** in `pkg/dlna` that runs automatically on `server.port + 1` (default `8097`).

## Rationale

- DLNA is the most widely supported network media protocol for consumer electronics.
- No client app installation required — devices auto-discover AetherStream on the local network.
- Direct play avoids transcoding overhead for compatible formats.
- Go has mature SSDP and UPnP libraries; implementation is lightweight.

## Consequences

- Positive: Instant compatibility with thousands of existing devices.
- Positive: Reduces server load when direct play is possible.
- Negative: UPnP protocol is verbose and aging; some modern devices prefer native apps.
- Negative: SSDP multicast may be blocked by router firewalls or VLANs.
- Mitigation: Document firewall requirements in DEPLOYMENT.md.

## Alternatives Considered

- **AirPlay 2 / Chromecast only** — insufficient; many TVs do not support these.
- **Custom native apps** — high barrier for users; DLNA works out-of-the-box.
- **mDNS/Bonjour** — complementary, not a replacement for DLNA media serving.
