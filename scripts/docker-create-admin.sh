#!/usr/bin/env sh
# Allow-list a phone number as platform_admin or super_admin in the database
# of the Compose project in the current directory (local dev stack by default).
# POSTGRES_USER/POSTGRES_DB default to the values in .env.prod.example (koom).
set -eu

PHONE="${1:-}"
ROLE="${2:-super_admin}"

if [ -z "$PHONE" ]; then
  echo "Usage: sh scripts/docker-create-admin.sh +996000000000 [platform_admin|super_admin]" >&2
  exit 1
fi

# E.164: '+' followed by 8-15 digits. Validation is defence in depth; the value
# is passed to psql as a variable and quoted with :'phone', never spliced
# into SQL text.
if ! printf '%s' "$PHONE" | grep -Eq '^\+[1-9][0-9]{7,14}$'; then
  echo "Phone must be in E.164 format, e.g. +996000000000" >&2
  exit 1
fi

if [ "$ROLE" != "platform_admin" ] && [ "$ROLE" != "super_admin" ]; then
  echo "Role must be platform_admin or super_admin" >&2
  exit 1
fi

docker compose exec -T postgres psql \
  -v ON_ERROR_STOP=1 \
  -v phone="$PHONE" \
  -v role="$ROLE" \
  -U "${POSTGRES_USER:-koom}" -d "${POSTGRES_DB:-koom}" <<'SQL'
INSERT INTO admin_phone_allowlist (phone, role)
VALUES (:'phone', :'role')
ON CONFLICT (phone) DO UPDATE
SET role = EXCLUDED.role,
    enabled = true,
    updated_at = now();
SQL

echo "Admin phone allowlist updated: $PHONE -> $ROLE"
