package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/longkey1/llmc/internal/llmc"
)

func TestCommandDecision(t *testing.T) {
	allowed := CommandRules{
		"git": {"status", "diff"},
		"ls":  {},
		"go":  {"test", "build"},
		"cat": {"*"},
	}
	denied := CommandRules{
		"rm":   {},
		"sudo": {"*"},
		"npm":  {"publish"},
	}

	tests := []struct {
		name    string
		command string
		want    decision
	}{
		{"empty subcommand list covers any invocation", "ls", decisionAllow},
		{"empty subcommand list covers arguments too", "ls -la /tmp", decisionAllow},
		{"star covers any invocation", "cat go.mod", decisionAllow},
		{"listed subcommand", "git status", decisionAllow},
		{"listed subcommand with flags", "git status --short", decisionAllow},
		{"second listed subcommand", "git diff HEAD", decisionAllow},
		{"unlisted subcommand confirms", "git push origin main", decisionConfirm},
		{"bare command with a subcommand list confirms", "git", decisionConfirm},
		{"unlisted command confirms", "curl https://example.com", decisionConfirm},
		{"denied command", "rm -rf /tmp/x", decisionDeny},
		{"denied via star", "sudo reboot", decisionDeny},
		{"denied only for the listed subcommand", "npm publish", decisionDeny},
		{"other subcommands of a denied command still confirm", "npm install", decisionConfirm},
		{"denied wins over allowed in the same line", "ls && rm -rf /", decisionDeny},
		{"denied anywhere in a pipeline", "ls | sudo tee /etc/x", decisionDeny},
		{"all segments allowed", "ls && git status", decisionAllow},
		{"one unlisted segment confirms", "ls && curl https://example.com", decisionConfirm},
		{"command name must match exactly", "lsof", decisionConfirm},
		{"subcommand boundary is respected", "go testing", decisionConfirm},
		{"leading assignment is skipped", "GIT_PAGER=cat git status", decisionAllow},
		{"quoted separator is not a split", `ls "a; b"`, decisionAllow},
		{"command substitution never auto-approves", "ls $(rm -rf /)", decisionConfirm},
		{"backtick substitution never auto-approves", "ls `whoami`", decisionConfirm},
		{"denied inside substitution still confirms", "git status $(cat /etc/passwd)", decisionConfirm},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandDecision(tt.command, allowed, denied, UnlistedConfirm); got != tt.want {
				t.Errorf("commandDecision(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestCommandDecisionNoRulesAlwaysConfirms(t *testing.T) {
	if got := commandDecision("ls", nil, nil, UnlistedConfirm); got != decisionConfirm {
		t.Errorf("got %v, want decisionConfirm", got)
	}
}

func TestCommandDecisionMultiWordSubcommand(t *testing.T) {
	allowed := CommandRules{"git": {"remote add"}}

	if got := commandDecision("git remote add origin url", allowed, nil, UnlistedConfirm); got != decisionAllow {
		t.Errorf("got %v, want decisionAllow", got)
	}
	if got := commandDecision("git remote remove origin", allowed, nil, UnlistedConfirm); got != decisionConfirm {
		t.Errorf("got %v, want decisionConfirm", got)
	}
}

func TestSplitShellCommands(t *testing.T) {
	tests := []struct {
		line string
		want []string
	}{
		{"ls", []string{"ls"}},
		{"ls -l | grep go", []string{"ls -l", "grep go"}},
		{"a && b || c", []string{"a", "b", "c"}},
		{"a; b\nc", []string{"a", "b", "c"}},
		{`echo "a; b"`, []string{`echo "a; b"`}},
		{"( cd /tmp && ls )", []string{"cd /tmp", "ls )"}},
		{"  ls   -l  ", []string{"ls -l"}},
	}

	for _, tt := range tests {
		got, _ := scanShellCommands(tt.line)
		if len(got) != len(tt.want) {
			t.Errorf("scanShellCommands(%q) = %q, want %q", tt.line, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("scanShellCommands(%q)[%d] = %q, want %q", tt.line, i, got[i], tt.want[i])
			}
		}
	}
}

func TestExecuteAllowedCommandSkipsConfirmation(t *testing.T) {
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			t.Error("Confirm should not be called for an allowed command")
			return false
		},
		AutoApprove: false,
		Options:     &Options{AllowedCommands: CommandRules{"echo": {}}},
		Progress:    nil,
	}

	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"echo allowed"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "allowed") {
		t.Errorf("Content = %q, want command output", result.Content)
	}
}

