ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';

UPDATE users
SET phone = phone_number
WHERE (phone IS NULL OR phone = '')
  AND phone_number IS NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_role_check'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('user', 'platform_admin', 'super_admin'));
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON users (phone) WHERE phone IS NOT NULL AND phone <> '';
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);

CREATE TABLE IF NOT EXISTS admin_phone_allowlist (
    phone TEXT PRIMARY KEY,
    role TEXT NOT NULL CHECK (role IN ('platform_admin', 'super_admin')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_creation_requests (
    id TEXT PRIMARY KEY,
    requester_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    applicant_name TEXT NOT NULL,
    position TEXT NOT NULL,
    organization_name TEXT NOT NULL,
    organization_type TEXT NOT NULL,
    region TEXT NOT NULL,
    official_phone TEXT NOT NULL,
    official_email TEXT NOT NULL,
    website TEXT NOT NULL DEFAULT '',
    group_title TEXT NOT NULL,
    group_description TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL,
    documents TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'needs_more_info')),
    admin_comment TEXT NOT NULL DEFAULT '',
    created_group_id TEXT REFERENCES groups(id) ON DELETE SET NULL,
    reviewed_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_group_creation_requests_requester ON group_creation_requests (requester_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_group_creation_requests_status ON group_creation_requests (status, created_at DESC);
