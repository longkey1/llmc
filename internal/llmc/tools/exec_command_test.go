package tools

import (
	"context"
	"strings"
	"testing"
)

func TestRunExecCommand(t *testing.T) {
	got, err := runExecCommand(context.Background(), `{"command":"echo hello"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "exit code: 0") {
		t.Errorf("got %q, want exit code 0 prefix", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("got %q, want command output", got)
	}
}

func TestRunExecCommandNonZeroExit(t *testing.T) {
	got, err := runExecCommand(context.Background(), `{"command":"exit 3"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "exit code: 3") {
		t.Errorf("got %q, want exit code 3 prefix", got)
	}
}

func TestRunExecCommandCombinesStderr(t *testing.T) {
	got, err := runExecCommand(context.Background(), `{"command":"echo out; echo err >&2"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "out") || !strings.Contains(got, "err") {
		t.Errorf("got %q, want stdout and stderr", got)
	}
}

func TestRunExecCommandContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runExecCommand(ctx, `{"command":"sleep 5"}`, nil)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
