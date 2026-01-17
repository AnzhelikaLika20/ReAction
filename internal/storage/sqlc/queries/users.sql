-- name: CreateUser :one
INSERT INTO users (
    phone_number
) VALUES ($1)
RETURNING *;

-- name: GetUserByPhone :one
SELECT * FROM users 
WHERE phone_number = $1;

-- name: UpdateUserLastAuth :exec
UPDATE users 
SET 
    updated_at = NOW()
WHERE phone_number = $1;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE phone_number = $1;