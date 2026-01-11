-- name: CreateUser :one
INSERT INTO users (phone_number, is_active) 
VALUES ($1, $2) 
RETURNING *;

-- name: GetUserByPhone :one
SELECT * FROM users 
WHERE phone_number = $1;

-- name: UpdateUserStatus :exec
UPDATE users 
SET is_active = $1, updated_at = NOW() 
WHERE phone_number = $2;

-- name: GetAllUsers :many
SELECT * FROM users 
ORDER BY created_at DESC;