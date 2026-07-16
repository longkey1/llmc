package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
		model:   "gemini:gemini-2.0-flash",
		baseURL: baseURL,
		token:   "test-token",
	})
}

func TestExtractGroundingCitations(t *testing.T) {
	tests := []struct {
		name     string
		metadata *GeminiGroundingMetadata
		want     string
	}{
		{
			name:     "no chunks",
			metadata: &GeminiGroundingMetadata{},
			want:     "",
		},
		{
			name: "single chunk",
			metadata: &GeminiGroundingMetadata{
				GroundingChunks: []GeminiGroundingChunk{
					{Web: &GeminiWebChunk{URI: "https://example.com", Title: "Example"}},
				},
			},
			want: "[1] Example - https://example.com",
		},
		{
			name: "duplicate uris deduplicated",
			metadata: &GeminiGroundingMetadata{
				GroundingChunks: []GeminiGroundingChunk{
					{Web: &GeminiWebChunk{URI: "https://example.com", Title: "Example"}},
					{Web: &GeminiWebChunk{URI: "https://example.com", Title: "Duplicate"}},
					{Web: &GeminiWebChunk{URI: "https://other.example.com", Title: "Other"}},
				},
			},
			want: "[1] Example - https://example.com\n[3] Other - https://other.example.com",
		},
		{
			name: "empty title becomes Source",
			metadata: &GeminiGroundingMetadata{
				GroundingChunks: []GeminiGroundingChunk{
					{Web: &GeminiWebChunk{URI: "https://example.com"}},
				},
			},
			want: "[1] Source - https://example.com",
		},
		{
			name: "nil web and empty uri skipped",
			metadata: &GeminiGroundingMetadata{
				GroundingChunks: []GeminiGroundingChunk{
					{Web: nil},
					{Web: &GeminiWebChunk{URI: ""}},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractGroundingCitations(tt.metadata); got != tt.want {
				t.Errorf("extractGroundingCitations() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChat(t *testing.T) {
	var gotReq GeminiRequest
	var gotPath, gotKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiResponseContent{
						Parts: []GeminiResponsePart{{Text: "Hello from Gemini"}},
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

	if got != "Hello from Gemini" {
		t.Errorf("Chat() = %q, want %q", got, "Hello from Gemini")
	}
	if want := "/models/gemini-2.0-flash:generateContent"; gotPath != want {
		t.Errorf("request path = %v, want %v", gotPath, want)
	}
	if gotKey != "test-token" {
		t.Errorf("key query param = %v, want test-token", gotKey)
	}
	if len(gotReq.Contents) != 1 || len(gotReq.Contents[0].Parts) != 1 || gotReq.Contents[0].Parts[0].Text != "hello" {
		t.Errorf("request contents = %+v, want single part with 'hello'", gotReq.Contents)
	}
	if len(gotReq.Tools) != 0 {
		t.Errorf("request tools = %v, want none", gotReq.Tools)
	}
}

func TestChatWebSearch(t *testing.T) {
	var gotReq GeminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiResponseContent{
						Parts: []GeminiResponsePart{{Text: "grounded answer"}},
					},
				},
			},
			GroundingMetadata: &GeminiGroundingMetadata{
				GroundingChunks: []GeminiGroundingChunk{
					{Web: &GeminiWebChunk{URI: "https://example.com", Title: "Src"}},
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

	if len(gotReq.Tools) != 1 || gotReq.Tools[0].GoogleSearch == nil {
		t.Errorf("request tools = %+v, want google_search tool", gotReq.Tools)
	}
	want := "grounded answer\n\n---\nSources:\n[1] Src - https://example.com"
	if got != want {
		t.Errorf("Chat() = %q, want %q", got, want)
	}
}

func TestChatWebSearchEmptyResponseRetryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Known Gemini API issue: grounding metadata but no text parts
		resp := GeminiResponse{
			Candidates: []GeminiCandidate{
				{Content: GeminiResponseContent{Parts: []GeminiResponsePart{}}},
			},
			GroundingMetadata: &GeminiGroundingMetadata{
				WebSearchQueries: []string{"query"},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	p.SetWebSearch(true)

	_, err := p.Chat(context.Background(), "search this")
	if err == nil {
		t.Fatal("Chat() expected error for empty web search response, got nil")
	}
	if !strings.Contains(err.Error(), "web search returned empty response") {
		t.Errorf("Chat() error = %q, want it to mention empty web search response", err.Error())
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
			status:      http.StatusBadRequest,
			response:    `{"error":"bad"}`,
			wantErrPart: "API request failed (HTTP 400)",
		},
		{
			name:        "empty candidates",
			status:      http.StatusOK,
			response:    `{"candidates":[]}`,
			wantErrPart: "no response from API",
		},
		{
			name:        "empty parts",
			status:      http.StatusOK,
			response:    `{"candidates":[{"content":{"parts":[]}}]}`,
			wantErrPart: "no response from API",
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
	var gotReq GeminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiResponseContent{
						Parts: []GeminiResponsePart{{Text: "history reply"}},
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
	if gotReq.SystemInstruction == nil || len(gotReq.SystemInstruction.Parts) != 1 || gotReq.SystemInstruction.Parts[0].Text != "be helpful" {
		t.Errorf("system_instruction = %+v, want single part 'be helpful'", gotReq.SystemInstruction)
	}

	wantContents := []struct {
		role string
		text string
	}{
		{"user", "first question"},
		{"model", "first answer"}, // assistant is mapped to model
		{"user", "second question"},
	}
	if len(gotReq.Contents) != len(wantContents) {
		t.Fatalf("contents length = %d, want %d", len(gotReq.Contents), len(wantContents))
	}
	for i, want := range wantContents {
		c := gotReq.Contents[i]
		if c.Role != want.role || len(c.Parts) != 1 || c.Parts[0].Text != want.text {
			t.Errorf("contents[%d] = %+v, want role=%s text=%s", i, c, want.role, want.text)
		}
	}
}

func TestChatWithHistoryNoSystemPrompt(t *testing.T) {
	var gotReq GeminiRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		resp := GeminiResponse{
			Candidates: []GeminiCandidate{
				{
					Content: GeminiResponseContent{
						Parts: []GeminiResponsePart{{Text: "reply"}},
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
	if _, err := p.ChatWithHistory(context.Background(), "", nil, "question"); err != nil {
		t.Fatalf("ChatWithHistory() unexpected error: %v", err)
	}
	if gotReq.SystemInstruction != nil {
		t.Errorf("system_instruction = %+v, want nil when system prompt is empty", gotReq.SystemInstruction)
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("request path = %v, want /models", r.URL.Path)
		}
		if got := r.URL.Query().Get("key"); got != "test-token" {
			t.Errorf("key query param = %v, want test-token", got)
		}
		resp := ModelsAPIResponse{
			Models: []GeminiModelData{
				{
					Name:                       "models/gemini-2.0-flash",
					Description:                "Fast model",
					SupportedGenerationMethods: []string{"generateContent", "countTokens"},
				},
				{
					Name:                       "models/embedding-001",
					DisplayName:                "Embedding",
					SupportedGenerationMethods: []string{"embedContent"},
				},
				{
					Name:                       "models/gemini-1.5-pro",
					DisplayName:                "Gemini 1.5 Pro",
					SupportedGenerationMethods: []string{"generateContent"},
				},
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

	// embedding-001 is filtered out (no generateContent support)
	if len(models) != 2 {
		t.Fatalf("ListModels() = %d models, want 2", len(models))
	}
	// Sorted by ID descending, "models/" prefix stripped
	if models[0].ID != "gemini-2.0-flash" || models[1].ID != "gemini-1.5-pro" {
		t.Errorf("ListModels() order = [%s %s], want [gemini-2.0-flash gemini-1.5-pro]", models[0].ID, models[1].ID)
	}
	if models[0].Description != "Fast model" {
		t.Errorf("Description = %q, want %q", models[0].Description, "Fast model")
	}
	// Empty description falls back to display name
	if models[1].Description != "Gemini 1.5 Pro" {
		t.Errorf("Description = %q, want %q", models[1].Description, "Gemini 1.5 Pro")
	}
}
