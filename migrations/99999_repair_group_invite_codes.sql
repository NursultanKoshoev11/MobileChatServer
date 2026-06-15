-- Repair missing or invalid group invite codes to the visible AAA-666 format.
-- This fixes older rows created by approved official-group requests and older compact-code migrations.

ALTER TABLE groups ADD COLUMN IF NOT EXISTS invite_code TEXT;

DO $$
DECLARE
    rec RECORD;
    seq INTEGER := 0;
    candidate TEXT;
    letters CONSTANT TEXT := 'ABCDEFGHJKLMNPQRSTUVWXYZ';
BEGIN
    FOR rec IN
        SELECT id, invite_code
        FROM groups
        WHERE invite_code IS NULL
           OR trim(invite_code) = ''
           OR upper(trim(invite_code)) !~ '^[A-Z]{3}-[0-9]{3}$'
        ORDER BY created_at, id
    LOOP
        IF rec.invite_code IS NOT NULL
           AND upper(trim(rec.invite_code)) ~ '^[A-Z]{3}[0-9]{3}$'
           AND NOT EXISTS (
               SELECT 1 FROM groups
               WHERE upper(trim(invite_code)) = substr(upper(trim(rec.invite_code)), 1, 3) || '-' || substr(upper(trim(rec.invite_code)), 4, 3)
                 AND id <> rec.id
           ) THEN
            candidate := substr(upper(trim(rec.invite_code)), 1, 3) || '-' || substr(upper(trim(rec.invite_code)), 4, 3);
        ELSE
            LOOP
                candidate :=
                    substr(letters, ((seq / 100000) % 26) + 1, 1) ||
                    substr(letters, ((seq / 10000) % 26) + 1, 1) ||
                    substr(letters, ((seq / 1000) % 26) + 1, 1) ||
                    '-' ||
                    lpad((seq % 1000)::TEXT, 3, '0');

                seq := seq + 1;

                EXIT WHEN NOT EXISTS (
                    SELECT 1 FROM groups
                    WHERE upper(trim(invite_code)) = candidate
                      AND id <> rec.id
                );
            END LOOP;
        END IF;

        UPDATE groups
        SET invite_code = candidate,
            updated_at = now()
        WHERE id = rec.id;
    END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_invite_code_unique
ON groups(invite_code)
WHERE invite_code IS NOT NULL AND invite_code <> '';
