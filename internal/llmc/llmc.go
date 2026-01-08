package llmc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/openai"
	"github.com/spf13/viper"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Provider        string   `toml:"provider" mapstructure:"provider"`
	BaseURL         string   `toml:"base_url" mapstructure:"base_url"`
	Model           string   `toml:"model" mapstructure:"model"`
	Token           string   `toml:"token" mapstructure:"token"`
	PromptDirs      []string `toml:"prompt_dirs" mapstructure:"prompt_dirs"`
	EnableWebSearch bool     `toml:"enable_web_search" mapstructure:"enable_web_search"`
}

// GetModel returns the model name
func (c *Config) GetModel() string {
	return c.Model
}

// GetBaseURL returns the base URL
func (c *Config) GetBaseURL() string {
	return c.BaseURL
}

// GetToken returns the API token
func (c *Config) GetToken() string {
	return c.Token
}

// NewDefaultConfig returns a new Config with default values
func NewDefaultConfig(promptDir string) *Config {
	return &Config{
		Provider:        openai.ProviderName,
		BaseURL:         openai.DefaultBaseURL,
		Model:           openai.DefaultModel,
		Token:           "",
		PromptDirs:      []string{promptDir},
		EnableWebSearch: false,
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
}

// NewProvider creates a new provider instance based on the configuration
func NewProvider(config *Config) (Provider, error) {
	switch config.Provider {
	case openai.ProviderName:
		return openai.NewProvider(config), nil
	case gemini.ProviderName:
		return gemini.NewProvider(config), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
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
