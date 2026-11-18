#!/usr/bin/env bash
# Backup restore drill: restore the latest database backup into a throwaway
# PostgreSQL container, run sanity queries and tear everything down.
#
# It never connects to the live database: the scratch container runs with
# --network none, gets a random password and a unique name, and is removed on
# every exit path. Run it on the production host (or any host with a copy of
# the backups) from the repository directory:
#
#   sudo BACKUP_AGE_IDENTITY_FILE=/root/koom-age-identity.txt scripts/restore-drill.sh
#
# Environment:
#   BACKUP_DIR               where backups live (default: ./backups)
#   BACKUP_PREFIX            backup file prefix (default: mobilechat)
#   DRILL_BACKUP_FILE        restore this file instead of the newest one
#   BACKUP_AGE_IDENTITY_FILE age identity file, needed for *.dump.age backups
#   DRILL_PG_IMAGE           scratch image (default: postgres:16-alpine)
#   DRILL_AGE_IMAGE          image providing `age` when it is not installed on
#                            the host (default: mobilechat-backup:prod)
#   DRILL_MIN_USERS          minimum expected rows in users (default: 1)
#   DRILL_MAX_AGE_HOURS      fail if the newest backup is older (default: 36)
set -Eeuo pipefail
umask 077

BACKUP_DIR="${BACKUP_DIR:-./backups}"
BACKUP_PREFIX="${BACKUP_PREFIX:-mobilechat}"
DRILL_PG_IMAGE="${DRILL_PG_IMAGE:-postgres:16-alpine}"
DRILL_AGE_IMAGE="${DRILL_AGE_IMAGE:-mobilechat-backup:prod}"
DRILL_MIN_USERS="${DRILL_MIN_USERS:-1}"
DRILL_MAX_AGE_HOURS="${DRILL_MAX_AGE_HOURS:-36}"

log() { printf '[restore-drill] %s\n' "$*"; }
die() { printf '[restore-drill] FAILED: %s\n' "$*" >&2; exit 1; }

[[ "$BACKUP_PREFIX" =~ ^[A-Za-z0-9][A-Za-z0-9_-]*$ ]] || die "BACKUP_PREFIX contains unsupported characters"
[[ "$DRILL_MIN_USERS" =~ ^[0-9]+$ ]] || die "DRILL_MIN_USERS must be an integer"
[[ "$DRILL_MAX_AGE_HOURS" =~ ^[0-9]+$ ]] || die "DRILL_MAX_AGE_HOURS must be an integer"
command -v docker >/dev/null || die "docker is required"

if [[ -n "${DRILL_BACKUP_FILE:-}" ]]; then
  backup_file="$DRILL_BACKUP_FILE"
else
  backup_file="$(find "$BACKUP_DIR" -maxdepth 1 -type f \
    \( -name "$BACKUP_PREFIX-*.dump" -o -name "$BACKUP_PREFIX-*.dump.age" \) \
    ! -name "$BACKUP_PREFIX-files-*" -printf '%T@ %p\n' 2>/dev/null \
    | sort -nr | head -n 1 | cut -d' ' -f2-)"
fi
[[ -n "$backup_file" && -f "$backup_file" ]] || die "no database backup found in $BACKUP_DIR (prefix $BACKUP_PREFIX)"
backup_file="$(cd "$(dirname "$backup_file")" && pwd)/$(basename "$backup_file")"
log "backup: $backup_file"

age_hours=$(( ( $(date +%s) - $(stat -c %Y "$backup_file") ) / 3600 ))
log "backup age: ${age_hours}h"
if (( DRILL_MAX_AGE_HOURS > 0 && age_hours > DRILL_MAX_AGE_HOURS )); then
  die "newest backup is ${age_hours}h old (limit ${DRILL_MAX_AGE_HOURS}h); the backup job may be failing"
fi

if [[ -f "$backup_file.sha256" ]]; then
  expected="$(awk 'NR==1 {print $1}' "$backup_file.sha256")"
  actual="$(sha256sum "$backup_file" | awk '{print $1}')"
  [[ -n "$expected" && "$expected" == "$actual" ]] || die "checksum mismatch for $backup_file"
  log "checksum OK"
else
  die "missing $backup_file.sha256"
fi

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/koom-restore-drill.XXXXXXXX")"
chmod 700 "$work_dir"
container="koom-restore-drill-$(date -u +%Y%m%d%H%M%S)-$$"
container_started=0
cleanup() {
  local status=$?
  trap - EXIT
  if (( container_started == 1 )); then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
  rm -rf -- "$work_dir"
  if (( status == 0 )); then
    log "teardown complete"
  else
    printf '[restore-drill] teardown complete after failure (exit %s)\n' "$status" >&2
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

dump_file="$work_dir/restore.dump"
if [[ "$backup_file" == *.age ]]; then
  identity="${BACKUP_AGE_IDENTITY_FILE:-}"
  [[ -n "$identity" && -r "$identity" ]] || die "backup is encrypted; set BACKUP_AGE_IDENTITY_FILE to a readable age identity file"
  if command -v age >/dev/null; then
    age --decrypt -i "$identity" -o "$dump_file" "$backup_file"
  else
    log "age not found on host; decrypting with $DRILL_AGE_IMAGE"
    docker run --rm --network none --entrypoint age \
      -v "$(cd "$(dirname "$identity")" && pwd)/$(basename "$identity"):/run/identity:ro" \
      -v "$backup_file:/run/backup.age:ro" \
      "$DRILL_AGE_IMAGE" --decrypt -i /run/identity /run/backup.age > "$dump_file"
  fi
else
  cp -- "$backup_file" "$dump_file"
fi
# The postgres user inside the scratch container must be able to read it.
chmod 755 "$work_dir"
chmod 644 "$dump_file"

password="$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
log "starting scratch container $container ($DRILL_PG_IMAGE, no network)"
docker run -d --rm --name "$container" --network none \
  --label koom.restore-drill=true \
  -e POSTGRES_PASSWORD="$password" -e POSTGRES_DB=drill -e POSTGRES_USER=drill \
  -v "$work_dir:/drill:ro" \
  "$DRILL_PG_IMAGE" >/dev/null
container_started=1

# During first-boot initialisation the server listens on the Unix socket only,
# so waiting for TCP on 127.0.0.1 waits for the final server.
for attempt in $(seq 1 60); do
  if docker exec "$container" pg_isready -h 127.0.0.1 -U drill -d drill >/dev/null 2>&1; then
    break
  fi
  (( attempt < 60 )) || die "scratch database did not become ready"
  sleep 1
done

start=$(date +%s)
log "restoring into scratch database"
docker exec "$container" pg_restore --exit-on-error --no-owner --no-privileges \
  -U drill -d drill /drill/restore.dump
log "restore took $(( $(date +%s) - start ))s"

query() {
  docker exec "$container" psql -U drill -d drill -v ON_ERROR_STOP=1 -Atc "$1"
}
users="$(query 'SELECT count(*) FROM users')"
groups="$(query 'SELECT count(*) FROM groups')"
migrations="$(query 'SELECT count(*) FROM schema_migrations')"
latest_migration="$(query 'SELECT max(version) FROM schema_migrations')"
log "users=$users groups=$groups schema_migrations=$migrations latest=$latest_migration"

(( users >= DRILL_MIN_USERS )) || die "users has $users rows (expected at least $DRILL_MIN_USERS)"
(( migrations > 0 )) || die "schema_migrations is empty"

log "PASSED: $backup_file restores cleanly"
