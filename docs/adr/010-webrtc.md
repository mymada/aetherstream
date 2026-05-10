# ADR-010: WebRTC for Low-Latency Playback

## Status

Accepted

## Context

HLS/DASH streaming introduces 10–30 seconds of latency due to segment buffering. Some use cases (live TV, real-time monitoring) need sub-second latency.

## Decision

Add **WebRTC** support in `pkg/webrtc` for low-latency media playback, using the **Pion** WebRTC stack.

## Rationale

- Pion (`github.com/pion/webrtc`) is a pure-Go WebRTC implementation — no CGO, no external binaries.
- WebRTC supports UDP-based transport, NAT traversal (ICE/STUN/TURN), and browser-native playback without plugins.
- Complements HLS/DASH rather than replacing them; clients can choose based on latency needs.

## Consequences

- Positive: Sub-second latency for live streams and real-time scenarios.
- Positive: Browser support without custom players.
- Negative: WebRTC signaling is more complex than HTTP streaming.
- Negative: UDP may be blocked by some corporate firewalls; fallback to HLS/DASH required.
- Negative: Higher CPU usage for encoding multiple simulcast layers.

## Alternatives Considered

- **RTMP** — rejected; Flash-dependent, declining browser support.
- **SRT (Secure Reliable Transport)** — considered for future server-to-server use.
- **WHIP/WHEP** — may be adopted later for standardized WebRTC ingest/egress.
