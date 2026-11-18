-- group_invite_requests had UNIQUE (group_id, target_user_id, status). That
-- also allowed only one 'accepted' and one 'declined' row per user and group,
-- so a user who accepted an invite, left, and was invited again could not
-- accept (or decline twice) without a unique violation (HTTP 500).
-- Only one *pending* invite per user and group is meant to be unique; answered
-- invites are history. The old constraint already guaranteed at most one
-- pending row, so existing data satisfies the new index.
ALTER TABLE group_invite_requests
    DROP CONSTRAINT IF EXISTS group_invite_requests_group_id_target_user_id_status_key;

CREATE UNIQUE INDEX IF NOT EXISTS ux_group_invite_requests_pending
    ON group_invite_requests (group_id, target_user_id)
    WHERE status = 'pending';
