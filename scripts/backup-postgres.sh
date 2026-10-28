#!/bin/sh
set -eu
umask 077

BACKUP_DIR="/backups"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_PREFIX="${BACKUP_PREFIX:-koom}"
FILE="$BACKUP_DIR/$BACKUP_PREFIX-$TIMESTAMP.dump"
SHA_FILE="$FILE.sha256"
MEDIA_FILE="$BACKUP_DIR/$BACKUP_PREFIX-files-$TIMESTAMP.tar.gz"
MEDIA_SHA_FILE="$MEDIA_FILE.sha256"
SUCCESS_FILE="$BACKUP_DIR/backup-success"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_S3_URI="${BACKUP_S3_URI:-}"

case "$BACKUP_PREFIX" in
  ''|[!a-zA-Z0-9]*|*[!a-zA-Z0-9_-]*) echo "BACKUP_PREFIX contains unsupported characters" >&2; exit 1 ;;
esac
case "$RETENTION_DAYS" in
  ''|*[!0-9]*) echo "BACKUP_RETENTION_DAYS must be an integer" >&2; exit 1 ;;
esac
if [ "${#RETENTION_DAYS}" -gt 4 ] || [ "$RETENTION_DAYS" -lt 1 ] || [ "$RETENTION_DAYS" -gt 3650 ]; then
  echo "BACKUP_RETENTION_DAYS must be between 1 and 3650" >&2
  exit 1
fi
if [ -e "$FILE" ] || [ -e "$SHA_FILE" ] || [ -e "$MEDIA_FILE" ] || [ -e "$MEDIA_SHA_FILE" ]; then
  echo "backup for this timestamp already exists" >&2
  exit 1
fi
if [ ! -d /app-data ]; then
  echo "uploaded-file backup directory is unavailable" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR" || true

TMP_FILE=""
TMP_SHA_FILE=""
TMP_MEDIA_FILE=""
TMP_MEDIA_SHA_FILE=""
TMP_SUCCESS_FILE=""
RCLONE_CONFIG_FILE=""
cleanup() {
  [ -z "$TMP_FILE" ] || rm -f "$TMP_FILE"
  [ -z "$TMP_SHA_FILE" ] || rm -f "$TMP_SHA_FILE"
  [ -z "$TMP_MEDIA_FILE" ] || rm -f "$TMP_MEDIA_FILE"
  [ -z "$TMP_MEDIA_SHA_FILE" ] || rm -f "$TMP_MEDIA_SHA_FILE"
  [ -z "$TMP_SUCCESS_FILE" ] || rm -f "$TMP_SUCCESS_FILE"
  [ -z "$RCLONE_CONFIG_FILE" ] || rm -f "$RCLONE_CONFIG_FILE"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

TMP_FILE="$(mktemp "$BACKUP_DIR/.$BACKUP_PREFIX-$TIMESTAMP.dump.XXXXXX")"
TMP_SHA_FILE="$TMP_FILE.sha256"
TMP_MEDIA_FILE="$(mktemp "$BACKUP_DIR/.$BACKUP_PREFIX-files-$TIMESTAMP.tar.gz.XXXXXX")"
TMP_MEDIA_SHA_FILE="$TMP_MEDIA_FILE.sha256"

export PGPASSWORD="$POSTGRES_PASSWORD"
pg_dump --host=postgres --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --format=custom --no-owner --file="$TMP_FILE"
unset PGPASSWORD
chmod 600 "$TMP_FILE"
pg_restore --list "$TMP_FILE" >/dev/null
mv "$TMP_FILE" "$FILE"
TMP_FILE=""
sha256sum "$FILE" > "$TMP_SHA_FILE"
chmod 600 "$TMP_SHA_FILE"
sha256sum -c "$TMP_SHA_FILE" >/dev/null
mv "$TMP_SHA_FILE" "$SHA_FILE"
TMP_SHA_FILE=""

# Back up uploaded/public files separately from PostgreSQL.
tar -C /app-data -czf "$TMP_MEDIA_FILE" .
chmod 600 "$TMP_MEDIA_FILE"
tar -tzf "$TMP_MEDIA_FILE" >/dev/null
mv "$TMP_MEDIA_FILE" "$MEDIA_FILE"
TMP_MEDIA_FILE=""
sha256sum "$MEDIA_FILE" > "$TMP_MEDIA_SHA_FILE"
chmod 600 "$TMP_MEDIA_SHA_FILE"
sha256sum -c "$TMP_MEDIA_SHA_FILE" >/dev/null
mv "$TMP_MEDIA_SHA_FILE" "$MEDIA_SHA_FILE"
TMP_MEDIA_SHA_FILE=""

OFFSITE_STATUS="missing"
if [ -n "$BACKUP_S3_URI" ]; then
  case "$BACKUP_S3_URI" in
    s3://*) S3_PATH="${BACKUP_S3_URI#s3://}" ;;
    *) echo "BACKUP_S3_URI must start with s3://" >&2; exit 1 ;;
  esac
  case "$S3_PATH" in
    ''|/*|*\?*|*\#*|*[[:space:]]*) echo "BACKUP_S3_URI is invalid" >&2; exit 1 ;;
  esac
  S3_BUCKET="${S3_PATH%%/*}"
  case "$S3_BUCKET" in
    ''|*[!a-z0-9.-]*|.*|*.) echo "BACKUP_S3_URI must include a valid S3 bucket" >&2; exit 1 ;;
  esac
  if [ "${#S3_BUCKET}" -lt 3 ] || [ "${#S3_BUCKET}" -gt 63 ]; then
    echo "BACKUP_S3_URI must include a valid S3 bucket" >&2
    exit 1
  fi

  RCLONE_CONFIG_FILE="$(mktemp /tmp/rclone.XXXXXX)"
  cat > "$RCLONE_CONFIG_FILE" <<EOF
[offsite]
type = s3
provider = AWS
env_auth = true
region = ${AWS_REGION:-eu-north-1}
EOF
  chmod 600 "$RCLONE_CONFIG_FILE"
  DEST="offsite:${S3_PATH%/}/$(basename "$FILE")"
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$FILE" "$DEST" --s3-no-check-bucket
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$SHA_FILE" "$DEST.sha256" --s3-no-check-bucket
  MEDIA_DEST="offsite:${S3_PATH%/}/$(basename "$MEDIA_FILE")"
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$MEDIA_FILE" "$MEDIA_DEST" --s3-no-check-bucket
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$MEDIA_SHA_FILE" "$MEDIA_DEST.sha256" --s3-no-check-bucket
  OFFSITE_STATUS="uploaded"
else
  echo "warning: BACKUP_S3_URI is empty; backup is local only" >&2
fi

find "$BACKUP_DIR" -type f \( -name "$BACKUP_PREFIX-*.dump" -o -name "$BACKUP_PREFIX-*.dump.sha256" -o -name "$BACKUP_PREFIX-files-*.tar.gz" -o -name "$BACKUP_PREFIX-files-*.tar.gz.sha256" \) -mtime +"$RETENTION_DAYS" -delete
TMP_SUCCESS_FILE="$(mktemp "$BACKUP_DIR/.backup-success.XXXXXX")"
printf 'completed_at=%s\noffsite=%s\n' "$TIMESTAMP" "$OFFSITE_STATUS" > "$TMP_SUCCESS_FILE"
chmod 600 "$TMP_SUCCESS_FILE"
mv -f "$TMP_SUCCESS_FILE" "$SUCCESS_FILE"
TMP_SUCCESS_FILE=""

echo "database backup created and validated: $FILE"
echo "media backup created and validated: $MEDIA_FILE"
