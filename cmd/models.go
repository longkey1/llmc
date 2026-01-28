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

		// List models for each provider
		for i, targetProvider := range providers {
			if i > 0 {
				fmt.Println() // Add blank line between providers
			}

			// Get token for the specified provider
			token, err := cfg.GetToken(targetProvider)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Skipping %s - %v\n", targetProvider, err)
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
				fmt.Fprintf(os.Stderr, "Warning: Failed to list models for %s - %v\n", targetProvider, modelsErr)
				continue
			}

			if len(models) == 0 {
				fmt.Fprintf(os.Stderr, "Warning: No models returned from %s API\n", targetProvider)
				continue
			}

			// Display provider name
			fmt.Printf("Available models for %s:\n\n", targetProvider)

			// Calculate column width for model names (provider:model format)
			maxModelWidth := 15
			for _, model := range models {
				modelName := llmc.FormatModelString(targetProvider, model.ID)
				if len(modelName) > maxModelWidth {
					maxModelWidth = len(modelName)
				}
			}

			// Display header
			fmt.Printf("%-*s  %-10s  %s\n", maxModelWidth, "MODEL", "DEFAULT", "DESCRIPTION")
			fmt.Printf("%s  %s  %s\n",
				strings.Repeat("-", maxModelWidth),
				strings.Repeat("-", 10),
				strings.Repeat("-", 50))

			// Display models
			for _, model := range models {
				defaultMark := ""
				if model.IsDefault {
					defaultMark = "Yes"
				}
				modelName := llmc.FormatModelString(targetProvider, model.ID)
				fmt.Printf("%-*s  %-10s  %s\n",
					maxModelWidth,
					modelName,
					defaultMark,
					model.Description)
			}

			// Usage hint
			fmt.Printf("\nUse a model with: llmc chat --model <model> [message]\n")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
