package cmd

import (
	"testing"

	"github.com/longkey1/llmc/internal/llmc/config"
)

func TestMaskToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "empty token",
			token: "",
			want:  "********",
		},
		{
			name:  "short token fully masked",
			token: "abcd1234",
			want:  "********",
		},
		{
			name:  "long token shows head and tail",
			token: "sk-1234567890abcdef",
			want:  "sk-1...cdef",
		},
		{
			name:  "nine characters",
			token: "123456789",
			want:  "1234...6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskToken(tt.token); got != tt.want {
				t.Errorf("maskToken(%q) = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestResolveAndMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		provider string
		want     string
	}{
		{
			name:     "token set",
			cfg:      &config.Config{OpenAIToken: "sk-1234567890abcdef"},
			provider: "openai",
			want:     "sk-1...cdef",
		},
		{
			name:     "token not set",
			cfg:      &config.Config{},
			provider: "openai",
			want:     "(not set)",
		},
		{
			name:     "unsupported provider",
			cfg:      &config.Config{},
			provider: "unknown",
			want:     "(not set)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveAndMaskToken(tt.cfg, tt.provider); got != tt.want {
				t.Errorf("resolveAndMaskToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
