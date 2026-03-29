-- +goose Up

DROP INDEX IF EXISTS idx_sessions_messenger_account_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS messenger_account_id;

ALTER TABLE users DROP COLUMN IF EXISTS chats;

DROP INDEX IF EXISTS idx_users_phone_number;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP COLUMN IF EXISTS phone_number;

-- +goose Down
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_number VARCHAR(50);
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_login_identifier;
ALTER TABLE users ADD CONSTRAINT users_login_identifier CHECK (
    email IS NOT NULL OR phone_number IS NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_number ON users (phone_number) WHERE phone_number IS NOT NULL;

ALTER TABLE users ADD COLUMN IF NOT EXISTS chats BIGINT[] DEFAULT '{}';

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS messenger_account_id UUID REFERENCES user_messenger_accounts(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_sessions_messenger_account_id ON sessions (messenger_account_id);
