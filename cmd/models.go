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
	"github.com/longkey1/llmc/internal/openai"
	"github.com/spf13/cobra"
)

// modelsCmd represents the models command
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models for the configured provider",
	Long: `List all available models for the currently configured provider.
Fetches the latest model information directly from the provider's API.

The command uses the provider configured in your config file or via environment variables.

Example:
  llmc models           # List models for the configured provider`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Use configured provider
		targetProvider := config.Provider

		// Validate provider
		if targetProvider != openai.ProviderName && targetProvider != gemini.ProviderName {
			fmt.Fprintf(os.Stderr, "Error: unsupported provider '%s'\n", targetProvider)
			fmt.Fprintf(os.Stderr, "Supported providers: openai, gemini\n")
			fmt.Fprintf(os.Stderr, "Please configure a valid provider in your config file or via LLMC_PROVIDER environment variable.\n")
			os.Exit(1)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Listing models for provider: %s\n", targetProvider)
		}

		// Get models based on provider
		var models []llmc.ModelInfo
		var modelsErr error
		if targetProvider == openai.ProviderName {
			provider := openai.NewProvider(config)
			provider.SetDebug(verbose)
			models, modelsErr = provider.ListModels()
		} else if targetProvider == gemini.ProviderName {
			provider := gemini.NewProvider(config)
			provider.SetDebug(verbose)
			models, modelsErr = provider.ListModels()
		}

		if modelsErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", modelsErr)
			os.Exit(1)
		}

		if len(models) == 0 {
			fmt.Fprintf(os.Stderr, "Error: No models returned from API.\n")
			fmt.Fprintf(os.Stderr, "Please check your API token and network connection.\n")
			os.Exit(1)
		}

		// Display provider name
		fmt.Printf("Available models for %s:\n\n", targetProvider)

		// Calculate column width for model IDs
		maxIDWidth := 15
		for _, model := range models {
			if len(model.ID) > maxIDWidth {
				maxIDWidth = len(model.ID)
			}
		}

		// Display header
		fmt.Printf("%-*s  %-10s  %s\n", maxIDWidth, "MODEL ID", "DEFAULT", "DESCRIPTION")
		fmt.Printf("%s  %s  %s\n",
			strings.Repeat("-", maxIDWidth),
			strings.Repeat("-", 10),
			strings.Repeat("-", 50))

		// Display models
		for _, model := range models {
			defaultMark := ""
			if model.IsDefault {
				defaultMark = "Yes"
			}
			fmt.Printf("%-*s  %-10s  %s\n",
				maxIDWidth,
				model.ID,
				defaultMark,
				model.Description)
		}

		// Usage hint
		fmt.Printf("\nUse a model with: llmc chat --model <model-id> [message]\n")
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
