CREATE TABLE IF NOT EXISTS content_moderation_items (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('group_message', 'public_request', 'public_request_comment')),
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_id TEXT,
    title TEXT,
    body TEXT NOT NULL,
    request_type TEXT,
    interaction_mode TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    decision TEXT NOT NULL CHECK (decision IN ('review', 'block')),
    reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider TEXT NOT NULL DEFAULT 'rules',
    provider_model TEXT,
    provider_response_id TEXT,
    provider_scores JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_resource_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by TEXT REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_group_status_created
    ON content_moderation_items (group_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_content_moderation_author_created
    ON content_moderation_items (author_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_content_moderation_content_type_status
    ON content_moderation_items (content_type, status, created_at DESC);
