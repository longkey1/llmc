package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// useConfigFile points viper at a config file inside a temp directory and
// registers cleanup so global viper state does not leak between tests.
func useConfigFile(t *testing.T, content string) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	return dir
}

func TestConfigGetters(t *testing.T) {
	cfg := &Config{Model: "openai:gpt-4"}

	if got := cfg.GetModel(); got != "openai:gpt-4" {
		t.Errorf("GetModel() = %v, want %v", got, "openai:gpt-4")
	}

	provider, err := cfg.GetProvider()
	if err != nil {
		t.Fatalf("GetProvider() unexpected error: %v", err)
	}
	if provider != "openai" {
		t.Errorf("GetProvider() = %v, want %v", provider, "openai")
	}

	model, err := cfg.GetModelName()
	if err != nil {
		t.Fatalf("GetModelName() unexpected error: %v", err)
	}
	if model != "gpt-4" {
		t.Errorf("GetModelName() = %v, want %v", model, "gpt-4")
	}
}

func TestConfigGettersInvalidModel(t *testing.T) {
	cfg := &Config{Model: "no-colon"}

	if _, err := cfg.GetProvider(); err == nil {
		t.Error("GetProvider() expected error for invalid model format, got nil")
	}
	if _, err := cfg.GetModelName(); err == nil {
		t.Error("GetModelName() expected error for invalid model format, got nil")
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig("/tmp/prompts")

	if cfg.Model != "openai:gpt-4.1" {
		t.Errorf("Model = %v, want %v", cfg.Model, "openai:gpt-4.1")
	}
	if cfg.OpenAIBaseURL != "https://api.openai.com/v1" {
		t.Errorf("OpenAIBaseURL = %v, want %v", cfg.OpenAIBaseURL, "https://api.openai.com/v1")
	}
	if cfg.GeminiBaseURL != "https://generativelanguage.googleapis.com/v1beta" {
		t.Errorf("GeminiBaseURL = %v, want default", cfg.GeminiBaseURL)
	}
	if cfg.AnthropicBaseURL != "https://api.anthropic.com/v1" {
		t.Errorf("AnthropicBaseURL = %v, want default", cfg.AnthropicBaseURL)
	}
	if cfg.OpenAIToken != "" || cfg.GeminiToken != "" || cfg.AnthropicToken != "" {
		t.Error("tokens should be empty by default")
	}
	if len(cfg.PromptDirs) != 1 || cfg.PromptDirs[0] != "/tmp/prompts" {
		t.Errorf("PromptDirs = %v, want [/tmp/prompts]", cfg.PromptDirs)
	}
	if cfg.EnableWebSearch {
		t.Error("EnableWebSearch should be false by default")
	}
	if cfg.SessionMessageThreshold != 50 {
		t.Errorf("SessionMessageThreshold = %d, want 50", cfg.SessionMessageThreshold)
	}
	if cfg.SessionRetentionDays != 30 {
		t.Errorf("SessionRetentionDays = %d, want 30", cfg.SessionRetentionDays)
	}
}

func TestExpandEnvVar(t *testing.T) {
	t.Setenv("LLMC_TEST_TOKEN", "secret-value")
	t.Setenv("LLMC_TEST_EMPTY", "")

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "plain value not expanded",
			value: "plain-token",
			want:  "plain-token",
		},
		{
			name:  "dollar syntax",
			value: "$LLMC_TEST_TOKEN",
			want:  "secret-value",
		},
		{
			name:  "brace syntax",
			value: "${LLMC_TEST_TOKEN}",
			want:  "secret-value",
		},
		{
			name:  "unset variable returns empty",
			value: "$LLMC_TEST_UNSET_DOES_NOT_EXIST",
			want:  "",
		},
		{
			name:  "empty string",
			value: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandEnvVar(tt.value)
			if err != nil {
				t.Fatalf("expandEnvVar() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expandEnvVar(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestGetBaseURL(t *testing.T) {
	cfg := &Config{
		OpenAIBaseURL:    "https://openai.example.com/v1",
		GeminiBaseURL:    "https://gemini.example.com/v1beta",
		AnthropicBaseURL: "https://anthropic.example.com/v1",
	}

	tests := []struct {
		name     string
		cfg      *Config
		provider string
		want     string
		wantErr  bool
	}{
		{
			name:     "openai",
			cfg:      cfg,
			provider: "openai",
			want:     "https://openai.example.com/v1",
		},
		{
			name:     "gemini",
			cfg:      cfg,
			provider: "gemini",
			want:     "https://gemini.example.com/v1beta",
		},
		{
			name:     "anthropic",
			cfg:      cfg,
			provider: "anthropic",
			want:     "https://anthropic.example.com/v1",
		},
		{
			name:     "unsupported provider",
			cfg:      cfg,
			provider: "unknown",
			wantErr:  true,
		},
		{
			name:     "empty base URL",
			cfg:      &Config{},
			provider: "openai",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.GetBaseURL(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetBaseURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetBaseURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetToken(t *testing.T) {
	cfg := &Config{
		OpenAIToken:    "openai-token",
		GeminiToken:    "gemini-token",
		AnthropicToken: "anthropic-token",
	}

	tests := []struct {
		name     string
		cfg      *Config
		provider string
		want     string
		wantErr  bool
	}{
		{
			name:     "openai",
			cfg:      cfg,
			provider: "openai",
			want:     "openai-token",
		},
		{
			name:     "gemini",
			cfg:      cfg,
			provider: "gemini",
			want:     "gemini-token",
		},
		{
			name:     "anthropic",
			cfg:      cfg,
			provider: "anthropic",
			want:     "anthropic-token",
		},
		{
			name:     "unsupported provider",
			cfg:      cfg,
			provider: "unknown",
			wantErr:  true,
		},
		{
			name:     "empty token",
			cfg:      &Config{},
			provider: "gemini",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.GetToken(tt.provider)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("GetToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePathAbsolute(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	got, err := ResolvePath("/absolute/path")
	if err != nil {
		t.Fatalf("ResolvePath() unexpected error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("ResolvePath() = %v, want %v", got, "/absolute/path")
	}
}

func TestResolvePathWithoutConfigFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	got, err := ResolvePath("relative/path")
	if err != nil {
		t.Fatalf("ResolvePath() unexpected error: %v", err)
	}
	want := filepath.Join(cwd, "relative/path")
	if got != want {
		t.Errorf("ResolvePath() = %v, want %v", got, want)
	}
}

func TestResolvePathWithConfigFile(t *testing.T) {
	configDir := useConfigFile(t, "")

	got, err := ResolvePath("prompts")
	if err != nil {
		t.Fatalf("ResolvePath() unexpected error: %v", err)
	}
	want := filepath.Join(configDir, "prompts")
	if got != want {
		t.Errorf("ResolvePath() = %v, want %v", got, want)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("LLMC_TEST_OPENAI_TOKEN", "expanded-token")
	t.Setenv("LLMC_TEST_BASE_URL", "https://expanded.example.com/v1")

	configDir := useConfigFile(t, `
model = "openai:gpt-4"
openai_token = "$LLMC_TEST_OPENAI_TOKEN"
openai_base_url = "${LLMC_TEST_BASE_URL}"
gemini_token = "plain-gemini-token"
prompt_dirs = ["prompts", "/abs/prompts"]
`)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() unexpected error: %v", err)
	}

	if cfg.Model != "openai:gpt-4" {
		t.Errorf("Model = %v, want %v", cfg.Model, "openai:gpt-4")
	}
	if cfg.OpenAIToken != "expanded-token" {
		t.Errorf("OpenAIToken = %v, want %v", cfg.OpenAIToken, "expanded-token")
	}
	if cfg.OpenAIBaseURL != "https://expanded.example.com/v1" {
		t.Errorf("OpenAIBaseURL = %v, want %v", cfg.OpenAIBaseURL, "https://expanded.example.com/v1")
	}
	if cfg.GeminiToken != "plain-gemini-token" {
		t.Errorf("GeminiToken = %v, want %v", cfg.GeminiToken, "plain-gemini-token")
	}

	wantDirs := []string{filepath.Join(configDir, "prompts"), "/abs/prompts"}
	if len(cfg.PromptDirs) != len(wantDirs) {
		t.Fatalf("PromptDirs = %v, want %v", cfg.PromptDirs, wantDirs)
	}
	for i, want := range wantDirs {
		if cfg.PromptDirs[i] != want {
			t.Errorf("PromptDirs[%d] = %v, want %v", i, cfg.PromptDirs[i], want)
		}
	}
}
