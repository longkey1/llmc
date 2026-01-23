/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/session"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	model                 string
	prompt                string
	argFlags              []string
	useEditor             bool
	webSearch             bool
	ignoreWebSearchErrors bool
	sessionID             string
	newSession            bool
	sessionName           string
	interactive           bool
	ignoreThreshold       bool
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

You can specify the provider, model, and prompt using flags.
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

		// Validate session flags
		if sessionID != "" && newSession {
			fmt.Fprintf(os.Stderr, "Error: cannot specify both --session and --new-session\n")
			os.Exit(1)
		}

		// Cannot use prompt with existing session
		if sessionID != "" && prompt != "" {
			fmt.Fprintf(os.Stderr, "Error: cannot use --prompt with existing session\n")
			os.Exit(1)
		}

		// Interactive mode requires a session
		if interactive && sessionID == "" && !newSession {
			fmt.Fprintf(os.Stderr, "Error: interactive mode requires --session or --new-session\n")
			os.Exit(1)
		}

		// Get message from arguments, editor, or stdin (not required for interactive mode)
		var message string
		if !interactive {
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
		} else {
			// Interactive mode: optional initial message
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}
		}

		// Determine session mode
		var sess *session.Session
		var systemPrompt string
		var isNewSession bool

		if sessionID != "" {
			// Load existing session
			sess, err = session.FindSessionByPrefix(sessionID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Check message threshold
			threshold := config.SessionMessageThreshold
			if threshold > 0 && sess.MessageCount() >= threshold && !ignoreThreshold {
				fmt.Fprintf(os.Stderr, "\nWarning: Session %s has %d messages (threshold: %d).\n",
					sess.GetShortID(), sess.MessageCount(), threshold)
				fmt.Fprintf(os.Stderr, "Long sessions may impact performance and token usage.\n")
				fmt.Fprintf(os.Stderr, "\nOptions:\n")
				fmt.Fprintf(os.Stderr, "  1. Continue anyway with --ignore-threshold flag\n")
				fmt.Fprintf(os.Stderr, "  2. Summarize session: llmc sessions summarize %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "  3. Start a new session: llmc chat --new-session\n\n")

				// Ask for confirmation
				fmt.Fprint(os.Stderr, "Continue with this session? [y/N]: ")
				var response string
				fmt.Scanln(&response)

				if response != "y" && response != "Y" {
					fmt.Fprintln(os.Stderr, "Cancelled.")
					os.Exit(0)
				}
			}

			// Use session's system prompt and model
			systemPrompt = sess.SystemPrompt
			config.Model = sess.Provider + ":" + sess.Model

			if verbose {
				fmt.Fprintf(os.Stderr, "Continuing session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s:%s\n", sess.Provider, sess.Model)
				if systemPrompt != "" {
					fmt.Fprintf(os.Stderr, "System prompt: %s\n", systemPrompt)
				}
			}
		} else if newSession {
			// Create new session
			isNewSession = true

			// Format message with prompt if specified
			var formattedMessage string
			var promptModel *string
			var promptWebSearch *bool
			if prompt != "" {
				formattedMessage, promptModel, promptWebSearch, err = llmc.FormatMessage(message, prompt, config.PromptDirs, argFlags)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					os.Exit(1)
				}

				// Extract system prompt from formatted message
				if strings.HasPrefix(formattedMessage, "System: ") {
					parts := strings.SplitN(formattedMessage, "\n\nUser: ", 2)
					if len(parts) == 2 {
						systemPrompt = strings.TrimPrefix(parts[0], "System: ")
						message = parts[1] // Use formatted user message
					}
				}

				// Apply model from prompt template
				if promptModel != nil {
					if _, _, err := llmc.ParseModelString(*promptModel); err != nil {
						fmt.Fprintf(os.Stderr, "Error: invalid model from prompt file: %v\n", err)
						os.Exit(1)
					}
					config.Model = *promptModel
					if verbose {
						fmt.Fprintf(os.Stderr, "Using model from prompt file: %s\n", config.Model)
					}
				}

				// Apply web search from prompt template
				if promptWebSearch != nil && !cmd.Flags().Changed("web-search") {
					config.EnableWebSearch = *promptWebSearch
				}
			}

			// Apply model with priority: flag > env > prompt template > config file
			envModel := os.Getenv("LLMC_MODEL")
			if cmd.Flags().Changed("model") {
				if _, _, err := llmc.ParseModelString(model); err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid model from flag: %v\n", err)
					os.Exit(1)
				}
				config.Model = model
			} else if envModel != "" {
				if _, _, err := llmc.ParseModelString(envModel); err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid model from environment: %v\n", err)
					os.Exit(1)
				}
				config.Model = envModel
			}

			// Parse provider and model
			provider, modelName, err := llmc.ParseModelString(config.Model)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid model format: %v\n", err)
				os.Exit(1)
			}

			// Create new session
			sess = session.NewSession(provider, modelName)
			sess.Name = sessionName
			sess.TemplateName = prompt
			sess.SystemPrompt = systemPrompt

			if verbose {
				fmt.Fprintf(os.Stderr, "Creating new session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s:%s\n", provider, modelName)
				if systemPrompt != "" {
					fmt.Fprintf(os.Stderr, "System prompt: %s\n", systemPrompt)
				}
			}
		} else {
			// Single-shot mode (no session)
			formattedMessage, promptModel, promptWebSearch, err := llmc.FormatMessage(message, prompt, config.PromptDirs, argFlags)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Apply model priority
			envModel := os.Getenv("LLMC_MODEL")
			if cmd.Flags().Changed("model") {
				if _, _, err := llmc.ParseModelString(model); err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid model from flag: %v\n", err)
					os.Exit(1)
				}
				config.Model = model
			} else if envModel != "" {
				if _, _, err := llmc.ParseModelString(envModel); err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid model from environment: %v\n", err)
					os.Exit(1)
				}
				config.Model = envModel
			} else if promptModel != nil {
				if _, _, err := llmc.ParseModelString(*promptModel); err != nil {
					fmt.Fprintf(os.Stderr, "Error: invalid model from prompt file: %v\n", err)
					os.Exit(1)
				}
				config.Model = *promptModel
			}

			// Select provider
			llmProvider, err := newProvider(config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Configure web search
			enableWebSearch := config.EnableWebSearch
			envWebSearch := os.Getenv("LLMC_ENABLE_WEB_SEARCH")
			if cmd.Flags().Changed("web-search") {
				enableWebSearch = webSearch
			} else if envWebSearch != "" {
				enableWebSearch = envWebSearch == "true" || envWebSearch == "1"
			} else if promptWebSearch != nil {
				enableWebSearch = *promptWebSearch
			}
			llmProvider.SetWebSearch(enableWebSearch)

			// Configure ignore web search errors
			enableIgnoreWebSearchErrors := config.IgnoreWebSearchErrors
			envIgnoreWebSearchErrors := os.Getenv("LLMC_IGNORE_WEB_SEARCH_ERRORS")
			if cmd.Flags().Changed("ignore-web-search-errors") {
				enableIgnoreWebSearchErrors = ignoreWebSearchErrors
			} else if envIgnoreWebSearchErrors != "" {
				enableIgnoreWebSearchErrors = envIgnoreWebSearchErrors == "true" || envIgnoreWebSearchErrors == "1"
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
			return
		}

		// Select provider
		llmProvider, err := newProvider(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Configure web search
		enableWebSearch := config.EnableWebSearch
		envWebSearch := os.Getenv("LLMC_ENABLE_WEB_SEARCH")
		if cmd.Flags().Changed("web-search") {
			enableWebSearch = webSearch
		} else if envWebSearch != "" {
			enableWebSearch = envWebSearch == "true" || envWebSearch == "1"
		}
		llmProvider.SetWebSearch(enableWebSearch)

		// Configure ignore web search errors
		enableIgnoreWebSearchErrors := config.IgnoreWebSearchErrors
		envIgnoreWebSearchErrors := os.Getenv("LLMC_IGNORE_WEB_SEARCH_ERRORS")
		if cmd.Flags().Changed("ignore-web-search-errors") {
			enableIgnoreWebSearchErrors = ignoreWebSearchErrors
		} else if envIgnoreWebSearchErrors != "" {
			enableIgnoreWebSearchErrors = envIgnoreWebSearchErrors == "true" || envIgnoreWebSearchErrors == "1"
		}
		llmProvider.SetIgnoreWebSearchErrors(enableIgnoreWebSearchErrors)
		llmProvider.SetDebug(verbose)

		// If message is provided, send it
		if message != "" {
			// Session mode: add message to session
			sess.AddMessage("user", message)

			// Send message with history (exclude the last message which was just added)
			historyMessages := sess.Messages[:len(sess.Messages)-1]
			response, err := llmProvider.ChatWithHistory(sess.SystemPrompt, historyMessages, message)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Add assistant response to session
			sess.AddMessage("assistant", response)

			// Save session
			if err := session.SaveSession(sess); err != nil {
				fmt.Fprintf(os.Stderr, "Error saving session: %v\n", err)
				os.Exit(1)
			}

			// Print response
			fmt.Println(response)
		}

		// If new session, print session info
		if isNewSession {
			fmt.Fprintf(os.Stderr, "\nSession created: %s\n", sess.GetShortID())
			sessionDir, _ := session.GetSessionDir()
			fmt.Fprintf(os.Stderr, "Path: %s/%s.json\n", sessionDir, sess.ID)
			if !interactive {
				fmt.Fprintf(os.Stderr, "\nNext time, use:\n  llmc chat -s %s \"your message\"\n", sess.GetShortID())
			}
		}

		// If interactive mode, start the loop
		if interactive {
			if err := runInteractiveMode(sess, llmProvider); err != nil {
				fmt.Fprintf(os.Stderr, "Error in interactive mode: %v\n", err)
				os.Exit(1)
			}
		}
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
	chatCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Name of the prompt template (without .toml extension)")
	chatCmd.Flags().StringArrayVar(&argFlags, "arg", []string{}, "Key-value pairs for prompt template (format: key:value)")
	chatCmd.Flags().BoolVarP(&useEditor, "editor", "e", false, "Use default editor (from EDITOR environment variable) to compose message")
	chatCmd.Flags().BoolVar(&webSearch, "web-search", false, "Enable web search for real-time information")
	chatCmd.Flags().BoolVar(&ignoreWebSearchErrors, "ignore-web-search-errors", false, "Automatically retry without web search if web search fails to return a response")

	// Session flags
	chatCmd.Flags().StringVarP(&sessionID, "session", "s", "", "Session ID (short or full UUID, or 'latest' for most recent session)")
	chatCmd.Flags().BoolVarP(&newSession, "new-session", "n", false, "Create a new session")
	chatCmd.Flags().StringVar(&sessionName, "session-name", "", "Name for the new session (optional)")
	chatCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Start interactive mode for multi-turn conversations")
	chatCmd.Flags().BoolVar(&ignoreThreshold, "ignore-threshold", false, "Ignore session message threshold warning")
}

// runInteractiveMode starts an interactive chat session
func runInteractiveMode(sess *session.Session, llmProvider llmc.Provider) error {
	// Print session header
	fmt.Fprintf(os.Stderr, "\n=== Interactive Session [%s] ===\n", sess.GetShortID())
	fmt.Fprintf(os.Stderr, "Provider: %s, Model: %s\n", sess.Provider, sess.Model)
	if sess.SystemPrompt != "" {
		fmt.Fprintf(os.Stderr, "System Prompt: %s\n", sess.SystemPrompt)
	}
	fmt.Fprintf(os.Stderr, "Type '/help' for commands, '/exit' or 'Ctrl+D' to quit\n")
	fmt.Fprintf(os.Stderr, "===================================\n\n")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Display prompt
		fmt.Fprint(os.Stderr, "You> ")

		// Read input
		if !scanner.Scan() {
			// EOF (Ctrl+D) or error
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("input error: %w", err)
			}
			// Clean EOF
			fmt.Fprintln(os.Stderr, "\nGoodbye!")
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Skip empty input
		if input == "" {
			continue
		}

		// Handle special commands
		if strings.HasPrefix(input, "/") {
			if handleSpecialCommand(input, sess) {
				// Continue loop if command was handled
				continue
			}
			// Exit if command returned false
			break
		}

		// Add user message to session
		sess.AddMessage("user", input)

		// Get conversation history (excluding the just-added message)
		historyMessages := sess.Messages[:len(sess.Messages)-1]

		// Send message with history
		response, err := llmProvider.ChatWithHistory(sess.SystemPrompt, historyMessages, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			// Remove the failed message from history
			sess.Messages = sess.Messages[:len(sess.Messages)-1]
			continue
		}

		// Add assistant response
		sess.AddMessage("assistant", response)

		// Save session after each turn
		if err := session.SaveSession(sess); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
		}

		// Print response
		fmt.Printf("\nAssistant> %s\n\n", response)
	}

	return nil
}

