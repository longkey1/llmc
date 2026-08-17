package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

const writeFileMaxContent = 1024 * 1024

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func writeFileDef() llmc.ToolDef {
	return llmc.ToolDef{
		Name:        NameWriteFile,
		Description: "Write text content to a local file (creating or overwriting it). The parent directory must already exist. Relative paths are resolved from the current working directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The full content to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
		RequiresConfirmation: true,
	}
}

func runWriteFile(ctx context.Context, arguments string, _ *Options) (string, error) {
	var args writeFileArgs
	if err := unmarshalArgs(arguments, &args); err != nil {
		return "", err
	}
	if len(args.Content) > writeFileMaxContent {
		return "", fmt.Errorf("content too large: %d bytes (max %d)", len(args.Content), writeFileMaxContent)
	}
	if err := refuseSymlink(args.Path); err != nil {
		return "", err
	}

	path, err := expandHome(args.Path)
	if err != nil {
		return "", err
	}

	// Intentionally no MkdirAll: the confirmation prompt describes a file
	// write only, so the effect must not extend to creating directories.
	if err := os.WriteFile(path, []byte(args.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %v", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(args.Content), args.Path), nil
}

// writeFileSummary describes a pending write for the confirmation prompt,
// including whether an existing file would be overwritten and a short
// preview of the content. The path is shown canonicalized so that ".." and
// symlinked parents are visible rather than hidden behind what the model
// wrote.
func writeFileSummary(args writeFileArgs) string {
	path := args.Path
	if resolved, err := resolvePath(path); err == nil {
		path = resolved
	}

	action := "Create file"
	if _, err := os.Stat(path); err == nil {
		action = "Overwrite file"
	}

	const previewLines = 10
	lines := strings.Split(args.Content, "\n")
	preview := lines
	truncated := false
	if len(lines) > previewLines {
		preview = lines[:previewLines]
		truncated = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s (%d bytes)\n", action, path, len(args.Content))
	for _, line := range preview {
		fmt.Fprintf(&b, "  | %s\n", line)
	}
	if truncated {
		fmt.Fprintf(&b, "  | ... (%d more lines)\n", len(lines)-previewLines)
	}
	return strings.TrimRight(b.String(), "\n")
}
