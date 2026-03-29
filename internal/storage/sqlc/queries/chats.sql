-- name: GetMessengerAccountSelectedChats :one
SELECT selected_chat_ids FROM user_messenger_accounts
WHERE id = $1 AND user_id = $2;

-- name: UpdateMessengerAccountSelectedChats :exec
UPDATE user_messenger_accounts
SET selected_chat_ids = $3,
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: AddChatToMessengerAccount :exec
UPDATE user_messenger_accounts AS u
SET selected_chat_ids = CASE
    WHEN sqlc.arg(chat_id)::bigint = ANY (COALESCE(u.selected_chat_ids, '{}')) THEN u.selected_chat_ids
    ELSE array_append(u.selected_chat_ids, sqlc.arg(chat_id)::bigint)
END,
    updated_at = NOW()
WHERE u.id = sqlc.arg(id) AND u.user_id = sqlc.arg(user_id);

-- name: RemoveChatFromMessengerAccount :exec
UPDATE user_messenger_accounts
SET selected_chat_ids = array_remove(selected_chat_ids, $3),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: ClearMessengerAccountChats :exec
UPDATE user_messenger_accounts
SET selected_chat_ids = '{}',
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;
