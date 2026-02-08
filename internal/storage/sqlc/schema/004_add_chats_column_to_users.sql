-- +goose Up
ALTER TABLE users 
ADD COLUMN IF NOT EXISTS chats BIGINT[] DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS chats;