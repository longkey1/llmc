package llmc

// Message represents a single message in a conversation (for session support)
type Message struct {
	Role      string `json:"role"`      // "user" or "assistant"
	Content   string `json:"content"`   // Message content
	Timestamp any    `json:"timestamp"` // time.Time, but use any to avoid import cycle
}
