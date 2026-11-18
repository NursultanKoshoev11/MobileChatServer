-- A device push token identifies one device. When a different account signs
-- in on that device the token must move to that account, otherwise the
-- previous account keeps receiving the new user's notifications.
-- Keep only the most recently registered owner of each token, then enforce
-- global uniqueness.
DELETE FROM push_tokens pt
USING (
    SELECT ctid,
           ROW_NUMBER() OVER (PARTITION BY token ORDER BY updated_at DESC, created_at DESC, user_id DESC) AS rn
    FROM push_tokens
) ranked
WHERE pt.ctid = ranked.ctid AND ranked.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS ux_push_tokens_token ON push_tokens (token);
