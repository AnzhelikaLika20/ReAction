-- name: CreateSession :one
INSERT INTO sessions (token_hash, phone_number) 
VALUES ($1, $2) 
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions 
WHERE token_hash = $1;

-- name: GetSessionsByPhoneNumber :many
SELECT * FROM sessions 
WHERE phone_number = $1 
ORDER BY created_at DESC;

-- name: DeleteSession :exec
DELETE FROM sessions 
WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions 
WHERE phone_number = $1;

-- name: CleanupOldSessions :exec
DELETE FROM sessions 
WHERE created_at < NOW() - INTERVAL '30 days';