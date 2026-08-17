// Package tools provides the built-in tools the model can invoke during a
// tool loop (fetch_url, read_file, exec_command, write_file) and the Executor
// that dispatches tool calls, including the user-confirmation gate for
// destructive tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/longkey1/llmc/internal/llmc"
)

// Tool names.
const (
	NameFetchURL    = "fetch_url"
	NameReadFile    = "read_file"
	NameExecCommand = "exec_command"
	NameWriteFile   = "write_file"
)

// EnvMode selects which environment variables are passed to commands run by
// exec_command.
type EnvMode string

const (
	// EnvModeFiltered inherits the environment except variables whose names
	// look like credentials (and llmc's own LLMC_* variables). Default.
	EnvModeFiltered EnvMode = "filtered"
	// EnvModeMinimal passes only a small fixed set of variables.
	EnvModeMinimal EnvMode = "minimal"
	// EnvModeAll inherits the parent environment unchanged.
	EnvModeAll EnvMode = "all"
)

// Options carries the user-configured tool policy.
type Options struct {
	// AllowedCommands are exec_command rules that skip the confirmation
	// prompt. They do not gate execution unless Unlisted is UnlistedDeny;
	// see commandDecision.
	AllowedCommands CommandRules
	// DeniedCommands are exec_command rules that are refused outright,
	// ahead of AllowedCommands and even when AutoApprove is set.
	DeniedCommands CommandRules
	// Unlisted selects what happens to commands the allow rules don't
	// cover. Empty means UnlistedConfirm.
	Unlisted UnlistedAction
	// WriteAllowedPaths are write_file rules that skip the confirmation
	// prompt.
	WriteAllowedPaths PathRules
	// WriteDeniedPaths are write_file rules that are refused outright,
	// ahead of WriteAllowedPaths and even when AutoApprove is set.
	WriteDeniedPaths PathRules
	// WriteUnlisted selects what happens to paths the write allow rules
	// don't cover. Empty means UnlistedConfirm.
	WriteUnlisted UnlistedAction
	// ReadDeniedPaths are read_file rules that are refused outright.
	// read_file needs no confirmation, so it has no allow rules.
	ReadDeniedPaths PathRules
	// EnvMode selects the environment passed to exec_command. Empty means
	// EnvModeFiltered.
	EnvMode EnvMode
	// EnvPassthrough names variables to pass through regardless of EnvMode.
	EnvPassthrough []string
}

func (o *Options) unlistedAction() UnlistedAction {
	if o != nil && o.Unlisted == UnlistedDeny {
		return UnlistedDeny
	}
	return UnlistedConfirm
}

func (o *Options) writeUnlistedAction() UnlistedAction {
	if o != nil && o.WriteUnlisted == UnlistedDeny {
		return UnlistedDeny
	}
	return UnlistedConfirm
}

// Definitions returns the definitions of the built-in tools available under
// opts, in the order they are advertised to the model. exec_command and
// write_file are omitted when nothing could possibly succeed, so the model
// doesn't propose calls that would only be refused.
func Definitions(opts *Options) []llmc.ToolDef {
	defs := []llmc.ToolDef{
		fetchURLDef(),
		readFileDef(),
	}
	if canRunCommands(opts) {
		defs = append(defs, execCommandDef())
	}
	if canWriteFiles(opts) {
		defs = append(defs, writeFileDef())
	}
	return defs
}

// ConfirmFunc asks the user to approve a tool call. summary is a
// human-readable description of what will be executed.
type ConfirmFunc func(call llmc.ToolCall, summary string) bool

// Executor dispatches tool calls to the built-in tool implementations.
type Executor struct {
	// Confirm gates tools that require confirmation. A nil Confirm denies
	// them (used when stdin is not a TTY).
	Confirm ConfirmFunc
	// AutoApprove skips the confirmation prompt (--yes). It does not
	// override Options.DeniedCommands.
	AutoApprove bool
	// Options is the configured tool policy. A nil Options means defaults.
	Options *Options
	// Progress, if non-nil, receives human-readable progress lines.
	Progress func(format string, a ...any)
}

// Execute runs a single tool call and always returns a ToolResult; failures
// (unknown tool, bad arguments, denial) are reported to the model via
// IsError instead of an error so the loop can continue.
func (e *Executor) Execute(ctx context.Context, call llmc.ToolCall) llmc.ToolResult {
	def, run, err := lookupTool(call.Name)
	if err != nil {
		return e.errorResult(call, err.Error())
	}

	verdict, denyReason := e.decide(call)
	switch verdict {
	case decisionDeny:
		return e.errorResult(call, denyReason)
	case decisionAllow:
		// Pre-approved by configuration; skip the prompt.
	case decisionConfirm:
		if def.RequiresConfirmation && !e.AutoApprove {
			if denied := e.confirmOrDeny(call); denied != nil {
				return *denied
			}
		}
	}

	content, err := run(ctx, call.Arguments, e.Options)
	if err != nil {
		return e.errorResult(call, err.Error())
	}
	return llmc.ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    content,
		IsError:    false,
	}
}

