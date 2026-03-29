package partitionkey

import "strconv"

// separates user_id and chat_id
const sep = "\x00"

func UserChat(userID string, chatID int64) string {
	u := userID
	if u == "" {
		u = "_"
	}
	return u + sep + strconv.FormatInt(chatID, 10)
}

func UserChatBytes(userID string, chatID int64) []byte {
	return []byte(UserChat(userID, chatID))
}
