package prompt

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

// TemplateOptions carries the optional per-template settings (model, web
// search, tools) from a prompt file. Nil fields mean "not specified"; the
// caller falls back to the next configuration source.
type TemplateOptions struct {
	Model     *string
	WebSearch *bool
	Tools     *bool
}

// FormatMessage renders the message with the named prompt template.
// It returns the rendered system and user prompts separately, plus the
// template's optional settings. Returning system and user separately
// (instead of a combined string) lets callers pass the system prompt through
// the provider API's dedicated system field without fragile re-parsing. With
// no template, system is empty, user is the message unchanged, and opts has
// all-nil fields.
func FormatMessage(message string, promptName string, promptDirs []string, args []string) (system, user string, opts *TemplateOptions, err error) {
	emptyOpts := &TemplateOptions{Model: nil, WebSearch: nil, Tools: nil}
	if promptName == "" {
		return "", message, emptyOpts, nil
	}

	// Add .toml extension if not present
	promptFile := promptName
	if !strings.HasSuffix(promptFile, ".toml") {
		promptFile = promptFile + ".toml"
	}

	// Search for prompt file in all directories (including subdirectories)
	var promptPath string
	var found bool
	for _, promptDir := range promptDirs {
		// promptDir is already an absolute path
		candidatePath := filepath.Join(promptDir, promptFile)
		if _, err := os.Stat(candidatePath); err == nil {
			promptPath = candidatePath
			found = true
			// Continue searching to find later occurrences (later directories take precedence)
		}
	}

	if !found {
		return "", "", nil, fmt.Errorf("prompt file '%s' not found in any of the prompt directories: %v", promptFile, promptDirs)
	}

	// Load prompt template
	promptTemplate, err := LoadPrompt(promptPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("error loading prompt file: %v", err)
	}

	// Process command line arguments
	argMap, err := processArgs(args)
	if err != nil {
		return "", "", nil, fmt.Errorf("error processing arguments: %v", err)
	}

	// Create a map of all replacements
	replacements := make(map[string]string)
	replacements["input"] = message
	maps.Copy(replacements, argMap)

	// Format both prompts with all replacements
	systemPrompt := promptTemplate.System
	userPrompt := promptTemplate.User
	for key, value := range replacements {
		placeholder := fmt.Sprintf("{{%s}}", key)
		systemPrompt = strings.ReplaceAll(systemPrompt, placeholder, value)
		userPrompt = strings.ReplaceAll(userPrompt, placeholder, value)
	}

	// Validate model format if specified in prompt.
	// "@alias" references are passed through; they are expanded and
	// validated by the caller against the config-defined alias map.
	if promptTemplate.Model != nil && !llmc.IsModelAlias(*promptTemplate.Model) {
		if _, _, err := llmc.ParseModelString(*promptTemplate.Model); err != nil {
			return "", "", nil, fmt.Errorf("invalid model format in prompt template: %w", err)
		}
	}

	return systemPrompt, userPrompt, &TemplateOptions{
		Model:     promptTemplate.Model,
		WebSearch: promptTemplate.WebSearch,
		Tools:     promptTemplate.Tools,
	}, nil
}

// processArgs processes the command line arguments and returns a map of key-value pairs
func processArgs(args []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, arg := range args {
		// Handle quoted values
		arg = strings.TrimSpace(arg)
		if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) {
			arg = strings.Trim(arg, `"`)
		}

		// Split on first unescaped colon
		var key, value string
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument format: %s. Expected format: key:value", arg)
		}

		key = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])

		// Remove escape characters from value
		value = strings.ReplaceAll(value, `\:`, ":")
		value = strings.ReplaceAll(value, `\"`, `"`)

		if key == "input" {
			return nil, fmt.Errorf("'input' is a reserved keyword and cannot be used as a key")
		}
		result[key] = value
	}
	return result, nil
}
