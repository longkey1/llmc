package llmc

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/longkey1/llmc/internal/config"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/openai"
	"github.com/spf13/viper"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Provider  string `toml:"provider" mapstructure:"provider"`
	BaseURL   string `toml:"base_url" mapstructure:"base_url"`
	Model     string `toml:"model" mapstructure:"model"`
	Token     string `toml:"token" mapstructure:"token"`
	PromptDir string `toml:"prompt_dir" mapstructure:"prompt_dir"`
}

// NewDefaultConfig returns a new Config with default values
func NewDefaultConfig(promptDir string) *config.Config {
	return &config.Config{
		Provider:  openai.ProviderName,
		BaseURL:   openai.DefaultBaseURL,
		Model:     openai.DefaultModel,
		Token:     "",
		PromptDir: promptDir,
	}
}

// LoadConfig loads configuration from viper
func LoadConfig() (*config.Config, error) {
	config := &config.Config{}
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}
	return config, nil
}

// Provider defines the interface for LLM providers
type Provider interface {
	Chat(message string) (string, error)
}

// NewProvider creates a new provider instance based on the configuration
func NewProvider(config *config.Config) (Provider, error) {
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
	System string `toml:"system"`
	User   string `toml:"user"`
}

// FormatPrompt formats the prompt with the given input message
func (p *Prompt) FormatPrompt(input string) (string, string) {
	systemPrompt := strings.ReplaceAll(p.System, "{{input}}", input)
	userPrompt := strings.ReplaceAll(p.User, "{{input}}", input)
	return systemPrompt, userPrompt
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
func FormatMessage(message string, promptName string, promptDir string) (string, error) {
	if promptName == "" {
		return message, nil
	}

	// Add .toml extension if not present
	promptFile := promptName
	if !strings.HasSuffix(promptFile, ".toml") {
		promptFile = promptFile + ".toml"
	}

	// Construct full path to prompt file
	promptPath := filepath.Join(promptDir, promptFile)

	// Load prompt template
	promptTemplate, err := LoadPrompt(promptPath)
	if err != nil {
		return "", fmt.Errorf("error loading prompt file: %v", err)
	}

	// Format message with prompt
	systemPrompt, userPrompt := promptTemplate.FormatPrompt(message)
	return fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userPrompt), nil
}
