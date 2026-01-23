package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

// FormatMessage formats the message with prompt if specified
// Returns the formatted message, the model specified in the prompt file (if any), and web search setting (if any)
func FormatMessage(message string, promptName string, promptDirs []string, args []string) (string, *string, *bool, error) {
	if promptName == "" {
		return message, nil, nil, nil
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
		return "", nil, nil, fmt.Errorf("prompt file '%s' not found in any of the prompt directories: %v", promptFile, promptDirs)
	}

	// Load prompt template
	promptTemplate, err := LoadPrompt(promptPath)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error loading prompt file: %v", err)
	}

	// Process command line arguments
	argMap, err := processArgs(args)
	if err != nil {
		return "", nil, nil, fmt.Errorf("error processing arguments: %v", err)
	}

	// Create a map of all replacements
	replacements := make(map[string]string)
	replacements["input"] = message
	for key, value := range argMap {
		replacements[key] = value
	}

	// Format both prompts with all replacements
	systemPrompt := promptTemplate.System
	userPrompt := promptTemplate.User
	for key, value := range replacements {
		placeholder := fmt.Sprintf("{{%s}}", key)
		systemPrompt = strings.ReplaceAll(systemPrompt, placeholder, value)
		userPrompt = strings.ReplaceAll(userPrompt, placeholder, value)
	}

	// Validate model format if specified in prompt
	if promptTemplate.Model != nil {
		if _, _, err := llmc.ParseModelString(*promptTemplate.Model); err != nil {
			return "", nil, nil, fmt.Errorf("invalid model format in prompt template: %w", err)
		}
	}

	return fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userPrompt), promptTemplate.Model, promptTemplate.WebSearch, nil
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
