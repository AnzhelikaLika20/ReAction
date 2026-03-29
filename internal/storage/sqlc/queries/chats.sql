-- name: GetMessengerAccountSelectedChats :one
SELECT selected_chat_ids FROM user_messenger_accounts
WHERE id = $1 AND user_id = $2;

-- name: UpdateMessengerAccountSelectedChats :exec
UPDATE user_messenger_accounts
SET selected_chat_ids = $3,
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;

-- name: AddChatToMessengerAccount :exec
UPDATE user_messenger_accounts
SET selected_chat_ids = array_append(selected_chat_ids, $3),
    updated_at = NOW()
WHERE id = $1 AND user_id = $2;

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
