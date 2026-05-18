-- Normalize old group invite codes to the short format used by the app.
-- Expected visible format: ABC-123 (7 characters including dash).
-- New groups already receive this format from randomInviteCode().

ALTER TABLE groups ADD COLUMN IF NOT EXISTS invite_code TEXT;

UPDATE groups
SET invite_code = upper(substr(md5(id || ':letters'), 1, 3) || '-' || substr(md5(id || ':digits'), 1, 3))
WHERE invite_code IS NULL
   OR invite_code !~ '^[A-Z0-9]{3}-[A-Z0-9]{3}$';

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_invite_code
ON groups(invite_code)
WHERE invite_code IS NOT NULL;