// handleSpecialCommand processes special commands in interactive mode
// Returns true to continue the loop, false to exit
func handleSpecialCommand(command string, sess *session.Session) bool {
	command = strings.ToLower(strings.TrimSpace(command))

	switch command {
	case "/help", "/h":
		fmt.Fprintln(os.Stderr, "\nAvailable commands:")
		fmt.Fprintln(os.Stderr, "  /help, /h     - Show this help message")
		fmt.Fprintln(os.Stderr, "  /info, /i     - Show session information")
		fmt.Fprintln(os.Stderr, "  /clear, /c    - Clear screen (Unix/Linux only)")
		fmt.Fprintln(os.Stderr, "  /exit, /quit  - Exit interactive mode")
		fmt.Fprintln(os.Stderr, "  Ctrl+D        - Exit interactive mode")
		fmt.Fprintln(os.Stderr, "")
		return true

	case "/info", "/i":
		fmt.Fprintln(os.Stderr, "\nSession Information:")
		fmt.Fprintf(os.Stderr, "  ID: %s\n", sess.GetShortID())
		fmt.Fprintf(os.Stderr, "  Full ID: %s\n", sess.ID)
		if sess.Name != "" {
			fmt.Fprintf(os.Stderr, "  Name: %s\n", sess.Name)
		}
		fmt.Fprintf(os.Stderr, "  Provider: %s\n", sess.Provider)
		fmt.Fprintf(os.Stderr, "  Model: %s\n", sess.Model)
		fmt.Fprintf(os.Stderr, "  Messages: %d\n", sess.MessageCount())
		fmt.Fprintf(os.Stderr, "  Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
		if sess.TemplateName != "" {
			fmt.Fprintf(os.Stderr, "  Template: %s\n", sess.TemplateName)
		}
		fmt.Fprintln(os.Stderr, "")
		return true

	case "/clear", "/c":
		// Clear screen (Unix/Linux)
		fmt.Print("\033[H\033[2J")
		return true

	case "/exit", "/quit", "/q":
		fmt.Fprintln(os.Stderr, "Goodbye!")
		return false

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s (type '/help' for available commands)\n", command)
		return true
	}
}
