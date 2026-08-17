package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/longkey1/llmc/internal/llmc"
)

func testToolDefs() []llmc.ToolDef {
	return []llmc.ToolDef{
		{
			Name:        "fetch_url",
			Description: "Fetch a URL",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"url": map[string]any{"type": "string"}},
				"required":   []string{"url"},
			},
			RequiresConfirmation: false,
		},
	}
}

func TestChatWithToolsRequestsAndLoop(t *testing.T) {
	var requests []ResponsesAPIRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ResponsesAPIRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			// First request: the model asks to call fetch_url.
			_, _ = w.Write([]byte(`{
				"id": "resp_1", "status": "completed",
				"output": [
					{"type": "function_call", "call_id": "call_abc", "name": "fetch_url", "arguments": "{\"url\":\"https://example.com\"}"}
				]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "resp_2", "status": "completed",
			"output": [
				{"type": "message", "content": [{"text": "summary of the page"}]}
			]
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	// First turn: user message only.
	history := []llmc.Message{{Role: "user", Content: "summarize https://example.com"}}
	turn, err := p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(turn.ToolCalls))
	}
	call := turn.ToolCalls[0]
	if call.ID != "call_abc" || call.Name != "fetch_url" {
		t.Errorf("tool call = %+v", call)
	}

	// Verify the first request carried the function tool definition.
	req1 := requests[0]
	if req1.Instructions != "be brief" {
		t.Errorf("instructions = %q", req1.Instructions)
	}
	if len(req1.Tools) != 1 || req1.Tools[0].Type != "function" || req1.Tools[0].Name != "fetch_url" {
		t.Errorf("tools = %+v, want one function tool", req1.Tools)
	}
	if req1.Tools[0].Parameters["type"] != "object" {
		t.Errorf("parameters not forwarded: %+v", req1.Tools[0].Parameters)
	}

	// Second turn: history now includes the assistant tool call and its result.
	history = append(history,
		llmc.Message{Role: "assistant", ToolCalls: []llmc.ToolCall{call}},
		llmc.Message{Role: "tool", Content: "<html>page</html>", ToolCallID: "call_abc", ToolName: "fetch_url"},
	)
	turn, err = p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "summary of the page" || len(turn.ToolCalls) != 0 {
		t.Errorf("turn = %+v, want final text", turn)
	}

	// Verify the second request's wire format.
	req2 := requests[1]
	wantInput := []InputItem{
		{Role: "user", Content: "summarize https://example.com"},
		{Type: "function_call", CallID: "call_abc", Name: "fetch_url", Arguments: `{"url":"https://example.com"}`},
		{Type: "function_call_output", CallID: "call_abc", Output: "<html>page</html>"},
	}
	if len(req2.Input) != len(wantInput) {
		t.Fatalf("second request input = %+v, want %d items", req2.Input, len(wantInput))
	}
	for i, want := range wantInput {
		if req2.Input[i] != want {
			t.Errorf("input[%d] = %+v, want %+v", i, req2.Input[i], want)
		}
	}
}

func TestChatWithToolsWebSearchCoexists(t *testing.T) {
	var gotReq ResponsesAPIRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r","status":"completed","output":[{"type":"message","content":[{"text":"ok"}]}]}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	p.SetWebSearch(true)

	_, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotReq.Tools) != 2 {
		t.Fatalf("tools = %+v, want web_search + function", gotReq.Tools)
	}
	if gotReq.Tools[0].Type != "web_search" || gotReq.Tools[1].Type != "function" {
		t.Errorf("tool types = %s, %s", gotReq.Tools[0].Type, gotReq.Tools[1].Type)
	}
}

func TestChatWithToolsTextAlongsideCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "r", "status": "completed",
			"output": [
				{"type": "message", "content": [{"text": "let me check"}]},
				{"type": "function_call", "call_id": "c1", "name": "fetch_url", "arguments": "{}"}
			]
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	turn, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "let me check" || len(turn.ToolCalls) != 1 {
		t.Errorf("turn = %+v, want text and one call", turn)
	}
}

func TestChatWithToolsEmptyOutputError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r","status":"completed","output":[]}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	_, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err == nil {
		t.Error("expected error for empty output")
	}
}
