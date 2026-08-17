package ollama

import (
	"context"
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

func newTestProvider(baseURL, token string) *Provider {
	return NewProvider(&testConfig{
		model:   "ollama:llama3",
		baseURL: baseURL,
		token:   token,
	})
}

func TestChat(t *testing.T) {
	var gotReq ChatCompletionsRequest
	var gotAuth, gotPath string
	var gotAuthSet bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, gotAuthSet = r.Header["Authorization"]
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ChatCompletionsResponse{
			ID: "chatcmpl-1",
			Choices: []ChatChoice{
				{Message: ChatMessage{Role: "assistant", Content: "Hello from Ollama"}},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	got, err := p.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if got != "Hello from Ollama" {
		t.Errorf("Chat() = %q, want %q", got, "Hello from Ollama")
	}
	if gotPath != "/chat/completions" {
		t.Errorf("request path = %v, want /chat/completions", gotPath)
	}
	// Token is optional: no Authorization header when the token is empty
	if gotAuthSet {
		t.Errorf("Authorization header = %q, want no header for empty token", gotAuth)
	}
	if gotReq.Model != "llama3" {
		t.Errorf("request model = %v, want llama3 (provider prefix stripped)", gotReq.Model)
	}
	// Chat delegates to ChatWithHistory, so the messages are a single-message array
	wantMessages := []ChatMessage{{Role: "user", Content: "hello"}}
	if len(gotReq.Messages) != 1 || gotReq.Messages[0] != wantMessages[0] {
		t.Errorf("request messages = %v, want %v", gotReq.Messages, wantMessages)
	}
}

func TestChatSendsAuthorizationWhenTokenSet(t *testing.T) {
	var gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := ChatCompletionsResponse{
			Choices: []ChatChoice{
				{Message: ChatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "test-token")
	if _, err := p.Chat(context.Background(), "hello"); err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %v, want Bearer test-token", gotAuth)
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
			name:        "http error status",
			status:      http.StatusNotFound,
			response:    `{"error":{"message":"model 'missing' not found","type":"api_error"}}`,
			wantErrPart: "API request failed (HTTP 404)",
		},
		{
			name:        "empty choices",
			status:      http.StatusOK,
			response:    `{"id":"chatcmpl-1","choices":[]}`,
			wantErrPart: "empty response",
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

			p := newTestProvider(server.URL, "")
			_, err := p.Chat(context.Background(), "hello")
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
	var gotReq ChatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ChatCompletionsResponse{
			Choices: []ChatChoice{
				{Message: ChatMessage{Role: "assistant", Content: "history reply"}},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	history := []llmc.Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
	}

	got, err := p.ChatWithHistory(context.Background(), "be helpful", history, "second question")
	if err != nil {
		t.Fatalf("ChatWithHistory() unexpected error: %v", err)
	}

	if got != "history reply" {
		t.Errorf("ChatWithHistory() = %q, want %q", got, "history reply")
	}
	// System prompt is sent as the leading "system" role message
	wantMessages := []ChatMessage{
		{Role: "system", Content: "be helpful"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
	}
	if len(gotReq.Messages) != len(wantMessages) {
		t.Fatalf("messages length = %d, want %d", len(gotReq.Messages), len(wantMessages))
	}
	for i, want := range wantMessages {
		if gotReq.Messages[i] != want {
			t.Errorf("messages[%d] = %+v, want %+v", i, gotReq.Messages[i], want)
		}
	}
}

func TestChatWithHistoryOmitsEmptySystemPrompt(t *testing.T) {
	var gotReq ChatCompletionsRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ChatCompletionsResponse{
			Choices: []ChatChoice{
				{Message: ChatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	if _, err := p.ChatWithHistory(context.Background(), "", nil, "hello"); err != nil {
		t.Fatalf("ChatWithHistory() unexpected error: %v", err)
	}

	if len(gotReq.Messages) != 1 || gotReq.Messages[0].Role != "user" {
		t.Errorf("messages = %v, want a single user message without system message", gotReq.Messages)
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("request path = %v, want /models", r.URL.Path)
		}
		resp := ModelsAPIResponse{
			Data: []ModelData{
				{ID: "llama3:latest", Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "library"},
				{ID: "qwen3:latest", Created: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).Unix(), OwnedBy: "library"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("ListModels() = %d models, want 2", len(models))
	}
	// Sorted by ID descending
	if models[0].ID != "qwen3:latest" || models[1].ID != "llama3:latest" {
		t.Errorf("ListModels() order = [%s %s], want [qwen3:latest llama3:latest]", models[0].ID, models[1].ID)
	}
	// Created timestamp rendered in JST
	if want := "Modified: 2026-02-01 09:00:00 JST"; models[0].Description != want {
		t.Errorf("Description = %q, want %q", models[0].Description, want)
	}
}

func TestListModelsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	if _, err := p.ListModels(context.Background()); err == nil {
		t.Error("ListModels() expected error for HTTP 500, got nil")
	}
}
