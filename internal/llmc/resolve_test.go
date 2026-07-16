package llmc

import (
	"errors"
	"reflect"
	"testing"
)

// modelInfos converts a list of model IDs into []ModelInfo for tests.
func modelInfos(ids ...string) []ModelInfo {
	models := make([]ModelInfo, 0, len(ids))
	for _, id := range ids {
		models = append(models, ModelInfo{ID: id})
	}
	return models
}

// litellmModels mirrors the ID shapes served by a LiteLLM proxy
// (route-prefixed IDs, mixed old/new naming, "@"-dated vertex snapshots).
var litellmModels = modelInfos(
	"anthropic/claude-3-7-sonnet-20250219",
	"anthropic/claude-3-haiku-20240307",
	"anthropic/claude-4-sonnet-20250514",
	"anthropic/claude-fable-5",
	"anthropic/claude-haiku-4-5",
	"anthropic/claude-opus-4-1",
	"anthropic/claude-opus-4-8",
	"anthropic/claude-sonnet-4-20250514",
	"anthropic/claude-sonnet-4-5",
	"anthropic/claude-sonnet-4-5-20250929",
	"anthropic/claude-sonnet-4-5@20250929",
	"anthropic/claude-sonnet-4-6",
	"anthropic/claude-sonnet-5",
	"openai/gpt-4o",
	"openai/gpt-4o-2024-11-20",
	"openai/gpt-4o-mini",
	"openai/gpt-5",
	"openai/gpt-5-2025-08-07",
	"openai/gpt-5-mini",
	"openai/gpt-5-pro",
	"openai/gpt-5.1",
	"openai/gpt-5.2",
	"openai/gpt-5.2-chat-latest",
	"openai/gpt-5.2-pro",
	"openai/gpt-5.3-codex",
	"openai/gpt-5.4-mini",
	"openai/gpt-5.5",
	"openai/gpt-5.5-pro",
	"openai/gpt-5.6",
	"openai/o3",
	"openai/o3-2025-04-16",
	"openai/o3-mini",
	"vertex_ai/claude-sonnet-4@20250514",
	"vertex_ai/gemini-2.0-flash",
	"vertex_ai/gemini-2.0-flash-001",
	"vertex_ai/gemini-2.5-flash",
	"vertex_ai/gemini-2.5-flash-lite",
	"vertex_ai/gemini-2.5-pro",
	"vertex_ai/gemini-3.1-pro-preview",
	"vertex_ai/gemini-3.5-flash",
)

// anthropicModels mirrors the official Anthropic /models IDs
// (pre-4.6 dated snapshots plus 4.6+ dateless canonical IDs).
var anthropicModels = modelInfos(
	"claude-3-5-haiku-20241022",
	"claude-3-7-sonnet-20250219",
	"claude-haiku-4-5-20251001",
	"claude-opus-4-1-20250805",
	"claude-opus-4-20250514",
	"claude-opus-4-7",
	"claude-opus-4-8",
	"claude-sonnet-4-20250514",
	"claude-sonnet-4-5-20250929",
	"claude-sonnet-4-6",
	"claude-sonnet-5",
	"claude-fable-5",
)

// openaiModels mirrors official OpenAI IDs (dotted versions, dashed dates).
var openaiModels = modelInfos(
	"gpt-4.1",
	"gpt-4o",
	"gpt-4o-2024-11-20",
	"gpt-4o-mini",
	"gpt-5",
	"gpt-5-2025-08-07",
	"gpt-5.1",
	"gpt-5.2",
	"gpt-5.2-chat-latest",
	"gpt-5.6",
	"o1",
	"o1-2024-12-17",
	"o3",
	"o3-mini",
)

// geminiModels mirrors official Gemini IDs (version in the middle of the ID).
var geminiModels = modelInfos(
	"gemini-2.0-flash",
	"gemini-2.0-flash-001",
	"gemini-2.5-flash",
	"gemini-2.5-flash-lite",
	"gemini-2.5-pro",
	"gemini-3.1-pro-preview",
	"gemini-3.5-flash",
	"gemini-flash-latest",
)

