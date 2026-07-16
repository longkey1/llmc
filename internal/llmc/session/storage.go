package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// AmbiguousIDError is returned when multiple sessions match a prefix
type AmbiguousIDError struct {
	Prefix  string
	Matches []Session
}

func (e *AmbiguousIDError) Error() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Ambiguous session ID %q. Multiple matches found:", e.Prefix))
	for _, match := range e.Matches {
		lines = append(lines, fmt.Sprintf("- %s (%s, %s, %d messages)",
			match.GetShortID(),
			match.Model,
			match.CreatedAt.Format("2006-01-02"),
			match.MessageCount()))
	}
	lines = append(lines, "")
	lines = append(lines, "Please use a longer prefix or run 'llmc sessions list'.")
	return strings.Join(lines, "\n")
}

// GetSessionDir returns the directory where sessions are stored
// If a config file is used, sessions are stored in the same directory as the config file.
// Otherwise, defaults to $HOME/.config/llmc/sessions
func GetSessionDir() (string, error) {
	configFile := viper.ConfigFileUsed()

	if configFile != "" {
		// Use the same directory as the config file
		configDir := filepath.Dir(configFile)

		// Make the path absolute if it's relative
		if !filepath.IsAbs(configDir) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("failed to get current working directory: %w", err)
			}
			configDir = filepath.Join(cwd, configDir)
		}

		sessionDir := filepath.Join(configDir, "sessions")
		return sessionDir, nil
	}

	// Fallback to default location
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	sessionDir := filepath.Join(home, ".config", "llmc", "sessions")
	return sessionDir, nil
}

// SaveSession saves a session to disk
func SaveSession(session *Session) error {
	sessionDir, err := GetSessionDir()
	if err != nil {
		return err
	}

	// Create session directory if it doesn't exist.
	// Sessions hold conversation content, so keep them private (0700/0600).
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Serialize session to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Write via a temp file + rename so a crash mid-write cannot corrupt an
	// existing session file (full UUID as filename)
	sessionFile := filepath.Join(sessionDir, session.ID+".json")
	tmp, err := os.CreateTemp(sessionDir, session.ID+".tmp-")
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write session file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write session file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	if err := os.Rename(tmpName, sessionFile); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk by full ID
func LoadSession(id string) (*Session, error) {
	sessionDir, err := GetSessionDir()
	if err != nil {
		return nil, err
	}

	sessionFile := filepath.Join(sessionDir, id+".json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s\n\nRun 'llmc sessions list' to see available sessions", id)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w\n\nThe session file may be corrupted", err)
	}

	return &session, nil
}

// DeleteSession deletes a session from disk by full ID
func DeleteSession(id string) error {
	sessionDir, err := GetSessionDir()
	if err != nil {
		return err
	}

	sessionFile := filepath.Join(sessionDir, id+".json")
	if err := os.Remove(sessionFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", id)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	return nil
}

// ListSessions returns all sessions sorted by UpdatedAt (newest first)
func ListSessions() ([]Session, error) {
	sessionDir, err := GetSessionDir()
	if err != nil {
		return nil, err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Read all files in session directory
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract ID from filename (remove .json extension)
		id := strings.TrimSuffix(entry.Name(), ".json")
		session, err := LoadSession(id)
		if err != nil {
			// Skip corrupted session files
			continue
		}
		sessions = append(sessions, *session)
	}

	// Sort by UpdatedAt (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// FindSessionByPrefix finds a session by short ID prefix (minimum 4 characters)
// Returns error if multiple matches are found (AmbiguousIDError)
// Special case: "latest" returns the most recently updated session
func FindSessionByPrefix(prefix string) (*Session, error) {
	// Special case: "latest" returns the most recent session
	if prefix == "latest" {
		return GetLatestSession()
	}

	// Validate minimum prefix length
	if len(prefix) < 4 {
		return nil, fmt.Errorf("session ID prefix must be at least 4 characters (got %d)", len(prefix))
	}

	// Check if it's a full UUID (36 characters with 4 dashes)
	if len(prefix) == 36 && strings.Count(prefix, "-") == 4 {
		return LoadSession(prefix)
	}

	// Match the prefix against filenames so only matching sessions are
	// actually read from disk
	sessionDir, err := GetSessionDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s\n\nRun 'llmc sessions list' to see available sessions", prefix)
		}
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var matches []Session
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		session, err := LoadSession(id)
		if err != nil {
			// Skip corrupted session files
			continue
		}
		matches = append(matches, *session)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("session not found: %s\n\nRun 'llmc sessions list' to see available sessions", prefix)
	}

	if len(matches) > 1 {
		return nil, &AmbiguousIDError{
			Prefix:  prefix,
			Matches: matches,
		}
	}

	return &matches[0], nil
}

// GetLatestSession returns the most recently updated session
func GetLatestSession() (*Session, error) {
	sessions, err := ListSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found\n\nCreate a new session with: llmc chat --new-session \"your message\"")
	}

	// Sessions are already sorted by UpdatedAt (newest first)
	return &sessions[0], nil
}
