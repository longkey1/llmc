package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("file content"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := runReadFile(context.Background(), `{"path":"`+path+`"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "file content" {
		t.Errorf("got %q, want file content", got)
	}
}

func TestRunReadFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	_, err := runReadFile(context.Background(), `{"path":"`+path+`"}`, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRunReadFileDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := runReadFile(context.Background(), `{"path":"`+dir+`"}`, nil)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("err = %v, want directory error", err)
	}
}

func TestRunWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	got, err := runWriteFile(context.Background(), `{"path":"`+path+`","content":"written"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "wrote 7 bytes") {
		t.Errorf("got %q, want byte count", got)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "written" {
		t.Errorf("file content = %q, want written", content)
	}
}

func TestRunWriteFileMissingParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "out.txt")
	_, err := runWriteFile(context.Background(), `{"path":"`+path+`","content":"x"}`, nil)
	if err == nil {
		t.Error("expected error for missing parent directory")
	}
}

func TestWriteFileSummary(t *testing.T) {
	dir := t.TempDir()
	newPath := filepath.Join(dir, "new.txt")

	summary := writeFileSummary(writeFileArgs{Path: newPath, Content: "line1\nline2"})
	if !strings.HasPrefix(summary, "Create file:") {
		t.Errorf("summary = %q, want Create file prefix", summary)
	}
	if !strings.Contains(summary, "| line1") {
		t.Errorf("summary = %q, want content preview", summary)
	}

	existingPath := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existingPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary = writeFileSummary(writeFileArgs{Path: existingPath, Content: "new"})
	if !strings.HasPrefix(summary, "Overwrite file:") {
		t.Errorf("summary = %q, want Overwrite file prefix", summary)
	}

	long := strings.Repeat("x\n", 20)
	summary = writeFileSummary(writeFileArgs{Path: newPath, Content: long})
	if !strings.Contains(summary, "more lines)") {
		t.Errorf("summary = %q, want preview truncation note", summary)
	}
}
