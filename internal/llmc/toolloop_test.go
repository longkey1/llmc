package llmc

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeToolChatter returns scripted TurnResults and records the messages it
// received on each call.
type fakeToolChatter struct {
	turns    []*TurnResult
	calls    int
	received [][]Message
}

func (f *fakeToolChatter) ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []ToolDef) (*TurnResult, error) {
	snapshot := make([]Message, len(messages))
	copy(snapshot, messages)
	f.received = append(f.received, snapshot)

	if f.calls >= len(f.turns) {
		return nil, fmt.Errorf("unexpected call %d", f.calls)
	}
	turn := f.turns[f.calls]
	f.calls++
	return turn, nil
}

// fakeExecutor echoes the call name back as the result content.
type fakeExecutor struct {
	executed []ToolCall
}

func (f *fakeExecutor) Execute(ctx context.Context, call ToolCall) ToolResult {
	f.executed = append(f.executed, call)
	return ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    "result of " + call.Name,
		IsError:    false,
	}
}

func userMessage(content string) Message {
	return Message{Role: "user", Content: content, ToolCalls: nil, ToolCallID: "", ToolName: "", ToolIsError: false}
}

func TestRunToolLoopNoToolCalls(t *testing.T) {
	chatter := &fakeToolChatter{turns: []*TurnResult{{Text: "direct answer", ToolCalls: nil}}}
	exec := &fakeExecutor{}

	text, appended, err := RunToolLoop(context.Background(), chatter, "sys",
		[]Message{userMessage("hi")}, nil, exec, LoopHooks{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "direct answer" {
		t.Errorf("text = %q, want direct answer", text)
	}
	if len(appended) != 1 || appended[0].Role != "assistant" || appended[0].Content != "direct answer" {
		t.Errorf("appended = %+v, want single assistant message", appended)
	}
	if len(exec.executed) != 0 {
		t.Errorf("executor called %d times, want 0", len(exec.executed))
	}
}

func TestRunToolLoopSingleRound(t *testing.T) {
	chatter := &fakeToolChatter{turns: []*TurnResult{
		{Text: "", ToolCalls: []ToolCall{{ID: "c1", Name: "fetch_url", Arguments: `{"url":"x"}`}}},
		{Text: "final answer", ToolCalls: nil},
	}}
	exec := &fakeExecutor{}

	text, appended, err := RunToolLoop(context.Background(), chatter, "sys",
		[]Message{userMessage("go")}, nil, exec, LoopHooks{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "final answer" {
		t.Errorf("text = %q, want final answer", text)
	}

	wantRoles := []string{"assistant", "tool", "assistant"}
	if len(appended) != len(wantRoles) {
		t.Fatalf("appended %d messages, want %d: %+v", len(appended), len(wantRoles), appended)
	}
	for i, role := range wantRoles {
		if appended[i].Role != role {
			t.Errorf("appended[%d].Role = %q, want %q", i, appended[i].Role, role)
		}
	}
	if appended[1].ToolCallID != "c1" || appended[1].ToolName != "fetch_url" {
		t.Errorf("tool message = %+v, want call ID c1", appended[1])
	}

	// The second request must include the assistant tool call and its result.
	second := chatter.received[1]
	if len(second) != 3 {
		t.Fatalf("second request has %d messages, want 3", len(second))
	}
	if second[1].Role != "assistant" || len(second[1].ToolCalls) != 1 {
		t.Errorf("second[1] = %+v, want assistant with tool call", second[1])
	}
	if second[2].Role != "tool" || second[2].Content != "result of fetch_url" {
		t.Errorf("second[2] = %+v, want tool result", second[2])
	}
}

func TestRunToolLoopParallelCalls(t *testing.T) {
	chatter := &fakeToolChatter{turns: []*TurnResult{
		{Text: "", ToolCalls: []ToolCall{
			{ID: "c1", Name: "fetch_url", Arguments: "{}"},
			{ID: "c2", Name: "read_file", Arguments: "{}"},
		}},
		{Text: "done", ToolCalls: nil},
	}}
	exec := &fakeExecutor{}

	_, appended, err := RunToolLoop(context.Background(), chatter, "",
		[]Message{userMessage("go")}, nil, exec, LoopHooks{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exec.executed) != 2 {
		t.Fatalf("executor called %d times, want 2", len(exec.executed))
	}

	wantRoles := []string{"assistant", "tool", "tool", "assistant"}
	if len(appended) != len(wantRoles) {
		t.Fatalf("appended %d messages, want %d", len(appended), len(wantRoles))
	}
	if appended[1].ToolCallID != "c1" || appended[2].ToolCallID != "c2" {
		t.Errorf("tool results out of order: %+v", appended[1:3])
	}
}

func TestRunToolLoopExceedsIterations(t *testing.T) {
	// Every turn requests another tool call, so the loop can never finish.
	turns := make([]*TurnResult, MaxToolIterations)
	for i := range turns {
		turns[i] = &TurnResult{Text: "", ToolCalls: []ToolCall{{ID: fmt.Sprintf("c%d", i), Name: "fetch_url", Arguments: "{}"}}}
	}
	chatter := &fakeToolChatter{turns: turns}
	exec := &fakeExecutor{}

	_, _, err := RunToolLoop(context.Background(), chatter, "",
		[]Message{userMessage("go")}, nil, exec, LoopHooks{})
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Errorf("err = %v, want iteration limit error", err)
	}
	if chatter.calls != MaxToolIterations {
		t.Errorf("made %d requests, want %d", chatter.calls, MaxToolIterations)
	}
}

func TestRunToolLoopHooks(t *testing.T) {
	chatter := &fakeToolChatter{turns: []*TurnResult{
		{Text: "", ToolCalls: []ToolCall{{ID: "c1", Name: "fetch_url", Arguments: "{}"}}},
		{Text: "done", ToolCalls: nil},
	}}
	exec := &fakeExecutor{}

	var requests, requestDones, toolCalls, toolDones int
	hooks := LoopHooks{
		OnRequest: func() func() {
			requests++
			return func() { requestDones++ }
		},
		OnToolCall: func(call ToolCall) { toolCalls++ },
		OnToolDone: func(call ToolCall, result ToolResult) { toolDones++ },
	}

	_, _, err := RunToolLoop(context.Background(), chatter, "",
		[]Message{userMessage("go")}, nil, exec, hooks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 2 || requestDones != 2 {
		t.Errorf("OnRequest = %d/%d, want 2/2", requests, requestDones)
	}
	if toolCalls != 1 || toolDones != 1 {
		t.Errorf("OnToolCall/OnToolDone = %d/%d, want 1/1", toolCalls, toolDones)
	}
}

func TestRunToolLoopDoesNotMutateHistory(t *testing.T) {
	chatter := &fakeToolChatter{turns: []*TurnResult{
		{Text: "", ToolCalls: []ToolCall{{ID: "c1", Name: "fetch_url", Arguments: "{}"}}},
		{Text: "done", ToolCalls: nil},
	}}
	history := make([]Message, 0, 8) // spare capacity to catch aliasing appends
	history = append(history, userMessage("go"))

	_, _, err := RunToolLoop(context.Background(), chatter, "", history, nil, &fakeExecutor{}, LoopHooks{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("history length changed to %d", len(history))
	}
}
