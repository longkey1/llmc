package llmc

import "time"

// Message represents a single message in a conversation (for session support)
type Message struct {
	Role    string `json:"role"`    // "user", "assistant" or "tool"
	Content string `json:"content"` // Message content
	// ToolCalls holds tool invocations requested by an assistant message.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a "tool" message to the assistant tool call it answers.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the tool's name on "tool" messages; Gemini matches
	// function responses by name instead of call ID.
	ToolName string `json:"tool_name,omitempty"`
	// ToolIsError marks a "tool" message as a failed execution.
	ToolIsError bool      `json:"tool_is_error,omitempty"`
	Timestamp   time.Time `json:"timestamp"` // When the message was added
}

// SanitizeHistory converts a history that may contain tool-loop messages into
// the plain user/assistant form expected by ChatWithHistory: "tool" messages
// are dropped, assistant tool calls are stripped, and assistant messages left
// with no content are removed. This lets a session recorded with tools
// enabled be continued with tools disabled.
func SanitizeHistory(messages []Message) []Message {
	sanitized := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "tool" {
			continue
		}
		if len(msg.ToolCalls) > 0 {
			if msg.Content == "" {
				continue
			}
			msg.ToolCalls = nil
			msg.ToolCallID = ""
			msg.ToolName = ""
			msg.ToolIsError = false
		}
		sanitized = append(sanitized, msg)
	}
	return sanitized
}
