package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/longkey1/llmc/internal/anthropic"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/openai"
)

// newProviderByName creates a new provider instance for the given provider name.
func newProviderByName(cfg *config.Config, provider string) (llmc.Provider, error) {
	switch provider {
	case openai.ProviderName:
		return openai.NewProvider(cfg), nil
	case gemini.ProviderName:
		return gemini.NewProvider(cfg), nil
	case anthropic.ProviderName:
		return anthropic.NewProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: openai, gemini, anthropic)", provider)
	}
}

// newProvider creates a new provider instance based on the configuration
func newProvider(cfg *config.Config) (llmc.Provider, error) {
	provider, _, err := llmc.ParseModelString(cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("invalid model format: %w", err)
	}

	return newProviderByName(cfg, provider)
}

// resolveModelAlias resolves a "provider:base@latest" model string in cfg.Model
// into a concrete "provider:model" string by querying the provider's available
// models. It is a no-op when cfg.Model does not use the @latest suffix.
func resolveModelAlias(cfg *config.Config) error {
	provider, model, err := llmc.ParseModelString(cfg.Model)
	if err != nil {
		return fmt.Errorf("invalid model format: %w", err)
	}

	if !strings.HasSuffix(model, llmc.LatestSuffix) {
		return nil
	}

	base := strings.TrimSuffix(model, llmc.LatestSuffix)

	p, err := newProviderByName(cfg, provider)
	if err != nil {
		return err
	}
	p.SetDebug(verbose)

	models, err := p.ListModels()
	if err != nil {
		return fmt.Errorf("resolving %s%s: %w", base, llmc.LatestSuffix, err)
	}

	resolved, err := llmc.ResolveLatestModel(models, base)
	if err != nil {
		return err
	}

	cfg.Model = llmc.FormatModelString(provider, resolved)

	if verbose {
		fmt.Fprintf(os.Stderr, "Resolved %s:%s%s -> %s\n", provider, base, llmc.LatestSuffix, cfg.Model)
	}

	return nil
}
