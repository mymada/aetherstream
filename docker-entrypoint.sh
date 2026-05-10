#!/bin/sh
set -e

# AetherStream entrypoint script
# Initializes database and creates admin user on first run

DATA_DIR="${DATA_DIR:-/data}"
MEDIA_DIR="${MEDIA_DIR:-/media}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin123}"

# Ensure directories exist with correct permissions
mkdir -p "$DATA_DIR" "$MEDIA_DIR"
chown -R aetherstream:aetherstream "$DATA_DIR" "$MEDIA_DIR" 2>/dev/null || true

# Check if database exists
if [ ! -f "$DATA_DIR/aetherstream.db" ]; then
    echo "=== AetherStream First Run ==="
    echo "Initializing database at $DATA_DIR/aetherstream.db"
    echo "Media directory: $MEDIA_DIR"
    echo "Admin user: $ADMIN_USER"
    echo ""
    echo "To change defaults, set env vars:"
    echo "  ADMIN_USER, ADMIN_PASS, DATA_DIR, MEDIA_DIR"
    echo ""
fi

# Run the server as aetherstream user (skip if already running as that user)
if [ "$(id -u)" = "1000" ]; then
    exec /app/aetherstream "$@"
else
    exec su-exec aetherstream /app/aetherstream "$@"
fi
