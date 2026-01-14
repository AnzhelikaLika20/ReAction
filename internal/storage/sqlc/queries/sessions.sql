-- name: CreateSession :one
INSERT INTO sessions (
    token_hash,
    phone_number
) VALUES ($1, $2)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions 
WHERE token_hash = $1;

-- name: GetSessionByPhone :one
SELECT * FROM sessions 
WHERE phone_number = $1 
ORDER BY created_at DESC 
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions 
WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions 
WHERE phone_number = $1;
