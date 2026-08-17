package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	execCommandTimeout   = 60 * time.Second
	execCommandMaxOutput = 64 * 1024
)

type execCommandArgs struct {
	Command string `json:"command"`
}

func execCommandDef() llmc.ToolDef {
	return llmc.ToolDef{
		Name:        NameExecCommand,
		Description: "Execute a shell command and return its combined stdout/stderr output and exit code. The command runs with `sh -c` in the current working directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			"required": []string{"command"},
		},
		RequiresConfirmation: true,
	}
}

func runExecCommand(ctx context.Context, arguments string, opts *Options) (string, error) {
	var args execCommandArgs
	if err := unmarshalArgs(arguments, &args); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, execCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
	cmd.Env = buildEnv(opts)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		switch {
		case ctx.Err() != nil:
			return "", fmt.Errorf("command timed out after %s", execCommandTimeout)
		case errors.As(err, &exitErr):
			exitCode = exitErr.ExitCode()
		default:
			return "", fmt.Errorf("executing command: %v", err)
		}
	}

	// A non-zero exit code is reported in the content (not as IsError) so
	// the model can interpret the failure itself.
	return fmt.Sprintf("exit code: %d\n\n%s", exitCode, truncate(string(output), execCommandMaxOutput)), nil
}

// minimalEnvNames are the variables kept in EnvModeMinimal: enough for most
// commands to run, with nothing project- or credential-specific.
var minimalEnvNames = []string{"PATH", "HOME", "USER", "SHELL", "LANG", "LC_ALL", "TERM", "TZ", "PWD"}

// secretNameFragments identify variables treated as credentials by
// EnvModeFiltered. Matching is on the variable name, not its value.
var secretNameFragments = []string{
	"TOKEN", "SECRET", "PASSWORD", "PASSWD", "API_KEY", "APIKEY",
	"ACCESS_KEY", "CREDENTIAL", "PRIVATE_KEY", "SESSION_KEY",
}

// buildEnv assembles the environment for the child process according to the
// configured mode. It returns nil only for EnvModeAll, which lets os/exec
// inherit the parent environment as-is.
func buildEnv(opts *Options) []string {
	mode := EnvModeFiltered
	var passthrough []string
	if opts != nil {
		if opts.EnvMode != "" {
			mode = opts.EnvMode
		}
		passthrough = opts.EnvPassthrough
	}

	if mode == EnvModeAll {
		return nil
	}

	allowed := make(map[string]bool, len(passthrough))
	for _, name := range passthrough {
		allowed[strings.ToUpper(strings.TrimSpace(name))] = true
	}
	if mode == EnvModeMinimal {
		for _, name := range minimalEnvNames {
			allowed[name] = true
		}
	}

	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if allowed[strings.ToUpper(name)] {
			env = append(env, entry)
			continue
		}
		if mode == EnvModeMinimal || isSecretEnvName(name) {
			continue
		}
		env = append(env, entry)
	}
	return env
}

// isSecretEnvName reports whether a variable name looks like a credential or
// belongs to llmc's own configuration (which includes provider tokens).
func isSecretEnvName(name string) bool {
	upper := strings.ToUpper(name)
	if strings.HasPrefix(upper, "LLMC_") {
		return true
	}
	for _, fragment := range secretNameFragments {
		if strings.Contains(upper, fragment) {
			return true
		}
	}
	return false
}
