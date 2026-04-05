-- +goose Up
DROP TABLE IF EXISTS sessions;

-- +goose Down
CREATE TABLE sessions (
    token_hash VARCHAR(512) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
