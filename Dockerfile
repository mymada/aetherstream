# syntax=docker/dockerfile:1
# AetherStream — Multi-stage Docker build
# Stage 1: Build (Go + CGO for SQLite)
FROM golang:1.25-alpine AS builder

# Install build dependencies: gcc, musl-dev for CGO/SQLite
RUN apk add --no-cache git ca-certificates tzdata build-base

WORKDIR /src

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with CGO enabled (required for mattn/go-sqlite3) + FTS5
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_CFLAGS="-DSQLITE_ENABLE_FTS5"
RUN go build -ldflags='-w -s -extldflags "-static"' -o /bin/aetherstream ./cmd/aetherstream

# Stage 2: Runtime (Alpine with FFmpeg)
FROM alpine:3.21 AS runtime

# Install runtime dependencies: FFmpeg + SSL certs + su-exec
RUN apk add --no-cache ffmpeg ca-certificates tzdata su-exec

# Create directories for data and media
RUN mkdir -p /data /media /app/web/static

# Create non-root user with same UID as host (for volume permissions)
RUN adduser -D -u 1000 aetherstream && \
    chown -R aetherstream:aetherstream /data /media /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/aetherstream /app/aetherstream

# Copy web assets if they exist
COPY --chown=aetherstream:aetherstream web/ /app/web/

# Copy entrypoint
COPY --chown=aetherstream:aetherstream docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# Don't switch to non-root user — let entrypoint handle permissions
# USER aetherstream

# Expose API (8080) and DLNA (8097)
EXPOSE 8080 8097

# Healthcheck on API /system/info
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/system/info || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/aetherstream"]
