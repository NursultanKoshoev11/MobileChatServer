-- Replace old invalid group invite codes with the AAA-666 format.
-- Some older groups stored the internal group ID as invite_code, for example G-BFC0723824A867C0.

DO $$
DECLARE
    rec RECORD;
    seq INTEGER := 0;
    candidate TEXT;
    letters CONSTANT TEXT := 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
BEGIN
    FOR rec IN
        SELECT id
        FROM groups
        WHERE invite_code IS NULL
           OR trim(invite_code) = ''
           OR upper(trim(invite_code)) !~ '^[A-Z]{3}-[0-9]{3}$'
        ORDER BY id
    LOOP
        LOOP
            candidate :=
                substr(letters, ((seq / 100000) % 26) + 1, 1) ||
                substr(letters, ((seq / 10000) % 26) + 1, 1) ||
                substr(letters, ((seq / 1000) % 26) + 1, 1) ||
                '-' ||
                lpad((seq % 1000)::TEXT, 3, '0');

            seq := seq + 1;

            EXIT WHEN NOT EXISTS (
                SELECT 1
                FROM groups
                WHERE upper(trim(invite_code)) = candidate
            );
        END LOOP;

        UPDATE groups
        SET invite_code = candidate,
            updated_at = now()
        WHERE id = rec.id;
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_invite_code_unique
ON groups(invite_code)
WHERE invite_code IS NOT NULL AND invite_code <> '';
