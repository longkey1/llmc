/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
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
			if targetProvider == openai.ProviderName {
				cfg.OpenAIToken = token
			} else if targetProvider == gemini.ProviderName {
				cfg.GeminiToken = token
			} else if targetProvider == anthropic.ProviderName {
				cfg.AnthropicToken = token
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "Listing models for provider: %s\n", targetProvider)
			}

			// Get models
			var models []llmc.ModelInfo
			var modelsErr error
			if targetProvider == openai.ProviderName {
				provider := openai.NewProvider(cfg)
				provider.SetDebug(verbose)
				models, modelsErr = provider.ListModels()
			} else if targetProvider == gemini.ProviderName {
				provider := gemini.NewProvider(cfg)
				provider.SetDebug(verbose)
				models, modelsErr = provider.ListModels()
			} else if targetProvider == anthropic.ProviderName {
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

			// Build display rows, inserting a synthetic "@latest" row at the
			// head of each family that has a dated/versioned variant.
			rows := buildModelRows(result.provider, result.models)

			// Calculate column widths
			maxModelWidth := 15
			maxModelIDWidth := 15
			for _, row := range rows {
				if len(row.model) > maxModelWidth {
					maxModelWidth = len(row.model)
				}
				if len(row.modelID) > maxModelIDWidth {
					maxModelIDWidth = len(row.modelID)
				}
			}

			// Display header
			fmt.Printf("%-*s  %-*s  %-10s  %s\n", maxModelWidth, "MODEL", maxModelIDWidth, "MODEL ID", "DEFAULT", "DESCRIPTION")

			// Display rows
			for _, row := range rows {
				fmt.Printf("%-*s  %-*s  %-10s  %s\n",
					maxModelWidth,
					row.model,
					maxModelIDWidth,
					row.modelID,
					row.defaultMark,
					row.description)
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

// modelRow is a single line in the `llmc models` output.
type modelRow struct {
	model       string // MODEL column ("provider:id" or "provider:base@latest")
	modelID     string // MODEL ID column (concrete model ID)
	defaultMark string // DEFAULT column
	description string // DESCRIPTION column
}

// buildModelRows expands the model list into display rows, inserting a synthetic
// "provider:base@latest" row at the head of each family that has at least one
// dated/versioned variant. The resolved target of each @latest row is computed
// with llmc.ResolveLatestModel so the listing matches actual resolution.
func buildModelRows(provider string, models []llmc.ModelInfo) []modelRow {
	// Find family bases that have a variant distinct from the base itself.
	familyHasVariant := make(map[string]bool)
	for _, m := range models {
		if base := deriveModelBase(m.ID); m.ID != base {
			familyHasVariant[base] = true
		}
	}

	// Resolve an @latest row for each qualifying base.
	latestRows := make(map[string]modelRow)
	for base := range familyHasVariant {
		resolved, err := llmc.ResolveLatestModel(models, base)
		if err != nil {
			continue
		}
		latestRows[base] = modelRow{
			model:       llmc.FormatModelString(provider, base+llmc.LatestSuffix),
			modelID:     resolved,
			description: "-> latest of " + base,
		}
	}

	emitted := make(map[string]bool)
	rows := make([]modelRow, 0, len(models)+len(latestRows))
	for _, m := range models {
		base := deriveModelBase(m.ID)
		if row, ok := latestRows[base]; ok && !emitted[base] {
			rows = append(rows, row)
			emitted[base] = true
		}

		defaultMark := ""
		if m.IsDefault {
			defaultMark = "Yes"
		}
		rows = append(rows, modelRow{
			model:       llmc.FormatModelString(provider, m.ID),
			modelID:     m.ID,
			defaultMark: defaultMark,
			description: m.Description,
		})
	}

	return rows
}

// deriveModelBase strips a trailing date/version suffix from a model ID to obtain
// its family base. It only removes recognizable date-shaped or numeric-version
// tails, never the version digits embedded in a model name (e.g. "gpt-4").
//
//	gpt-4o-2024-11-20        -> gpt-4o      (-YYYY-MM-DD)
//	claude-opus-4-5-20251101 -> claude-opus-4-5 (-YYYYMMDD)
//	gpt-4-0613               -> gpt-4       (-MMDD / numeric tail)
//	gpt-4o                   -> gpt-4o      (no suffix)
func deriveModelBase(id string) string {
	parts := strings.Split(id, "-")

	// Trailing -YYYY-MM-DD (4-2-2 digit groups).
	if len(parts) >= 4 &&
		isDigits(parts[len(parts)-1], 2) &&
		isDigits(parts[len(parts)-2], 2) &&
		isDigits(parts[len(parts)-3], 4) {
		return strings.Join(parts[:len(parts)-3], "-")
	}

	// Trailing numeric-only tail of 2+ digits (e.g. -20251101, -0613, -002).
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if len(last) >= 2 && isDigits(last, len(last)) {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}

	return id
}

// isDigits reports whether s has exactly n characters, all of them ASCII digits.
func isDigits(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
