package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/llmc/session"
	"github.com/spf13/cobra"
)

// sessionsCmd represents the sessions command
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage conversation sessions",
	Long: `Manage conversation sessions including listing, viewing, and deleting sessions.

Sessions allow you to maintain conversation history across multiple interactions.`,
}

// sessionsListCmd represents the sessions list command
var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  `List all conversation sessions sorted by most recently updated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := session.ListSessions()
		if err != nil {
			return fmt.Errorf("listing sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
			fmt.Println("\nCreate a new session with:")
			fmt.Println("  llmc chat --new-session \"your message\"")
			return nil
		}

		// Print table header
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMODEL\tCREATED\tMESSAGES\tNAME")
		fmt.Fprintln(w, "--\t-----\t-------\t--------\t----")

		// Print each session
		for _, sess := range sessions {
			name := sess.Name
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				sess.GetShortID(),
				sess.Model,
				sess.CreatedAt.Format("2006-01-02"),
				sess.MessageCount(),
				name,
			)
		}
		w.Flush()

		fmt.Println("\nUse 'llmc sessions show <id>' to view session details.")
		return nil
	},
}

// sessionsShowCmd represents the sessions show command
var sessionsShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show session details and history",
	Long: `Show detailed information about a session including all messages.

The ID can be a short ID (minimum 4 characters), full UUID, or "latest" for the most recent session.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]

		// Find session by prefix
		sess, err := session.FindSessionByPrefix(sessionID)
		if err != nil {
			return fmt.Errorf("finding session: %w", err)
		}

		// Print session info
		fmt.Printf("Session: %s\n", sess.ID)
		if sess.Name != "" {
			fmt.Printf("Name: %s\n", sess.Name)
		}
		if sess.ParentID != "" {
			fmt.Printf("Parent: %s\n", sess.ParentID)
		}
		fmt.Printf("Model: %s\n", sess.Model)
		fmt.Printf("Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", sess.UpdatedAt.Format("2006-01-02 15:04:05"))
		if sess.TemplateName != "" {
			fmt.Printf("Template: %s\n", sess.TemplateName)
		}
		if sess.SystemPrompt != "" {
			fmt.Printf("System Prompt: %s\n", sess.SystemPrompt)
		}
		fmt.Printf("Messages: %d\n", sess.MessageCount())
		fmt.Println()

		// Print message history
		if len(sess.Messages) == 0 {
			fmt.Println("No messages in this session.")
			return nil
		}

		fmt.Println("Message History:")
		fmt.Println("----------------")
		for i, msg := range sess.Messages {
			timestamp := ""
			if t, ok := msg.Timestamp.(string); ok {
				// Parse timestamp if it's a string
				timestamp = t
			} else {
				timestamp = fmt.Sprintf("%v", msg.Timestamp)
			}

			roleLabel := "You"
			if msg.Role == "assistant" {
				roleLabel = "Assistant"
			}

			fmt.Printf("\n[%d] %s (%s):\n%s\n",
				i+1,
				roleLabel,
				timestamp,
				msg.Content,
			)
		}

		fmt.Printf("\nContinue this session with:\n  llmc chat -s %s \"your message\"\n", sess.GetShortID())
		return nil
	},
}

// sessionsDeleteCmd represents the sessions delete command
var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a session",
	Long: `Delete a conversation session permanently.

The ID can be a short ID (minimum 4 characters), full UUID, or "latest" for the most recent session.

Warning: This action cannot be undone.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]

		// Find session by prefix
		sess, err := session.FindSessionByPrefix(sessionID)
		if err != nil {
			return fmt.Errorf("finding session: %w", err)
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete session %s? [y/N]: ", sess.GetShortID())
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}

		// Delete the session
		if err := session.DeleteSession(sess.ID); err != nil {
			return fmt.Errorf("deleting session: %w", err)
		}

		fmt.Printf("Session %s deleted successfully.\n", sess.GetShortID())
		return nil
	},
}

// sessionsRenameCmd represents the sessions rename command
var sessionsRenameCmd = &cobra.Command{
	Use:   "rename <id> <name>",
	Short: "Rename a session",
	Long: `Rename a conversation session.

