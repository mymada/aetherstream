# ADR-013: Live TV / DVR with M3U and EPG

## Status

Accepted

## Context

Users want to stream and record live television through AetherStream, similar to Jellyfin's Live TV/DVR capabilities. This requires channel management, EPG (Electronic Program Guide) parsing, and scheduled recording.

## Decision

Implement **Live TV / DVR** in `pkg/livetv` supporting **M3U playlist** channel sources and **XMLTV EPG** parsing.

## Rationale

- M3U is the universal standard for IPTV channel lists; widely supported by providers.
- XMLTV is the standard format for EPG data; parsers are well-understood.
- Go's `encoding/xml` handles XMLTV without external dependencies.
- Recording uses the existing FFmpeg orchestration layer (`pkg/transcode`).

## Consequences

- Positive: Full Jellyfin parity for Live TV use case.
- Positive: Reuses existing transcoding and streaming infrastructure.
- Negative: EPG parsing can be brittle with provider-specific XML quirks.
- Negative: Scheduled recording requires a background task runner (`pkg/tasks`).
- Mitigation: Defensive XML parsing with fallback for missing fields.

## Alternatives Considered

- **HDHomeRun native API** — considered as a specific tuner integration, not a replacement for M3U.
- **DVB-C/T/S direct tuning** — rejected due to hardware diversity and OS-specific drivers.
- **Commercial EPG APIs** — may be added as optional metadata enrichment.
