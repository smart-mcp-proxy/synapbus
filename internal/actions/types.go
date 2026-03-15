package actions

// Action represents a callable operation in the system.
type Action struct {
	Name        string    `json:"name"`
	Category    string    `json:"category"`    // messaging, channels, swarm, attachments
	Description string    `json:"description"`
	Params      []Param   `json:"params"`
	Returns     string    `json:"returns"` // Human-readable return description
	Examples    []Example `json:"examples"`
}

// Param describes an action parameter.
type Param struct {
	Name        string `json:"name"`
	Type        string `json:"type"`        // string, number, boolean
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// Example shows a usage example for the action.
type Example struct {
	Description string `json:"description"`
	Code        string `json:"code"` // JS code example using call()
}

// SearchResult is an action with a relevance score.
type SearchResult struct {
	Action Action  `json:"action"`
	Score  float64 `json:"score"`
}
