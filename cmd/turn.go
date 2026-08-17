package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/llmc/tools"
	"github.com/spf13/cobra"
)

// turnConfig carries the per-turn settings resolved from flags, environment,
// prompt template and config file.
type turnConfig struct {
	systemPrompt string
	toolsEnabled bool
	autoApprove  bool
	toolOptions  *tools.Options
}

// newTurnConfig resolves the turn settings for a command invocation.
func newTurnConfig(cmd *cobra.Command, cfg *config.Config, systemPrompt string, promptTools *bool) turnConfig {
	return turnConfig{
		systemPrompt: systemPrompt,
		toolsEnabled: resolveTools(cmd, cfg, promptTools),
		autoApprove:  autoApproveTool,
		toolOptions: &tools.Options{
			AllowedCommands: tools.CommandRules(cfg.ExecAllowedCommands),
			DeniedCommands:  tools.CommandRules(cfg.ExecDeniedCommands),
			Unlisted:        tools.UnlistedAction(cfg.ExecUnlisted),
			EnvMode:         tools.EnvMode(cfg.ExecEnvMode),
			EnvPassthrough:  cfg.ExecEnvPassthrough,

			WriteAllowedPaths: cfg.WriteAllowedPaths,
			WriteDeniedPaths:  cfg.WriteDeniedPaths,
			WriteUnlisted:     tools.UnlistedAction(cfg.WriteUnlisted),
			ReadDeniedPaths:   cfg.ReadDeniedPaths,
		},
	}
}

// runTurn executes one conversation turn. With tools disabled it behaves like
// the plain ChatWithHistory call (after stripping any tool-loop messages from
// the history). With tools enabled it drives the tool-calling loop and
// returns every message generated during the turn for session persistence.
func runTurn(ctx context.Context, p llmc.Provider, history []llmc.Message,
	newMessage string, tc turnConfig) (string, []llmc.Message, error) {
	if !tc.toolsEnabled {
		stopSpinner := startSpinner()
		response, err := p.ChatWithHistory(ctx, tc.systemPrompt, llmc.SanitizeHistory(history), newMessage)
		stopSpinner()
		if err != nil {
			return "", nil, err
		}
		return response, []llmc.Message{{Role: "assistant", Content: response, Timestamp: time.Now()}}, nil
	}

	chatter, ok := p.(llmc.ToolChatter)
	if !ok {
		return "", nil, fmt.Errorf("the selected provider does not support tools")
	}

	executor := &tools.Executor{
		Confirm:     nil,
		AutoApprove: tc.autoApprove,
		Options:     tc.toolOptions,
		Progress:    nil,
	}
	if stdinIsTerminal() {
		executor.Confirm = confirmToolExecution
	}

	// History from earlier tool-enabled turns (including tool messages) is
	// passed through so the model keeps its full context.
	messages := make([]llmc.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, llmc.Message{
		Role:      "user",
		Content:   newMessage,
		Timestamp: time.Now(),
	})

	hooks := llmc.LoopHooks{
		OnRequest:  startSpinner,
		OnToolCall: printToolCall,
		OnToolDone: printToolDone,
	}

	return llmc.RunToolLoop(ctx, chatter, tc.systemPrompt, messages, tools.Definitions(tc.toolOptions), executor, hooks)
}

// confirmToolExecution prompts the user on stderr to approve a tool call.
func confirmToolExecution(call llmc.ToolCall, summary string) bool {
	fmt.Fprintf(os.Stderr, "\n%s\n", summary)
	fmt.Fprint(os.Stderr, "Execute? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)
	return response == "y" || response == "Y"
}

// printToolCall reports a starting tool execution on stderr.
func printToolCall(call llmc.ToolCall) {
	fmt.Fprintf(os.Stderr, "→ %s: %s\n", call.Name, summarizeArguments(call))
}

// printToolDone reports a finished tool execution on stderr.
func printToolDone(call llmc.ToolCall, result llmc.ToolResult) {
	if result.IsError {
		fmt.Fprintf(os.Stderr, "✗ %s (%s)\n", call.Name, truncateWithEllipsis(result.Content, 60))
		return
	}
	fmt.Fprintf(os.Stderr, "✓ %s (%s)\n", call.Name, formatByteSize(len(result.Content)))
}

// summarizeArguments renders tool call arguments for progress output: the
// main string argument when parseable, the raw JSON otherwise.
func summarizeArguments(call llmc.ToolCall) string {
	const maxLen = 80

	var args map[string]any
	if json.Unmarshal([]byte(call.Arguments), &args) == nil {
		for _, key := range []string{"command", "url", "path"} {
			if v, ok := args[key].(string); ok {
				return truncateWithEllipsis(v, maxLen)
			}
		}
	}
	return truncateWithEllipsis(call.Arguments, maxLen)
}

// formatByteSize renders a byte count in a compact human-readable form.
func formatByteSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1fKB", float64(n)/1024)
}
