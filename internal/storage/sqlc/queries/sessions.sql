-- name: CreateSession :one
INSERT INTO sessions (
    token_hash,
    user_id
) VALUES ($1, $2)
RETURNING token_hash, user_id, created_at;

-- name: GetSession :one
SELECT token_hash, user_id, created_at FROM sessions 
WHERE token_hash = $1;

-- name: GetSessionByUserID :one
SELECT token_hash, user_id, created_at FROM sessions 
WHERE user_id = $1 
ORDER BY created_at DESC 
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions 
WHERE token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions 
WHERE user_id = $1;
