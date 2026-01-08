package cmd

import (
	"fmt"

	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/openai"
)

// newProvider creates a new provider instance based on the configuration
func newProvider(config *llmc.Config) (llmc.Provider, error) {
	switch config.Provider {
	case openai.ProviderName:
		return openai.NewProvider(config), nil
	case gemini.ProviderName:
		return gemini.NewProvider(config), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}
