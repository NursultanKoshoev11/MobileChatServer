ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT;

UPDATE users
SET phone = phone_number
WHERE (phone IS NULL OR phone = '')
  AND phone_number IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON users (phone) WHERE phone IS NOT NULL AND phone <> '';
