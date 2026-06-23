#!/usr/bin/env bash
set -euo pipefail

# Enables local/staging test auth so any phone number can sign in with code 123.
# This must not be used for real production access.
# Usage:
#   bash scripts/enable_test_auth_123_any_phone.sh
#   bash scripts/enable_test_auth_123_any_phone.sh --env-file .env.prod
#   bash scripts/enable_test_auth_123_any_phone.sh --env-file .env.prod --commit

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$ROOT_DIR/internal/service/phone_auth_service.go"
ENV_FILE=""
DO_COMMIT="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="${2:-}"
      shift 2
      ;;
    --commit)
      DO_COMMIT="true"
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$TARGET" ]]; then
  echo "Target file not found: $TARGET" >&2
  exit 1
fi

python3 - "$TARGET" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()

old_is_test = '''func (s *PhoneAuthService) isTestAuthMobile(mobile string) bool {
	if isPublicDemoAuthMobile(mobile) {
		return true
	}
	if !s.cfg.TestAuthEnabled {
		return false
	}
	return normalizeTestValue(mobile) == normalizeTestValue(s.cfg.TestAuthPhone)
}
'''

new_is_test = '''func (s *PhoneAuthService) isTestAuthMobile(mobile string) bool {
	if isPublicDemoAuthMobile(mobile) {
		return true
	}
	if !s.cfg.TestAuthEnabled {
		return false
	}
	configuredPhone := normalizeTestValue(s.cfg.TestAuthPhone)
	if configuredPhone == "" || configuredPhone == "*" || strings.EqualFold(configuredPhone, "any") {
		return true
	}
	return normalizeTestValue(mobile) == configuredPhone
}
'''

old_expected = '''func (s *PhoneAuthService) expectedTestAuthCode(mobile string) string {
	if isPublicDemoAuthMobile(mobile) {
		return publicDemoAuthCode
	}
	return strings.TrimSpace(s.cfg.TestAuthCode)
}
'''

new_expected = '''func (s *PhoneAuthService) expectedTestAuthCode(mobile string) string {
	if isPublicDemoAuthMobile(mobile) {
		return publicDemoAuthCode
	}
	code := strings.TrimSpace(s.cfg.TestAuthCode)
	if code == "" {
		return publicDemoAuthCode
	}
	return code
}
'''

changed = False
if old_is_test in text:
    text = text.replace(old_is_test, new_is_test)
    changed = True
elif new_is_test in text:
    print("isTestAuthMobile already patched")
else:
    raise SystemExit("Could not find isTestAuthMobile block to patch")

if old_expected in text:
    text = text.replace(old_expected, new_expected)
    changed = True
elif new_expected in text:
    print("expectedTestAuthCode already patched")
else:
    raise SystemExit("Could not find expectedTestAuthCode block to patch")

if changed:
    path.write_text(text)
    print(f"Patched {path}")
else:
    print("No Go changes needed")
PY

upsert_env() {
  local file="$1"
  local key="$2"
  local value="$3"
  touch "$file"
  if grep -qE "^${key}=" "$file"; then
    sed -i.bak "s|^${key}=.*|${key}=${value}|" "$file"
  else
    printf '\n%s=%s\n' "$key" "$value" >> "$file"
  fi
}

if [[ -n "$ENV_FILE" ]]; then
  ENV_PATH="$ROOT_DIR/$ENV_FILE"
  upsert_env "$ENV_PATH" "APP_ENV" "staging"
  upsert_env "$ENV_PATH" "TEST_AUTH_ENABLED" "true"
  upsert_env "$ENV_PATH" "TEST_AUTH_PHONE" "*"
  upsert_env "$ENV_PATH" "TEST_AUTH_CODE" "123"
  upsert_env "$ENV_PATH" "TEST_AUTH_DISPLAY_NAME" "Koom Test User"
  echo "Updated $ENV_PATH"
  echo "Backup files with .bak extension may have been created by sed."
fi

gofmt -w "$TARGET"

if [[ "$DO_COMMIT" == "true" ]]; then
  git -C "$ROOT_DIR" add internal/service/phone_auth_service.go
  if [[ -n "$ENV_FILE" ]]; then
    git -C "$ROOT_DIR" add "$ENV_FILE"
  fi
  git -C "$ROOT_DIR" commit -m "Enable wildcard test auth mode"
fi

echo "Done."
echo "Test mode values: TEST_AUTH_ENABLED=true, TEST_AUTH_PHONE=*, TEST_AUTH_CODE=123"
echo "Restart backend after pushing/deploying."
