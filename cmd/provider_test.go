package cmd

import (
	"testing"

	"github.com/longkey1/llmc/internal/anthropic"
	"github.com/longkey1/llmc/internal/gemini"
	"github.com/longkey1/llmc/internal/llmc/config"
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
			}
		})
	}
}
