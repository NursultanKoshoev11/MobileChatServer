#!/bin/sh
set -eu
umask 077

BACKUP_DIR="/backups"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
FILE="$BACKUP_DIR/mobilechat-$TIMESTAMP.dump"
SHA_FILE="$FILE.sha256"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_S3_URI="${BACKUP_S3_URI:-}"

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR" || true

export PGPASSWORD="$POSTGRES_PASSWORD"
pg_dump --host=postgres --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --format=custom --no-owner --file="$FILE"
chmod 600 "$FILE"
sha256sum "$FILE" > "$SHA_FILE"
chmod 600 "$SHA_FILE"

# Validate the custom-format archive before declaring success.
pg_restore --list "$FILE" >/dev/null
sha256sum -c "$SHA_FILE" >/dev/null

if [ -n "$BACKUP_S3_URI" ]; then
  case "$BACKUP_S3_URI" in
    s3://*) S3_PATH="${BACKUP_S3_URI#s3://}" ;;
    *) echo "BACKUP_S3_URI must start with s3://" >&2; exit 1 ;;
  esac
  cat > /tmp/rclone.conf <<EOF
[offsite]
type = s3
provider = AWS
env_auth = true
region = ${AWS_REGION:-eu-north-1}
EOF
  DEST="offsite:${S3_PATH%/}/$(basename "$FILE")"
  rclone --config /tmp/rclone.conf copyto "$FILE" "$DEST" --s3-no-check-bucket
  rclone --config /tmp/rclone.conf copyto "$SHA_FILE" "$DEST.sha256" --s3-no-check-bucket
  rm -f /tmp/rclone.conf
  echo "offsite backup uploaded: $BACKUP_S3_URI/$(basename "$FILE")"
else
  echo "warning: BACKUP_S3_URI is empty; backup is local only" >&2
fi

find "$BACKUP_DIR" -type f \( -name 'mobilechat-*.dump' -o -name 'mobilechat-*.dump.sha256' \) -mtime +"$RETENTION_DAYS" -delete
echo "backup created and validated: $FILE"
