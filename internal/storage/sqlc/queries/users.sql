-- name: CreateUserWithCredentials :one
INSERT INTO users (
    email,
    password_hash
) VALUES (lower($1), $2)
RETURNING id, email, password_hash, is_active, created_at, updated_at, email_verified_at, email_verification_token_hash, email_verification_expires_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, is_active, created_at, updated_at, email_verified_at, email_verification_token_hash, email_verification_expires_at FROM users 
WHERE lower(email) = lower($1);

-- name: GetUserByID :one
SELECT id, email, password_hash, is_active, created_at, updated_at, email_verified_at, email_verification_token_hash, email_verification_expires_at FROM users 
WHERE id = $1;

-- name: UpdateUserLastAuth :exec
UPDATE users 
SET updated_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE id = $1;

-- name: SetUserEmailVerificationToken :exec
UPDATE users
SET email_verification_token_hash = $2,
    email_verification_expires_at = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: VerifyUserEmailByTokenHash :one
UPDATE users
SET email_verified_at = NOW(),
    email_verification_token_hash = NULL,
    email_verification_expires_at = NULL,
    updated_at = NOW()
WHERE email_verification_token_hash = $1
  AND email_verification_expires_at IS NOT NULL
  AND email_verification_expires_at > NOW()
RETURNING id, email, password_hash, is_active, created_at, updated_at, email_verified_at, email_verification_token_hash, email_verification_expires_at;
