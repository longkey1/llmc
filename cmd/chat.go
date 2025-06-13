/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/openai"
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
		// Load configuration
		config := llmc.Config{
			Provider:  provider,
			Model:     model,
			BaseURL:   baseURL,
			Token:     viper.GetString("token"),
			PromptDir: viper.GetString("prompt_dir"),
		}

		// Select provider
		var llmProvider llmc.Provider
		switch config.Provider {
		case "openai":
			llmProvider = openai.NewProvider(config)
		case "gemini":
			llmProvider = gemini.NewProvider(config)
		default:
			fmt.Fprintf(os.Stderr, "Unsupported provider: %s\n", config.Provider)
			os.Exit(1)
		}

		// Load prompt if specified
		var promptTemplate *llmc.Prompt
		if prompt != "" {
			// Add .toml extension if not present
			promptFile := prompt
			if !strings.HasSuffix(promptFile, ".toml") {
				promptFile = promptFile + ".toml"
			}
			// Construct full path to prompt file
			promptPath := filepath.Join(config.PromptDir, promptFile)

			var err error
			promptTemplate, err = llmc.LoadPrompt(promptPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading prompt file: %v\n", err)
				os.Exit(1)
			}
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
		if promptTemplate != nil {
			systemPrompt, userPrompt := promptTemplate.FormatPrompt(message)
			message = fmt.Sprintf("System: %s\n\nUser: %s", systemPrompt, userPrompt)
		}

		// Send message and print response
		response, err := llmProvider.Chat(message)
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
	chatCmd.Flags().StringVarP(&provider, "provider", "p", "", "LLM provider (openai or gemini)")
	chatCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use")
	chatCmd.Flags().StringVarP(&baseURL, "base-url", "b", "", "Base URL for the API")
	chatCmd.Flags().StringVarP(&prompt, "prompt", "f", "", "Name of the prompt template (without .toml extension)")
}
