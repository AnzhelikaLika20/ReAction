-- +goose Up

CREATE TYPE messenger_provider AS ENUM ('telegram');

CREATE TYPE messenger_connection_status AS ENUM ('pending', 'connected');

CREATE TABLE user_messenger_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider messenger_provider NOT NULL,
    label TEXT,

    connection_status messenger_connection_status NOT NULL DEFAULT 'pending',

    selected_chat_ids BIGINT[] NOT NULL DEFAULT '{}',

    connected_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_messenger_accounts_user_id ON user_messenger_accounts (user_id);
CREATE INDEX idx_user_messenger_accounts_user_provider ON user_messenger_accounts (user_id, provider);

-- +goose Down
DROP TABLE IF EXISTS user_messenger_accounts;
DROP TYPE IF EXISTS messenger_connection_status;
DROP TYPE IF EXISTS messenger_provider;