The ID can be a short ID (minimum 4 characters), full UUID, or "latest" for the most recent session.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]
		newName := args[1]

		// Find session by prefix
		sess, err := session.FindSessionByPrefix(sessionID)
		if err != nil {
			return fmt.Errorf("finding session: %w", err)
		}

		// Update session name
		sess.Name = newName

		// Save session
		if err := session.SaveSession(sess); err != nil {
			return fmt.Errorf("saving session: %w", err)
		}

		fmt.Printf("Session %s renamed to \"%s\".\n", sess.GetShortID(), newName)
		return nil
	},
}

// sessionsClearCmd represents the sessions clear command
var sessionsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete old sessions",
	Long: `Delete old conversation sessions permanently.

By default, deletes sessions created more than 30 days ago.
Use --before to specify a different date, or --all to delete all sessions.

Warning: This action cannot be undone.

Examples:
  llmc sessions clear                      # Delete sessions older than 30 days (default)
  llmc sessions clear --before 2024-01-01  # Delete sessions created before 2024-01-01
  llmc sessions clear --before 2024-12     # Delete sessions created before 2024-12-01
  llmc sessions clear --all                # Delete all sessions`,
	RunE: func(cmd *cobra.Command, args []string) error {
		beforeDateStr, _ := cmd.Flags().GetString("before")
		deleteAll, _ := cmd.Flags().GetBool("all")

		sessions, err := session.ListSessions()
		if err != nil {
			return fmt.Errorf("listing sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions to delete.")
			return nil
		}

		// Determine filter behavior
		var sessionsToDelete []session.Session
		var beforeDate time.Time

		if deleteAll {
			// Delete all sessions
			sessionsToDelete = sessions
		} else {
			// Parse or use default date
			if beforeDateStr != "" {
				// Parse the before date
				var err error
				beforeDate, err = parseDate(beforeDateStr)
				if err != nil {
					return fmt.Errorf("parsing date: %w", err)
				}
			} else {
				// Load config to get retention days
				cfg, err := config.LoadConfig()
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
				// Default: configured retention days (default 30)
				beforeDate = time.Now().AddDate(0, 0, -cfg.SessionRetentionDays)
			}

			// Filter sessions created before the specified date
			for _, sess := range sessions {
				if sess.CreatedAt.Before(beforeDate) {
					sessionsToDelete = append(sessionsToDelete, sess)
				}
			}

			if len(sessionsToDelete) == 0 {
				fmt.Printf("No sessions found created before %s.\n", beforeDate.Format("2006-01-02"))
				return nil
			}
		}

		// Protect parent sessions that are referenced by child sessions
		// Build a map of session IDs to delete for quick lookup
		toDeleteMap := make(map[string]bool)
		for _, sess := range sessionsToDelete {
			toDeleteMap[sess.ID] = true
		}

		// Find parent sessions that should be protected
		protectedParents := make(map[string]session.Session)
		for _, sess := range sessions {
			// If this session is not being deleted but its parent is
			if !toDeleteMap[sess.ID] && sess.ParentID != "" && toDeleteMap[sess.ParentID] {
				// Find the parent session in sessionsToDelete
				for _, parent := range sessionsToDelete {
					if parent.ID == sess.ParentID {
						protectedParents[parent.ID] = parent
						break
					}
				}
			}
		}

		// Remove protected parents from deletion list
		if len(protectedParents) > 0 {
			var filteredSessions []session.Session
			for _, sess := range sessionsToDelete {
				if _, isProtected := protectedParents[sess.ID]; !isProtected {
					filteredSessions = append(filteredSessions, sess)
				}
			}
			sessionsToDelete = filteredSessions

			// Display notice about protected sessions
			fmt.Fprintf(os.Stderr, "\nNotice: The following sessions were not deleted (referenced by child sessions):\n")
			for _, parent := range protectedParents {
				fmt.Fprintf(os.Stderr, "  - %s (created: %s)\n", parent.GetShortID(), parent.CreatedAt.Format("2006-01-02"))
			}
			fmt.Fprintln(os.Stderr)
		}

		// Check if there are any sessions left to delete
		if len(sessionsToDelete) == 0 {
			fmt.Println("No sessions to delete after excluding protected parent sessions.")
			return nil
		}

		// Confirm deletion
		if deleteAll {
			fmt.Printf("Are you sure you want to delete all %d sessions? [y/N]: ", len(sessionsToDelete))
		} else if beforeDateStr != "" {
			fmt.Printf("Are you sure you want to delete %d sessions created before %s? [y/N]: ",
				len(sessionsToDelete), beforeDate.Format("2006-01-02"))
		} else {
			// Load config to get retention days for display
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			fmt.Printf("Are you sure you want to delete %d sessions older than %d days (created before %s)? [y/N]: ",
				len(sessionsToDelete), cfg.SessionRetentionDays, beforeDate.Format("2006-01-02"))
		}
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}

		// Delete sessions
		deleted := 0
		failed := 0
		for _, sess := range sessionsToDelete {
			if err := session.DeleteSession(sess.ID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete session %s: %v\n", sess.GetShortID(), err)
				failed++
			} else {
				deleted++
			}
		}

		fmt.Printf("Successfully deleted %d sessions", deleted)
		if failed > 0 {
			fmt.Printf(" (%d failed)", failed)
		}
		fmt.Println(".")
		return nil
	},
}

