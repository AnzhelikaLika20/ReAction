-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_token_hash TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verification_expires_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS email_verification_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verification_token_hash;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
