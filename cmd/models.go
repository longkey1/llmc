/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/longkey1/llmc/internal/anthropic"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/openai"
	"github.com/spf13/cobra"
)

// modelsCmd represents the models command
var modelsCmd = &cobra.Command{
	Use:   "models [provider]",
	Short: "List available models for the specified provider(s)",
	Long: `List all available models for the specified provider.
Fetches the latest model information directly from the provider's API.

Supported providers: openai, gemini, anthropic

If no provider is specified, lists models from all providers.

Example:
  llmc models              # List models from all providers
  llmc models openai       # List OpenAI models
  llmc models gemini       # List Gemini models
  llmc models anthropic    # List Anthropic models`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config to get tokens
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Save the original default model for comparison
		originalModel := cfg.Model

		// Determine which providers to list
		var providers []string
		providerExplicitlySpecified := len(args) > 0

		if !providerExplicitlySpecified {
			// No provider specified, list all
			providers = []string{openai.ProviderName, gemini.ProviderName, anthropic.ProviderName}
		} else {
			targetProvider := args[0]
			// Validate provider
			if targetProvider != openai.ProviderName && targetProvider != gemini.ProviderName && targetProvider != anthropic.ProviderName {
				return fmt.Errorf("unsupported provider '%s'\nSupported providers: openai, gemini, anthropic", targetProvider)
			}
			providers = []string{targetProvider}
		}

		// Collect results and errors for all providers
		type providerResult struct {
			provider string
			models   []llmc.ModelInfo
			err      error
		}

		var results []providerResult

		// List models for each provider
		for _, targetProvider := range providers {
			result := providerResult{provider: targetProvider}

			// Extract default model ID for this provider
			var defaultModelID string
			if originalModel != "" {
				provider, model, err := llmc.ParseModelString(originalModel)
				if err == nil && provider == targetProvider {
					defaultModelID = model
				}
			}

			// Get token for the specified provider
			token, err := cfg.GetToken(targetProvider)
			if err != nil {
				// If provider was not explicitly specified, skip silently
				if !providerExplicitlySpecified {
					continue
				}
				// If provider was explicitly specified, return error
				result.err = fmt.Errorf("failed to get token: %w", err)
				results = append(results, result)
				continue
			}

			// Temporarily set the token and model for provider initialization
			cfg.Model = llmc.FormatModelString(targetProvider, "temp")
			switch targetProvider {
			case openai.ProviderName:
				cfg.OpenAIToken = token
			case gemini.ProviderName:
				cfg.GeminiToken = token
			case anthropic.ProviderName:
				cfg.AnthropicToken = token
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Listing models for provider: %s\n", targetProvider)
			}

			// Get models
			var models []llmc.ModelInfo
			var modelsErr error
			switch targetProvider {
			case openai.ProviderName:
				provider := openai.NewProvider(cfg)
				provider.SetDebug(verbose)
				models, modelsErr = provider.ListModels()
			case gemini.ProviderName:
				provider := gemini.NewProvider(cfg)
				provider.SetDebug(verbose)
				models, modelsErr = provider.ListModels()
			case anthropic.ProviderName:
				provider := anthropic.NewProvider(cfg)
				provider.SetDebug(verbose)
				models, modelsErr = provider.ListModels()
			}

			if modelsErr != nil {
				result.err = fmt.Errorf("failed to list models: %w", modelsErr)
				results = append(results, result)
				continue
			}

			if len(models) == 0 {
				result.err = fmt.Errorf("no models returned from API")
				results = append(results, result)
				continue
			}

			// Set IsDefault based on the original default model
			if defaultModelID != "" {
				for i := range models {
					if models[i].ID == defaultModelID {
						models[i].IsDefault = true
					}
				}
			}

			result.models = models
			results = append(results, result)
		}

		// Display successful results first
		successCount := 0
		for _, result := range results {
			if result.err != nil {
				continue
			}

			if successCount > 0 {
				fmt.Println() // Add blank line between providers
			}
			successCount++

			// Display provider name
			fmt.Printf("Available models for %s:\n\n", result.provider)

			// Map resolved model IDs to the aliases that point at them
			aliasesByModel, unmatchedAliases := resolveAliasesByModel(cfg, result.provider, result.models)
			for _, alias := range unmatchedAliases {
				fmt.Fprintf(os.Stderr, "Warning: alias %s matches no model on %s\n", alias, result.provider)
			}

			// Calculate column widths
			maxModelWidth := 15
			maxModelIDWidth := 15
			maxAliasWidth := 5
			maxDescWidth := 50
			for _, model := range result.models {
				modelName := llmc.FormatModelString(result.provider, model.ID)
				if len(modelName) > maxModelWidth {
					maxModelWidth = len(modelName)
				}
				if len(model.ID) > maxModelIDWidth {
					maxModelIDWidth = len(model.ID)
				}
				if aliases := aliasesByModel[model.ID]; len(aliases) > maxAliasWidth {
					maxAliasWidth = len(aliases)
				}
				if len(model.Description) > maxDescWidth {
					maxDescWidth = len(model.Description)
				}
			}

			// Display header
			fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n", maxModelWidth, "MODEL", maxModelIDWidth, "MODEL ID", maxDescWidth, "DESCRIPTION", maxAliasWidth, "ALIAS", "DEFAULT")
			fmt.Printf("%s  %s  %s  %s  %s\n",
				strings.Repeat("-", maxModelWidth),
				strings.Repeat("-", maxModelIDWidth),
				strings.Repeat("-", maxDescWidth),
				strings.Repeat("-", maxAliasWidth),
				strings.Repeat("-", 10))

			// Display models
			for _, model := range result.models {
				defaultMark := ""
				if model.IsDefault {
					defaultMark = "Yes"
				}
				modelName := llmc.FormatModelString(result.provider, model.ID)
				fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
					maxModelWidth,
					modelName,
					maxModelIDWidth,
					model.ID,
					maxDescWidth,
					model.Description,
					maxAliasWidth,
					aliasesByModel[model.ID],
					defaultMark)
			}

		}

		// Display errors at the end
		errorCount := 0
		for _, result := range results {
			if result.err == nil {
				continue
			}

			if errorCount == 0 && successCount > 0 {
				fmt.Println() // Add blank line before error section
			}
			errorCount++

			fmt.Fprintf(os.Stderr, "Warning: Skipping %s - %v\n", result.provider, result.err)
		}

		return nil
	},
}

