package config

import (
	"fmt"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/viper"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Model                   string              `toml:"model" mapstructure:"model"`                 // Format: "provider:model" (e.g., "openai:gpt-4")
	ModelAliases            map[string]string   `toml:"model_aliases" mapstructure:"model_aliases"` // Alias name -> "provider:model-family" (used via "@alias")
	OpenAIBaseURL           string              `toml:"openai_base_url" mapstructure:"openai_base_url"`
	OpenAIToken             string              `toml:"openai_token" mapstructure:"openai_token"`
	GeminiBaseURL           string              `toml:"gemini_base_url" mapstructure:"gemini_base_url"`
	GeminiToken             string              `toml:"gemini_token" mapstructure:"gemini_token"`
	AnthropicBaseURL        string              `toml:"anthropic_base_url" mapstructure:"anthropic_base_url"`
	AnthropicToken          string              `toml:"anthropic_token" mapstructure:"anthropic_token"`
	OllamaBaseURL           string              `toml:"ollama_base_url" mapstructure:"ollama_base_url"`
	OllamaToken             string              `toml:"ollama_token" mapstructure:"ollama_token"`
	PromptDirs              []string            `toml:"prompt_dirs" mapstructure:"prompt_dirs"`
	EnableWebSearch         bool                `toml:"enable_web_search" mapstructure:"enable_web_search"`
	EnableTools             bool                `toml:"enable_tools" mapstructure:"enable_tools"`                           // Enable built-in tool calling (fetch_url, read_file, exec_command, write_file)
	ExecAllowedCommands     map[string][]string `toml:"exec_allowed_commands" mapstructure:"exec_allowed_commands"`         // Command name -> subcommands that skip the confirmation prompt (empty or ["*"] = all)
	ExecDeniedCommands      map[string][]string `toml:"exec_denied_commands" mapstructure:"exec_denied_commands"`           // Command name -> subcommands that are always refused (empty or ["*"] = all)
	ExecUnlisted            string              `toml:"exec_unlisted" mapstructure:"exec_unlisted"`                         // What to do with commands not in exec_allowed_commands: "confirm" (default) or "deny"
	ExecEnvMode             string              `toml:"exec_env_mode" mapstructure:"exec_env_mode"`                         // "filtered" (default), "minimal", or "all"
	ExecEnvPassthrough      []string            `toml:"exec_env_passthrough" mapstructure:"exec_env_passthrough"`           // Environment variables passed to exec_command regardless of mode
	WriteAllowedPaths       []string            `toml:"write_allowed_paths" mapstructure:"write_allowed_paths"`             // write_file paths that skip the confirmation prompt
	WriteDeniedPaths        []string            `toml:"write_denied_paths" mapstructure:"write_denied_paths"`               // write_file paths that are always refused
	WriteUnlisted           string              `toml:"write_unlisted" mapstructure:"write_unlisted"`                       // What to do with paths not in write_allowed_paths: "confirm" (default) or "deny"
	ReadDeniedPaths         []string            `toml:"read_denied_paths" mapstructure:"read_denied_paths"`                 // read_file paths that are always refused
	SessionMessageThreshold int                 `toml:"session_message_threshold" mapstructure:"session_message_threshold"` // 0 = disabled
	SessionRetentionDays    int                 `toml:"session_retention_days" mapstructure:"session_retention_days"`       // Number of days to retain sessions (default: 30)
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

// DefaultReadDeniedPaths returns the paths read_file refuses out of the box:
// credential stores, and llmc's own config directory (which holds the
// provider tokens).
func DefaultReadDeniedPaths() []string {
	return []string{"~/.ssh", "~/.aws", "~/.gnupg", "~/.config/llmc", ".env", "*.pem"}
}

// DefaultWriteDeniedPaths returns the paths write_file refuses out of the
// box: credential stores, llmc's own config directory, and git internals.
func DefaultWriteDeniedPaths() []string {
	return []string{"~/.ssh", "~/.aws", "~/.gnupg", "~/.config/llmc", ".git"}
}

// NewDefaultConfig returns a new Config with default values
func NewDefaultConfig(promptDir string) *Config {
	return &Config{
		Model:                   "openai:gpt-4.1", // Changed to "provider:model" format
		OpenAIBaseURL:           "https://api.openai.com/v1",
		OpenAIToken:             "", // No default, use LLMC_OPENAI_TOKEN env var or set in config file
		GeminiBaseURL:           "https://generativelanguage.googleapis.com/v1beta",
		GeminiToken:             "", // No default, use LLMC_GEMINI_TOKEN env var or set in config file
		AnthropicBaseURL:        "https://api.anthropic.com/v1",
		AnthropicToken:          "", // No default, use LLMC_ANTHROPIC_TOKEN env var or set in config file
		OllamaBaseURL:           "http://localhost:11434/v1",
		OllamaToken:             "", // Optional: local Ollama needs no token; set for authenticated remote servers
		PromptDirs:              []string{promptDir},
		EnableWebSearch:         false,
		EnableTools:             false,
		ExecAllowedCommands:     map[string][]string{},
		ExecDeniedCommands:      map[string][]string{},
		ExecUnlisted:            "confirm",  // Ask before running a command that isn't pre-approved
		ExecEnvMode:             "filtered", // Strip credential-looking variables from exec_command
		ExecEnvPassthrough:      []string{},
		WriteAllowedPaths:       []string{},
		WriteDeniedPaths:        DefaultWriteDeniedPaths(),
		WriteUnlisted:           "confirm", // Ask before writing to a path that isn't pre-approved
		ReadDeniedPaths:         DefaultReadDeniedPaths(),
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

	// Expand environment variables in tokens and base URLs
	config.OpenAIToken, _ = expandEnvVar(config.OpenAIToken)
	config.GeminiToken, _ = expandEnvVar(config.GeminiToken)
	config.AnthropicToken, _ = expandEnvVar(config.AnthropicToken)
	config.OllamaToken, _ = expandEnvVar(config.OllamaToken)
	config.OpenAIBaseURL, _ = expandEnvVar(config.OpenAIBaseURL)
	config.GeminiBaseURL, _ = expandEnvVar(config.GeminiBaseURL)
	config.AnthropicBaseURL, _ = expandEnvVar(config.AnthropicBaseURL)
	config.OllamaBaseURL, _ = expandEnvVar(config.OllamaBaseURL)

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
