-- User-generated content reports and user blocks (App Store 1.2 / Google Play UGC).

CREATE TABLE IF NOT EXISTS content_reports (
    id TEXT PRIMARY KEY,
    reporter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('public_request', 'comment', 'message', 'user')),
    target_id TEXT NOT NULL,
    -- Group that owns the reported content; NULL for reports about a user.
    group_id TEXT REFERENCES groups(id) ON DELETE CASCADE,
    -- Author of the reported content (or the reported user). Kept nullable so a
    -- report about anonymized content survives the author's account deletion.
    target_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL CHECK (reason IN ('spam', 'abuse', 'illegal', 'other')),
    details TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    resolution TEXT CHECK (resolution IS NULL OR resolution IN ('dismissed', 'content_removed')),
    resolved_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One open report per reporter and target; makes report creation idempotent
-- even under concurrent requests.
CREATE UNIQUE INDEX IF NOT EXISTS ux_content_reports_open_reporter_target
    ON content_reports (reporter_id, target_type, target_id)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_content_reports_group_status_created
    ON content_reports (group_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_content_reports_status_created
    ON content_reports (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_content_reports_target
    ON content_reports (target_type, target_id);

CREATE INDEX IF NOT EXISTS idx_content_reports_target_user
    ON content_reports (target_user_id);

CREATE TABLE IF NOT EXISTS user_blocks (
    blocker_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);

CREATE INDEX IF NOT EXISTS idx_user_blocks_blocked ON user_blocks (blocked_id);
