ALTER TABLE public_requests ADD COLUMN IF NOT EXISTS interaction_mode TEXT NOT NULL DEFAULT 'discussion';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'public_requests_request_type_check'
    ) THEN
        ALTER TABLE public_requests DROP CONSTRAINT public_requests_request_type_check;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'public_requests_interaction_mode_check'
    ) THEN
        ALTER TABLE public_requests DROP CONSTRAINT public_requests_interaction_mode_check;
    END IF;
END $$;

ALTER TABLE public_requests
    ADD CONSTRAINT public_requests_request_type_check
    CHECK (request_type IN ('announcement', 'suggestion', 'complaint', 'requirement', 'problem', 'idea'));

ALTER TABLE public_requests
    ADD CONSTRAINT public_requests_interaction_mode_check
    CHECK (interaction_mode IN ('read_only', 'vote_only', 'discussion'));

CREATE INDEX IF NOT EXISTS idx_public_requests_interaction_mode ON public_requests (interaction_mode);
