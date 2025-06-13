package llmc

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the configuration for the LLM provider
type Config struct {
	Provider  string `toml:"provider"`
	BaseURL   string `toml:"base_url"`
	Token     string `toml:"token"`
	Model     string `toml:"model"`
	PromptDir string `toml:"prompt_dir"`
}

// Provider defines the interface for LLM providers
type Provider interface {
	Chat(message string) (string, error)
}

// Prompt represents the structure of a TOML prompt file
type Prompt struct {
	System string `toml:"system"`
	User   string `toml:"user"`
}

// LoadPrompt loads a prompt file and returns its contents
func LoadPrompt(filePath string) (*Prompt, error) {
	var prompt Prompt
	if _, err := toml.DecodeFile(filePath, &prompt); err != nil {
		return nil, fmt.Errorf("error decoding prompt file: %v", err)
	}
	return &prompt, nil
}

// FormatPrompt formats the prompt with the given input message
func (p *Prompt) FormatPrompt(input string) (string, string) {
	systemPrompt := strings.ReplaceAll(p.System, "{{input}}", input)
	userPrompt := strings.ReplaceAll(p.User, "{{input}}", input)
	return systemPrompt, userPrompt
}
