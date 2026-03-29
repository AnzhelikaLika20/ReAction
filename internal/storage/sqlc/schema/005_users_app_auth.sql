-- +goose Up
ALTER TABLE scenarios DROP CONSTRAINT IF EXISTS scenarios_phone_number_fkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_phone_number_fkey;

ALTER TABLE users ADD COLUMN IF NOT EXISTS id UUID DEFAULT gen_random_uuid();
UPDATE users SET id = gen_random_uuid() WHERE id IS NULL;
ALTER TABLE users ALTER COLUMN id SET NOT NULL;

ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE users ALTER COLUMN phone_number DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_number ON users (phone_number) WHERE phone_number IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email)) WHERE email IS NOT NULL;

ALTER TABLE users ADD PRIMARY KEY (id);

ALTER TABLE users ADD CONSTRAINT users_login_identifier CHECK (
    email IS NOT NULL OR phone_number IS NOT NULL
);

ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS user_id UUID;
UPDATE scenarios s SET user_id = u.id FROM users u WHERE s.phone_number = u.phone_number;
DELETE FROM scenarios WHERE user_id IS NULL;
ALTER TABLE scenarios ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE scenarios ADD CONSTRAINT scenarios_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE scenarios DROP COLUMN IF EXISTS phone_number;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS user_id UUID;
UPDATE sessions s SET user_id = u.id FROM users u WHERE s.phone_number = u.phone_number;
DELETE FROM sessions WHERE user_id IS NULL;
ALTER TABLE sessions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE sessions DROP COLUMN IF EXISTS phone_number;
ALTER TABLE sessions ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS phone_number VARCHAR(50);
UPDATE sessions s SET phone_number = u.phone_number FROM users u WHERE s.user_id = u.id AND u.phone_number IS NOT NULL;
ALTER TABLE sessions DROP COLUMN IF EXISTS user_id;
ALTER TABLE sessions ALTER COLUMN phone_number SET NOT NULL;
ALTER TABLE sessions ADD CONSTRAINT sessions_phone_number_fkey FOREIGN KEY (phone_number) REFERENCES users(phone_number) ON DELETE CASCADE;

ALTER TABLE scenarios DROP CONSTRAINT IF EXISTS scenarios_user_id_fkey;
ALTER TABLE scenarios ADD COLUMN IF NOT EXISTS phone_number VARCHAR(50);
UPDATE scenarios s SET phone_number = u.phone_number FROM users u WHERE s.user_id = u.id AND u.phone_number IS NOT NULL;
ALTER TABLE scenarios DROP COLUMN IF EXISTS user_id;
ALTER TABLE scenarios ALTER COLUMN phone_number SET NOT NULL;
ALTER TABLE scenarios ADD CONSTRAINT scenarios_phone_number_fkey FOREIGN KEY (phone_number) REFERENCES users(phone_number) ON DELETE CASCADE;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_login_identifier;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
DROP INDEX IF EXISTS idx_users_email_lower;
DROP INDEX IF EXISTS idx_users_phone_number;

ALTER TABLE users DROP COLUMN IF EXISTS id;
ALTER TABLE users DROP COLUMN IF EXISTS email;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE users ALTER COLUMN phone_number SET NOT NULL;
ALTER TABLE users ADD PRIMARY KEY (phone_number);
