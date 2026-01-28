/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

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

Supported providers: openai, gemini

If no provider is specified, lists models from all providers.

Example:
  llmc models           # List models from all providers
  llmc models openai    # List OpenAI models
  llmc models gemini    # List Gemini models`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config to get tokens
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Determine which providers to list
		var providers []string
		if len(args) == 0 {
			// No provider specified, list all
			providers = []string{openai.ProviderName, gemini.ProviderName}
		} else {
			targetProvider := args[0]
			// Validate provider
			if targetProvider != openai.ProviderName && targetProvider != gemini.ProviderName {
				return fmt.Errorf("unsupported provider '%s'\nSupported providers: openai, gemini", targetProvider)
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

			// Get token for the specified provider
			token, err := cfg.GetToken(targetProvider)
			if err != nil {
				result.err = fmt.Errorf("failed to get token: %w", err)
				results = append(results, result)
				continue
			}

			// Temporarily set the token and model for provider initialization
			cfg.Model = llmc.FormatModelString(targetProvider, "temp")
			if targetProvider == openai.ProviderName {
				cfg.OpenAIToken = token
			} else {
				cfg.GeminiToken = token
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
			} else {
				provider := gemini.NewProvider(cfg)
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

			// Calculate column widths
			maxModelWidth := 15
			maxModelIDWidth := 15
			for _, model := range result.models {
				modelName := llmc.FormatModelString(result.provider, model.ID)
				if len(modelName) > maxModelWidth {
					maxModelWidth = len(modelName)
				}
				if len(model.ID) > maxModelIDWidth {
					maxModelIDWidth = len(model.ID)
				}
			}

			// Display header
			fmt.Printf("%-*s  %-*s  %-10s  %s\n", maxModelWidth, "MODEL", maxModelIDWidth, "MODEL ID", "DEFAULT", "DESCRIPTION")
			fmt.Printf("%s  %s  %s  %s\n",
				strings.Repeat("-", maxModelWidth),
				strings.Repeat("-", maxModelIDWidth),
				strings.Repeat("-", 10),
				strings.Repeat("-", 50))

			// Display models
			for _, model := range result.models {
				defaultMark := ""
				if model.IsDefault {
					defaultMark = "Yes"
				}
				modelName := llmc.FormatModelString(result.provider, model.ID)
				fmt.Printf("%-*s  %-*s  %-10s  %s\n",
					maxModelWidth,
					modelName,
					maxModelIDWidth,
					model.ID,
					defaultMark,
					model.Description)
			}

			// Usage hint
			fmt.Printf("\nUse a model with: llmc chat --model <model> [message]\n")
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

func init() {
	rootCmd.AddCommand(modelsCmd)
}
