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

type CheckResult struct {
	Detected    bool              `json:"detected"`
	Confidence  float64           `json:"confidence"`
	Reason      string            `json:"reason"`
	ContextType string            `json:"contextType"`
	Metadata    map[string]string `json:"metadata"`
}
