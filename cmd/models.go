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
	Use:   "models [provider]",
	Short: "List available models for the provider",
	Long: `List all available models for the specified provider.
If no provider is specified, lists models for the currently configured provider.

Available providers: openai, gemini

Examples:
  llmc models           # List models for the configured provider
  llmc models openai    # List OpenAI models
  llmc models gemini    # List Gemini models`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Determine target provider (use configured provider if no arg)
		targetProvider := config.Provider
		if len(args) > 0 {
			targetProvider = args[0]
		}

		// Validate provider
		if targetProvider != openai.ProviderName && targetProvider != gemini.ProviderName {
			fmt.Fprintf(os.Stderr, "Error: unsupported provider '%s'\n", targetProvider)
			fmt.Fprintf(os.Stderr, "Available providers: openai, gemini\n")
			os.Exit(1)
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "Listing models for provider: %s\n", targetProvider)
		}

		// Get models based on provider
		var models []llmc.ModelInfo
		if targetProvider == openai.ProviderName {
			provider := openai.NewProvider(config)
			models = provider.ListModels()
		} else if targetProvider == gemini.ProviderName {
			provider := gemini.NewProvider(config)
			models = provider.ListModels()
		}

		if len(models) == 0 {
			fmt.Fprintf(os.Stderr, "Error: Failed to retrieve models from API.\n")
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
