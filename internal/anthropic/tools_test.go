package anthropic

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
	var requests []MessagesAPIRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req MessagesAPIRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{
				"id": "msg_1", "type": "message", "role": "assistant",
				"content": [
					{"type": "text", "text": "let me fetch that"},
					{"type": "tool_use", "id": "toolu_abc", "name": "fetch_url", "input": {"url": "https://example.com"}}
				],
				"stop_reason": "tool_use"
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "msg_2", "type": "message", "role": "assistant",
			"content": [{"type": "text", "text": "here is the summary"}],
			"stop_reason": "end_turn"
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)

	history := []llmc.Message{{Role: "user", Content: "summarize https://example.com"}}
	turn, err := p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "let me fetch that" {
		t.Errorf("text = %q", turn.Text)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(turn.ToolCalls))
	}
	call := turn.ToolCalls[0]
	if call.ID != "toolu_abc" || call.Name != "fetch_url" {
		t.Errorf("tool call = %+v", call)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil || args["url"] != "https://example.com" {
		t.Errorf("arguments = %q", call.Arguments)
	}

	// Verify the first request carried the tool definition.
	req1 := requests[0]
	if len(req1.Tools) != 1 || req1.Tools[0].Name != "fetch_url" {
		t.Errorf("tools = %+v", req1.Tools)
	}
	if req1.Tools[0].InputSchema["type"] != "object" {
		t.Errorf("input_schema not forwarded: %+v", req1.Tools[0].InputSchema)
	}
	if req1.System != "be brief" {
		t.Errorf("system = %q", req1.System)
	}

	// Second turn with the assistant tool_use and its result in history.
	history = append(history,
		llmc.Message{Role: "assistant", Content: "let me fetch that", ToolCalls: []llmc.ToolCall{call}},
		llmc.Message{Role: "tool", Content: "<html>page</html>", ToolCallID: "toolu_abc", ToolName: "fetch_url"},
	)
	turn, err = p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "here is the summary" || len(turn.ToolCalls) != 0 {
		t.Errorf("turn = %+v", turn)
	}

	// Verify wire format: user, assistant(text+tool_use), user(tool_result).
	req2 := requests[1]
	if len(req2.Messages) != 3 {
		t.Fatalf("second request has %d messages, want 3", len(req2.Messages))
	}
	assistant := req2.Messages[1]
	if assistant.Role != "assistant" || len(assistant.Content) != 2 {
		t.Fatalf("assistant message = %+v", assistant)
	}
	if assistant.Content[0].Type != "text" || assistant.Content[1].Type != "tool_use" {
		t.Errorf("assistant blocks = %s, %s", assistant.Content[0].Type, assistant.Content[1].Type)
	}
	if assistant.Content[1].ID != "toolu_abc" {
		t.Errorf("tool_use id = %q", assistant.Content[1].ID)
	}
	toolResult := req2.Messages[2]
	if toolResult.Role != "user" || len(toolResult.Content) != 1 {
		t.Fatalf("tool result message = %+v", toolResult)
	}
	block := toolResult.Content[0]
	if block.Type != "tool_result" || block.ToolUseID != "toolu_abc" || block.ResultContent != "<html>page</html>" {
		t.Errorf("tool_result block = %+v", block)
	}
}

func TestChatWithToolsMergesParallelResults(t *testing.T) {
	var gotReq MessagesAPIRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn"}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL)
	history := []llmc.Message{
		{Role: "user", Content: "go"},
		{Role: "assistant", ToolCalls: []llmc.ToolCall{
			{ID: "t1", Name: "fetch_url", Arguments: "{}"},
			{ID: "t2", Name: "read_file", Arguments: "{}"},
		}},
		{Role: "tool", Content: "r1", ToolCallID: "t1", ToolName: "fetch_url"},
		{Role: "tool", Content: "r2", ToolCallID: "t2", ToolName: "read_file", ToolIsError: true},
	}

	if _, err := p.ChatWithTools(context.Background(), "", history, testToolDefs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two consecutive tool results must be merged into one user message so
	// that roles keep alternating.
	if len(gotReq.Messages) != 3 {
		t.Fatalf("got %d messages, want 3: %+v", len(gotReq.Messages), gotReq.Messages)
	}
	merged := gotReq.Messages[2]
	if merged.Role != "user" || len(merged.Content) != 2 {
		t.Fatalf("merged message = %+v", merged)
	}
	if merged.Content[0].ToolUseID != "t1" || merged.Content[1].ToolUseID != "t2" {
		t.Errorf("tool_use_ids = %q, %q", merged.Content[0].ToolUseID, merged.Content[1].ToolUseID)
	}
	if merged.Content[0].IsError || !merged.Content[1].IsError {
		t.Errorf("is_error flags = %v, %v", merged.Content[0].IsError, merged.Content[1].IsError)
	}
}

func TestChatWithToolsWebSearchRejected(t *testing.T) {
	p := newTestProvider("http://unused")
	p.SetWebSearch(true)
	_, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err == nil {
		t.Error("expected error when web search is enabled")
	}
}