func TestResolveModelPattern(t *testing.T) {
	tests := []struct {
		name      string
		models    []ModelInfo
		pattern   string
		want      string
		wantErr   error
		minRanked int // minimum expected number of candidates (0 = don't check)
	}{
		// LiteLLM proxy fixtures
		{
			name:      "litellm claude-sonnet wildcard resolves newest",
			models:    litellmModels,
			pattern:   "anthropic/claude-sonnet-*",
			want:      "anthropic/claude-sonnet-5",
			minRanked: 6,
		},
		{
			name:    "litellm claude-opus wildcard",
			models:  litellmModels,
			pattern: "anthropic/claude-opus-*",
			want:    "anthropic/claude-opus-4-8",
		},
		{
			name:    "litellm major version pinning",
			models:  litellmModels,
			pattern: "anthropic/claude-sonnet-4-*",
			want:    "anthropic/claude-sonnet-4-6",
		},
		{
			name:    "litellm route prefix is part of the pattern",
			models:  litellmModels,
			pattern: "vertex_ai/claude-sonnet-*",
			want:    "vertex_ai/claude-sonnet-4@20250514",
		},
		{
			name:    "litellm gpt wildcard resolves dotted versions",
			models:  litellmModels,
			pattern: "openai/gpt-5*",
			want:    "openai/gpt-5.6",
		},
		{
			name:    "litellm tier name with middle wildcard",
			models:  litellmModels,
			pattern: "openai/gpt-*-pro",
			want:    "openai/gpt-5.5-pro",
		},
		{
			name:    "litellm gemini version in the middle",
			models:  litellmModels,
			pattern: "vertex_ai/gemini-*-flash",
			want:    "vertex_ai/gemini-3.5-flash",
		},
		{
			name:    "litellm trailing anchor excludes suffix variants",
			models:  litellmModels,
			pattern: "vertex_ai/gemini-*-pro",
			want:    "vertex_ai/gemini-2.5-pro",
		},
		{
			name:    "litellm preview matched when requested explicitly",
			models:  litellmModels,
			pattern: "vertex_ai/gemini-*-pro-preview",
			want:    "vertex_ai/gemini-3.1-pro-preview",
		},
		{
			name:    "litellm no match",
			models:  litellmModels,
			pattern: "anthropic/claude-banana-*",
			wantErr: ErrNoCandidates,
		},
		// Official Anthropic fixtures
		{
			name:    "anthropic official sonnet",
			models:  anthropicModels,
			pattern: "claude-sonnet-*",
			want:    "claude-sonnet-5",
		},
		{
			name:    "anthropic official opus",
			models:  anthropicModels,
			pattern: "claude-opus-*",
			want:    "claude-opus-4-8",
		},
		{
			name:    "anthropic official haiku",
			models:  anthropicModels,
			pattern: "claude-haiku-*",
			want:    "claude-haiku-4-5-20251001",
		},
		// Official OpenAI fixtures
		{
			name:    "openai gpt wildcard prefers newest dotted version",
			models:  openaiModels,
			pattern: "gpt-5*",
			want:    "gpt-5.6",
		},
		{
			name:    "openai gpt-4o tie-break prefers shortest ID over mini",
			models:  modelInfos("gpt-4o", "gpt-4o-2024-11-20", "gpt-4o-mini"),
			pattern: "gpt-4o*",
			want:    "gpt-4o",
		},
		{
			name:    "openai o3 date suffix wildcard",
			models:  modelInfos("o3", "o3-2025-04-16", "o3-mini"),
			pattern: "o3-2*",
			want:    "o3-2025-04-16",
		},
		// Official Gemini fixtures
		{
			name:    "gemini flash excludes lite and latest variants",
			models:  geminiModels,
			pattern: "gemini-*-flash",
			want:    "gemini-3.5-flash",
		},
		{
			name:    "gemini minor version pinning prefers stable suffix",
			models:  geminiModels,
			pattern: "gemini-2.0-flash*",
			want:    "gemini-2.0-flash-001",
		},
		// Ranking details
		{
			name:    "undated alias preferred over dated snapshot",
			models:  modelInfos("claude-sonnet-4-5-20250929", "claude-sonnet-4-5"),
			pattern: "claude-sonnet-4-*",
			want:    "claude-sonnet-4-5",
		},
		{
			name:    "later snapshot date wins",
			models:  modelInfos("gpt-5-2025-01-01", "gpt-5-2025-08-07"),
			pattern: "gpt-5-*",
			want:    "gpt-5-2025-08-07",
		},
		{
			name:    "deterministic tie-break by ascending ID",
			models:  modelInfos("claude-sonnet-4-5@20250929", "claude-sonnet-4-5-20250929"),
			pattern: "claude-sonnet-4-5*",
			want:    "claude-sonnet-4-5-20250929",
		},
		{
			name:    "empty model list",
			models:  nil,
			pattern: "claude-sonnet-*",
			wantErr: ErrNoCandidates,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveModelPattern(tt.models, tt.pattern)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ResolveModelPattern(%q) error = %v, want %v", tt.pattern, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveModelPattern(%q) unexpected error: %v", tt.pattern, err)
			}
			if got.Resolved != tt.want {
				t.Errorf("ResolveModelPattern(%q) = %q, want %q", tt.pattern, got.Resolved, tt.want)
			}
			if tt.minRanked > 0 && len(got.Candidates) < tt.minRanked {
				t.Errorf("ResolveModelPattern(%q) returned %d candidates, want at least %d", tt.pattern, len(got.Candidates), tt.minRanked)
			}
		})
	}
}

