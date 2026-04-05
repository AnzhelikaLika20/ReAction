-- name: ListMessengerAccountsByUserID :many
SELECT id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at
FROM user_messenger_accounts
WHERE user_id = $1
ORDER BY provider, created_at;

-- name: GetMessengerAccountByID :one
SELECT id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at
FROM user_messenger_accounts
WHERE id = $1;

-- name: GetMessengerAccountByIDForUser :one
SELECT id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at
FROM user_messenger_accounts
WHERE id = $1 AND user_id = $2;

-- name: GetLatestConnectedTelegramLabelByUserID :one
SELECT label FROM user_messenger_accounts
WHERE user_id = $1 AND connection_status = 'connected'::messenger_connection_status
ORDER BY connected_at DESC NULLS LAST, created_at DESC
LIMIT 1;

-- name: InsertMessengerAccount :one
INSERT INTO user_messenger_accounts (
    user_id,
    provider,
    label,
    connection_status,
    connected_at
) VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at;

-- name: UpdateMessengerAccountStatus :one
UPDATE user_messenger_accounts
SET
    connection_status = $3,
    connected_at = $4,
    updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at;

-- name: UpdateMessengerAccountLabel :one
UPDATE user_messenger_accounts
SET
    label = $3,
    updated_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, provider, label, connection_status, selected_chat_ids, connected_at, created_at, updated_at;

-- name: DeleteMessengerAccountForUser :one
DELETE FROM user_messenger_accounts
WHERE id = $1 AND user_id = $2
RETURNING id;
