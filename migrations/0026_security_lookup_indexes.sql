-- Invite-code joins look groups up by the normalized code. Index the exact
-- expression used by FindGroupIDByInviteCode / JoinByInviteCode so each attempt
-- is an index lookup instead of a full scan of groups.
CREATE INDEX IF NOT EXISTS idx_groups_invite_code_normalized
    ON groups ((REPLACE(UPPER(COALESCE(invite_code, '')), '-', '')));

-- The global SMS budget counts all codes issued in a recent window.
CREATE INDEX IF NOT EXISTS idx_phone_verification_codes_created
    ON phone_verification_codes (created_at);
