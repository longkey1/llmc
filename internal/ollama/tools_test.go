package ollama

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
	var requests []ChatCompletionsRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		requests = append(requests, req)

		w.Header().Set("Content-Type", "application/json")
		if len(requests) == 1 {
			_, _ = w.Write([]byte(`{
				"id": "chat_1",
				"choices": [{
					"message": {
						"role": "assistant", "content": "",
						"tool_calls": [{"id": "call_abc", "type": "function", "function": {"name": "fetch_url", "arguments": "{\"url\":\"https://example.com\"}"}}]
					},
					"finish_reason": "tool_calls"
				}]
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"id": "chat_2",
			"choices": [{"message": {"role": "assistant", "content": "here is the summary"}, "finish_reason": "stop"}]
		}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")

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

	// Verify the first request carried the tool definition and system prompt.
	req1 := requests[0]
	if len(req1.Tools) != 1 || req1.Tools[0].Type != "function" || req1.Tools[0].Function.Name != "fetch_url" {
		t.Errorf("tools = %+v", req1.Tools)
	}
	if req1.Messages[0].Role != "system" || req1.Messages[0].Content != "be brief" {
		t.Errorf("messages[0] = %+v, want system prompt", req1.Messages[0])
	}

	// Second turn with the assistant tool call and its result in history.
	history = append(history,
		llmc.Message{Role: "assistant", ToolCalls: []llmc.ToolCall{call}},
		llmc.Message{Role: "tool", Content: "<html>page</html>", ToolCallID: "call_abc", ToolName: "fetch_url"},
	)
	turn, err = p.ChatWithTools(context.Background(), "be brief", history, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "here is the summary" || len(turn.ToolCalls) != 0 {
		t.Errorf("turn = %+v", turn)
	}

	// Verify wire format: system, user, assistant(tool_calls), tool.
	req2 := requests[1]
	if len(req2.Messages) != 4 {
		t.Fatalf("second request has %d messages, want 4", len(req2.Messages))
	}
	assistant := req2.Messages[2]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("assistant message = %+v", assistant)
	}
	if assistant.ToolCalls[0].ID != "call_abc" || assistant.ToolCalls[0].Function.Name != "fetch_url" {
		t.Errorf("assistant tool_calls = %+v", assistant.ToolCalls)
	}
	toolMsg := req2.Messages[3]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "call_abc" || toolMsg.Content != "<html>page</html>" {
		t.Errorf("tool message = %+v", toolMsg)
	}
}

func TestChatWithToolsPlainAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c","choices":[{"message":{"role":"assistant","content":"direct"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	p := newTestProvider(server.URL, "")
	turn, err := p.ChatWithTools(context.Background(), "", []llmc.Message{{Role: "user", Content: "hi"}}, testToolDefs())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turn.Text != "direct" || len(turn.ToolCalls) != 0 {
		t.Errorf("turn = %+v", turn)
	}
}
