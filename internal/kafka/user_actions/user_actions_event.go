package user_actions

import "time"

type UserActionEvent struct {
	SessionID  string `json:"session_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	ScenarioID string `json:"scenario_id,omitempty"`
	ChatID     int64  `json:"chat_id,omitempty"`
	Reminder   *ReminderDetail `json:"reminder,omitempty"`
}

type ReminderDetail struct {
	ReminderID  string    `json:"reminder_id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Date        time.Time `json:"date"`
	EndDate     time.Time `json:"end_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
