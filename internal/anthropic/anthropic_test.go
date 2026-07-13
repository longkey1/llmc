package anthropic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

// testConfig implements the Config interface for tests.
type testConfig struct {
	model   string
	baseURL string
	token   string
}

func (c *testConfig) GetModel() string                           { return c.model }
func (c *testConfig) GetBaseURL(provider string) (string, error) { return c.baseURL, nil }
func (c *testConfig) GetToken(provider string) (string, error)   { return c.token, nil }

func newTestProvider(baseURL string) *Provider {
	return NewProvider(&testConfig{
		model:   "anthropic:claude-3-5-sonnet-20241022",
		baseURL: baseURL,
		token:   "test-token",
	})
}

func TestChat(t *testing.T) {
	var gotReq MessagesAPIRequest
	var gotAPIKey, gotVersion, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := MessagesAPIResponse{
			ID:   "msg-1",
			Type: "message",
			Role: "assistant",
			Content: []ResponseContent{
				{Type: "text", Text: "Hello from Claude"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	got, err := p.Chat("hello")
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if got != "Hello from Claude" {
		t.Errorf("Chat() = %q, want %q", got, "Hello from Claude")
	}
	if gotPath != "/messages" {
		t.Errorf("request path = %v, want /messages", gotPath)
	}
	if gotAPIKey != "test-token" {
		t.Errorf("x-api-key header = %v, want test-token", gotAPIKey)
	}
	if gotVersion != AnthropicVersion {
		t.Errorf("anthropic-version header = %v, want %v", gotVersion, AnthropicVersion)
	}
	if gotReq.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("request model = %v, want claude-3-5-sonnet-20241022 (provider prefix stripped)", gotReq.Model)
	}
	if gotReq.MaxTokens != 8192 {
		t.Errorf("request max_tokens = %d, want 8192", gotReq.MaxTokens)
	}
	if len(gotReq.Messages) != 1 {
		t.Fatalf("request messages = %d, want 1", len(gotReq.Messages))
	}
	msg := gotReq.Messages[0]
	if msg.Role != "user" || len(msg.Content) != 1 || msg.Content[0].Type != "text" || msg.Content[0].Text != "hello" {
		t.Errorf("request message = %+v, want user text block with 'hello'", msg)
	}
}

func TestChatJoinsMultipleTextBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := MessagesAPIResponse{
			Content: []ResponseContent{
				{Type: "text", Text: "first"},
				{Type: "tool_use"},
				{Type: "text", Text: "second"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	got, err := p.Chat("hello")
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}
	if got != "first\nsecond" {
		t.Errorf("Chat() = %q, want %q", got, "first\nsecond")
	}
}

func TestChatWebSearchUnsupported(t *testing.T) {
	p := newTestProvider("http://unused.invalid")
	p.SetWebSearch(true)

	if _, err := p.Chat("hello"); err == nil {
		t.Error("Chat() expected error when web search enabled, got nil")
	}
	if _, err := p.ChatWithHistory("", nil, "hello"); err == nil {
		t.Error("ChatWithHistory() expected error when web search enabled, got nil")
	}
}

func TestChatErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		response    string
		wantErrPart string
	}{
		{
			name:        "http error with parsed api error",
			status:      http.StatusBadRequest,
			response:    `{"error":{"type":"invalid_request_error","message":"bad request body"}}`,
			wantErrPart: "API error: bad request body",
		},
		{
			name:        "http error without parseable error",
			status:      http.StatusInternalServerError,
			response:    `oops`,
			wantErrPart: "API request failed (HTTP 500)",
		},
		{
			name:        "api error in 200 response",
			status:      http.StatusOK,
			response:    `{"id":"m1","error":{"type":"overloaded_error","message":"overloaded"}}`,
			wantErrPart: "API error: overloaded",
		},
		{
			name:        "empty content",
			status:      http.StatusOK,
			response:    `{"id":"m1","content":[]}`,
			wantErrPart: "empty response",
		},
		{
			name:        "no text content",
			status:      http.StatusOK,
			response:    `{"id":"m1","content":[{"type":"tool_use"}]}`,
			wantErrPart: "no text content found",
		},
		{
			name:        "invalid json",
			status:      http.StatusOK,
			response:    `{not-json`,
			wantErrPart: "failed to parse API response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if _, err := w.Write([]byte(tt.response)); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			}))
			defer server.Close()

			p := newTestProvider(server.URL)
			_, err := p.Chat("hello")
			if err == nil {
				t.Fatal("Chat() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("Chat() error = %q, want it to contain %q", err.Error(), tt.wantErrPart)
			}
		})
	}
}

func TestChatWithHistory(t *testing.T) {
	var gotReq MessagesAPIRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := MessagesAPIResponse{
			Content: []ResponseContent{
				{Type: "text", Text: "history reply"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	history := []llmc.Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
	}

	got, err := p.ChatWithHistory("be helpful", history, "second question")
	if err != nil {
		t.Fatalf("ChatWithHistory() unexpected error: %v", err)
	}

	if got != "history reply" {
		t.Errorf("ChatWithHistory() = %q, want %q", got, "history reply")
	}
	if gotReq.System != "be helpful" {
		t.Errorf("system prompt = %q, want %q", gotReq.System, "be helpful")
	}

	wantMessages := []struct {
		role string
		text string
	}{
		{"user", "first question"},
		{"assistant", "first answer"},
		{"user", "second question"},
	}
	if len(gotReq.Messages) != len(wantMessages) {
		t.Fatalf("messages length = %d, want %d", len(gotReq.Messages), len(wantMessages))
	}
	for i, want := range wantMessages {
		msg := gotReq.Messages[i]
		if msg.Role != want.role || len(msg.Content) != 1 || msg.Content[0].Text != want.text {
			t.Errorf("messages[%d] = %+v, want role=%s text=%s", i, msg, want.role, want.text)
		}
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("request path = %v, want /models", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-token" {
			t.Errorf("x-api-key header = %v, want test-token", got)
		}
		resp := ModelsAPIResponse{
			Data: []ModelData{
				{ID: "claude-3-5-haiku-20241022", DisplayName: "Claude 3.5 Haiku"},
				{ID: "claude-3-5-sonnet-20241022", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	models, err := p.ListModels()
	if err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("ListModels() = %d models, want 2", len(models))
	}
	// Sorted by ID descending
	if models[0].ID != "claude-3-5-sonnet-20241022" || models[1].ID != "claude-3-5-haiku-20241022" {
		t.Errorf("ListModels() order = [%s %s], want sonnet before haiku", models[0].ID, models[1].ID)
	}
	// Missing display name falls back to created time in JST
	if want := "Created: 2026-01-01 09:00:00 JST"; models[0].Description != want {
		t.Errorf("Description = %q, want %q", models[0].Description, want)
	}
	if models[1].Description != "Claude 3.5 Haiku" {
		t.Errorf("Description = %q, want %q", models[1].Description, "Claude 3.5 Haiku")
	}
}
