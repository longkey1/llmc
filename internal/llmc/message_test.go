package llmc

import (
	"testing"
)

func TestSanitizeHistory(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "fetch the page"},
		{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "c1", Name: "fetch_url", Arguments: `{"url":"https://example.com"}`}}},
		{Role: "tool", Content: "<html>...</html>", ToolCallID: "c1", ToolName: "fetch_url"},
		{Role: "assistant", Content: "Here is a summary.", ToolCalls: []ToolCall{{ID: "c2", Name: "read_file", Arguments: `{"path":"a.txt"}`}}},
		{Role: "tool", Content: "denied", ToolCallID: "c2", ToolName: "read_file", ToolIsError: true},
		{Role: "assistant", Content: "Done."},
	}

	got := SanitizeHistory(messages)

	want := []struct {
		role    string
		content string
	}{
		{"user", "fetch the page"},
		{"assistant", "Here is a summary."},
		{"assistant", "Done."},
	}

	if len(got) != len(want) {
		t.Fatalf("SanitizeHistory() returned %d messages, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Role != w.role || got[i].Content != w.content {
			t.Errorf("message[%d] = {%q, %q}, want {%q, %q}", i, got[i].Role, got[i].Content, w.role, w.content)
		}
		if len(got[i].ToolCalls) != 0 || got[i].ToolCallID != "" || got[i].ToolName != "" || got[i].ToolIsError {
			t.Errorf("message[%d] still carries tool fields: %+v", i, got[i])
		}
	}
}

func TestSanitizeHistoryPlainMessagesUnchanged(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}

	got := SanitizeHistory(messages)

	if len(got) != 2 {
		t.Fatalf("SanitizeHistory() returned %d messages, want 2", len(got))
	}
	for i := range messages {
		if got[i].Role != messages[i].Role || got[i].Content != messages[i].Content {
			t.Errorf("message[%d] changed: got %+v, want %+v", i, got[i], messages[i])
		}
	}
}
