-- +goose Up
CREATE TABLE users (
    phone_number VARCHAR(50) PRIMARY KEY,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;