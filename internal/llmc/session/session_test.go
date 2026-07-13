package session

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := NewSession("openai:gpt-4")

	if len(s.ID) != 36 {
		t.Errorf("ID length = %d, want 36 (UUID)", len(s.ID))
	}
	if s.Model != "openai:gpt-4" {
		t.Errorf("Model = %v, want %v", s.Model, "openai:gpt-4")
	}
	if s.ParentID != "" || s.Name != "" || s.TemplateName != "" || s.SystemPrompt != "" {
		t.Error("new session should have empty optional fields")
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		t.Error("timestamps should be set")
	}
	if len(s.Messages) != 0 {
		t.Errorf("Messages length = %d, want 0", len(s.Messages))
	}
}

func TestAddMessage(t *testing.T) {
	s := NewSession("openai:gpt-4")
	before := s.UpdatedAt

	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi there")

	if s.MessageCount() != 2 {
		t.Fatalf("MessageCount() = %d, want 2", s.MessageCount())
	}
	if s.Messages[0].Role != "user" || s.Messages[0].Content != "hello" {
		t.Errorf("Messages[0] = %+v, want role=user content=hello", s.Messages[0])
	}
	if s.Messages[1].Role != "assistant" || s.Messages[1].Content != "hi there" {
		t.Errorf("Messages[1] = %+v, want role=assistant content=hi there", s.Messages[1])
	}
	if s.UpdatedAt.Before(before) {
		t.Error("UpdatedAt should not go backwards after AddMessage")
	}
}

func TestGetShortID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "full uuid",
			id:   "550e8400-e29b-41d4-a716-446655440000",
			want: "550e8400",
		},
		{
			name: "exactly 8 characters",
			id:   "12345678",
			want: "12345678",
		},
		{
			name: "shorter than 8 characters",
			id:   "abc",
			want: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ID: tt.id}
			if got := s.GetShortID(); got != tt.want {
				t.Errorf("GetShortID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		id          string
		want        string
	}{
		{
			name:        "name set",
			sessionName: "my-session",
			id:          "550e8400-e29b-41d4-a716-446655440000",
			want:        "my-session",
		},
		{
			name:        "name empty falls back to short id",
			sessionName: "",
			id:          "550e8400-e29b-41d4-a716-446655440000",
			want:        "550e8400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ID: tt.id, Name: tt.sessionName}
			if got := s.GetDisplayName(); got != tt.want {
				t.Errorf("GetDisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionGetProviderAndModelName(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantModel    string
	}{
		{
			name:         "valid model string",
			model:        "anthropic:claude-3-5-sonnet-20241022",
			wantProvider: "anthropic",
			wantModel:    "claude-3-5-sonnet-20241022",
		},
		{
			name:         "invalid model string",
			model:        "invalid",
			wantProvider: "",
			wantModel:    "invalid", // GetModelName falls back to raw model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Model: tt.model}
			if got := s.GetProvider(); got != tt.wantProvider {
				t.Errorf("GetProvider() = %v, want %v", got, tt.wantProvider)
			}
			if got := s.GetModelName(); got != tt.wantModel {
				t.Errorf("GetModelName() = %v, want %v", got, tt.wantModel)
			}
		})
	}
}
