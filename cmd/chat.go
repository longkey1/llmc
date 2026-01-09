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
	model                 string
	baseURL               string
	prompt                string
	argFlags              []string
	useEditor             bool
	webSearch             bool
	ignoreWebSearchErrors bool
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
model = "optional-model-name"  # Optional: overrides the default model for this prompt
web_search = true  # Optional: enables web search for this prompt"`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration from file
		config, err := llmc.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
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
		formattedMessage, promptModel, promptWebSearch, err := llmc.FormatMessage(message, prompt, config.PromptDirs, argFlags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Apply model with consistent priority: flag > env > prompt template > config file
		envModel := os.Getenv("LLMC_MODEL")
		if cmd.Flags().Changed("model") {
			// 1. Command line flag takes highest priority
			if _, _, err := llmc.ParseModelString(model); err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid model from flag: %v\n", err)
				fmt.Fprintf(os.Stderr, "Model must be in format: provider:model (e.g., openai:gpt-4)\n")
				os.Exit(1)
			}
			config.Model = model
			if verbose {
				fmt.Fprintf(os.Stderr, "Using model from command line flag: %s\n", model)
			}
		} else if envModel != "" {
			// 2. Environment variable is second priority
			if _, _, err := llmc.ParseModelString(envModel); err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid model from environment: %v\n", err)
				fmt.Fprintf(os.Stderr, "Model must be in format: provider:model (e.g., openai:gpt-4)\n")
				os.Exit(1)
			}
			config.Model = envModel
			if verbose {
				fmt.Fprintf(os.Stderr, "Using model from environment variable: %s\n", envModel)
			}
		} else if promptModel != nil {
			// 3. Prompt template setting is third priority
			if _, _, err := llmc.ParseModelString(*promptModel); err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid model from prompt file: %v\n", err)
				fmt.Fprintf(os.Stderr, "Model must be in format: provider:model (e.g., openai:gpt-4)\n")
				os.Exit(1)
			}
			config.Model = *promptModel
			if verbose {
				fmt.Fprintf(os.Stderr, "Using model from prompt file: %s\n", config.Model)
			}
		} else if verbose {
			// 4. Config file or default
			fmt.Fprintf(os.Stderr, "Using model from config file: %s\n", config.Model)
		}

		// Override base URL with command line flag if provided (after model is finalized)
		if baseURL != "" {
			// Determine which provider's base URL to override
			provider, _, err := llmc.ParseModelString(config.Model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing model: %v\n", err)
				os.Exit(1)
			}
			switch provider {
			case "openai":
				config.OpenAIBaseURL = baseURL
			case "gemini":
				config.GeminiBaseURL = baseURL
			default:
				fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", provider)
				os.Exit(1)
			}
			if verbose {
				fmt.Fprintf(os.Stderr, "Overriding %s base URL with: %s\n", provider, baseURL)
			}
		}

		// Debug output
		if verbose {
			provider, modelName, _ := llmc.ParseModelString(config.Model)
			fmt.Fprintf(os.Stderr, "Model: %s (provider: %s, model: %s)\n", config.Model, provider, modelName)

			// Display the base URL for the current provider
			baseURLValue, err := config.GetBaseURL(provider)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get base URL: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "Base URL: %s\n", baseURLValue)
			}

			fmt.Fprintf(os.Stderr, "Prompt dirs: %v\n", config.PromptDirs)
		}

		// Select provider (after potential model override)
		llmProvider, err := newProvider(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Enable web search if specified in flag, env, prompt template, or config file
		// Priority: command line flag > environment variable > prompt template > config file
		var enableWebSearch bool
		envWebSearch := os.Getenv("LLMC_ENABLE_WEB_SEARCH")

		if cmd.Flags().Changed("web-search") {
			// 1. Command line flag takes highest priority
			enableWebSearch = webSearch
			if verbose {
				fmt.Fprintf(os.Stderr, "Using web search setting from command line flag: %v\n", webSearch)
			}
		} else if envWebSearch != "" {
			// 2. Environment variable is second priority
			enableWebSearch = envWebSearch == "true" || envWebSearch == "1"
			if verbose {
				fmt.Fprintf(os.Stderr, "Using web search setting from environment variable: %v\n", enableWebSearch)
			}
		} else if promptWebSearch != nil {
			// 3. Prompt template setting is third priority
			enableWebSearch = *promptWebSearch
			if verbose {
				fmt.Fprintf(os.Stderr, "Using web search setting from prompt file: %v\n", *promptWebSearch)
			}
		} else {
			// 4. Fall back to config file or default
			enableWebSearch = config.EnableWebSearch
			if verbose && config.EnableWebSearch {
				fmt.Fprintf(os.Stderr, "Using web search setting from config file: %v\n", config.EnableWebSearch)
			}
		}
		llmProvider.SetWebSearch(enableWebSearch)

		// Enable ignore web search errors if specified in flag, env, or config file
		// Priority: command line flag > environment variable > config file
		var enableIgnoreWebSearchErrors bool
		envIgnoreWebSearchErrors := os.Getenv("LLMC_IGNORE_WEB_SEARCH_ERRORS")

		if cmd.Flags().Changed("ignore-web-search-errors") {
			// 1. Command line flag takes highest priority
			enableIgnoreWebSearchErrors = ignoreWebSearchErrors
			if verbose {
				fmt.Fprintf(os.Stderr, "Using ignore web search errors setting from command line flag: %v\n", ignoreWebSearchErrors)
			}
		} else if envIgnoreWebSearchErrors != "" {
			// 2. Environment variable is second priority
			enableIgnoreWebSearchErrors = envIgnoreWebSearchErrors == "true" || envIgnoreWebSearchErrors == "1"
			if verbose {
				fmt.Fprintf(os.Stderr, "Using ignore web search errors setting from environment variable: %v\n", enableIgnoreWebSearchErrors)
			}
		} else {
			// 3. Fall back to config file or default
			enableIgnoreWebSearchErrors = config.IgnoreWebSearchErrors
			if verbose && config.IgnoreWebSearchErrors {
				fmt.Fprintf(os.Stderr, "Using ignore web search errors setting from config file: %v\n", config.IgnoreWebSearchErrors)
			}
		}
		llmProvider.SetIgnoreWebSearchErrors(enableIgnoreWebSearchErrors)
		llmProvider.SetDebug(verbose)

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
	chatCmd.Flags().StringVarP(&model, "model", "m", viper.GetString("model"), "Model to use (format: provider:model, e.g., openai:gpt-4)")
	chatCmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL override for the current provider's API")
	chatCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Name of the prompt template (without .toml extension)")
	chatCmd.Flags().StringArrayVar(&argFlags, "arg", []string{}, "Key-value pairs for prompt template (format: key:value)")
	chatCmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Use default editor (from EDITOR environment variable) to compose message")
	chatCmd.Flags().BoolVar(&webSearch, "web-search", false, "Enable web search for real-time information")
	chatCmd.Flags().BoolVar(&ignoreWebSearchErrors, "ignore-web-search-errors", false, "Automatically retry without web search if web search fails to return a response")
}
