#!/bin/sh
set -eu

if [ "${1:-}" = "" ]; then
  echo "Usage: ./scripts/restore-postgres.sh /backups/mobilechat-YYYYMMDDTHHMMSSZ.dump" >&2
  exit 1
fi

BACKUP_FILE="$1"

if [ ! -f "$BACKUP_FILE" ]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

if [ -f "$BACKUP_FILE.sha256" ]; then
  sha256sum -c "$BACKUP_FILE.sha256"
fi

export PGPASSWORD="$POSTGRES_PASSWORD"
pg_restore \
  --host=postgres \
  --username="$POSTGRES_USER" \
  --dbname="$POSTGRES_DB" \
  --clean \
  --if-exists \
  --no-owner \
  "$BACKUP_FILE"

echo "restore completed from: $BACKUP_FILE"
