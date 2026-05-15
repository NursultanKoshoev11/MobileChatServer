#!/usr/bin/env sh
set -eu

PHONE="${1:-}"
ROLE="${2:-super_admin}"

if [ -z "$PHONE" ]; then
  echo "Usage: sh scripts/docker-create-admin.sh +996000000000 [platform_admin|super_admin]"
  exit 1
fi

if [ "$ROLE" != "platform_admin" ] && [ "$ROLE" != "super_admin" ]; then
  echo "Role must be platform_admin or super_admin"
  exit 1
fi

docker compose exec -T postgres psql -U "${POSTGRES_USER:-mobilechat}" -d "${POSTGRES_DB:-mobilechat}" <<SQL
INSERT INTO admin_phone_allowlist (phone, role)
VALUES ('$PHONE', '$ROLE')
ON CONFLICT (phone) DO UPDATE
SET role = EXCLUDED.role,
    enabled = true,
    updated_at = now();
SQL

echo "Admin phone allowlist updated: $PHONE -> $ROLE"
