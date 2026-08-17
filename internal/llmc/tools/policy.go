package tools

import (
	"strings"
	"unicode"
)

// decision is the outcome of applying the configured command policy to a
// tool call.
type decision int

const (
	// decisionConfirm requires the interactive confirmation prompt.
	decisionConfirm decision = iota
	// decisionAllow skips the confirmation prompt.
	decisionAllow
	// decisionDeny refuses the call outright, even with AutoApprove.
	decisionDeny
)

// UnlistedAction selects what happens to an exec_command invocation that the
// allow rules do not cover.
type UnlistedAction string

const (
	// UnlistedConfirm falls back to the interactive confirmation prompt.
	// Default.
	UnlistedConfirm UnlistedAction = "confirm"
	// UnlistedDeny refuses anything not explicitly allowed, turning the
	// allow rules into a gate rather than a convenience.
	UnlistedDeny UnlistedAction = "deny"
)

// CommandRules maps a command name to the subcommands it covers. An empty
// list (or one containing "*") covers every subcommand.
//
//	git = ["status", "diff"]   // git status / git diff only
//	ls  = []                   // any ls invocation
//	rm  = ["*"]                // any rm invocation
type CommandRules map[string][]string

// commandDecision applies the allow/deny rules to a shell command.
//
// Rules are matched against each command in the shell line (split on unquoted
// `;`, `|`, `&` and newlines): the command name must match the rule key
// exactly, and the rest of the command must match one of the listed
// subcommands by whole-word prefix, so "status" covers "status --short".
//
// Auto-approval requires every command in the line to be covered by the allow
// rules. A denied command anywhere in the line refuses the whole call, ahead
// of the allow rules and of AutoApprove. Anything left over is handled per
// unlisted: sent to the confirmation prompt (default), or refused.
//
// A line is never auto-approved when it contains a construct that prefix
// matching cannot account for — command substitution hides a command
// entirely, and a redirection turns a read-only command into a write.
//
// Matching a shell string cannot be a security boundary. With UnlistedConfirm
// the allow rules only decide whether the user is asked; with UnlistedDeny
// they narrow what can run at all, but even then allowing a command grants
// everything that command itself can do.
func commandDecision(command string, allowed, denied CommandRules, unlisted UnlistedAction) decision {
	segments, hasUnchecked := scanShellCommands(command)

	for _, segment := range segments {
		if matchesRules(segment, denied) {
			return decisionDeny
		}
	}

	if hasUnchecked {
		return unlistedDecision(unlisted)
	}
	for _, segment := range segments {
		if !matchesRules(segment, allowed) {
			return unlistedDecision(unlisted)
		}
	}
	return decisionAllow
}

func unlistedDecision(unlisted UnlistedAction) decision {
	if unlisted == UnlistedConfirm {
		return decisionConfirm
	}
	return decisionDeny
}

// canRunCommands reports whether any exec_command invocation could possibly
// succeed under the given options. When it cannot, the tool is not advertised
// to the model at all.
func canRunCommands(opts *Options) bool {
	if opts.unlistedAction() == UnlistedConfirm {
		return true
	}
	// Deny mode implies non-nil opts, so the allow rules are readable here.
	return len(opts.AllowedCommands) > 0
}

// canWriteFiles reports whether any write_file call could possibly succeed
// under the given options.
func canWriteFiles(opts *Options) bool {
	if opts.writeUnlistedAction() == UnlistedConfirm {
		return true
	}
	return len(opts.WriteAllowedPaths) > 0
}

// matchesRules reports whether a single normalized command matches the rules.
func matchesRules(command string, rules CommandRules) bool {
	name, rest, _ := strings.Cut(command, " ")
	subcommands, ok := rules[name]
	if !ok {
		return false
	}
	if len(subcommands) == 0 {
		return true
	}

	for _, sub := range subcommands {
		sub = normalizeSpaces(sub)
		switch {
		case sub == "*":
			return true
		case sub == "":
			continue
		case rest == sub || strings.HasPrefix(rest, sub+" "):
			return true
		}
	}
	return false
}

// scanShellCommands splits a shell line into individual commands on unquoted
// separators and reports whether the line contains a construct that prefix
// matching cannot verify (command substitution or redirection). Quoted
// characters are ignored for both purposes, so `echo "a; b > c"` is a single
// command with no redirection.
func scanShellCommands(line string) ([]string, bool) {
	var (
		segments     []string
		current      strings.Builder
		inSingle     bool
		inDouble     bool
		hasUnchecked bool
	)

	flush := func() {
		if s := normalizeCommand(current.String()); s != "" {
			segments = append(segments, s)
		}
		current.Reset()
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case inSingle || inDouble:
			// Quoted text is data, not shell syntax.
		case c == '`', c == '>', c == '<':
			hasUnchecked = true
		case c == '$' && i+1 < len(line) && line[i+1] == '(':
			hasUnchecked = true
		case c == ';' || c == '|' || c == '&' || c == '\n':
			flush()
			continue
		}
		current.WriteByte(c)
	}
	flush()

	return segments, hasUnchecked
}

// normalizeCommand strips grouping punctuation and leading variable
// assignments so that "FOO=1 git status" matches the rule key "git".
func normalizeCommand(segment string) string {
	segment = normalizeSpaces(strings.TrimLeft(segment, "({ \t"))

	for {
		field, rest, found := strings.Cut(segment, " ")
		if !found || !isAssignment(field) {
			break
		}
		segment = rest
	}
	return segment
}

// isAssignment reports whether a field is a leading VAR=value assignment.
func isAssignment(field string) bool {
	name, _, ok := strings.Cut(field, "=")
	if !ok || name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
