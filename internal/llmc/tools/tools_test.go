package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/longkey1/llmc/internal/llmc"
)

func TestDefinitions(t *testing.T) {
	defs := Definitions(nil)
	if len(defs) != 4 {
		t.Fatalf("Definitions() returned %d tools, want 4", len(defs))
	}

	wantConfirmation := map[string]bool{
		NameFetchURL:    false,
		NameReadFile:    false,
		NameExecCommand: true,
		NameWriteFile:   true,
	}
	for _, def := range defs {
		want, ok := wantConfirmation[def.Name]
		if !ok {
			t.Errorf("unexpected tool %q", def.Name)
			continue
		}
		if def.RequiresConfirmation != want {
			t.Errorf("%s: RequiresConfirmation = %v, want %v", def.Name, def.RequiresConfirmation, want)
		}
		if def.Description == "" {
			t.Errorf("%s: empty description", def.Name)
		}
		if def.Parameters["type"] != "object" {
			t.Errorf("%s: parameters type = %v, want object", def.Name, def.Parameters["type"])
		}
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	e := &Executor{Confirm: nil, AutoApprove: false, Progress: nil}
	result := e.Execute(context.Background(), llmc.ToolCall{ID: "c1", Name: "no_such_tool", Arguments: "{}"})

	if !result.IsError {
		t.Error("expected IsError for unknown tool")
	}
	if result.ToolCallID != "c1" {
		t.Errorf("ToolCallID = %q, want c1", result.ToolCallID)
	}
	if !strings.Contains(result.Content, "unknown tool") {
		t.Errorf("Content = %q, want to contain 'unknown tool'", result.Content)
	}
}

func TestExecuteInvalidArguments(t *testing.T) {
	e := &Executor{Confirm: nil, AutoApprove: false, Progress: nil}
	result := e.Execute(context.Background(), llmc.ToolCall{ID: "c1", Name: NameReadFile, Arguments: "not json"})

	if !result.IsError {
		t.Error("expected IsError for invalid arguments")
	}
	if !strings.Contains(result.Content, "invalid tool arguments") {
		t.Errorf("Content = %q, want to contain 'invalid tool arguments'", result.Content)
	}
}

func TestExecuteConfirmationNilConfirmDenies(t *testing.T) {
	e := &Executor{Confirm: nil, AutoApprove: false, Progress: nil}
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"echo should-not-run"}`,
	})

	if !result.IsError {
		t.Error("expected IsError when confirmation is unavailable")
	}
	if !strings.Contains(result.Content, "not a TTY") {
		t.Errorf("Content = %q, want non-TTY denial message", result.Content)
	}
}

func TestExecuteConfirmationDeniedByUser(t *testing.T) {
	e := &Executor{
		Confirm:     func(call llmc.ToolCall, summary string) bool { return false },
		AutoApprove: false,
		Progress:    nil,
	}
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"echo should-not-run"}`,
	})

	if !result.IsError {
		t.Error("expected IsError when user denies")
	}
	if result.Content != "Execution denied by the user." {
		t.Errorf("Content = %q, want user denial message", result.Content)
	}
}

func TestExecuteConfirmationApproved(t *testing.T) {
	var gotSummary string
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			gotSummary = summary
			return true
		},
		AutoApprove: false,
		Progress:    nil,
	}
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"echo hello"}`,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Errorf("Content = %q, want command output", result.Content)
	}
	if !strings.Contains(gotSummary, "echo hello") {
		t.Errorf("summary = %q, want to contain the command", gotSummary)
	}
}

func TestExecuteAutoApproveSkipsConfirm(t *testing.T) {
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			t.Error("Confirm should not be called with AutoApprove")
			return false
		},
		AutoApprove: true,
		Progress:    nil,
	}
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"echo auto"}`,
	})

	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "auto") {
		t.Errorf("Content = %q, want command output", result.Content)
	}
}
