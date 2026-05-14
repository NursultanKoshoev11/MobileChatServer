ALTER TABLE public_requests ADD COLUMN IF NOT EXISTS hidden_at TIMESTAMPTZ;
ALTER TABLE public_requests ADD COLUMN IF NOT EXISTS hidden_by TEXT REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_public_requests_visible_group_created ON public_requests (group_id, created_at DESC) WHERE hidden_at IS NULL;
