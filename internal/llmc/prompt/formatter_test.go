package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePrompt writes a prompt TOML file into dir and returns its path.
func writePrompt(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write prompt file: %v", err)
	}
	return path
}

func TestLoadPrompt(t *testing.T) {
	dir := t.TempDir()
	path := writePrompt(t, dir, "test.toml", `
system = "You are a helpful assistant."
user = "Translate: {{input}}"
model = "openai:gpt-4"
web_search = true
`)

	p, err := LoadPrompt(path)
	if err != nil {
		t.Fatalf("LoadPrompt() unexpected error: %v", err)
	}

	if p.System != "You are a helpful assistant." {
		t.Errorf("System = %q, want %q", p.System, "You are a helpful assistant.")
	}
	if p.User != "Translate: {{input}}" {
		t.Errorf("User = %q, want %q", p.User, "Translate: {{input}}")
	}
	if p.Model == nil || *p.Model != "openai:gpt-4" {
		t.Errorf("Model = %v, want openai:gpt-4", p.Model)
	}
	if p.WebSearch == nil || !*p.WebSearch {
		t.Errorf("WebSearch = %v, want true", p.WebSearch)
	}
}

func TestLoadPromptOptionalFieldsOmitted(t *testing.T) {
	dir := t.TempDir()
	path := writePrompt(t, dir, "minimal.toml", `
system = "sys"
user = "usr"
`)

	p, err := LoadPrompt(path)
	if err != nil {
		t.Fatalf("LoadPrompt() unexpected error: %v", err)
	}
	if p.Model != nil {
		t.Errorf("Model = %v, want nil", p.Model)
	}
	if p.WebSearch != nil {
		t.Errorf("WebSearch = %v, want nil", p.WebSearch)
	}
}

func TestLoadPromptInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := writePrompt(t, dir, "broken.toml", `system = [unclosed`)

	if _, err := LoadPrompt(path); err == nil {
		t.Error("LoadPrompt() expected error for invalid TOML, got nil")
	}
}

func TestFormatMessage(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "translate.toml", `
system = "You translate to {{lang}}."
user = "Text: {{input}}"
`)
	writePrompt(t, dir, "with-model.toml", `
system = "sys"
user = "usr {{input}}"
model = "gemini:gemini-2.0-flash"
web_search = true
`)
	writePrompt(t, dir, "bad-model.toml", `
system = "sys"
user = "usr"
model = "not-a-model"
`)

	tests := []struct {
		name          string
		message       string
		promptName    string
		promptDirs    []string
		args          []string
		want          string
		wantModel     string
		wantWebSearch *bool
		wantErr       bool
	}{
		{
			name:       "no prompt returns message as-is",
			message:    "hello",
			promptName: "",
			want:       "hello",
		},
		{
			name:       "placeholder replacement with args",
			message:    "hello world",
			promptName: "translate",
			promptDirs: []string{dir},
			args:       []string{"lang:Japanese"},
			want:       "System: You translate to Japanese.\n\nUser: Text: hello world",
		},
		{
			name:       "explicit toml extension",
			message:    "hi",
			promptName: "translate.toml",
			promptDirs: []string{dir},
			args:       []string{"lang:French"},
			want:       "System: You translate to French.\n\nUser: Text: hi",
		},
		{
			name:       "model and web search from prompt",
			message:    "msg",
			promptName: "with-model",
			promptDirs: []string{dir},
			want:       "System: sys\n\nUser: usr msg",
			wantModel:  "gemini:gemini-2.0-flash",
		},
		{
			name:       "prompt not found",
			message:    "msg",
			promptName: "missing",
			promptDirs: []string{dir},
			wantErr:    true,
		},
		{
			name:       "invalid model in prompt template",
			message:    "msg",
			promptName: "bad-model",
			promptDirs: []string{dir},
			wantErr:    true,
		},
		{
			name:       "invalid argument format",
			message:    "msg",
			promptName: "translate",
			promptDirs: []string{dir},
			args:       []string{"no-colon-arg"},
			wantErr:    true,
		},
		{
			name:       "reserved input key",
			message:    "msg",
			promptName: "translate",
			promptDirs: []string{dir},
			args:       []string{"input:override"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, model, webSearch, err := FormatMessage(tt.message, tt.promptName, tt.promptDirs, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FormatMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("FormatMessage() = %q, want %q", got, tt.want)
			}
			if tt.wantModel == "" {
				if model != nil {
					t.Errorf("FormatMessage() model = %v, want nil", *model)
				}
			} else if model == nil || *model != tt.wantModel {
				t.Errorf("FormatMessage() model = %v, want %v", model, tt.wantModel)
			}
			if tt.promptName == "with-model" && (webSearch == nil || !*webSearch) {
				t.Errorf("FormatMessage() webSearch = %v, want true", webSearch)
			}
		})
	}
}

func TestFormatMessageLaterDirectoryTakesPrecedence(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writePrompt(t, dir1, "dup.toml", `
system = "from dir1"
user = "{{input}}"
`)
	writePrompt(t, dir2, "dup.toml", `
system = "from dir2"
user = "{{input}}"
`)

	got, _, _, err := FormatMessage("msg", "dup", []string{dir1, dir2}, nil)
	if err != nil {
		t.Fatalf("FormatMessage() unexpected error: %v", err)
	}
	if !strings.Contains(got, "from dir2") {
		t.Errorf("FormatMessage() = %q, want prompt from later directory (dir2)", got)
	}
}

func TestProcessArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "simple key value",
			args: []string{"lang:Japanese"},
			want: map[string]string{"lang": "Japanese"},
		},
		{
			name: "multiple args",
			args: []string{"lang:Japanese", "tone:formal"},
			want: map[string]string{"lang": "Japanese", "tone": "formal"},
		},
		{
			name: "value split on first colon only",
			args: []string{"url:https://example.com"},
			want: map[string]string{"url": "https://example.com"},
		},
		{
			name: "quoted argument",
			args: []string{`"lang:Japanese"`},
			want: map[string]string{"lang": "Japanese"},
		},
		{
			name: "escaped colon and quote in value",
			args: []string{`key:a\:b\"c`},
			want: map[string]string{"key": `a:b"c`},
		},
		{
			name: "whitespace trimmed",
			args: []string{"  lang : Japanese  "},
			want: map[string]string{"lang": "Japanese"},
		},
		{
			name: "empty args",
			args: nil,
			want: map[string]string{},
		},
		{
			name:    "missing colon",
			args:    []string{"invalid"},
			wantErr: true,
		},
		{
			name:    "reserved input key",
			args:    []string{"input:value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("processArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("processArgs() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("processArgs()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}
