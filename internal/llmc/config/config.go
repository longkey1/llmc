package config

import (
	"fmt"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/viper"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Model                   string   `toml:"model" mapstructure:"model"` // Format: "provider:model" (e.g., "openai:gpt-4")
	OpenAIBaseURL           string   `toml:"openai_base_url" mapstructure:"openai_base_url"`
	OpenAIToken             string   `toml:"openai_token" mapstructure:"openai_token"`
	GeminiBaseURL           string   `toml:"gemini_base_url" mapstructure:"gemini_base_url"`
	GeminiToken             string   `toml:"gemini_token" mapstructure:"gemini_token"`
	AnthropicBaseURL        string   `toml:"anthropic_base_url" mapstructure:"anthropic_base_url"`
	AnthropicToken          string   `toml:"anthropic_token" mapstructure:"anthropic_token"`
	PromptDirs              []string `toml:"prompt_dirs" mapstructure:"prompt_dirs"`
	EnableWebSearch         bool     `toml:"enable_web_search" mapstructure:"enable_web_search"`
	SessionMessageThreshold int      `toml:"session_message_threshold" mapstructure:"session_message_threshold"` // 0 = disabled
	SessionRetentionDays    int      `toml:"session_retention_days" mapstructure:"session_retention_days"`       // Number of days to retain sessions (default: 30)
}

// GetModel returns the model name
func (c *Config) GetModel() string {
	return c.Model
}

// GetProvider extracts provider name from the model string
func (c *Config) GetProvider() (string, error) {
	provider, _, err := llmc.ParseModelString(c.Model)
	return provider, err
}

// GetModelName extracts model name from the model string
func (c *Config) GetModelName() (string, error) {
	_, model, err := llmc.ParseModelString(c.Model)
	return model, err
}

// NewDefaultConfig returns a new Config with default values
func NewDefaultConfig(promptDir string) *Config {
	return &Config{
		Model:                   "openai:gpt-4.1", // Changed to "provider:model" format
		OpenAIBaseURL:           "https://api.openai.com/v1",
		OpenAIToken:             "$OPENAI_API_KEY", // Default to env var
		GeminiBaseURL:           "https://generativelanguage.googleapis.com/v1beta",
		GeminiToken:             "$GEMINI_API_KEY",
		AnthropicBaseURL:        "https://api.anthropic.com/v1",
		AnthropicToken:          "$ANTHROPIC_API_KEY",
		PromptDirs:              []string{promptDir},
		EnableWebSearch:         false,
		SessionMessageThreshold: 50, // Default threshold (0 = disabled)
		SessionRetentionDays:    30, // Default: delete sessions older than 30 days
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
