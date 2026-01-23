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
	Use:   "models <provider>",
	Short: "List available models for the specified provider",
	Long: `List all available models for the specified provider.
Fetches the latest model information directly from the provider's API.

Supported providers: openai, gemini

Example:
  llmc models openai    # List OpenAI models
  llmc models gemini    # List Gemini models`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetProvider := args[0]

		// Validate provider
		if targetProvider != openai.ProviderName && targetProvider != gemini.ProviderName {
			fmt.Fprintf(os.Stderr, "Error: unsupported provider '%s'\n", targetProvider)
			fmt.Fprintf(os.Stderr, "Supported providers: openai, gemini\n")
			os.Exit(1)
		}

		// Load config to get token
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Get token for the specified provider
		token, err := cfg.GetToken(targetProvider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "Please configure %s_token in your config file\n", targetProvider)
			os.Exit(1)
		}

		// Temporarily set the token and model for provider initialization
		cfg.Model = fmt.Sprintf("%s:temp", targetProvider)
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
		fmt.Printf("\nUse a model with: llmc chat --model %s:<model-id> [message]\n", targetProvider)
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
}
