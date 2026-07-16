package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/longkey1/llmc/internal/anthropic"
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

	return newProviderByName(cfg, provider)
}

// newProviderByName creates a provider instance for the given provider name.
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

// validateModelOrAlias validates that s is either a "provider:model" string or
// an "@alias" reference. Alias existence is checked later by resolveModelAlias.
func validateModelOrAlias(s string) error {
	if llmc.IsModelAlias(s) {
		return nil
	}
	_, _, err := llmc.ParseModelString(s)
	return err
}

// expandModelAlias looks up an "@alias" reference in the config alias map and
// returns the configured "provider:model" value (which may contain wildcards).
func expandModelAlias(cfg *config.Config, s string) (string, error) {
	name := strings.TrimPrefix(s, llmc.AliasPrefix)
	if value, ok := cfg.ModelAliases[name]; ok {
		return value, nil
	}

	names := make([]string, 0, len(cfg.ModelAliases))
	for n := range cfg.ModelAliases {
		names = append(names, llmc.AliasPrefix+n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "", fmt.Errorf("model alias %q is not defined (add [model_aliases] to your config file)", s)
	}
	return "", fmt.Errorf("model alias %q is not defined (defined aliases: %s)", s, strings.Join(names, ", "))
}

// resolveModelAlias resolves cfg.Model when it is an "@alias" reference.
// The alias is expanded via the config alias map; if the configured value
// contains a wildcard (e.g., "openai:anthropic/claude-sonnet-*"), it is
// resolved to the newest matching model from the provider's model list.
//
// Non-alias models are left untouched, and alias values without wildcards are
// used as-is — neither case calls the provider API. An undefined alias is an
// error, as is a wildcard that cannot be resolved.
func resolveModelAlias(ctx context.Context, cfg *config.Config) error {
	if !llmc.IsModelAlias(cfg.Model) {
		return nil
	}
	input := cfg.Model

	expanded, err := expandModelAlias(cfg, input)
	if err != nil {
		return err
	}

	provider, pattern, err := llmc.ParseModelString(expanded)
	if err != nil {
		return fmt.Errorf("invalid model alias value for %q: %w", input, err)
	}
	cfg.Model = expanded

	if !llmc.HasModelPattern(pattern) {
		if verbose {
			fmt.Fprintf(os.Stderr, "Resolved %s -> %s\n", input, cfg.Model)
		}
		return nil
	}

	p, err := newProviderByName(cfg, provider)
	if err != nil {
		return fmt.Errorf("resolving model alias %q: %w", input, err)
	}
	p.SetDebug(verbose)

	models, err := p.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("listing models to resolve alias %q (%s): %w", input, expanded, err)
	}

	resolution, err := llmc.ResolveModelPattern(models, pattern)
	if err != nil {
		return fmt.Errorf("resolving model alias %q (%s): %w", input, expanded, err)
	}

	cfg.Model = llmc.FormatModelString(provider, resolution.Resolved)
	if verbose {
		fmt.Fprintf(os.Stderr, "Resolved %s -> %s\n", input, cfg.Model)
		for i, c := range resolution.Candidates {
			if i >= 5 {
				fmt.Fprintf(os.Stderr, "  ... and %d more candidates\n", len(resolution.Candidates)-i)
				break
			}
			fmt.Fprintf(os.Stderr, "  candidate: %s\n", c.ID)
		}
	}
	return nil
}
