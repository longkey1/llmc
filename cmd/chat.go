/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	provider string
	model    string
	baseURL  string
	prompt   string
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message to the LLM",
	Long: `Send a single message to the LLM and print the response.
This command performs a one-time API call to the specified LLM provider.
It does not maintain conversation history or provide interactive chat functionality.

If no message is provided as an argument, it reads from stdin.

You can specify the provider, model, base URL, and prompt using flags.
If not specified, the values will be taken from the configuration file.

The prompt file should be in TOML format with the following structure:
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from file
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Override with command line flags if provided
		if provider != "" {
			config.Provider = provider
		}
		if model != "" {
			config.Model = model
		}
		if baseURL != "" {
			config.BaseURL = baseURL
		}

		// Debug output
		if verbose {
			fmt.Fprintf(os.Stderr, "Provider: %s\n", config.Provider)
			fmt.Fprintf(os.Stderr, "Model: %s\n", config.Model)
			fmt.Fprintf(os.Stderr, "Base URL: %s\n", config.BaseURL)
			fmt.Fprintf(os.Stderr, "Token: %s\n", config.Token)
			fmt.Fprintf(os.Stderr, "Prompt dir: %s\n", config.PromptDir)
		}

		// Select provider
		llmProvider, err := llmc.NewProvider(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Get message from arguments or stdin
		var message string
		if len(args) > 0 {
			message = strings.Join(args, " ")
		} else {
			// Read from stdin
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
				os.Exit(1)
			}
			message = strings.TrimSpace(input)
		}

		// Format message with prompt if specified
		formattedMessage, err := llmc.FormatMessage(message, prompt, config.PromptDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Send message and print response
		response, err := llmProvider.Chat(formattedMessage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(response)
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add command options
	chatCmd.Flags().StringVar(&provider, "provider", viper.GetString("provider"), "LLM provider (openai or gemini)")
	chatCmd.Flags().StringVarP(&model, "model", "m", viper.GetString("model"), "Model to use")
	chatCmd.Flags().StringVar(&baseURL, "base-url", viper.GetString("base_url"), "Base URL for the API")
	chatCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Name of the prompt template (without .toml extension)")
}
