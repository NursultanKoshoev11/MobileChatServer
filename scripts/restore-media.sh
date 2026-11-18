#!/bin/sh
# Restore an uploaded-media archive made by backup-postgres.sh
# (<prefix>-files-<timestamp>.tar.gz[.age]) into a target directory.
#
# The current contents of the target are NOT deleted: they are moved aside to
# <target>/.pre-restore-<timestamp>/ so the restore can be undone; remove that
# directory once the restore has been verified.
#
# The backup service mounts ./data read-only, so mount it read-write for the
# restore, for example (stop the API first):
#   docker compose -f docker-compose.prod.yml --env-file .env.prod stop api
#   docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm \
#     -v ./data:/restore-target \
#     -e RESTORE_MEDIA_TARGET=/restore-target -e RESTORE_CONFIRM=/restore-target \
#     -e BACKUP_AGE_IDENTITY_FILE=/run/koom-age-identity \
#     -v /root/koom-age-identity.txt:/run/koom-age-identity:ro \
#     backup /scripts/restore-media.sh /backups/<prefix>-files-<timestamp>.tar.gz.age
#
# Environment:
#   RESTORE_MEDIA_TARGET      directory to restore into (required)
#   RESTORE_CONFIRM           required; must equal RESTORE_MEDIA_TARGET
#   BACKUP_AGE_IDENTITY_FILE  age identity (secret key) file for *.age inputs
#   RESTORE_SKIP_CHECKSUM=1   restore even if no .sha256 file is present
set -eu
umask 077

usage() {
  echo "Usage: RESTORE_MEDIA_TARGET=/dir RESTORE_CONFIRM=/dir $0 /backups/<prefix>-files-YYYYMMDDTHHMMSSZ.tar.gz[.age]" >&2
  exit 1
}

[ "${1:-}" != "" ] || usage
ARCHIVE="$1"
TARGET="${RESTORE_MEDIA_TARGET:-}"
[ -n "$TARGET" ] || usage

case "$TARGET" in
  /*) ;;
  *) echo "RESTORE_MEDIA_TARGET must be an absolute path" >&2; exit 1 ;;
esac
TARGET="${TARGET%/}"
if [ -z "$TARGET" ] || [ "$TARGET" = "/" ]; then
  echo "Refusing to restore into /" >&2
  exit 1
fi
if [ ! -d "$TARGET" ]; then
  echo "Target directory does not exist: $TARGET" >&2
  exit 1
fi
if [ ! -w "$TARGET" ]; then
  echo "Target directory is not writable (is it mounted read-only?): $TARGET" >&2
  exit 1
fi
if [ ! -f "$ARCHIVE" ]; then
  echo "Archive not found: $ARCHIVE" >&2
  exit 1
fi
if [ "${RESTORE_CONFIRM:-}" != "$TARGET" ]; then
  echo "Refusing to restore: this replaces the contents of '$TARGET' (old contents are moved aside)." >&2
  echo "Re-run with RESTORE_CONFIRM=$TARGET to confirm." >&2
  exit 2
fi

if [ -f "$ARCHIVE.sha256" ]; then
  expected="$(awk 'NR==1 {print $1}' "$ARCHIVE.sha256")"
  actual="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
  if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    echo "Checksum mismatch for $ARCHIVE" >&2
    exit 1
  fi
  echo "checksum OK: $ARCHIVE"
elif [ "${RESTORE_SKIP_CHECKSUM:-}" != "1" ]; then
  echo "No $ARCHIVE.sha256 found; set RESTORE_SKIP_CHECKSUM=1 to restore anyway." >&2
  exit 1
fi

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
STAGE="$TARGET/.restore-staging-$STAMP"
ASIDE="$TARGET/.pre-restore-$STAMP"
WORK_DIR=""
cleanup() {
  [ -z "$WORK_DIR" ] || rm -rf "$WORK_DIR"
  # Only an unfinished staging directory is removed; ASIDE is kept on purpose.
  [ ! -d "$STAGE" ] || rm -rf "$STAGE"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

INPUT="$ARCHIVE"
case "$ARCHIVE" in
  *.age)
    if ! command -v age >/dev/null 2>&1; then
      echo "The archive is age-encrypted but 'age' is not installed; rebuild the backup image." >&2
      exit 1
    fi
    if [ -z "${BACKUP_AGE_IDENTITY_FILE:-}" ] || [ ! -r "$BACKUP_AGE_IDENTITY_FILE" ]; then
      echo "Set BACKUP_AGE_IDENTITY_FILE to a readable age identity file to decrypt $ARCHIVE." >&2
      exit 1
    fi
    WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/koom-media-restore.XXXXXX")"
    INPUT="$WORK_DIR/media.tar.gz"
    age --decrypt -i "$BACKUP_AGE_IDENTITY_FILE" -o "$INPUT" "$ARCHIVE"
    ;;
esac

# Reject archives with absolute paths or parent-directory components.
if tar -tzf "$INPUT" | grep -Eq '^/|(^|/)\.\.(/|$)'; then
  echo "Archive contains unsafe paths; refusing to extract." >&2
  exit 1
fi

mkdir "$STAGE"
tar -xzf "$INPUT" -C "$STAGE"

mkdir "$ASIDE"
find "$TARGET" -mindepth 1 -maxdepth 1 \
  ! -name '.restore-staging-*' ! -name '.pre-restore-*' \
  -exec mv -- {} "$ASIDE"/ \;
find "$STAGE" -mindepth 1 -maxdepth 1 -exec mv -- {} "$TARGET"/ \;
rmdir "$STAGE"

echo "media restored into $TARGET from: $ARCHIVE"
echo "previous contents were moved to: $ASIDE (delete it after verifying the restore)"
echo "check ownership: the API runs as uid 10001 and must be able to read and write $TARGET"
