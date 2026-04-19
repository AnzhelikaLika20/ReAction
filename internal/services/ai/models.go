package ai

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type CompletionOptions struct {
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
}

type ContextCheck struct {
	Type        string            `json:"type"` // "promise", "deadline", "intent"
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters"`
}

type UserScenarioForAI struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	TriggerPhrase string `json:"trigger_phrase"`
}

type ExistingReminderForAI struct {
	ScenarioId string `json:"scenario_id"`
	Title      string `json:"title"`
	DateTime   string `json:"datetime"`
}

type ReminderFromAI struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DateTime    string `json:"datetime"`
	EndDateTime string `json:"end_datetime,omitempty"`
}

type CheckResult struct {
	Detected    bool              `json:"detected"`
	Confidence  float64           `json:"confidence"`
	Reason      string            `json:"reason"`
	ScenarioID  string            `json:"scenario_id,omitempty"`
	Reminder    *ReminderFromAI   `json:"reminder,omitempty"`
	ContextType string            `json:"contextType"`
	Metadata    map[string]string `json:"metadata"`
}
