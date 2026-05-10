# ADR-003: FFmpeg as External Transcoding Engine

## Status

Accepted

## Context

AetherStream needs real-time transcoding for adaptive streaming (HLS/DASH). We evaluated embedding a transcoder vs. shelling out to FFmpeg.

## Decision

Use **FFmpeg as an external binary**, orchestrated via Go `os/exec`.

## Rationale

- FFmpeg is the industry standard; supports virtually all codecs and hardware accelerations (NVENC, QSV, VAAPI, AMF).
- Re-implementing transcoding in Go would be error-prone and incomplete.
- FFmpeg profiles can be adjusted without recompiling AetherStream.

## Consequences

- Positive: Full codec and hardware acceleration support.
- Positive: Profile changes are configuration-only.
- Negative: Process management complexity (zombie processes, resource limits).
- Negative: Requires FFmpeg installed on host or in container image.
- Mitigation: Job manager limits concurrent transcodes; Docker image includes FFmpeg.

## Alternatives Considered

- **Go-native transcoding** — rejected; no mature Go library matches FFmpeg's coverage.
- **GStreamer** — considered but rejected due to higher operational complexity.
