#!/bin/sh
# Daily PostgreSQL + uploaded-media backup, run inside the `backup` service of
# docker-compose.prod.yml (image built from Dockerfile.backup).
#
# Optional encryption (recommended): set BACKUP_AGE_RECIPIENT to an age public
# key (age1...) and/or BACKUP_AGE_RECIPIENTS_FILE to a file of recipients. Each
# artifact is validated in plaintext first, then encrypted with `age` before it
# is written to $BACKUP_DIR or uploaded; the .sha256 file covers the encrypted
# file. Without a recipient the old plaintext behaviour is kept and a WARNING
# is logged on every run.
#
# Optional S3 server-side encryption: BACKUP_S3_SSE=AES256 or aws:kms
# (BACKUP_S3_SSE_KMS_KEY_ID optionally selects the KMS key).
set -eu
umask 077

BACKUP_DIR="/backups"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_PREFIX="${BACKUP_PREFIX:-koom}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"
BACKUP_S3_URI="${BACKUP_S3_URI:-}"
BACKUP_S3_SSE="${BACKUP_S3_SSE:-}"
BACKUP_S3_SSE_KMS_KEY_ID="${BACKUP_S3_SSE_KMS_KEY_ID:-}"
BACKUP_AGE_RECIPIENT="${BACKUP_AGE_RECIPIENT:-}"
BACKUP_AGE_RECIPIENTS_FILE="${BACKUP_AGE_RECIPIENTS_FILE:-}"
SUCCESS_FILE="$BACKUP_DIR/backup-success"

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

# --- encryption settings -----------------------------------------------------
ENCRYPT=0
if [ -n "$BACKUP_AGE_RECIPIENT" ] || [ -n "$BACKUP_AGE_RECIPIENTS_FILE" ]; then
  ENCRYPT=1
  if ! command -v age >/dev/null 2>&1; then
    echo "ERROR: BACKUP_AGE_RECIPIENT/BACKUP_AGE_RECIPIENTS_FILE is set but the 'age' binary is missing." >&2
    echo "ERROR: rebuild the backup image (docker compose -f docker-compose.prod.yml build backup) so Dockerfile.backup installs age." >&2
    echo "ERROR: refusing to write an unencrypted backup while encryption is configured." >&2
    exit 1
  fi
  for recipient in $BACKUP_AGE_RECIPIENT; do
    case "$recipient" in
      age1*) ;;
      *) echo "BACKUP_AGE_RECIPIENT must contain age public keys (age1...); never put an identity/secret key here" >&2; exit 1 ;;
    esac
  done
  if [ -n "$BACKUP_AGE_RECIPIENTS_FILE" ] && [ ! -r "$BACKUP_AGE_RECIPIENTS_FILE" ]; then
    echo "BACKUP_AGE_RECIPIENTS_FILE is not readable: $BACKUP_AGE_RECIPIENTS_FILE" >&2
    exit 1
  fi
  if [ -n "$BACKUP_AGE_RECIPIENTS_FILE" ] && grep -q 'AGE-SECRET-KEY' "$BACKUP_AGE_RECIPIENTS_FILE"; then
    echo "BACKUP_AGE_RECIPIENTS_FILE contains a secret key; it must only list public recipients" >&2
    exit 1
  fi
  ENC_SUFFIX=".age"
else
  echo "WARNING: ************************************************************" >&2
  echo "WARNING: backups are NOT encrypted. Set BACKUP_AGE_RECIPIENT (age public key)" >&2
  echo "WARNING: or BACKUP_AGE_RECIPIENTS_FILE so dumps and media archives are encrypted" >&2
  echo "WARNING: before they are stored locally or uploaded off-site." >&2
  echo "WARNING: ************************************************************" >&2
  ENC_SUFFIX=""
fi

case "$BACKUP_S3_SSE" in
  ''|AES256|aws:kms) ;;
  *) echo "BACKUP_S3_SSE must be empty, AES256 or aws:kms" >&2; exit 1 ;;
esac
case "$BACKUP_S3_SSE_KMS_KEY_ID" in
  *[!A-Za-z0-9:/_.-]*) echo "BACKUP_S3_SSE_KMS_KEY_ID contains unsupported characters" >&2; exit 1 ;;
