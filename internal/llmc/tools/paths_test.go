package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/longkey1/llmc/internal/llmc"
)

func TestMatchesPathRulesPrefix(t *testing.T) {
	// matchesPathRules takes a canonical path; on macOS the temp dir sits
	// under the /var -> /private/var symlink, so resolve it first.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rules := PathRules{filepath.Join(dir, "allowed")}

	tests := []struct {
		path string
		want bool
	}{
		{filepath.Join(dir, "allowed"), true},
		{filepath.Join(dir, "allowed", "a.txt"), true},
		{filepath.Join(dir, "allowed", "deep", "b.txt"), true},
		{filepath.Join(dir, "other", "a.txt"), false},
		// A sibling sharing the rule's name prefix must not match: this is
		// why containment uses filepath.Rel and not a string prefix.
		{filepath.Join(dir, "allowed-other", "a.txt"), false},
	}
	for _, tt := range tests {
		if got := matchesPathRules(tt.path, rules); got != tt.want {
			t.Errorf("matchesPathRules(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMatchesPathRulesComponent(t *testing.T) {
	rules := PathRules{".git", "*.pem", ".env"}

	tests := []struct {
		path string
		want bool
	}{
		{"/proj/.git/config", true},
		{"/proj/sub/.git", true},
		{"/proj/key.pem", true},
		{"/proj/.env", true},
		{"/proj/main.go", false},
		{"/proj/gitignore", false},
		{"/proj/pemfile", false},
	}
	for _, tt := range tests {
		if got := matchesPathRules(tt.path, rules); got != tt.want {
			t.Errorf("matchesPathRules(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestResolvePathCleansTraversal(t *testing.T) {
	dir := t.TempDir()
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	got, err := resolvePath(filepath.Join(dir, "a", "..", "b.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(resolvedDir, "b.txt"); got != want {
		t.Errorf("resolvePath() = %q, want %q", got, want)
	}
}

func TestResolvePathResolvesSymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	got, err := resolvePath(filepath.Join(link, "file.txt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resolvedReal, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(resolvedReal, "file.txt"); got != want {
		t.Errorf("resolvePath() = %q, want %q", got, want)
	}
}

func TestPathDecisionTraversalCannotEscapeDenyRule(t *testing.T) {
	dir := t.TempDir()
	secrets := filepath.Join(dir, "secrets")
	if err := os.Mkdir(secrets, 0o755); err != nil {
		t.Fatal(err)
	}
	denied := PathRules{secrets}

	// The literal string doesn't look like it touches secrets/, but the
	// canonical path does.
	sneaky := filepath.Join(dir, "work", "..", "secrets", "token")
	if got := pathDecision(sneaky, nil, denied, UnlistedConfirm); got != decisionDeny {
		t.Errorf("pathDecision(%q) = %v, want decisionDeny", sneaky, got)
	}
}

func TestPathDecisionSymlinkedParentCannotEscapeDenyRule(t *testing.T) {
	dir := t.TempDir()
	secrets := filepath.Join(dir, "secrets")
	if err := os.Mkdir(secrets, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "innocent")
	if err := os.Symlink(secrets, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	target := filepath.Join(link, "token")
	if got := pathDecision(target, nil, PathRules{secrets}, UnlistedConfirm); got != decisionDeny {
		t.Errorf("pathDecision(%q) = %v, want decisionDeny", target, got)
	}
}

func TestPathDecisionPrecedence(t *testing.T) {
	dir := t.TempDir()
	allowed := PathRules{dir}
	denied := PathRules{".env"}

	if got := pathDecision(filepath.Join(dir, "main.go"), allowed, denied, UnlistedConfirm); got != decisionAllow {
		t.Errorf("allowed path = %v, want decisionAllow", got)
	}
	// Deny wins inside an allowed directory.
	if got := pathDecision(filepath.Join(dir, ".env"), allowed, denied, UnlistedConfirm); got != decisionDeny {
		t.Errorf("denied path inside allowed dir = %v, want decisionDeny", got)
	}
	// Outside the allow rules: confirm by default, refuse in deny mode.
	outside := filepath.Join(t.TempDir(), "x")
	if got := pathDecision(outside, allowed, denied, UnlistedConfirm); got != decisionConfirm {
		t.Errorf("unlisted path = %v, want decisionConfirm", got)
	}
	if got := pathDecision(outside, allowed, denied, UnlistedDeny); got != decisionDeny {
		t.Errorf("unlisted path in deny mode = %v, want decisionDeny", got)
	}
}

func TestRunWriteFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(real, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := runWriteFile(context.Background(), `{"path":"`+link+`","content":"new"}`, nil)
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("err = %v, want symlink refusal", err)
	}

	content, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Errorf("target was modified through the symlink: %q", content)
	}
}

func TestExecuteWriteFileDeniedPath(t *testing.T) {
	dir := t.TempDir()
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			t.Error("Confirm should not be called for a denied path")
			return true
		},
		AutoApprove: true,
		Options:     &Options{WriteDeniedPaths: PathRules{".env"}},
		Progress:    nil,
	}

	target := filepath.Join(dir, ".env")
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameWriteFile, Arguments: `{"path":"` + target + `","content":"SECRET=1"}`,
	})
	if !result.IsError {
		t.Fatal("expected denial for a denied write path")
	}
	if !strings.Contains(result.Content, "writing to this path is not permitted") {
		t.Errorf("Content = %q, want write denial message", result.Content)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("the file was written despite the deny rule")
	}
}

func TestExecuteWriteFileAllowedPathSkipsConfirmation(t *testing.T) {
	dir := t.TempDir()
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			t.Error("Confirm should not be called for an allowed path")
			return false
		},
		AutoApprove: false,
		Options:     &Options{WriteAllowedPaths: PathRules{dir}},
		Progress:    nil,
	}

	target := filepath.Join(dir, "out.txt")
	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameWriteFile, Arguments: `{"path":"` + target + `","content":"ok"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if content, err := os.ReadFile(target); err != nil || string(content) != "ok" {
		t.Errorf("file content = %q (err %v), want ok", content, err)
	}
}

func TestExecuteReadFileDeniedPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.pem")
	if err := os.WriteFile(target, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}

	e := &Executor{
		Confirm:     nil,
		AutoApprove: false,
		Options:     &Options{ReadDeniedPaths: PathRules{"*.pem"}},
		Progress:    nil,
	}

	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameReadFile, Arguments: `{"path":"` + target + `"}`,
	})
	if !result.IsError {
		t.Fatal("expected denial for a denied read path")
	}
	if strings.Contains(result.Content, "PRIVATE KEY") {
		t.Error("the denied file's content leaked into the result")
	}
}

func TestExecuteReadFileUnlistedPathRuns(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &Executor{
		Confirm:     nil,
		AutoApprove: false,
		Options:     &Options{ReadDeniedPaths: PathRules{"*.pem"}},
		Progress:    nil,
	}

	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameReadFile, Arguments: `{"path":"` + target + `"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content != "hello" {
		t.Errorf("Content = %q, want hello", result.Content)
	}
}

func TestDefinitionsOmitsWriteFileWhenUnusable(t *testing.T) {
	hasWrite := func(defs []llmc.ToolDef) bool {
		for _, def := range defs {
			if def.Name == NameWriteFile {
				return true
			}
		}
		return false
	}

	if hasWrite(Definitions(&Options{WriteUnlisted: UnlistedDeny})) {
		t.Error("write_file should be omitted when deny mode allows no path")
	}
	if !hasWrite(Definitions(&Options{WriteUnlisted: UnlistedDeny, WriteAllowedPaths: PathRules{"/tmp"}})) {
		t.Error("write_file should be advertised when deny mode allows a path")
	}
	if !hasWrite(Definitions(nil)) {
		t.Error("write_file should be advertised by default")
	}
}
