ALTER TABLE public_requests
    ADD COLUMN IF NOT EXISTS interaction_mode TEXT NOT NULL DEFAULT 'discussion'
    CHECK (interaction_mode IN ('read_only', 'vote_only', 'discussion'));
