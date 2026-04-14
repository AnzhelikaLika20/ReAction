package chat_updates

type ChatUpdateMessageEvent struct {
	UserID    string `json:"user_id,omitempty"`
	SessionID string `json:"session_id"`
	EventType string `json:"event_type"` // "message_new", "message_sent", "message_edited"
	ChatID    int64  `json:"chat_id"`
	ChatTitle string `json:"chat_title,omitempty"`
	Text      string `json:"text"`
	SenderID  int64  `json:"sender_id"`
	IsOutgoing bool  `json:"is_outgoing"`
	Timestamp int64  `json:"timestamp"`
}