func TestHasModelPattern(t *testing.T) {
	if !HasModelPattern("anthropic/claude-sonnet-*") {
		t.Error("HasModelPattern() = false for wildcard pattern, want true")
	}
	if HasModelPattern("anthropic/claude-sonnet-5") {
		t.Error("HasModelPattern() = true for concrete ID, want false")
	}
}

func TestParseModelID(t *testing.T) {
	tests := []struct {
		id          string
		wantNames   []string
		wantVersion []int
		wantDate    string
	}{
		{"anthropic/claude-sonnet-5", []string{"anthropic", "claude", "sonnet"}, []int{5}, ""},
		{"anthropic/claude-sonnet-4-5-20250929", []string{"anthropic", "claude", "sonnet"}, []int{4, 5}, "20250929"},
		{"vertex_ai/claude-sonnet-4@20250514", []string{"vertex_ai", "claude", "sonnet"}, []int{4}, "20250514"},
		{"anthropic/claude-3-7-sonnet-20250219", []string{"anthropic", "claude", "sonnet"}, []int{3, 7}, "20250219"},
		{"openai/gpt-5.2", []string{"openai", "gpt"}, []int{5, 2}, ""},
		{"gpt-5-2025-08-07", []string{"gpt"}, []int{5}, "20250807"},
		{"gpt-4o-mini", []string{"gpt", "4o", "mini"}, nil, ""},
		{"o3-2025-04-16", []string{"o3"}, nil, "20250416"},
		{"gemini-2.5-flash-lite", []string{"gemini", "flash", "lite"}, []int{2, 5}, ""},
		{"gemini-2.0-flash-001", []string{"gemini", "flash"}, []int{2, 0, 1}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			names, version, date := parseModelID(tt.id)
			if !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("parseModelID(%q) names = %v, want %v", tt.id, names, tt.wantNames)
			}
			if !reflect.DeepEqual(version, tt.wantVersion) {
				t.Errorf("parseModelID(%q) version = %v, want %v", tt.id, version, tt.wantVersion)
			}
			if date != tt.wantDate {
				t.Errorf("parseModelID(%q) date = %q, want %q", tt.id, date, tt.wantDate)
			}
		})
	}
}

func TestCompareVersionVectors(t *testing.T) {
	tests := []struct {
		a, b []int
		want int
	}{
		{[]int{5}, []int{4, 6}, 1},
		{[]int{4, 6}, []int{4, 5}, 1},
		{[]int{4}, []int{4, 6}, -1},
		{[]int{4, 5}, []int{4, 5}, 0},
		{nil, nil, 0},
		{nil, []int{1}, -1},
		{[]int{5, 6}, []int{5, 2}, 1},
	}

	for _, tt := range tests {
		if got := compareVersionVectors(tt.a, tt.b); got != tt.want {
			t.Errorf("compareVersionVectors(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
