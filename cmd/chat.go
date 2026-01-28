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
	"github.com/longkey1/llmc/internal/llmc/config"
	promptpkg "github.com/longkey1/llmc/internal/llmc/prompt"
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
	ignoreThreshold       bool
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message to the LLM",
	Long: `Send a message to the LLM and print the response.
This command performs a one-time API call to the specified LLM provider.

For interactive multi-turn conversations, use 'llmc sessions start' instead.

If no message is provided as an argument, it reads from stdin.
If --editor flag is set, it opens the default editor (from EDITOR environment variable) to compose the message.

You can specify the provider, model, and prompt using flags.
If not specified, the values will be taken from the configuration file.

The prompt file should be in TOML format with the following structure:
system = "System prompt with optional {{input}} placeholder"
user = "User prompt with optional {{input}} placeholder"
model = "optional-model-name"  # Optional: overrides the default model for this prompt
web_search = true  # Optional: enables web search for this prompt"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration from file
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Validate session flags
		if sessionID != "" && newSession {
			return fmt.Errorf("cannot specify both --session and --new-session")
		}

		// Cannot use prompt with existing session
		if sessionID != "" && prompt != "" {
			return fmt.Errorf("cannot use --prompt with existing session")
		}

		// Get message from arguments, editor, or stdin
		var message string
		if useEditor {
			message, err = getMessageFromEditor()
			if err != nil {
				return fmt.Errorf("getting message from editor: %w", err)
			}
		} else if len(args) > 0 {
			message = strings.Join(args, " ")
		} else {
			// Read from stdin
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading from stdin: %w", err)
			}
			message = strings.TrimSpace(string(input))
		}

		// Determine session mode
		var sess *session.Session
		var systemPrompt string
		var isNewSession bool

		if sessionID != "" {
			// Load existing session
			sess, err = session.FindSessionByPrefix(sessionID)
			if err != nil {
				return fmt.Errorf("finding session: %w", err)
			}

			// Check message threshold
			threshold := cfg.SessionMessageThreshold
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
					return nil
				}
			}

			// Use session's system prompt and model
			systemPrompt = sess.SystemPrompt
			cfg.Model = sess.Model

			if verbose {
				fmt.Fprintf(os.Stderr, "Continuing session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s\n", sess.Model)
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
				formattedMessage, promptModel, promptWebSearch, err = promptpkg.FormatMessage(message, prompt, cfg.PromptDirs, argFlags)
				if err != nil {
					return fmt.Errorf("formatting message with prompt: %w", err)
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
						return fmt.Errorf("invalid model from prompt file: %w", err)
					}
					cfg.Model = *promptModel
					if verbose {
						fmt.Fprintf(os.Stderr, "Using model from prompt file: %s\n", cfg.Model)
					}
				}

				// Apply web search from prompt template
				if promptWebSearch != nil && !cmd.Flags().Changed("web-search") {
					cfg.EnableWebSearch = *promptWebSearch
				}
			}

			// Apply model with priority: flag > env > prompt template > config file
			envModel := os.Getenv("LLMC_MODEL")
			if cmd.Flags().Changed("model") {
				if _, _, err := llmc.ParseModelString(model); err != nil {
					return fmt.Errorf("invalid model from flag: %w", err)
				}
				cfg.Model = model
			} else if envModel != "" {
				if _, _, err := llmc.ParseModelString(envModel); err != nil {
					return fmt.Errorf("invalid model from environment: %w", err)
				}
				cfg.Model = envModel
			}

			// Create new session
			sess = session.NewSession(cfg.Model)
			sess.Name = sessionName
			sess.TemplateName = prompt
			sess.SystemPrompt = systemPrompt

			if verbose {
				fmt.Fprintf(os.Stderr, "Creating new session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s\n", sess.Model)
				if systemPrompt != "" {
					fmt.Fprintf(os.Stderr, "System prompt: %s\n", systemPrompt)
				}
			}
		} else {
			// Single-shot mode (no session)
			formattedMessage, promptModel, promptWebSearch, err := promptpkg.FormatMessage(message, prompt, cfg.PromptDirs, argFlags)
			if err != nil {
				return fmt.Errorf("formatting message with prompt: %w", err)
			}

			// Apply model priority
			envModel := os.Getenv("LLMC_MODEL")
			if cmd.Flags().Changed("model") {
				if _, _, err := llmc.ParseModelString(model); err != nil {
					return fmt.Errorf("invalid model from flag: %w", err)
				}
				cfg.Model = model
			} else if envModel != "" {
				if _, _, err := llmc.ParseModelString(envModel); err != nil {
					return fmt.Errorf("invalid model from environment: %w", err)
				}
				cfg.Model = envModel
			} else if promptModel != nil {
				if _, _, err := llmc.ParseModelString(*promptModel); err != nil {
					return fmt.Errorf("invalid model from prompt file: %w", err)
				}
				cfg.Model = *promptModel
			}

			// Select provider
			llmProvider, err := newProvider(cfg)
			if err != nil {
				return fmt.Errorf("creating provider: %w", err)
			}

			// Configure web search
			enableWebSearch := cfg.EnableWebSearch
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
			enableIgnoreWebSearchErrors := cfg.IgnoreWebSearchErrors
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
				return fmt.Errorf("chat request failed: %w", err)
			}
			fmt.Println(response)
			return nil
		}

		// Select provider
		llmProvider, err := newProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}

		// Configure web search
		enableWebSearch := cfg.EnableWebSearch
		envWebSearch := os.Getenv("LLMC_ENABLE_WEB_SEARCH")
		if cmd.Flags().Changed("web-search") {
			enableWebSearch = webSearch
		} else if envWebSearch != "" {
			enableWebSearch = envWebSearch == "true" || envWebSearch == "1"
		}
		llmProvider.SetWebSearch(enableWebSearch)

		// Configure ignore web search errors
		enableIgnoreWebSearchErrors := cfg.IgnoreWebSearchErrors
		envIgnoreWebSearchErrors := os.Getenv("LLMC_IGNORE_WEB_SEARCH_ERRORS")
		if cmd.Flags().Changed("ignore-web-search-errors") {
			enableIgnoreWebSearchErrors = ignoreWebSearchErrors
		} else if envIgnoreWebSearchErrors != "" {
			enableIgnoreWebSearchErrors = envIgnoreWebSearchErrors == "true" || envIgnoreWebSearchErrors == "1"
		}
		llmProvider.SetIgnoreWebSearchErrors(enableIgnoreWebSearchErrors)
		llmProvider.SetDebug(verbose)

		// Session mode: add message to session
		sess.AddMessage("user", message)

		// Send message with history (exclude the last message which was just added)
		historyMessages := sess.Messages[:len(sess.Messages)-1]

		response, err := llmProvider.ChatWithHistory(sess.SystemPrompt, historyMessages, message)

		if err != nil {
			return fmt.Errorf("chat request failed: %w", err)
		}

		// Add assistant response to session
		sess.AddMessage("assistant", response)

		// Save session
		if err := session.SaveSession(sess); err != nil {
			return fmt.Errorf("saving session: %w", err)
		}

		// Print response
		fmt.Println(response)

		// If new session, print session info
		if isNewSession {
			fmt.Fprintf(os.Stderr, "\nSession created: %s\n", sess.GetShortID())
			sessionDir, _ := session.GetSessionDir()
			fmt.Fprintf(os.Stderr, "Path: %s/%s.json\n", sessionDir, sess.ID)
			fmt.Fprintf(os.Stderr, "\nNext time, use:\n  llmc chat -s %s \"your message\"\n", sess.GetShortID())
			fmt.Fprintf(os.Stderr, "For interactive mode, use:\n  llmc sessions start %s\n", sess.GetShortID())
		}

		return nil
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
	chatCmd.Flags().BoolVar(&ignoreThreshold, "ignore-threshold", false, "Ignore session message threshold warning")
}