package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// useTempSessionDir points viper at a config file inside a temp directory so
// that GetSessionDir resolves to <tmp>/sessions, and registers cleanup so
// global viper state does not leak between tests.
func useTempSessionDir(t *testing.T) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	return filepath.Join(dir, "sessions")
}

func TestGetSessionDir(t *testing.T) {
	want := useTempSessionDir(t)

	got, err := GetSessionDir()
	if err != nil {
		t.Fatalf("GetSessionDir() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("GetSessionDir() = %v, want %v", got, want)
	}
}

func TestSaveLoadDeleteSession(t *testing.T) {
	sessionDir := useTempSessionDir(t)

	s := NewSession("openai:gpt-4")
	s.Name = "test-session"
	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi")

	if err := SaveSession(s); err != nil {
		t.Fatalf("SaveSession() unexpected error: %v", err)
	}

	sessionFile := filepath.Join(sessionDir, s.ID+".json")
	if _, err := os.Stat(sessionFile); err != nil {
		t.Fatalf("session file not created: %v", err)
	}

	loaded, err := LoadSession(s.ID)
	if err != nil {
		t.Fatalf("LoadSession() unexpected error: %v", err)
	}
	if loaded.ID != s.ID {
		t.Errorf("loaded ID = %v, want %v", loaded.ID, s.ID)
	}
	if loaded.Name != "test-session" {
		t.Errorf("loaded Name = %v, want %v", loaded.Name, "test-session")
	}
	if loaded.Model != "openai:gpt-4" {
		t.Errorf("loaded Model = %v, want %v", loaded.Model, "openai:gpt-4")
	}
	if loaded.MessageCount() != 2 {
		t.Errorf("loaded MessageCount() = %d, want 2", loaded.MessageCount())
	}
	if loaded.Messages[0].Role != "user" || loaded.Messages[0].Content != "hello" {
		t.Errorf("loaded Messages[0] = %+v, want role=user content=hello", loaded.Messages[0])
	}

	if err := DeleteSession(s.ID); err != nil {
		t.Fatalf("DeleteSession() unexpected error: %v", err)
	}
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session file should be removed after DeleteSession")
	}
}

func TestLoadSessionNotFound(t *testing.T) {
	useTempSessionDir(t)

	if _, err := LoadSession("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Error("LoadSession() expected error for missing session, got nil")
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	useTempSessionDir(t)

	if err := DeleteSession("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Error("DeleteSession() expected error for missing session, got nil")
	}
}

func TestListSessions(t *testing.T) {
	sessionDir := useTempSessionDir(t)

	// Empty directory returns no sessions
	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("ListSessions() = %d sessions, want 0", len(sessions))
	}

	// Create sessions with distinct UpdatedAt values
	base := time.Now()
	var ids []string
	for i := range 3 {
		s := NewSession("openai:gpt-4")
		s.UpdatedAt = base.Add(time.Duration(i) * time.Hour)
		if err := SaveSession(s); err != nil {
			t.Fatalf("SaveSession() unexpected error: %v", err)
		}
		ids = append(ids, s.ID)
	}

	// Corrupted file is skipped
	if err := os.WriteFile(filepath.Join(sessionDir, "broken.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}
	// Non-JSON file is ignored
	if err := os.WriteFile(filepath.Join(sessionDir, "notes.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatalf("failed to write non-json file: %v", err)
	}

	sessions, err = ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() unexpected error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("ListSessions() = %d sessions, want 3", len(sessions))
	}

	// Sorted by UpdatedAt, newest first
	if sessions[0].ID != ids[2] || sessions[1].ID != ids[1] || sessions[2].ID != ids[0] {
		t.Errorf("ListSessions() order = [%s %s %s], want [%s %s %s]",
			sessions[0].ID, sessions[1].ID, sessions[2].ID, ids[2], ids[1], ids[0])
	}
}

func TestFindSessionByPrefix(t *testing.T) {
	useTempSessionDir(t)

	s1 := NewSession("openai:gpt-4")
	s1.ID = "aaaa1111-0000-0000-0000-000000000001"
	s1.UpdatedAt = time.Now()
	s2 := NewSession("openai:gpt-4")
	s2.ID = "aaaa2222-0000-0000-0000-000000000002"
	s2.UpdatedAt = time.Now().Add(time.Hour)
	for _, s := range []*Session{s1, s2} {
		if err := SaveSession(s); err != nil {
			t.Fatalf("SaveSession() unexpected error: %v", err)
		}
	}

	tests := []struct {
		name          string
		prefix        string
		wantID        string
		wantErr       bool
		wantAmbiguous bool
	}{
		{
			name:   "unique prefix",
			prefix: "aaaa1",
			wantID: s1.ID,
		},
		{
			name:   "full uuid",
			prefix: s2.ID,
			wantID: s2.ID,
		},
		{
			name:   "latest alias returns most recently updated",
			prefix: "latest",
			wantID: s2.ID,
		},
		{
			name:    "prefix too short",
			prefix:  "aaa",
			wantErr: true,
		},
		{
			name:    "no match",
			prefix:  "ffff9999",
			wantErr: true,
		},
		{
			name:          "ambiguous prefix",
			prefix:        "aaaa",
			wantErr:       true,
			wantAmbiguous: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindSessionByPrefix(tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FindSessionByPrefix() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantAmbiguous {
				ambErr, ok := errors.AsType[*AmbiguousIDError](err)
				if !ok {
					t.Fatalf("FindSessionByPrefix() error = %T, want *AmbiguousIDError", err)
				}
				if ambErr.Prefix != tt.prefix {
					t.Errorf("AmbiguousIDError.Prefix = %v, want %v", ambErr.Prefix, tt.prefix)
				}
				if len(ambErr.Matches) != 2 {
					t.Errorf("AmbiguousIDError.Matches = %d, want 2", len(ambErr.Matches))
				}
			}
			if tt.wantErr {
				return
			}
			if got.ID != tt.wantID {
				t.Errorf("FindSessionByPrefix() ID = %v, want %v", got.ID, tt.wantID)
			}
		})
	}
}

func TestGetLatestSessionEmpty(t *testing.T) {
	useTempSessionDir(t)

	if _, err := GetLatestSession(); err == nil {
		t.Error("GetLatestSession() expected error when no sessions exist, got nil")
	}
}

func TestAmbiguousIDErrorMessage(t *testing.T) {
	err := &AmbiguousIDError{
		Prefix: "aaaa",
		Matches: []Session{
			{ID: "aaaa1111-0000-0000-0000-000000000001", Model: "openai:gpt-4", CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
			{ID: "aaaa2222-0000-0000-0000-000000000002", Model: "gemini:gemini-2.0-flash", CreatedAt: time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)},
		},
	}

	msg := err.Error()
	for _, want := range []string{
		`Ambiguous session ID "aaaa"`,
		"aaaa1111 (openai:gpt-4, 2026-01-02, 0 messages)",
		"aaaa2222 (gemini:gemini-2.0-flash, 2026-03-04, 0 messages)",
		"Please use a longer prefix",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
}
