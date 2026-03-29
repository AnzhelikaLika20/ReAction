-- name: CreateUserWithCredentials :one
INSERT INTO users (
    email,
    password_hash
) VALUES (lower($1), $2)
RETURNING id, email, password_hash, is_active, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, is_active, created_at, updated_at FROM users 
WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT id, email, password_hash, is_active, created_at, updated_at FROM users 
WHERE id = $1;

-- name: UpdateUserLastAuth :exec
UPDATE users 
SET updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE id = $1;
