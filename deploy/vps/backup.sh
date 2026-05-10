#!/bin/bash
set -e

# AetherStream Backup Script
# Backs up SQLite DB + config to timestamped archive

DATA_DIR="${DATA_DIR:-/opt/aetherstream/data}"
BACKUP_DIR="${BACKUP_DIR:-/opt/aetherstream/backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"

echo "=== AetherStream Backup ==="
echo "Data: $DATA_DIR"
echo "Backup: $BACKUP_DIR"
echo "Retention: $RETENTION_DAYS days"
echo ""

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/aetherstream_backup_$TIMESTAMP.tar.gz"

echo "[1/3] Stopping container..."
docker stop aetherstream 2>/dev/null || true

echo "[2/3] Creating backup..."
tar -czf "$BACKUP_FILE" -C "$DATA_DIR" .

echo "[3/3] Starting container..."
docker start aetherstream 2>/dev/null || true

echo ""
echo "Backup created: $BACKUP_FILE"
echo "Size: $(du -h "$BACKUP_FILE" | cut -f1)"

# Cleanup old backups
echo "Cleaning up backups older than $RETENTION_DAYS days..."
find "$BACKUP_DIR" -name "aetherstream_backup_*.tar.gz" -mtime +$RETENTION_DAYS -delete

echo "Done."
