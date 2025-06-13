package config

// Config holds the configuration for the LLM provider
type Config struct {
	Provider  string `toml:"provider" mapstructure:"provider"`
	BaseURL   string `toml:"base_url" mapstructure:"base_url"`
	Model     string `toml:"model" mapstructure:"model"`
	Token     string `toml:"token" mapstructure:"token"`
	PromptDir string `toml:"prompt_dir" mapstructure:"prompt_dir"`
}
