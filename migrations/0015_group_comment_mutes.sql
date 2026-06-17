CREATE TABLE IF NOT EXISTS group_comment_mutes (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    muted_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    muted_until TIMESTAMPTZ,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    unmuted_at TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_comment_mutes_active
    ON group_comment_mutes (group_id, user_id, muted_until)
    WHERE unmuted_at IS NULL;
