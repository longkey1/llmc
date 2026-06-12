// Package llmc provides the core abstractions for LLM providers.
// This package defines the Provider interface that all LLM provider implementations
// (openai, gemini, etc.) must implement.
package llmc

import (
	"fmt"
	"sort"
	"strings"
)

// LatestSuffix is the suffix appended to a model base name to request the
// latest snapshot in that family, e.g. "openai:gpt-4o@latest".
const LatestSuffix = "@latest"

// previewMarkers are tokens that identify preview/experimental model variants.
// Models containing any of these as a hyphen-delimited token are excluded from
// "@latest" resolution by default (unless the requested base itself contains one).
var previewMarkers = []string{"preview", "experimental", "exp", "beta", "alpha", "rc", "nightly"}

// ModelInfo represents information about an available model from a provider.
type ModelInfo struct {
	ID          string // Model identifier (e.g., "gpt-4", "gemini-pro")
	Description string // Human-readable description of the model
	IsDefault   bool   // Whether this is the default model for the provider
}

// Provider defines the interface for LLM providers.
// All provider implementations (openai, gemini, etc.) must implement this interface.
//
// Example usage:
//
//	provider := openai.NewProvider(cfg)
//	provider.SetWebSearch(true)
//	response, err := provider.Chat("Hello, world!")
type Provider interface {
	// Chat sends a single message and returns the response.
	Chat(message string) (string, error)

	// ChatWithHistory sends a message with conversation history.
	// The systemPrompt is prepended to the conversation.
	// messages contains the conversation history (user and assistant messages).
	// newMessage is the new user message to send.
	ChatWithHistory(systemPrompt string, messages []Message, newMessage string) (string, error)

	// SetWebSearch enables or disables web search for the provider.
	SetWebSearch(enabled bool)

	// SetIgnoreWebSearchErrors configures whether to ignore web search errors.
	SetIgnoreWebSearchErrors(enabled bool)

	// SetDebug enables or disables debug output.
	SetDebug(enabled bool)

	// ListModels returns a list of available models for the provider.
	ListModels() ([]ModelInfo, error)
}

// ParseModelString parses a model string in "provider:model" format.
// Returns (provider, model, error).
//
// Example:
//
//	provider, model, err := ParseModelString("openai:gpt-4")
//	// provider = "openai", model = "gpt-4"
func ParseModelString(modelStr string) (string, string, error) {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid model format: %s (expected format: provider:model, e.g., openai:gpt-4)", modelStr)
	}

	provider := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])

	if provider == "" || model == "" {
		return "", "", fmt.Errorf("provider and model cannot be empty")
	}

	return provider, model, nil
}

// hasMarkerToken reports whether id contains marker as a hyphen-delimited token.
// This avoids false matches such as "exp" inside an unrelated word.
func hasMarkerToken(id, marker string) bool {
	for _, token := range strings.Split(id, "-") {
		if token == marker {
			return true
		}
	}
	return false
}

// isPreview reports whether id contains any preview/experimental marker token.
func isPreview(id string) bool {
	for _, marker := range previewMarkers {
		if hasMarkerToken(id, marker) {
			return true
		}
	}
	return false
}

// ResolveLatestModel resolves a model family base to its latest snapshot ID
// from the given list of models.
//
// A model is considered part of the base family when its ID either equals base
// exactly, or starts with base+"-" followed by a digit (a date or version
// suffix). Variants whose distinguishing token is a word (e.g. "gpt-4o-mini")
// are therefore not pulled in by "gpt-4o@latest".
//
// preview/experimental variants are excluded by default, unless the requested
// base itself contains a preview marker (i.e. the caller explicitly opted in).
//
// The candidate with the lexicographically greatest ID is returned, which for
// date/version suffixes corresponds to the newest snapshot.
func ResolveLatestModel(models []ModelInfo, base string) (string, error) {
	includePreview := isPreview(base)

	var candidates []string
	for _, m := range models {
		if !isLatestCandidate(m.ID, base) {
			continue
		}
		if !includePreview && isPreview(m.ID) {
			continue
		}
		candidates = append(candidates, m.ID)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no models found matching '%s%s'", base, LatestSuffix)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i] > candidates[j]
	})

	return candidates[0], nil
}

// isLatestCandidate reports whether id belongs to the base family: either an
// exact match, or base+"-"+<digit...> (a date/version suffix).
func isLatestCandidate(id, base string) bool {
	if id == base {
		return true
	}
	prefix := base + "-"
	if !strings.HasPrefix(id, prefix) {
		return false
	}
	rest := id[len(prefix):]
	return len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9'
}

// FormatModelString formats provider and model into "provider:model" format.
//
// Example:
//
//	modelStr := FormatModelString("openai", "gpt-4")
//	// modelStr = "openai:gpt-4"
func FormatModelString(provider, model string) string {
	return fmt.Sprintf("%s:%s", provider, model)
}