// resolveAliasesByModel resolves every config-defined alias targeting the
// given provider against the already-fetched model list (no extra API call).
// It returns a map of model ID to a comma-separated list of the aliases that
// resolve to it, plus the aliases whose pattern matched no model.
func resolveAliasesByModel(cfg *config.Config, provider string, models []llmc.ModelInfo) (map[string]string, []string) {
	names := make([]string, 0, len(cfg.ModelAliases))
	for name := range cfg.ModelAliases {
		names = append(names, name)
	}
	sort.Strings(names)

	aliasesByModel := make(map[string]string)
	var unmatched []string
	for _, name := range names {
		aliasProvider, pattern, err := llmc.ParseModelString(cfg.ModelAliases[name])
		if err != nil || aliasProvider != provider {
			continue
		}

		resolved := pattern
		if llmc.HasModelPattern(pattern) {
			resolution, err := llmc.ResolveModelPattern(models, pattern)
			if err != nil {
				unmatched = append(unmatched, llmc.AliasPrefix+name)
				continue
			}
			resolved = resolution.Resolved
		}

		if existing, ok := aliasesByModel[resolved]; ok {
			aliasesByModel[resolved] = existing + ", " + llmc.AliasPrefix + name
		} else {
			aliasesByModel[resolved] = llmc.AliasPrefix + name
		}
	}
	return aliasesByModel, unmatched
}

// modelsResolveCmd resolves a model alias or wildcard pattern to a concrete model
var modelsResolveCmd = &cobra.Command{
	Use:   "resolve <@alias|provider:model-pattern>",
	Short: "Resolve a model alias or wildcard pattern to a concrete model",
	Long: `Resolve a config-defined model alias ("@alias") or a wildcard model
pattern ("provider:model-pattern") to the newest matching concrete model,
using the provider's model list.

The resolved "provider:model" is printed to stdout, so the output can be
used directly in scripts.

Example:
  llmc models resolve @sonnet
  llmc models resolve "openai:anthropic/claude-sonnet-*"
  llmc chat -m "$(llmc models resolve @sonnet)" "Hello"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		input := args[0]
		if llmc.IsModelAlias(input) {
			expanded, err := expandModelAlias(cfg, input)
			if err != nil {
				return err
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "Alias %s -> %s\n", input, expanded)
			}
			input = expanded
		}

		provider, pattern, err := llmc.ParseModelString(input)
		if err != nil {
			return fmt.Errorf("invalid model format: %w", err)
		}

		if !llmc.HasModelPattern(pattern) {
			fmt.Println(llmc.FormatModelString(provider, pattern))
			return nil
		}

		cfg.Model = input
		p, err := newProviderByName(cfg, provider)
		if err != nil {
			return err
		}
		p.SetDebug(verbose)

		models, err := p.ListModels()
		if err != nil {
			return fmt.Errorf("listing models for %s: %w", provider, err)
		}

		resolution, err := llmc.ResolveModelPattern(models, pattern)
		if err != nil {
			return fmt.Errorf("resolving %q: %w (check available models with: llmc models %s)", input, err, provider)
		}

		if verbose {
			for _, c := range resolution.Candidates {
				fmt.Fprintf(os.Stderr, "candidate: %s\n", c.ID)
			}
		}
		fmt.Println(llmc.FormatModelString(provider, resolution.Resolved))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(modelsResolveCmd)
}
