
-- +goose Up
CREATE TABLE sessions (
    token_hash VARCHAR(512) PRIMARY KEY,
    phone_number VARCHAR(50) NOT NULL REFERENCES users(phone_number) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE sessions;