// parseDate parses a date string in various formats and returns a time.Time
// Supported formats: YYYY-MM-DD, YYYY-MM, YYYY
func parseDate(dateStr string) (time.Time, error) {
	// Try YYYY-MM-DD format
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t, nil
	}

	// Try YYYY-MM format (use first day of month)
	if t, err := time.Parse("2006-01", dateStr); err == nil {
		return t, nil
	}

	// Try YYYY format (use first day of year)
	if t, err := time.Parse("2006", dateStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid date format: %s (use YYYY-MM-DD, YYYY-MM, or YYYY)", dateStr)
}

// sessionsSummarizeCmd represents the sessions summarize command
var sessionsSummarizeCmd = &cobra.Command{
	Use:   "summarize <id>",
	Short: "Summarize a session and create a new one",
	Long: `Summarize a conversation session and create a new session with the summary.

The original session is preserved and the new session has its ParentID set.
The ID can be a short ID (minimum 4 characters), full UUID, or "latest" for the most recent session.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID := args[0]

		// Find session by prefix
		sess, err := session.FindSessionByPrefix(sessionID)
		if err != nil {
			return fmt.Errorf("finding session: %w", err)
		}

		if sess.MessageCount() == 0 {
			return fmt.Errorf("session %s has no messages to summarize", sess.GetShortID())
		}

		// Collect all ancestor sessions
		ancestors, err := collectAncestorSessions(sess)
		if err != nil {
			return fmt.Errorf("collecting ancestor sessions: %w", err)
		}

		// Count total messages
		totalMessages := 0
		for _, ancestorSess := range ancestors {
			// Skip first message if session has a parent (it's a summary)
			if ancestorSess.ParentID != "" && ancestorSess.MessageCount() > 0 {
				totalMessages += ancestorSess.MessageCount() - 1
			} else {
				totalMessages += ancestorSess.MessageCount()
			}
		}
		// Add current session messages (skip first if it has parent)
		if sess.ParentID != "" && sess.MessageCount() > 0 {
			totalMessages += sess.MessageCount() - 1
		} else {
			totalMessages += sess.MessageCount()
		}

		fmt.Fprintf(os.Stderr, "Summarizing %d messages from session %s", totalMessages, sess.GetShortID())
		if len(ancestors) > 0 {
			fmt.Fprintf(os.Stderr, " and %d ancestor session(s)", len(ancestors))
		}
		fmt.Fprintf(os.Stderr, "...\n")

		// Build conversation history for summarization including ancestors
		var conversationText strings.Builder
		messageNum := 1

		// Add ancestor messages first (oldest to newest)
		for _, ancestorSess := range ancestors {
			startIdx := 0
			// Skip first message if this ancestor has a parent (it's a summary)
			if ancestorSess.ParentID != "" && ancestorSess.MessageCount() > 0 {
				startIdx = 1
			}

			for i := startIdx; i < len(ancestorSess.Messages); i++ {
				msg := ancestorSess.Messages[i]
				role := "User"
				if msg.Role == "assistant" {
					role = "Assistant"
				}
				conversationText.WriteString(fmt.Sprintf("[Message %d] %s: %s\n\n", messageNum, role, msg.Content))
				messageNum++
			}
		}

		// Add current session messages
		startIdx := 0
		if sess.ParentID != "" && sess.MessageCount() > 0 {
			startIdx = 1
		}
		for i := startIdx; i < len(sess.Messages); i++ {
			msg := sess.Messages[i]
			role := "User"
			if msg.Role == "assistant" {
				role = "Assistant"
			}
			conversationText.WriteString(fmt.Sprintf("[Message %d] %s: %s\n\n", messageNum, role, msg.Content))
			messageNum++
		}

		// Create summarization prompt
		summarizationPrompt := fmt.Sprintf(`Please summarize the following conversation in 3-5 concise paragraphs.
Focus on:
- Main topics discussed
- Key decisions made
- Current status or next steps

Conversation history:

%s`, conversationText.String())

		// Load config
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Use the original session's model for summarization
		cfg.Model = sess.Model

		// Create provider
		llmProvider, err := newProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}
		llmProvider.SetDebug(verbose)

		fmt.Fprintf(os.Stderr, "Generating summary using %s...\n", sess.Model)

		// Generate summary
		summary, err := llmProvider.Chat(summarizationPrompt)
		if err != nil {
			return fmt.Errorf("generating summary: %w", err)
		}

		// Create new session with summary
		newSess := session.NewSession(sess.Model)
		newSess.ParentID = sess.ID
		newSess.SystemPrompt = sess.SystemPrompt
		newSess.TemplateName = sess.TemplateName

		// Add summary as first user message with context
		summaryMessage := fmt.Sprintf("Previous conversation summary:\n\n%s", summary)
		newSess.AddMessage("user", summaryMessage)

		// Save new session
		if err := session.SaveSession(newSess); err != nil {
			return fmt.Errorf("saving new session: %w", err)
		}

		fmt.Fprintf(os.Stderr, "\nNew session created: %s (parent: %s)\n", newSess.GetShortID(), sess.GetShortID())
		sessionDir, _ := session.GetSessionDir()
		fmt.Fprintf(os.Stderr, "Path: %s/%s.json\n", sessionDir, newSess.ID)
		fmt.Fprintf(os.Stderr, "\nContinue with:\n  llmc chat -s %s \"your message\"\n", newSess.GetShortID())
		return nil
	},
}

// collectAncestorSessions collects all ancestor sessions by following ParentID chain
// Returns sessions in order from oldest ancestor to direct parent
func collectAncestorSessions(sess *session.Session) ([]*session.Session, error) {
	var ancestors []*session.Session
	visited := make(map[string]bool)
	currentID := sess.ParentID

	for currentID != "" {
		// Check for circular reference
		if visited[currentID] {
			return nil, fmt.Errorf("circular reference detected in session ancestry")
		}
		visited[currentID] = true

		// Find parent session
		parent, err := session.FindSessionByPrefix(currentID)
		if err != nil {
			// Parent not found - break the chain
			fmt.Fprintf(os.Stderr, "Warning: parent session %s not found, stopping ancestry traversal\n", currentID)
			break
		}

		// Prepend to maintain chronological order (oldest first)
		ancestors = append([]*session.Session{parent}, ancestors...)

		// Move to next ancestor
		currentID = parent.ParentID
	}

	return ancestors, nil
}

// sessionsStartCmd represents the sessions start command
var sessionsStartCmd = &cobra.Command{
	Use:   "start [session-id]",
	Short: "Start an interactive session",
	Long: `Start an interactive chat session with continuous conversation.

You can either start a new session or continue an existing one by providing its ID.
The ID can be a short ID (minimum 4 characters), full UUID, or "latest" for the most recent session.

Examples:
  llmc sessions start                # Start a new interactive session
  llmc sessions start 550e8400       # Continue session 550e8400 in interactive mode
  llmc sessions start latest         # Continue latest session in interactive mode`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		var sess *session.Session

		// Check if session ID is provided
		if len(args) > 0 {
			sessionID := args[0]

			// Find session by prefix
			sess, err = session.FindSessionByPrefix(sessionID)
			if err != nil {
				return fmt.Errorf("finding session: %w", err)
			}

			// Use session's model
			cfg.Model = sess.Model

			if verbose {
				fmt.Fprintf(os.Stderr, "Continuing session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s\n", sess.Model)
			}
		} else {
			// Create new session
			sess = session.NewSession(cfg.Model)

			if verbose {
				fmt.Fprintf(os.Stderr, "Creating new session: %s\n", sess.GetShortID())
				fmt.Fprintf(os.Stderr, "Model: %s\n", sess.Model)
			}

			// Save the new session
			if err := session.SaveSession(sess); err != nil {
				return fmt.Errorf("saving session: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Session created: %s\n", sess.GetShortID())
			sessionDir, _ := session.GetSessionDir()
			fmt.Fprintf(os.Stderr, "Path: %s/%s.json\n\n", sessionDir, sess.ID)
		}

		// Create provider
		llmProvider, err := newProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}
		llmProvider.SetDebug(verbose)

		// Start interactive mode
		if err := runInteractiveMode(sess, llmProvider); err != nil {
			return fmt.Errorf("interactive mode: %w", err)
		}

		return nil
	},
}

