CREATE INDEX IF NOT EXISTS idx_messages_group_id_created_at
    ON messages (group_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_requests_group_id_status
    ON public_requests (group_id, status, created_at DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_content_moderation_group_id_status
    ON content_moderation_items (group_id, status, created_at DESC);
