package openai

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

func newTestProvider(baseURL string) *Provider {
	return NewProvider(&testConfig{
		model:   "openai:gpt-4",
		baseURL: baseURL,
		token:   "test-token",
	})
}

func TestExtractCitations(t *testing.T) {
	tests := []struct {
		name        string
		annotations []ResponsesAPIAnnotation
		want        string
	}{
		{
			name:        "no annotations",
			annotations: nil,
			want:        "",
		},
		{
			name: "single citation",
			annotations: []ResponsesAPIAnnotation{
				{Type: "url_citation", Title: "Example", URL: "https://example.com"},
			},
			want: "[1] Example - https://example.com",
		},
		{
			name: "duplicate urls deduplicated",
			annotations: []ResponsesAPIAnnotation{
				{Type: "url_citation", Title: "Example", URL: "https://example.com"},
				{Type: "url_citation", Title: "Example again", URL: "https://example.com"},
				{Type: "url_citation", Title: "Other", URL: "https://other.example.com"},
			},
			want: "[1] Example - https://example.com\n[2] Other - https://other.example.com",
		},
		{
			name: "empty title becomes Source",
			annotations: []ResponsesAPIAnnotation{
				{Type: "url_citation", URL: "https://example.com"},
			},
			want: "[1] Source - https://example.com",
		},
		{
			name: "non url_citation and empty url skipped",
			annotations: []ResponsesAPIAnnotation{
				{Type: "file_citation", Title: "File", URL: "https://file.example.com"},
				{Type: "url_citation", Title: "No URL", URL: ""},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractCitations(tt.annotations); got != tt.want {
				t.Errorf("extractCitations() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChat(t *testing.T) {
	var gotReq ResponsesAPIRequest
	var gotAuth, gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ResponsesAPIResponse{
			ID:     "resp-1",
			Status: "completed",
			Output: []ResponsesAPIOutput{
				{
					Type: "message",
					Content: []ResponsesAPIContent{
						{Text: "Hello from OpenAI"},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	got, err := p.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if got != "Hello from OpenAI" {
		t.Errorf("Chat() = %q, want %q", got, "Hello from OpenAI")
	}
	if gotPath != "/responses" {
		t.Errorf("request path = %v, want /responses", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %v, want Bearer test-token", gotAuth)
	}
	if gotReq.Model != "gpt-4" {
		t.Errorf("request model = %v, want gpt-4 (provider prefix stripped)", gotReq.Model)
	}
	// Chat delegates to ChatWithHistory, so the input is a single-message array
	wantInput := []InputItem{{Role: "user", Content: "hello"}}
	if len(gotReq.Input) != 1 || gotReq.Input[0] != wantInput[0] {
		t.Errorf("request input = %v, want %v", gotReq.Input, wantInput)
	}
	if len(gotReq.Tools) != 0 {
		t.Errorf("request tools = %v, want none", gotReq.Tools)
	}
}

func TestChatWebSearchAddsTool(t *testing.T) {
	var gotReq ResponsesAPIRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ResponsesAPIResponse{
			Output: []ResponsesAPIOutput{
				{Type: "web_search_call"},
				{
					Type: "message",
					Content: []ResponsesAPIContent{
						{
							Text: "answer",
							Annotations: []ResponsesAPIAnnotation{
								{Type: "url_citation", Title: "Src", URL: "https://example.com"},
							},
						},
					},
				},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	p.SetWebSearch(true)

	got, err := p.Chat(context.Background(), "search this")
	if err != nil {
		t.Fatalf("Chat() unexpected error: %v", err)
	}

	if len(gotReq.Tools) != 1 || gotReq.Tools[0].Type != "web_search" {
		t.Errorf("request tools = %v, want [{web_search}]", gotReq.Tools)
	}
	want := "answer\n\n---\nSources:\n[1] Src - https://example.com"
	if got != want {
		t.Errorf("Chat() = %q, want %q", got, want)
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
			status:      http.StatusUnauthorized,
			response:    `{}`,
			wantErrPart: "API request failed (HTTP 401)",
		},
		{
			name:        "api error in response",
			status:      http.StatusOK,
			response:    `{"id":"r1","status":"failed","error":{"code":"bad","message":"something broke"}}`,
			wantErrPart: "API error: something broke",
		},
		{
			name:        "empty output",
			status:      http.StatusOK,
			response:    `{"id":"r1","status":"completed","output":[]}`,
			wantErrPart: "empty response",
		},
		{
			name:        "no message output",
			status:      http.StatusOK,
			response:    `{"id":"r1","status":"completed","output":[{"type":"web_search_call"}]}`,
			wantErrPart: "no message found",
		},
		{
			name:        "message without content",
			status:      http.StatusOK,
			response:    `{"id":"r1","status":"completed","output":[{"type":"message"}]}`,
			wantErrPart: "message has no content",
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
	var gotBody struct {
		Model        string             `json:"model"`
		Instructions string             `json:"instructions"`
		Input        []InputItem        `json:"input"`
		Tools        []ResponsesAPITool `json:"tools"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := ResponsesAPIResponse{
			Output: []ResponsesAPIOutput{
				{
					Type: "message",
					Content: []ResponsesAPIContent{
						{Text: "history reply"},
					},
				},
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

	got, err := p.ChatWithHistory(context.Background(), "be helpful", history, "second question")
	if err != nil {
		t.Fatalf("ChatWithHistory() unexpected error: %v", err)
	}

	if got != "history reply" {
		t.Errorf("ChatWithHistory() = %q, want %q", got, "history reply")
	}
	if gotBody.Instructions != "be helpful" {
		t.Errorf("instructions = %q, want %q", gotBody.Instructions, "be helpful")
	}
	wantInput := []InputItem{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
	}
	if len(gotBody.Input) != len(wantInput) {
		t.Fatalf("input length = %d, want %d", len(gotBody.Input), len(wantInput))
	}
	for i, want := range wantInput {
		if gotBody.Input[i] != want {
			t.Errorf("input[%d] = %+v, want %+v", i, gotBody.Input[i], want)
		}
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("request path = %v, want /models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %v, want Bearer test-token", got)
		}
		resp := ModelsAPIResponse{
			Data: []ModelData{
				{ID: "gpt-4", Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()},
				{ID: "gpt-4.1", Created: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).Unix()},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() unexpected error: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("ListModels() = %d models, want 2", len(models))
	}
	// Sorted by ID descending
	if models[0].ID != "gpt-4.1" || models[1].ID != "gpt-4" {
		t.Errorf("ListModels() order = [%s %s], want [gpt-4.1 gpt-4]", models[0].ID, models[1].ID)
	}
	// Created timestamp rendered in JST
	if want := "Created: 2026-02-01 09:00:00 JST"; models[0].Description != want {
		t.Errorf("Description = %q, want %q", models[0].Description, want)
	}
}

func TestListModelsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	if _, err := p.ListModels(context.Background()); err == nil {
		t.Error("ListModels() expected error for HTTP 500, got nil")
	}
}
