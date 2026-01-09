package cmd

import (
	"fmt"

	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/openai"
)

// newProvider creates a new provider instance based on the configuration
func newProvider(config *llmc.Config) (llmc.Provider, error) {
	provider, _, err := llmc.ParseModelString(config.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid model format: %w", err)
	}

	switch provider {
	case openai.ProviderName:
		return openai.NewProvider(config), nil
	case gemini.ProviderName:
		return gemini.NewProvider(config), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: openai, gemini)", provider)
	}
}
