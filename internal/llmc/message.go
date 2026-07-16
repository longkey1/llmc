package llmc

import "time"

// Message represents a single message in a conversation (for session support)
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When the message was added
}
