-- Ensure every group has an invite code so users can join by code or QR.
UPDATE groups
SET invite_code = upper(substr(md5(random()::text || clock_timestamp()::text), 1, 10))
WHERE invite_code IS NULL OR invite_code = '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_invite_code_unique
ON groups(invite_code)
WHERE invite_code IS NOT NULL AND invite_code <> '';
