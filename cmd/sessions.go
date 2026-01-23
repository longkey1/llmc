package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

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
		fmt.Fprintln(w, "ID\tPROVIDER\tMODEL\tCREATED\tMESSAGES\tNAME")
		fmt.Fprintln(w, "--\t--------\t-----\t-------\t--------\t----")

		// Print each session
		for _, sess := range sessions {
			name := sess.Name
			if name == "" {
				name = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				sess.GetShortID(),
				sess.Provider,
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
		fmt.Printf("Provider: %s\n", sess.Provider)
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
	Short: "Delete all sessions",
	Long: `Delete all conversation sessions permanently.

Warning: This action cannot be undone.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := session.ListSessions()
		if err != nil {
			return fmt.Errorf("listing sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions to delete.")
			return nil
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete all %d sessions? [y/N]: ", len(sessions))
		var response string
		fmt.Scanln(&response)

		if response != "y" && response != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}

		// Delete all sessions
		deleted := 0
		failed := 0
		for _, sess := range sessions {
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

		fmt.Fprintf(os.Stderr, "Summarizing %d messages from session %s...\n", sess.MessageCount(), sess.GetShortID())

		// Build conversation history for summarization
		var conversationText strings.Builder
		for i, msg := range sess.Messages {
			role := "User"
			if msg.Role == "assistant" {
				role = "Assistant"
			}
			conversationText.WriteString(fmt.Sprintf("[Message %d] %s: %s\n\n", i+1, role, msg.Content))
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
		cfg.Model = sess.Provider + ":" + sess.Model

		// Create provider
		llmProvider, err := newProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating provider: %w", err)
		}
		llmProvider.SetDebug(verbose)

		fmt.Fprintf(os.Stderr, "Generating summary using %s:%s...\n", sess.Provider, sess.Model)

		// Generate summary
		summary, err := llmProvider.Chat(summarizationPrompt)
		if err != nil {
			return fmt.Errorf("generating summary: %w", err)
		}

		// Create new session with summary
		newSess := session.NewSession(sess.Provider, sess.Model)
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

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsRenameCmd)
	sessionsCmd.AddCommand(sessionsClearCmd)
	sessionsCmd.AddCommand(sessionsSummarizeCmd)
}
