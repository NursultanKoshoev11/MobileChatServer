DO $$
DECLARE
    group_row RECORD;
    alphabet TEXT := 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    candidate TEXT;
    i INT;
BEGIN
    FOR group_row IN
        SELECT id
        FROM groups
        WHERE invite_code IS NULL
           OR btrim(invite_code) = ''
           OR length(btrim(invite_code)) <> 6
    LOOP
        LOOP
            candidate := '';
            FOR i IN 1..6 LOOP
                candidate := candidate || substr(alphabet, 1 + floor(random() * length(alphabet))::int, 1);
            END LOOP;

            IF NOT EXISTS (
                SELECT 1
                FROM groups
                WHERE upper(invite_code) = upper(candidate)
                  AND id <> group_row.id
            ) THEN
                UPDATE groups
                SET invite_code = candidate,
                    updated_at = now()
                WHERE id = group_row.id;
                EXIT;
            END IF;
        END LOOP;
    END LOOP;
END $$;
