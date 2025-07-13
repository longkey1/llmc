/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	provider  string
	model     string
	baseURL   string
	prompt    string
	argFlags  []string
	useEditor bool
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message to the LLM",
	Long: `Send a single message to the LLM and print the response.
This command performs a one-time API call to the specified LLM provider.
It does not maintain conversation history or provide interactive chat functionality.

If no message is provided as an argument, it reads from stdin.
If --editor flag is set, it opens the default editor (from EDITOR environment variable) to compose the message.

You can specify the provider, model, base URL, and prompt using flags.
If not specified, the values will be taken from the configuration file.

The prompt file should be in TOML format with the following structure:
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"
model = "optional-model-name"  # Optional: overrides the default model for this prompt"`,
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
			fmt.Fprintf(os.Stderr, "Prompt dirs: %v\n", config.PromptDirs)
		}

		// Get message from arguments, editor, or stdin
		var message string
		if useEditor {
			message, err = getMessageFromEditor()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else if len(args) > 0 {
			message = strings.Join(args, " ")
		} else {
			// Read from stdin
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			message = strings.TrimSpace(string(input))
		}

		// Format message with prompt and arguments
		formattedMessage, promptModel, err := llmc.FormatMessage(message, prompt, config.PromptDirs, argFlags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Override model with prompt file model if specified
		if promptModel != nil {
			config.Model = *promptModel
			if verbose {
				fmt.Fprintf(os.Stderr, "Using model from prompt file: %s\n", config.Model)
			}
		}

		// Select provider (after potential model override)
		llmProvider, err := llmc.NewProvider(config)
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

// getMessageFromEditor opens the default editor and returns the edited message
func getMessageFromEditor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return "", fmt.Errorf("EDITOR environment variable is not set")
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "llmc-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Open the editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to open editor: %v", err)
	}

	// Read the edited content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %v", err)
	}

	return strings.TrimSpace(string(content)), nil
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Add command options
	chatCmd.Flags().StringVar(&provider, "provider", viper.GetString("provider"), "LLM provider (openai or gemini)")
	chatCmd.Flags().StringVarP(&model, "model", "m", viper.GetString("model"), "Model to use")
	chatCmd.Flags().StringVar(&baseURL, "base-url", viper.GetString("base_url"), "Base URL for the API")
	chatCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Name of the prompt template (without .toml extension)")
	chatCmd.Flags().StringArrayVar(&argFlags, "arg", []string{}, "Key-value pairs for prompt template (format: key:value)")
	chatCmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Use default editor (from EDITOR environment variable) to compose message")
}
