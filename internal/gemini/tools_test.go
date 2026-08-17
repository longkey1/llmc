package gemini

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
	var requests []GeminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req GeminiRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{
				"candidates": [{"content": {"parts": [
					{"functionCall": {"name": "fetch_url", "args": {"url": "https://example.com"}}}
				]}}]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"candidates": [{"content": {"parts": [{"text": "here is the summary"}]}}]
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	history := []llmc.Message{{Role: "user", Content: "summarize https://example.com"}}
	turn, err := p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(turn.ToolCalls))
	}
	call := turn.ToolCalls[0]
	if call.ID != "call-0" || call.Name != "fetch_url" {
		t.Errorf("tool call = %+v", call)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil || args["url"] != "https://example.com" {
		t.Errorf("arguments = %q", call.Arguments)
	}

	// Verify the first request carried the function declaration and system prompt.
	req1 := requests[0]
	if len(req1.Tools) != 1 || len(req1.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools = %+v", req1.Tools)
	}
	if req1.Tools[0].FunctionDeclarations[0].Name != "fetch_url" {
		t.Errorf("declaration = %+v", req1.Tools[0].FunctionDeclarations[0])
	}
	if req1.SystemInstruction == nil || req1.SystemInstruction.Parts[0].Text != "be brief" {
		t.Errorf("system_instruction = %+v", req1.SystemInstruction)
	}

	// Second turn with the model's function call and its result in history.
	history = append(history,
		llmc.Message{Role: "assistant", ToolCalls: []llmc.ToolCall{call}},
		llmc.Message{Role: "tool", Content: "<html>page</html>", ToolCallID: "call-0", ToolName: "fetch_url"},
	)
	turn, err = p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "here is the summary" || len(turn.ToolCalls) != 0 {
		t.Errorf("turn = %+v", turn)
	}

	// Verify wire format: user text, model functionCall, user functionResponse.
	req2 := requests[1]
	if len(req2.Contents) != 3 {
		t.Fatalf("second request has %d contents, want 3", len(req2.Contents))
	}
	model := req2.Contents[1]
	if model.Role != "model" || len(model.Parts) != 1 || model.Parts[0].FunctionCall == nil {
		t.Fatalf("model content = %+v", model)
	}
	if model.Parts[0].FunctionCall.Name != "fetch_url" {
		t.Errorf("functionCall = %+v", model.Parts[0].FunctionCall)
	}
	fnResp := req2.Contents[2]
	if fnResp.Role != "user" || len(fnResp.Parts) != 1 || fnResp.Parts[0].FunctionResponse == nil {
		t.Fatalf("functionResponse content = %+v", fnResp)
	}
	fr := fnResp.Parts[0].FunctionResponse
	if fr.Name != "fetch_url" || fr.Response["output"] != "<html>page</html>" {
		t.Errorf("functionResponse = %+v", fr)
	}
}

func TestChatWithToolsErrorResponseUsesErrorKey(t *testing.T) {
	var gotReq GeminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	history := []llmc.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", ToolCalls: []llmc.ToolCall{{ID: "call-0", Name: "exec_command", Arguments: "{}"}}},
		{Role: "tool", Content: "Execution denied by the user.", ToolCallID: "call-0", ToolName: "exec_command", ToolIsError: true},
	}

	if _, err := p.ChatWithTools(context.Background(), "", history, testToolDefs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fr := gotReq.Contents[2].Parts[0].FunctionResponse
	if fr == nil || fr.Response["error"] != "Execution denied by the user." {
		t.Errorf("functionResponse = %+v, want error key", fr)
	}
}

func TestChatWithToolsWebSearchCoexists(t *testing.T) {
	var gotReq GeminiRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	p.SetWebSearch(true)

	if _, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotReq.Tools) != 2 {
		t.Fatalf("tools = %+v, want google_search + function_declarations", gotReq.Tools)
	}
	if gotReq.Tools[0].GoogleSearch == nil || len(gotReq.Tools[1].FunctionDeclarations) != 1 {
		t.Errorf("tools = %+v", gotReq.Tools)
	}
}

func TestChatWithToolsMixedTextAndCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates": [{"content": {"parts": [
				{"text": "checking"},
				{"functionCall": {"name": "fetch_url", "args": {}}},
				{"functionCall": {"name": "read_file", "args": {}}}
			]}}]
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	turn, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "checking" {
		t.Errorf("text = %q", turn.Text)
	}
	if len(turn.ToolCalls) != 2 || turn.ToolCalls[0].ID != "call-0" || turn.ToolCalls[1].ID != "call-1" {
		t.Errorf("tool calls = %+v, want synthesized sequential IDs", turn.ToolCalls)
	}
}
