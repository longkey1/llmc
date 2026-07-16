package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/longkey1/llmc/internal/llmc/config"
	promptpkg "github.com/longkey1/llmc/internal/llmc/prompt"
	"github.com/longkey1/llmc/internal/llmc/session"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	model           string
	prompt          string
	argFlags        []string
	useEditor       bool
	webSearch       bool
	sessionID       string
	newSession      bool
	sessionName     string
	ignoreThreshold bool
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
		var promptWebSearch *bool

		if sessionID != "" {
			// Load existing session
			sess, err = session.FindSessionByPrefix(sessionID)
			if err != nil {
				return fmt.Errorf("finding session: %w", err)
			}

			proceed, err := confirmMessageThreshold(cfg, sess)
			if err != nil {
				return err
			}
			if !proceed {
				fmt.Fprintln(os.Stderr, "Cancelled.")
				return nil
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
		} else {
			// Single-shot or new-session mode: render the prompt template
			var promptModel *string
			systemPrompt, message, promptModel, promptWebSearch, err = promptpkg.FormatMessage(message, prompt, cfg.PromptDirs, argFlags)
			if err != nil {
				return fmt.Errorf("formatting message with prompt: %w", err)
			}

			if err := applyModelOverride(cmd, cfg, promptModel); err != nil {
				return err
			}

			// Resolve model alias to a concrete model (for a new session,
			// this happens before the model is pinned to the session)
			if err := resolveModelAlias(cmd.Context(), cfg); err != nil {
				return err
			}

			if newSession {
				isNewSession = true
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
			}
		}

		// Select provider
		llmProvider, err := newProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}
		llmProvider.SetWebSearch(resolveWebSearch(cmd, cfg, promptWebSearch))
		llmProvider.SetDebug(verbose)

		// Allow Ctrl+C to abort the in-flight request
		ctx, stop := requestContext(cmd.Context())
		defer stop()

		// Single-shot mode (no session)
		if sess == nil {
			stopSpinner := startSpinner()
			response, err := llmProvider.ChatWithHistory(ctx, systemPrompt, nil, message)
			stopSpinner()
			if err != nil {
				return fmt.Errorf("chat request failed: %w", err)
			}
			fmt.Println(response)
			return nil
		}

		// Session mode: add message to session
		sess.AddMessage("user", message)

		// Send message with history (exclude the last message which was just added)
		historyMessages := sess.Messages[:len(sess.Messages)-1]

		stopSpinner := startSpinner()
		response, err := llmProvider.ChatWithHistory(ctx, sess.SystemPrompt, historyMessages, message)
		stopSpinner()
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

// applyModelOverride applies the model priority (flag > environment > prompt
// template > config file) to cfg.Model, validating the chosen source.
func applyModelOverride(cmd *cobra.Command, cfg *config.Config, promptModel *string) error {
	envModel := os.Getenv("LLMC_MODEL")
	switch {
	case cmd.Flags().Changed("model"):
		if err := validateModelOrAlias(model); err != nil {
			return fmt.Errorf("invalid model from flag: %w", err)
		}
		cfg.Model = model
	case envModel != "":
		if err := validateModelOrAlias(envModel); err != nil {
			return fmt.Errorf("invalid model from environment: %w", err)
		}
		cfg.Model = envModel
	case promptModel != nil:
		if err := validateModelOrAlias(*promptModel); err != nil {
			return fmt.Errorf("invalid model from prompt file: %w", err)
		}
		cfg.Model = *promptModel
	}
	return nil
}

// resolveWebSearch returns the effective web search setting, applying the
// flag > environment > prompt template > config file priority.
func resolveWebSearch(cmd *cobra.Command, cfg *config.Config, promptWebSearch *bool) bool {
	if cmd.Flags().Changed("web-search") {
		return webSearch
	}
	if envWebSearch := os.Getenv("LLMC_ENABLE_WEB_SEARCH"); envWebSearch != "" {
		return envWebSearch == "true" || envWebSearch == "1"
	}
	if promptWebSearch != nil {
		return *promptWebSearch
	}
	return cfg.EnableWebSearch
}

// confirmMessageThreshold warns when a session exceeds the configured message
// threshold and asks the user whether to continue. It returns false when the
// user declines. When stdin is not a terminal (e.g., the message was piped
// in), the confirmation cannot be answered, so an error asking for
// --ignore-threshold is returned instead.
func confirmMessageThreshold(cfg *config.Config, sess *session.Session) (bool, error) {
	threshold := cfg.SessionMessageThreshold
	if threshold <= 0 || sess.MessageCount() < threshold || ignoreThreshold {
		return true, nil
	}

	fmt.Fprintf(os.Stderr, "\nWarning: Session %s has %d messages (threshold: %d).\n",
		sess.GetShortID(), sess.MessageCount(), threshold)
	fmt.Fprintf(os.Stderr, "Long sessions may impact performance and token usage.\n")
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	fmt.Fprintf(os.Stderr, "  1. Continue anyway with --ignore-threshold flag\n")
	fmt.Fprintf(os.Stderr, "  2. Summarize session: llmc sessions summarize %s\n", sess.GetShortID())
	fmt.Fprintf(os.Stderr, "  3. Start a new session: llmc chat --new-session\n\n")

	if !stdinIsTerminal() {
		return false, fmt.Errorf("session %s exceeds the message threshold; re-run with --ignore-threshold to continue", sess.GetShortID())
	}

	fmt.Fprint(os.Stderr, "Continue with this session? [y/N]: ")
	var response string
	_, _ = fmt.Scanln(&response)

	return response == "y" || response == "Y", nil
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

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

	// Session flags
	chatCmd.Flags().StringVarP(&sessionID, "session", "s", "", "Session ID (short or full UUID, or 'latest' for most recent session)")
	chatCmd.Flags().BoolVarP(&newSession, "new-session", "n", false, "Create a new session")
	chatCmd.Flags().StringVar(&sessionName, "session-name", "", "Name for the new session (optional)")
	chatCmd.Flags().BoolVar(&ignoreThreshold, "ignore-threshold", false, "Ignore session message threshold warning")
}
