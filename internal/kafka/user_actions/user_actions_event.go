package user_actions

import "time"

type UserActionEvent struct {
	SessionID string
	Reminder  *ReminderDetail `json:"reminder,omitempty"`
}

type ReminderDetail struct {
	ReminderID  string    `json:"reminder_id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}
