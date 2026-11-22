-- Records every group file written to disk by POST /api/groups/{id}/files.
-- Used for the per-user daily upload quota and to delete a user's files when
-- the account is deleted. Rows follow their group and uploader (ON DELETE
-- CASCADE); the files on disk are removed by the application. Files uploaded
-- before this migration have no row.
CREATE TABLE IF NOT EXISTS uploaded_files (
    id TEXT PRIMARY KEY,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    uploader_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    content_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_uploaded_files_uploader_created
    ON uploaded_files (uploader_id, created_at);
CREATE INDEX IF NOT EXISTS idx_uploaded_files_group
    ON uploaded_files (group_id);
