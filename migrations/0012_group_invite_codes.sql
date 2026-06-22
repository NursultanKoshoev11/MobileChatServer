ALTER TABLE groups ADD COLUMN IF NOT EXISTS invite_code TEXT;

DO $$
DECLARE
    rec record;
    candidate text;
    attempts integer;
BEGIN
    FOR rec IN
        SELECT id, invite_code
        FROM groups
        WHERE invite_code IS NULL
           OR trim(invite_code) = ''
           OR upper(trim(invite_code)) !~ '^[A-Z]{3}-[0-9]{3}$'
    LOOP
        candidate := NULL;
        IF rec.invite_code IS NOT NULL
           AND upper(trim(rec.invite_code)) ~ '^[A-Z]{3}[0-9]{3}$' THEN
            candidate := substr(upper(trim(rec.invite_code)), 1, 3) || '-' || substr(upper(trim(rec.invite_code)), 4, 3);
        END IF;

        attempts := 0;
        WHILE candidate IS NULL
           OR EXISTS (SELECT 1 FROM groups WHERE upper(trim(invite_code)) = candidate AND id <> rec.id)
        LOOP
            attempts := attempts + 1;
            candidate :=
                substr('ABCDEFGHJKLMNPQRSTUVWXYZ', (floor(random() * 24)::int) + 1, 1) ||
                substr('ABCDEFGHJKLMNPQRSTUVWXYZ', (floor(random() * 24)::int) + 1, 1) ||
                substr('ABCDEFGHJKLMNPQRSTUVWXYZ', (floor(random() * 24)::int) + 1, 1) || '-' ||
                substr('23456789', (floor(random() * 8)::int) + 1, 1) ||
                substr('23456789', (floor(random() * 8)::int) + 1, 1) ||
                substr('23456789', (floor(random() * 8)::int) + 1, 1);
            IF attempts > 100 THEN
                RAISE EXCEPTION 'could not generate unique invite code for group %', rec.id;
            END IF;
        END LOOP;

        UPDATE groups
        SET invite_code = candidate,
            updated_at = now()
        WHERE id = rec.id;
    END LOOP;
END $$;

DROP INDEX IF EXISTS idx_groups_invite_code;
CREATE UNIQUE INDEX IF NOT EXISTS idx_groups_invite_code_unique
ON groups(invite_code)
WHERE invite_code IS NOT NULL AND invite_code <> '';
