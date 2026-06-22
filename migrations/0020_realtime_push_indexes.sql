CREATE INDEX IF NOT EXISTS idx_push_tokens_token
    ON push_tokens (token);

CREATE INDEX IF NOT EXISTS idx_push_tokens_user_updated
    ON push_tokens (user_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_group_created_id
    ON messages (group_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_request_comments_request_created_id
    ON public_request_comments (request_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS idx_public_request_reads_group_user
    ON public_request_reads (group_id, user_id);