// runInteractiveMode starts an interactive chat session
func runInteractiveMode(sess *session.Session, llmProvider llmc.Provider) error {
	// Print session header
	fmt.Fprintf(os.Stderr, "\n=== Interactive Session [%s] ===\n", sess.GetShortID())
	fmt.Fprintf(os.Stderr, "Model: %s\n", sess.Model)
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

		// Start spinner
		done := make(chan bool)
		go showSpinner(done)

		// Send message with history
		response, err := llmProvider.ChatWithHistory(sess.SystemPrompt, historyMessages, input)

		// Stop spinner
		done <- true
		close(done)

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

// showSpinner displays a spinner animation while waiting for response
func showSpinner(done chan bool) {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			// Clear the spinner line
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		default:
			fmt.Fprintf(os.Stderr, "\r%s Waiting for response...", spinners[i])
			i = (i + 1) % len(spinners)
			time.Sleep(80 * time.Millisecond)
		}
	}
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

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsRenameCmd)
	sessionsCmd.AddCommand(sessionsClearCmd)
	sessionsCmd.AddCommand(sessionsSummarizeCmd)
	sessionsCmd.AddCommand(sessionsStartCmd)

	// sessionsClearCmd flags
	sessionsClearCmd.Flags().String("before", "", "Delete only sessions created before this date (format: YYYY-MM-DD, YYYY-MM, or YYYY)")
	sessionsClearCmd.Flags().Bool("all", false, "Delete all sessions (overrides retention days setting)")
}
