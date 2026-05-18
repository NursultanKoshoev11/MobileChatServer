-- Production support for public statistics dashboard.
-- Safe to run multiple times.

ALTER TABLE public_requests
    ADD COLUMN IF NOT EXISTS interaction_mode TEXT NOT NULL DEFAULT 'discussion';

ALTER TABLE public_requests
    ADD COLUMN IF NOT EXISTS hidden_at TIMESTAMPTZ;

ALTER TABLE public_requests
    ADD COLUMN IF NOT EXISTS hidden_by TEXT REFERENCES users(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'public_requests_request_type_check'
    ) THEN
        ALTER TABLE public_requests DROP CONSTRAINT public_requests_request_type_check;
    END IF;
END $$;

ALTER TABLE public_requests
    ADD CONSTRAINT public_requests_request_type_check
    CHECK (request_type IN ('announcement', 'suggestion', 'complaint', 'requirement', 'problem', 'idea'));

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'public_requests_interaction_mode_check'
    ) THEN
        ALTER TABLE public_requests DROP CONSTRAINT public_requests_interaction_mode_check;
    END IF;
END $$;

ALTER TABLE public_requests
    ADD CONSTRAINT public_requests_interaction_mode_check
    CHECK (interaction_mode IN ('read_only', 'vote_only', 'discussion'));

CREATE INDEX IF NOT EXISTS idx_public_requests_group_visible_created
    ON public_requests (group_id, created_at DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_requests_group_status_visible_created
    ON public_requests (group_id, status, created_at DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_requests_group_type_visible_created
    ON public_requests (group_id, request_type, created_at DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_requests_group_mode_visible_created
    ON public_requests (group_id, interaction_mode, created_at DESC)
    WHERE hidden_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_public_request_votes_request_type
    ON public_request_votes (request_id, vote_type);

CREATE INDEX IF NOT EXISTS idx_public_request_votes_created
    ON public_request_votes (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_public_request_comments_request_visible_created
    ON public_request_comments (request_id, created_at ASC)
    WHERE deleted_at IS NULL;
