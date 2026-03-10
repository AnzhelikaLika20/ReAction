-- name: GetUserChats :one
SELECT chats FROM users WHERE phone_number = $1;

-- name: UpdateUserChats :exec
UPDATE users 
SET chats = $2, 
    updated_at = NOW() 
WHERE phone_number = $1;

-- name: AddChatToUser :exec
UPDATE users 
SET chats = array_append(chats, $2),
    updated_at = NOW()
WHERE phone_number = $1;

-- name: RemoveChatFromUser :exec
UPDATE users 
SET chats = array_remove(chats, $2),
    updated_at = NOW()
WHERE phone_number = $1;

-- name: ClearUserChats :exec
UPDATE users 
SET chats = '{}',
    updated_at = NOW()
WHERE phone_number = $1;