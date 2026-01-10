package llmc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Model                 string   `toml:"model" mapstructure:"model"` // Format: "provider:model" (e.g., "openai:gpt-4")
	OpenAIBaseURL         string   `toml:"openai_base_url" mapstructure:"openai_base_url"`
	OpenAIToken           string   `toml:"openai_token" mapstructure:"openai_token"`
	GeminiBaseURL         string   `toml:"gemini_base_url" mapstructure:"gemini_base_url"`
	GeminiToken           string   `toml:"gemini_token" mapstructure:"gemini_token"`
	PromptDirs            []string `toml:"prompt_dirs" mapstructure:"prompt_dirs"`
	EnableWebSearch       bool     `toml:"enable_web_search" mapstructure:"enable_web_search"`
	IgnoreWebSearchErrors bool     `toml:"ignore_web_search_errors" mapstructure:"ignore_web_search_errors"`
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID          string
	Description string
	IsDefault   bool
}

// GetModel returns the model name
func (c *Config) GetModel() string {
	return c.Model
}

// GetBaseURL returns the base URL for the specified provider
// Resolves environment variable if value starts with "$" or "${"
func (c *Config) GetBaseURL(provider string) (string, error) {
	var baseURLValue string
	switch provider {
	case "openai":
		baseURLValue = c.OpenAIBaseURL
	case "gemini":
		baseURLValue = c.GeminiBaseURL
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if it's an environment variable reference
	if strings.HasPrefix(baseURLValue, "$") {
		var envVarName string
		// Support both $VAR and ${VAR} syntax
		if strings.HasPrefix(baseURLValue, "${") && strings.HasSuffix(baseURLValue, "}") {
			// Extract variable name from ${VAR} format
			envVarName = baseURLValue[2 : len(baseURLValue)-1]
		} else {
			// Extract variable name from $VAR format
			envVarName = strings.TrimPrefix(baseURLValue, "$")
		}

		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s is not set or empty", envVarName)
		}
		return envValue, nil
	}

	return baseURLValue, nil
}

// GetProvider extracts provider name from the model string
func (c *Config) GetProvider() (string, error) {
	provider, _, err := ParseModelString(c.Model)
	return provider, err
}

// GetModelName extracts model name from the model string
func (c *Config) GetModelName() (string, error) {
	_, model, err := ParseModelString(c.Model)
	return model, err
}

// GetToken returns the token for the specified provider
// Resolves environment variable if value starts with "$" or "${"
func (c *Config) GetToken(provider string) (string, error) {
	var tokenValue string
	switch provider {
	case "openai":
		tokenValue = c.OpenAIToken
	case "gemini":
		tokenValue = c.GeminiToken
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if it's an environment variable reference
	if strings.HasPrefix(tokenValue, "$") {
		var envVarName string
		// Support both $VAR and ${VAR} syntax
		if strings.HasPrefix(tokenValue, "${") && strings.HasSuffix(tokenValue, "}") {
			// Extract variable name from ${VAR} format
			envVarName = tokenValue[2 : len(tokenValue)-1]
		} else {
			// Extract variable name from $VAR format
			envVarName = strings.TrimPrefix(tokenValue, "$")
		}

		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s is not set or empty", envVarName)
		}
		return envValue, nil
	}

	// Validate that token is not empty
	if tokenValue == "" {
		return "", fmt.Errorf("%s token is not configured. Set it in config file (%s_token) or environment variable (LLMC_%s_TOKEN)", provider, provider, strings.ToUpper(provider))
	}

	return tokenValue, nil
}

// NewDefaultConfig returns a new Config with default values
func NewDefaultConfig(promptDir string) *Config {
	return &Config{
		Model:                 "openai:gpt-4.1", // Changed to "provider:model" format
		OpenAIBaseURL:         "https://api.openai.com/v1",
		OpenAIToken:           "$OPENAI_API_KEY", // Default to env var
		GeminiBaseURL:         "https://generativelanguage.googleapis.com/v1beta",
		GeminiToken:           "$GEMINI_API_KEY",
		PromptDirs:            []string{promptDir},
		EnableWebSearch:       false,
		IgnoreWebSearchErrors: false,
	}
}

// LoadConfig loads configuration from viper
func LoadConfig() (*Config, error) {
	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	// Convert prompt directories to absolute paths
	for i, promptDir := range config.PromptDirs {
		absPath, err := ResolvePath(promptDir)
		if err != nil {
			return nil, fmt.Errorf("error resolving prompt directory path '%s': %v", promptDir, err)
		}
		config.PromptDirs[i] = absPath
	}

	return config, nil
}

// ResolvePath converts a relative path to absolute path if needed
func ResolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Get config file directory as base directory
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If no config file is used, fall back to current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current working directory: %v", err)
		}
		return filepath.Join(cwd, path), nil
	}

	// Use config file directory as base
	configDir := filepath.Dir(configFile)

	// If configDir is relative, make it absolute
	if !filepath.IsAbs(configDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current working directory: %v", err)
		}
		configDir = filepath.Join(cwd, configDir)
	}

	resolvedPath := filepath.Join(configDir, path)
	return resolvedPath, nil
}

// Provider defines the interface for LLM providers
type Provider interface {
	Chat(message string) (string, error)
	SetWebSearch(enabled bool)
	SetIgnoreWebSearchErrors(enabled bool)
	SetDebug(enabled bool)
	ListModels() ([]ModelInfo, error)
}

// Prompt represents the structure of a TOML prompt file
type Prompt struct {
	System    string  `toml:"system"`
	User      string  `toml:"user"`
	Model     *string `toml:"model,omitempty"`
	WebSearch *bool   `toml:"web_search,omitempty"`
}

// LoadPrompt loads a prompt file and returns its contents
func LoadPrompt(filePath string) (*Prompt, error) {
	var prompt Prompt
	if _, err := toml.DecodeFile(filePath, &prompt); err != nil {
		return nil, fmt.Errorf("error decoding prompt file: %v", err)
	}
	return &prompt, nil
}

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
		if _, _, err := ParseModelString(*promptTemplate.Model); err != nil {
			return "", nil, nil, fmt.Errorf("invalid model format in prompt template: %w", err)
		}
	}

	return fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userPrompt), promptTemplate.Model, promptTemplate.WebSearch, nil
}

// ParseModelString parses a model string in "provider:model" format
// Returns (provider, model, error)
func ParseModelString(modelStr string) (string, string, error) {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid model format: %s (expected format: provider:model, e.g., openai:gpt-4)", modelStr)
	}

	provider := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])

	if provider == "" || model == "" {
		return "", "", fmt.Errorf("provider and model cannot be empty")
	}

	return provider, model, nil
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
