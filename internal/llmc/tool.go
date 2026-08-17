package llmc

// ToolDef describes a tool that the model can call. It is provider-neutral;
// each provider translates it into its own wire format.
type ToolDef struct {
	Name        string
	Description string
	// Parameters is a JSON Schema object (type: "object") describing the
	// tool's arguments.
	Parameters map[string]any
	// RequiresConfirmation marks tools whose execution must be confirmed
	// by the user before running (e.g. shell commands, file writes).
	RequiresConfirmation bool
}

// ToolCall is a request from the model to invoke a tool.
type ToolCall struct {
	// ID correlates the call with its result. Providers that don't issue
	// call IDs (Gemini) get a synthesized one.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Arguments is the raw JSON-encoded argument object as produced by
	// the model.
	Arguments string `json:"arguments"`
}

// ToolResult is the outcome of executing a single tool call.
type ToolResult struct {
	ToolCallID string
	Name       string
	Content    string
	// IsError marks failures (including user denial); the result is still
	// returned to the model so it can react.
	IsError bool
}

// TurnResult is one model response within a tool loop: optional text plus
// zero or more tool calls. An empty ToolCalls slice means the turn is final.
type TurnResult struct {
	Text      string
	ToolCalls []ToolCall
}
