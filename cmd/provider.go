package cmd

import (
	"fmt"

	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/openai"
)

// newProvider creates a new provider instance based on the configuration
func newProvider(cfg *config.Config) (llmc.Provider, error) {
	provider, _, err := llmc.ParseModelString(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid model format: %w", err)
	}

	switch provider {
	case openai.ProviderName:
		return openai.NewProvider(cfg), nil
	case gemini.ProviderName:
		return gemini.NewProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: openai, gemini)", provider)
	}
}
