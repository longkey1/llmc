package cmd

import (
	"context"
	"testing"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/spf13/cobra"
)

// fakeProvider implements llmc.Provider without tool support.
type fakeProvider struct {
	gotHistory []llmc.Message
	response   string
}

func (f *fakeProvider) Chat(ctx context.Context, message string) (string, error) {
	return f.response, nil
}

func (f *fakeProvider) ChatWithHistory(ctx context.Context, systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	f.gotHistory = messages
	return f.response, nil
}

func (f *fakeProvider) SetWebSearch(enabled bool)             {}
func (f *fakeProvider) SetIgnoreWebSearchErrors(enabled bool) {}
func (f *fakeProvider) SetDebug(enabled bool)                 {}
func (f *fakeProvider) ListModels(ctx context.Context) ([]llmc.ModelInfo, error) {
	return nil, nil
}

// fakeToolProvider adds ChatWithTools returning scripted turns.
type fakeToolProvider struct {
	fakeProvider
	turns []*llmc.TurnResult
	calls int
}

func (f *fakeToolProvider) ChatWithTools(ctx context.Context, systemPrompt string, messages []llmc.Message, tools []llmc.ToolDef) (*llmc.TurnResult, error) {
	turn := f.turns[f.calls]
	f.calls++
	return turn, nil
}

func TestRunTurnToolsDisabledSanitizesHistory(t *testing.T) {
	p := &fakeProvider{response: "answer"}
	history := []llmc.Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", ToolCalls: []llmc.ToolCall{{ID: "c1", Name: "fetch_url", Arguments: "{}"}}},
		{Role: "tool", Content: "result", ToolCallID: "c1", ToolName: "fetch_url"},
		{Role: "assistant", Content: "a1"},
	}

	response, appended, err := runTurn(context.Background(), p, history, "q2", turnConfig{systemPrompt: "sys"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "answer" {
		t.Errorf("response = %q", response)
	}
	if len(appended) != 1 || appended[0].Role != "assistant" || appended[0].Content != "answer" {
		t.Errorf("appended = %+v", appended)
	}

	// Tool-loop messages must be stripped before hitting ChatWithHistory.
	if len(p.gotHistory) != 2 {
		t.Fatalf("provider received %d history messages, want 2: %+v", len(p.gotHistory), p.gotHistory)
	}
	if p.gotHistory[0].Content != "q1" || p.gotHistory[1].Content != "a1" {
		t.Errorf("sanitized history = %+v", p.gotHistory)
	}
}

func TestRunTurnToolsEnabledUnsupportedProvider(t *testing.T) {
	p := &fakeProvider{response: "x"}
	_, _, err := runTurn(context.Background(), p, nil, "hi", turnConfig{toolsEnabled: true})
	if err == nil {
		t.Error("expected error for provider without tool support")
	}
}

func TestRunTurnToolsEnabledRunsLoop(t *testing.T) {
	p := &fakeToolProvider{turns: []*llmc.TurnResult{
		{Text: "", ToolCalls: []llmc.ToolCall{{ID: "c1", Name: "read_file", Arguments: `{"path":"/no/such/file"}`}}},
		{Text: "done", ToolCalls: nil},
	}}

	response, appended, err := runTurn(context.Background(), p, nil, "go", turnConfig{toolsEnabled: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response != "done" {
		t.Errorf("response = %q", response)
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
	// read_file on a missing path yields an error tool result, not a failure.
	if !appended[1].ToolIsError {
		t.Errorf("tool message = %+v, want ToolIsError", appended[1])
	}
}

func newToolsTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	cmd.Flags().BoolVar(&enableTools, "tools", false, "")
	return cmd
}

func TestResolveToolsPriority(t *testing.T) {
	origEnableTools := enableTools
	t.Cleanup(func() { enableTools = origEnableTools })

	boolPtr := func(b bool) *bool { return &b }

	t.Run("default is config value", func(t *testing.T) {
		cmd := newToolsTestCommand()
		cfg := &config.Config{EnableTools: true}
		if !resolveTools(cmd, cfg, nil) {
			t.Error("want config value true")
		}
	})

	t.Run("template overrides config", func(t *testing.T) {
		cmd := newToolsTestCommand()
		cfg := &config.Config{EnableTools: true}
		if resolveTools(cmd, cfg, boolPtr(false)) {
			t.Error("want template value false")
		}
	})

	t.Run("env overrides template", func(t *testing.T) {
		t.Setenv("LLMC_ENABLE_TOOLS", "true")
		cmd := newToolsTestCommand()
		cfg := &config.Config{EnableTools: false}
		if !resolveTools(cmd, cfg, boolPtr(false)) {
			t.Error("want env value true")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		t.Setenv("LLMC_ENABLE_TOOLS", "true")
		cmd := newToolsTestCommand()
		_ = cmd.Flags().Set("tools", "false")
		cfg := &config.Config{EnableTools: true}
		if resolveTools(cmd, cfg, boolPtr(true)) {
			t.Error("want flag value false")
		}
	})
}
