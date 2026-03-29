-- name: CreateUserWithCredentials :one
INSERT INTO users (
    email,
    password_hash
) VALUES (lower($1), $2)
RETURNING id, email, password_hash, phone_number, is_active, created_at, updated_at, chats;

-- name: CreateUserWithPhone :one
INSERT INTO users (
    phone_number
) VALUES ($1)
RETURNING id, email, password_hash, phone_number, is_active, created_at, updated_at, chats;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, phone_number, is_active, created_at, updated_at, chats FROM users 
WHERE lower(email) = lower($1);

-- name: GetUserByPhone :one
SELECT id, email, password_hash, phone_number, is_active, created_at, updated_at, chats FROM users 
WHERE phone_number = $1;

-- name: GetUserByID :one
SELECT id, email, password_hash, phone_number, is_active, created_at, updated_at, chats FROM users 
WHERE id = $1;

-- name: UpdateUserTelegramPhone :one
UPDATE users 
SET phone_number = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, email, password_hash, phone_number, is_active, created_at, updated_at, chats;

-- name: UpdateUserLastAuth :exec
UPDATE users 
SET updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE id = $1;
