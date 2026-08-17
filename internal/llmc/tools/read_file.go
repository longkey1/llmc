package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/longkey1/llmc/internal/llmc"
)

const readFileMaxSize = 256 * 1024

type readFileArgs struct {
	Path string `json:"path"`
}

func readFileDef() llmc.ToolDef {
	return llmc.ToolDef{
		Name:        NameReadFile,
		Description: "Read the content of a local file. Relative paths are resolved from the current working directory. Returns the file content as text (truncated if large).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
		RequiresConfirmation: false,
	}
}

func runReadFile(ctx context.Context, arguments string, _ *Options) (string, error) {
	var args readFileArgs
	if err := unmarshalArgs(arguments, &args); err != nil {
		return "", err
	}

	path, err := expandHome(args.Path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %v", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", args.Path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %v", err)
	}
	return truncate(string(content), readFileMaxSize), nil
}