esac
if [ -n "$BACKUP_S3_SSE_KMS_KEY_ID" ] && [ "$BACKUP_S3_SSE" != "aws:kms" ]; then
  echo "BACKUP_S3_SSE_KMS_KEY_ID requires BACKUP_S3_SSE=aws:kms" >&2
  exit 1
fi

FILE="$BACKUP_DIR/$BACKUP_PREFIX-$TIMESTAMP.dump$ENC_SUFFIX"
SHA_FILE="$FILE.sha256"
MEDIA_FILE="$BACKUP_DIR/$BACKUP_PREFIX-files-$TIMESTAMP.tar.gz$ENC_SUFFIX"
MEDIA_SHA_FILE="$MEDIA_FILE.sha256"

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
TMP_ENC_FILE=""
TMP_SHA_FILE=""
TMP_MEDIA_FILE=""
TMP_MEDIA_ENC_FILE=""
TMP_MEDIA_SHA_FILE=""
TMP_SUCCESS_FILE=""
RCLONE_CONFIG_FILE=""
cleanup() {
  for tmp in "$TMP_FILE" "$TMP_ENC_FILE" "$TMP_SHA_FILE" "$TMP_MEDIA_FILE" "$TMP_MEDIA_ENC_FILE" \
    "$TMP_MEDIA_SHA_FILE" "$TMP_SUCCESS_FILE" "$RCLONE_CONFIG_FILE"; do
    [ -z "$tmp" ] || rm -f "$tmp"
  done
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

# encrypt_to SRC DEST: age-encrypt SRC into DEST and sanity-check the output.
encrypt_to() {
  enc_src="$1"
  enc_dst="$2"
  set --
  for recipient in $BACKUP_AGE_RECIPIENT; do
    set -- "$@" -r "$recipient"
  done
  if [ -n "$BACKUP_AGE_RECIPIENTS_FILE" ]; then
    set -- "$@" -R "$BACKUP_AGE_RECIPIENTS_FILE"
  fi
  age --encrypt "$@" -o "$enc_dst" "$enc_src"
  chmod 600 "$enc_dst"
  if [ ! -s "$enc_dst" ] || [ "$(head -c 21 "$enc_dst")" != "age-encryption.org/v1" ]; then
    echo "age produced an invalid file for $enc_src" >&2
    exit 1
  fi
}

# finalize SRC DEST SHA: move a validated temp file into place and write a
# verified sha256 file next to it.
finalize() {
  mv "$1" "$2"
  sha_tmp="$(mktemp "$BACKUP_DIR/.sha256.XXXXXX")"
  TMP_SHA_FILE="$sha_tmp"
  sha256sum "$2" > "$sha_tmp"
  chmod 600 "$sha_tmp"
  sha256sum -c "$sha_tmp" >/dev/null
  mv "$sha_tmp" "$3"
  TMP_SHA_FILE=""
}

# --- PostgreSQL ----------------------------------------------------------------
TMP_FILE="$(mktemp "$BACKUP_DIR/.$BACKUP_PREFIX-$TIMESTAMP.dump.XXXXXX")"
export PGPASSWORD="$POSTGRES_PASSWORD"
pg_dump --host=postgres --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --format=custom --no-owner --file="$TMP_FILE"
unset PGPASSWORD
chmod 600 "$TMP_FILE"
# Validate the plaintext dump before encrypting it.
pg_restore --list "$TMP_FILE" >/dev/null
if [ "$ENCRYPT" = 1 ]; then
  TMP_ENC_FILE="$TMP_FILE.age"
  encrypt_to "$TMP_FILE" "$TMP_ENC_FILE"
  rm -f "$TMP_FILE"
  TMP_FILE=""
  finalize "$TMP_ENC_FILE" "$FILE" "$SHA_FILE"
  TMP_ENC_FILE=""
else
  finalize "$TMP_FILE" "$FILE" "$SHA_FILE"
  TMP_FILE=""
fi

# --- uploaded/public files (separately from PostgreSQL) ------------------------
TMP_MEDIA_FILE="$(mktemp "$BACKUP_DIR/.$BACKUP_PREFIX-files-$TIMESTAMP.tar.gz.XXXXXX")"
tar -C /app-data -czf "$TMP_MEDIA_FILE" .
chmod 600 "$TMP_MEDIA_FILE"
tar -tzf "$TMP_MEDIA_FILE" >/dev/null
if [ "$ENCRYPT" = 1 ]; then
  TMP_MEDIA_ENC_FILE="$TMP_MEDIA_FILE.age"
  encrypt_to "$TMP_MEDIA_FILE" "$TMP_MEDIA_ENC_FILE"
  rm -f "$TMP_MEDIA_FILE"
  TMP_MEDIA_FILE=""
  finalize "$TMP_MEDIA_ENC_FILE" "$MEDIA_FILE" "$MEDIA_SHA_FILE"
  TMP_MEDIA_ENC_FILE=""
else
  finalize "$TMP_MEDIA_FILE" "$MEDIA_FILE" "$MEDIA_SHA_FILE"
  TMP_MEDIA_FILE=""
fi

# --- off-site copy -------------------------------------------------------------
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
  {
    printf '[offsite]\ntype = s3\nprovider = AWS\nenv_auth = true\nregion = %s\n' "${AWS_REGION:-eu-north-1}"
    if [ -n "$BACKUP_S3_SSE" ]; then
      printf 'server_side_encryption = %s\n' "$BACKUP_S3_SSE"
    fi
    if [ -n "$BACKUP_S3_SSE_KMS_KEY_ID" ]; then
      printf 'sse_kms_key_id = %s\n' "$BACKUP_S3_SSE_KMS_KEY_ID"
    fi
  } > "$RCLONE_CONFIG_FILE"
  chmod 600 "$RCLONE_CONFIG_FILE"
  DEST="offsite:${S3_PATH%/}/$(basename "$FILE")"
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$FILE" "$DEST" --s3-no-check-bucket
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$SHA_FILE" "$DEST.sha256" --s3-no-check-bucket
  MEDIA_DEST="offsite:${S3_PATH%/}/$(basename "$MEDIA_FILE")"
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$MEDIA_FILE" "$MEDIA_DEST" --s3-no-check-bucket
  rclone --config "$RCLONE_CONFIG_FILE" copyto "$MEDIA_SHA_FILE" "$MEDIA_DEST.sha256" --s3-no-check-bucket
  rclone --config "$RCLONE_CONFIG_FILE" delete "offsite:${S3_PATH%/}" \
    --min-age "${RETENTION_DAYS}d" \
    --include "$BACKUP_PREFIX-*.dump" \
    --include "$BACKUP_PREFIX-*.dump.sha256" \
    --include "$BACKUP_PREFIX-*.dump.age" \
    --include "$BACKUP_PREFIX-*.dump.age.sha256" \
    --include "$BACKUP_PREFIX-files-*.tar.gz" \
    --include "$BACKUP_PREFIX-files-*.tar.gz.sha256" \
    --include "$BACKUP_PREFIX-files-*.tar.gz.age" \
    --include "$BACKUP_PREFIX-files-*.tar.gz.age.sha256"
  OFFSITE_STATUS="uploaded"
else
  echo "warning: BACKUP_S3_URI is empty; backup is local only" >&2
fi

find "$BACKUP_DIR" -maxdepth 1 -type f \( \
  -name "$BACKUP_PREFIX-*.dump" -o -name "$BACKUP_PREFIX-*.dump.sha256" \
  -o -name "$BACKUP_PREFIX-*.dump.age" -o -name "$BACKUP_PREFIX-*.dump.age.sha256" \
  -o -name "$BACKUP_PREFIX-files-*.tar.gz" -o -name "$BACKUP_PREFIX-files-*.tar.gz.sha256" \
  -o -name "$BACKUP_PREFIX-files-*.tar.gz.age" -o -name "$BACKUP_PREFIX-files-*.tar.gz.age.sha256" \
  \) -mtime +"$RETENTION_DAYS" -delete
TMP_SUCCESS_FILE="$(mktemp "$BACKUP_DIR/.backup-success.XXXXXX")"
if [ "$ENCRYPT" = 1 ]; then ENCRYPTION_STATUS="age"; else ENCRYPTION_STATUS="none"; fi
printf 'completed_at=%s\noffsite=%s\nencryption=%s\n' "$TIMESTAMP" "$OFFSITE_STATUS" "$ENCRYPTION_STATUS" > "$TMP_SUCCESS_FILE"
chmod 600 "$TMP_SUCCESS_FILE"
mv -f "$TMP_SUCCESS_FILE" "$SUCCESS_FILE"
TMP_SUCCESS_FILE=""

echo "database backup created and validated: $FILE"
echo "media backup created and validated: $MEDIA_FILE"
