package session

import (
	"time"

	"github.com/google/uuid"
	"github.com/longkey1/llmc/internal/llmc"
)

// Session represents a conversation session
type Session struct {
	ID           string         `json:"id"`            // UUID v4 (e.g., "550e8400-e29b-41d4-a716-446655440000")
	ParentID     string         `json:"parent_id"`     // Parent session ID (for summarized sessions)
	Name         string         `json:"name"`          // Optional session name (empty by default)
	TemplateName string         `json:"template_name"` // Prompt template name (reference info, can be empty)
	SystemPrompt string         `json:"system_prompt"` // System prompt snapshot (can be empty)
	Provider     string         `json:"provider"`      // Provider name ("openai" or "gemini")
	Model        string         `json:"model"`         // Model name (without provider prefix)
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Messages     []llmc.Message `json:"messages"`
}

// NewSession creates a new session with the given provider and model
func NewSession(provider, model string) *Session {
	now := time.Now()
	return &Session{
		ID:           uuid.New().String(),
		ParentID:     "",
		Name:         "",
		TemplateName: "",
		SystemPrompt: "",
		Provider:     provider,
		Model:        model,
		CreatedAt:    now,
		UpdatedAt:    now,
		Messages:     []llmc.Message{},
	}
}

// AddMessage adds a new message to the session
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, llmc.Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// GetShortID returns the shortened session ID (first 8 characters)
func (s *Session) GetShortID() string {
	if len(s.ID) >= 8 {
		return s.ID[:8]
	}
	return s.ID
}

// GetDisplayName returns the display name for the session
// If name is set, returns the name. Otherwise, returns the short ID.
func (s *Session) GetDisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.GetShortID()
}

// MessageCount returns the number of messages in the session
func (s *Session) MessageCount() int {
	return len(s.Messages)
}
