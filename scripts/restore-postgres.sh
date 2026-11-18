#!/bin/sh
# Restore a PostgreSQL backup made by backup-postgres.sh INTO THE CONFIGURED
# DATABASE, dropping existing objects first (pg_restore --clean). This is
# destructive. Stop the API first and take a fresh backup.
#
# Run inside the backup service (it has pg_restore and age), for example:
#   docker compose -f docker-compose.prod.yml --env-file .env.prod stop api
#   docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm \
#     -e RESTORE_CONFIRM=<POSTGRES_DB value> \
#     -e BACKUP_AGE_IDENTITY_FILE=/run/koom-age-identity \
#     -v /root/koom-age-identity.txt:/run/koom-age-identity:ro \
#     backup /scripts/restore-postgres.sh /backups/<prefix>-<timestamp>.dump.age
#
# Environment:
#   RESTORE_CONFIRM           required; must equal the target database name
#   RESTORE_DB                target database (default: $POSTGRES_DB)
#   RESTORE_PGHOST            target host (default: postgres)
#   BACKUP_AGE_IDENTITY_FILE  age identity (secret key) file for *.age inputs
#   RESTORE_SKIP_CHECKSUM=1   restore even if no .sha256 file is present
set -eu
umask 077

usage() {
  echo "Usage: RESTORE_CONFIRM=<db-name> $0 /backups/<prefix>-YYYYMMDDTHHMMSSZ.dump[.age]" >&2
  exit 1
}

[ "${1:-}" != "" ] || usage
BACKUP_FILE="$1"
TARGET_DB="${RESTORE_DB:-${POSTGRES_DB:-}}"
TARGET_HOST="${RESTORE_PGHOST:-postgres}"

if [ -z "$TARGET_DB" ]; then
  echo "Target database is unknown; set RESTORE_DB or POSTGRES_DB." >&2
  exit 1
fi
if [ ! -f "$BACKUP_FILE" ]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi
if [ "${RESTORE_CONFIRM:-}" != "$TARGET_DB" ]; then
  echo "Refusing to restore: this DROPS and replaces every object in database '$TARGET_DB' on host '$TARGET_HOST'." >&2
  echo "Re-run with RESTORE_CONFIRM=$TARGET_DB to confirm." >&2
  exit 2
fi

# Verify the checksum of the file as stored (encrypted or not). Only the hash
# field is compared, so both absolute and relative sha256 file formats work.
if [ -f "$BACKUP_FILE.sha256" ]; then
  expected="$(awk 'NR==1 {print $1}' "$BACKUP_FILE.sha256")"
  actual="$(sha256sum "$BACKUP_FILE" | awk '{print $1}')"
  if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    echo "Checksum mismatch for $BACKUP_FILE" >&2
    exit 1
  fi
  echo "checksum OK: $BACKUP_FILE"
elif [ "${RESTORE_SKIP_CHECKSUM:-}" != "1" ]; then
  echo "No $BACKUP_FILE.sha256 found; set RESTORE_SKIP_CHECKSUM=1 to restore anyway." >&2
  exit 1
fi

WORK_DIR=""
cleanup() {
  [ -z "$WORK_DIR" ] || rm -rf "$WORK_DIR"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

RESTORE_INPUT="$BACKUP_FILE"
case "$BACKUP_FILE" in
  *.age)
    if ! command -v age >/dev/null 2>&1; then
      echo "The backup is age-encrypted but 'age' is not installed; rebuild the backup image." >&2
      exit 1
    fi
    if [ -z "${BACKUP_AGE_IDENTITY_FILE:-}" ] || [ ! -r "$BACKUP_AGE_IDENTITY_FILE" ]; then
      echo "Set BACKUP_AGE_IDENTITY_FILE to a readable age identity file to decrypt $BACKUP_FILE." >&2
      exit 1
    fi
    WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/koom-restore.XXXXXX")"
    RESTORE_INPUT="$WORK_DIR/restore.dump"
    age --decrypt -i "$BACKUP_AGE_IDENTITY_FILE" -o "$RESTORE_INPUT" "$BACKUP_FILE"
    ;;
esac

# Make sure the archive is readable before touching the database.
pg_restore --list "$RESTORE_INPUT" >/dev/null

export PGPASSWORD="$POSTGRES_PASSWORD"
pg_restore \
  --host="$TARGET_HOST" \
  --username="$POSTGRES_USER" \
  --dbname="$TARGET_DB" \
  --clean \
  --if-exists \
  --no-owner \
  "$RESTORE_INPUT"
unset PGPASSWORD

echo "restore completed into database '$TARGET_DB' from: $BACKUP_FILE"
