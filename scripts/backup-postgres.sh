#!/bin/sh
set -eu

BACKUP_DIR="/backups"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
FILE="$BACKUP_DIR/mobilechat-$TIMESTAMP.dump"
SHA_FILE="$FILE.sha256"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

mkdir -p "$BACKUP_DIR"

export PGPASSWORD="$POSTGRES_PASSWORD"
pg_dump \
  --host=postgres \
  --username="$POSTGRES_USER" \
  --dbname="$POSTGRES_DB" \
  --format=custom \
  --no-owner \
  --file="$FILE"

sha256sum "$FILE" > "$SHA_FILE"
find "$BACKUP_DIR" -type f \( -name 'mobilechat-*.dump' -o -name 'mobilechat-*.dump.sha256' \) -mtime +"$RETENTION_DAYS" -delete

echo "backup created: $FILE"
