package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/longkey1/llmc/internal/anthropic"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc/config"
	"github.com/longkey1/llmc/internal/ollama"
	"github.com/longkey1/llmc/internal/openai"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		wantType string
		wantErr  bool
	}{
		{
			name:     "openai provider",
			model:    "openai:gpt-4",
			wantType: "openai",
		},
		{
			name:     "gemini provider",
			model:    "gemini:gemini-2.0-flash",
			wantType: "gemini",
		},
		{
			name:     "anthropic provider",
			model:    "anthropic:claude-3-5-sonnet-20241022",
			wantType: "anthropic",
		},
		{
			// Ollama model IDs contain a colon; ParseModelString splits on
			// the first one only
			name:     "ollama provider",
			model:    "ollama:llama3:latest",
			wantType: "ollama",
		},
		{
			name:    "unsupported provider",
			model:   "unknown:some-model",
			wantErr: true,
		},
		{
			name:    "invalid model format",
			model:   "no-colon",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Model: tt.model}
			got, err := newProvider(cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("newProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			switch tt.wantType {
			case "openai":
				if _, ok := got.(*openai.Provider); !ok {
					t.Errorf("newProvider() = %T, want *openai.Provider", got)
				}
			case "gemini":
				if _, ok := got.(*gemini.Provider); !ok {
					t.Errorf("newProvider() = %T, want *gemini.Provider", got)
				}
			case "anthropic":
				if _, ok := got.(*anthropic.Provider); !ok {
					t.Errorf("newProvider() = %T, want *anthropic.Provider", got)
				}
			case "ollama":
				if _, ok := got.(*ollama.Provider); !ok {
					t.Errorf("newProvider() = %T, want *ollama.Provider", got)
				}
			}
		})
	}
}

func TestValidateModelOrAlias(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid model string", value: "openai:gpt-4"},
		{name: "alias reference", value: "@sonnet"},
		{name: "invalid model string", value: "no-colon", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModelOrAlias(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateModelOrAlias(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestResolveModelAliasNoOp(t *testing.T) {
	cfg := &config.Config{
		Model:        "openai:gpt-4",
		ModelAliases: map[string]string{"sonnet": "openai:anthropic/claude-sonnet-*"},
	}

	if err := resolveModelAlias(context.Background(), cfg); err != nil {
		t.Fatalf("resolveModelAlias() unexpected error: %v", err)
	}
	if cfg.Model != "openai:gpt-4" {
		t.Errorf("cfg.Model = %q, want unchanged %q", cfg.Model, "openai:gpt-4")
	}
}

func TestResolveModelAliasUndefined(t *testing.T) {
	cfg := &config.Config{
		Model:        "@undefined",
		ModelAliases: map[string]string{"sonnet": "openai:anthropic/claude-sonnet-*"},
	}

	err := resolveModelAlias(context.Background(), cfg)
	if err == nil {
		t.Fatal("resolveModelAlias() expected error for undefined alias, got nil")
	}
	if !strings.Contains(err.Error(), "@sonnet") {
		t.Errorf("error should list defined aliases, got: %v", err)
	}
}

func TestResolveModelAliasInvalidValue(t *testing.T) {
	cfg := &config.Config{
		Model:        "@broken",
		ModelAliases: map[string]string{"broken": "no-colon"},
	}

	if err := resolveModelAlias(context.Background(), cfg); err == nil {
		t.Fatal("resolveModelAlias() expected error for invalid alias value, got nil")
	}
}

func TestResolveModelAliasPinnedValueSkipsAPI(t *testing.T) {
	// An alias value without wildcards is used as-is. No token is
	// configured, so this also proves no API call is attempted.
	cfg := &config.Config{
		Model:        "@sonnet",
		ModelAliases: map[string]string{"sonnet": "openai:anthropic/claude-sonnet-4-6"},
	}

	if err := resolveModelAlias(context.Background(), cfg); err != nil {
		t.Fatalf("resolveModelAlias() unexpected error: %v", err)
	}
	if cfg.Model != "openai:anthropic/claude-sonnet-4-6" {
		t.Errorf("cfg.Model = %q, want expanded value %q", cfg.Model, "openai:anthropic/claude-sonnet-4-6")
	}
}

func TestResolveModelAliasWildcardListModelsFailure(t *testing.T) {
	// A wildcard alias requires the model list; with no token configured,
	// ListModels fails and the command must fail with a clear error.
	cfg := &config.Config{
		Model:        "@sonnet",
		ModelAliases: map[string]string{"sonnet": "openai:anthropic/claude-sonnet-*"},
	}

	if err := resolveModelAlias(context.Background(), cfg); err == nil {
		t.Fatal("resolveModelAlias() expected error when ListModels fails for wildcard alias, got nil")
	}
}