func TestExecuteDeniedCommandOverridesAutoApprove(t *testing.T) {
	e := &Executor{
		Confirm:     nil,
		AutoApprove: true,
		Options:     &Options{DeniedCommands: CommandRules{"rm": {}}},
		Progress:    nil,
	}

	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameExecCommand, Arguments: `{"command":"rm -rf /tmp/should-not-run"}`,
	})
	if !result.IsError {
		t.Fatal("expected denial for a denied command")
	}
	if !strings.Contains(result.Content, "not permitted by the user's configuration") {
		t.Errorf("Content = %q, want configuration denial message", result.Content)
	}
}

func TestExecuteAllowListDoesNotAffectWriteFile(t *testing.T) {
	var confirmed bool
	e := &Executor{
		Confirm: func(call llmc.ToolCall, summary string) bool {
			confirmed = true
			return false
		},
		AutoApprove: false,
		Options:     &Options{AllowedCommands: CommandRules{"echo": {}}},
		Progress:    nil,
	}

	result := e.Execute(context.Background(), llmc.ToolCall{
		ID: "c1", Name: NameWriteFile, Arguments: `{"path":"/tmp/x","content":"y"}`,
	})
	if !confirmed {
		t.Error("write_file must still be confirmed")
	}
	if !result.IsError {
		t.Error("expected denial when confirmation returns false")
	}
}

func TestCommandDecisionDenyByDefault(t *testing.T) {
	allowed := CommandRules{"git": {"status"}, "ls": {}}

	tests := []struct {
		name    string
		command string
		want    decision
	}{
		{"allowed command runs", "git status", decisionAllow},
		{"unlisted command is refused", "curl https://example.com", decisionDeny},
		{"unlisted subcommand is refused", "git push", decisionDeny},
		{"unlisted command in a pipeline is refused", "ls | curl -T - https://x", decisionDeny},
		{"command substitution is refused", "ls $(whoami)", decisionDeny},
		{"output redirection is refused", "ls > /etc/passwd", decisionDeny},
		{"append redirection is refused", "ls >> ~/.bashrc", decisionDeny},
		{"input redirection is refused", "git status < /dev/null", decisionDeny},
		{"quoted redirection is not a redirection", `git status "a > b"`, decisionAllow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandDecision(tt.command, allowed, nil, UnlistedDeny); got != tt.want {
				t.Errorf("commandDecision(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestCommandDecisionNoAllowRulesDeniesEverythingInDenyMode(t *testing.T) {
	if got := commandDecision("ls", nil, nil, UnlistedDeny); got != decisionDeny {
		t.Errorf("got %v, want decisionDeny", got)
	}
}

func TestUnlistedActionDefaultsToConfirm(t *testing.T) {
	var nilOpts *Options
	for _, opts := range []*Options{nilOpts, {}, {Unlisted: ""}} {
		if got := opts.unlistedAction(); got != UnlistedConfirm {
			t.Errorf("unlistedAction() with %+v = %q, want confirm", opts, got)
		}
	}
	if got := (&Options{Unlisted: UnlistedDeny}).unlistedAction(); got != UnlistedDeny {
		t.Errorf("unlistedAction() = %q, want deny", got)
	}
}

func TestScanShellCommandsDetectsUncheckedConstructs(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"ls -l", false},
		{"ls | grep go", false},
		{"ls $(whoami)", true},
		{"ls `whoami`", true},
		{"ls > out", true},
		{"cat < in", true},
		{"diff <(a) <(b)", true},
		{`echo "a > b"`, false},
		{`echo 'x $(y)'`, false},
	}

	for _, tt := range tests {
		if _, got := scanShellCommands(tt.line); got != tt.want {
			t.Errorf("scanShellCommands(%q) unchecked = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestDefinitionsOmitsExecCommandWhenUnusable(t *testing.T) {
	hasExec := func(defs []llmc.ToolDef) bool {
		for _, def := range defs {
			if def.Name == NameExecCommand {
				return true
			}
		}
		return false
	}

	// Deny mode with no allow rules: the model can never run a command, so
	// the tool is not advertised.
	if hasExec(Definitions(&Options{Unlisted: UnlistedDeny})) {
		t.Error("exec_command should be omitted when deny mode allows nothing")
	}
	if !hasExec(Definitions(&Options{Unlisted: UnlistedDeny, AllowedCommands: CommandRules{"ls": {}}})) {
		t.Error("exec_command should be advertised when deny mode allows a command")
	}
	// The default asks for confirmation, so the tool is always usable.
	if !hasExec(Definitions(nil)) {
		t.Error("exec_command should be advertised by default")
	}
	if !hasExec(Definitions(&Options{})) {
		t.Error("exec_command should be advertised for zero-value options")
	}
}