// decide applies the configured policy to a call, returning the verdict and,
// for decisionDeny, the message sent back to the model. Tools without
// configurable rules fall through to their own RequiresConfirmation flag.
func (e *Executor) decide(call llmc.ToolCall) (decision, string) {
	switch call.Name {
	case NameExecCommand:
		var args execCommandArgs
		if err := unmarshalArgs(call.Arguments, &args); err != nil {
			// Malformed arguments are reported by the tool itself; never
			// auto-approve them.
			return decisionConfirm, ""
		}
		return e.commandVerdict(args.Command)
	case NameWriteFile:
		var args writeFileArgs
		if err := unmarshalArgs(call.Arguments, &args); err != nil {
			return decisionConfirm, ""
		}
		return e.writeVerdict(args.Path)
	case NameReadFile:
		var args readFileArgs
		if err := unmarshalArgs(call.Arguments, &args); err != nil {
			return decisionConfirm, ""
		}
		return e.readVerdict(args.Path)
	default:
		return decisionConfirm, ""
	}
}

func (e *Executor) commandVerdict(command string) (decision, string) {
	var allowed, denied CommandRules
	if e.Options != nil {
		allowed, denied = e.Options.AllowedCommands, e.Options.DeniedCommands
	}
	verdict := commandDecision(command, allowed, denied, e.Options.unlistedAction())
	return verdict, "Execution denied: this command is not permitted by the user's configuration. " +
		"Do not retry it; either complete the task another way or tell the user which command " +
		"would need to be permitted."
}

func (e *Executor) writeVerdict(path string) (decision, string) {
	var allowed, denied PathRules
	if e.Options != nil {
		allowed, denied = e.Options.WriteAllowedPaths, e.Options.WriteDeniedPaths
	}
	verdict := pathDecision(path, allowed, denied, e.Options.writeUnlistedAction())
	return verdict, "Execution denied: writing to this path is not permitted by the user's " +
		"configuration. Do not retry it; either write somewhere else or tell the user which " +
		"path would need to be permitted."
}

func (e *Executor) readVerdict(path string) (decision, string) {
	var denied PathRules
	if e.Options != nil {
		denied = e.Options.ReadDeniedPaths
	}
	// read_file needs no confirmation, so the deny rules are the only
	// control and anything else simply runs.
	verdict := pathDecision(path, nil, denied, UnlistedConfirm)
	return verdict, "Execution denied: reading this path is not permitted by the user's " +
		"configuration. Do not retry it; it may hold credentials. Tell the user which path " +
		"would need to be permitted if you genuinely need it."
}

// runFunc executes a tool with raw JSON arguments and returns its output.
// opts may be nil, meaning default policy.
type runFunc func(ctx context.Context, arguments string, opts *Options) (string, error)

func lookupTool(name string) (llmc.ToolDef, runFunc, error) {
	switch name {
	case NameFetchURL:
		return fetchURLDef(), runFetchURL, nil
	case NameReadFile:
		return readFileDef(), runReadFile, nil
	case NameExecCommand:
		return execCommandDef(), runExecCommand, nil
	case NameWriteFile:
		return writeFileDef(), runWriteFile, nil
	default:
		return llmc.ToolDef{}, nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// confirmOrDeny asks the user for approval and returns a denial result, or
// nil if the call was approved.
func (e *Executor) confirmOrDeny(call llmc.ToolCall) *llmc.ToolResult {
	if e.Confirm == nil {
		r := e.errorResult(call,
			"Execution denied: this tool requires interactive confirmation, but stdin is not a TTY. "+
				"The user did not approve this action. Do not retry; either complete the task without "+
				"this tool or tell the user to re-run with the --yes flag.")
		return &r
	}
	summary, err := confirmationSummary(call)
	if err != nil {
		r := e.errorResult(call, err.Error())
		return &r
	}
	if !e.Confirm(call, summary) {
		r := e.errorResult(call, "Execution denied by the user.")
		return &r
	}
	return nil
}

// confirmationSummary builds the human-readable description shown in the
// confirmation prompt.
func confirmationSummary(call llmc.ToolCall) (string, error) {
	switch call.Name {
	case NameExecCommand:
		var args execCommandArgs
		if err := unmarshalArgs(call.Arguments, &args); err != nil {
			return "", err
		}
		return fmt.Sprintf("Run command:\n  %s", args.Command), nil
	case NameWriteFile:
		var args writeFileArgs
		if err := unmarshalArgs(call.Arguments, &args); err != nil {
			return "", err
		}
		return writeFileSummary(args), nil
	default:
		return fmt.Sprintf("%s(%s)", call.Name, call.Arguments), nil
	}
}

func (e *Executor) errorResult(call llmc.ToolCall, message string) llmc.ToolResult {
	return llmc.ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    message,
		IsError:    true,
	}
}

func unmarshalArgs(arguments string, out any) error {
	if err := json.Unmarshal([]byte(arguments), out); err != nil {
		return fmt.Errorf("invalid tool arguments: %v", err)
	}
	return nil
}

// truncate limits s to maxLen bytes, appending a note when content was cut.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("\n... (truncated, %d bytes total)", len(s))
}
