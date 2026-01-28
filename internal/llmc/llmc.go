// Package llmc provides the core abstractions for LLM providers.
// This package defines the Provider interface that all LLM provider implementations
// (openai, gemini, etc.) must implement.
package llmc

import (
	"fmt"
	"strings"
)

// ModelInfo represents information about an available model from a provider.
type ModelInfo struct {
	ID          string // Model identifier (e.g., "gpt-4", "gemini-pro")
	Description string // Human-readable description of the model
	IsDefault   bool   // Whether this is the default model for the provider
}

// Provider defines the interface for LLM providers.
// All provider implementations (openai, gemini, etc.) must implement this interface.
//
// Example usage:
//
//	provider := openai.NewProvider(cfg)
//	provider.SetWebSearch(true)
//	response, err := provider.Chat("Hello, world!")
type Provider interface {
	// Chat sends a single message and returns the response.
	Chat(message string) (string, error)

	// ChatWithHistory sends a message with conversation history.
	// The systemPrompt is prepended to the conversation.
	// messages contains the conversation history (user and assistant messages).
	// newMessage is the new user message to send.
	ChatWithHistory(systemPrompt string, messages []Message, newMessage string) (string, error)

	// SetWebSearch enables or disables web search for the provider.
	SetWebSearch(enabled bool)

	// SetIgnoreWebSearchErrors configures whether to ignore web search errors.
	SetIgnoreWebSearchErrors(enabled bool)

	// SetDebug enables or disables debug output.
	SetDebug(enabled bool)

	// ListModels returns a list of available models for the provider.
	ListModels() ([]ModelInfo, error)
}

// ParseModelString parses a model string in "provider:model" format.
// Returns (provider, model, error).
//
// Example:
//
//	provider, model, err := ParseModelString("openai:gpt-4")
//	// provider = "openai", model = "gpt-4"
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

// FormatModelString formats provider and model into "provider:model" format.
//
// Example:
//
//	modelStr := FormatModelString("openai", "gpt-4")
//	// modelStr = "openai:gpt-4"
func FormatModelString(provider, model string) string {
	return fmt.Sprintf("%s:%s", provider, model)
}
