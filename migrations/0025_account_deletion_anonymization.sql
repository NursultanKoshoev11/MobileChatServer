ALTER TABLE public_requests
    ALTER COLUMN author_id DROP NOT NULL;

ALTER TABLE public_requests
    DROP CONSTRAINT IF EXISTS public_requests_author_id_fkey;

ALTER TABLE public_requests
    ADD CONSTRAINT public_requests_author_id_fkey
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE public_request_comments
    ALTER COLUMN author_id DROP NOT NULL;

ALTER TABLE public_request_comments
    DROP CONSTRAINT IF EXISTS public_request_comments_author_id_fkey;

ALTER TABLE public_request_comments
    ADD CONSTRAINT public_request_comments_author_id_fkey
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL;
