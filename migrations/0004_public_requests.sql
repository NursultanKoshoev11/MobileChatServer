CREATE TABLE IF NOT EXISTS public_requests (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_type TEXT NOT NULL CHECK (request_type IN ('suggestion', 'complaint', 'requirement', 'problem', 'idea')),
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'under_review', 'accepted', 'rejected', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_public_requests_group_created ON public_requests (group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_public_requests_author_created ON public_requests (author_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_public_requests_status ON public_requests (status, created_at DESC);

CREATE TABLE IF NOT EXISTS public_request_votes (
    request_id TEXT NOT NULL REFERENCES public_requests(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vote_type TEXT NOT NULL CHECK (vote_type IN ('support', 'oppose')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (request_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_public_request_votes_user ON public_request_votes (user_id);

CREATE TABLE IF NOT EXISTS public_request_comments (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL REFERENCES public_requests(id) ON DELETE CASCADE,
    author_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_public_request_comments_request_created ON public_request_comments (request_id, created_at ASC);
