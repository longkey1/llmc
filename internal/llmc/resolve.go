package llmc

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ErrNoCandidates is returned by ResolveModelPattern when no available model
// matches the requested pattern.
var ErrNoCandidates = errors.New("no models matching the requested pattern")

// AliasPrefix marks a model string as a reference to a config-defined model
// alias (e.g., "@sonnet" looks up model_aliases.sonnet).
const AliasPrefix = "@"

// IsModelAlias reports whether s refers to a config-defined model alias.
func IsModelAlias(s string) bool {
	return strings.HasPrefix(s, AliasPrefix)
}

// Candidate represents a model that matches a requested pattern, along with
// the version information extracted from its ID.
type Candidate struct {
	ID      string // Model identifier (e.g., "anthropic/claude-sonnet-4-5")
	Version []int  // Numeric version components in order of appearance (e.g., [4, 5])
	Date    string // Normalized snapshot date "YYYYMMDD", or "" if none
}

// Resolution holds the result of resolving a model pattern to a concrete model.
type Resolution struct {
	Input      string      // The pattern that was requested
	Resolved   string      // The concrete model ID that was selected
	Candidates []Candidate // All matching candidates, ranked best first
}

// HasModelPattern reports whether the model string contains a wildcard and
// therefore needs to be resolved against the provider's model list.
func HasModelPattern(model string) bool {
	return strings.Contains(model, "*")
}

// ResolveModelPattern resolves a wildcard model pattern
// (e.g., "anthropic/claude-sonnet-*") to the newest concrete model ID among
// the given models (e.g., "anthropic/claude-sonnet-5").
//
// "*" matches any sequence of characters (including "/"); the rest of the
// pattern is matched literally.
//
// Ranking (newest first): model IDs are split into tokens on "-", ".", "/",
// and "@". Numeric tokens form a version vector compared numerically and
// lexicographically ([5] > [4,6] > [4,5]; [4,6] > [4]). Date-like tokens
// (e.g., "20250929" or "2025-04-16") are snapshot dates, not versions: equal
// versions prefer undated IDs, then later dates, then ascending ID for
// determinism.
func ResolveModelPattern(models []ModelInfo, pattern string) (*Resolution, error) {
	re, err := compileModelPattern(pattern)
	if err != nil {
		return nil, err
	}

	var candidates []Candidate
	seen := make(map[string]bool)
	for _, m := range models {
		if seen[m.ID] || !re.MatchString(m.ID) {
			continue
		}
		seen[m.ID] = true
		_, version, date := parseModelID(m.ID)
		candidates = append(candidates, Candidate{ID: m.ID, Version: version, Date: date})
	}
	if len(candidates) == 0 {
		return nil, ErrNoCandidates
	}

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if c := compareVersionVectors(a.Version, b.Version); c != 0 {
			return c > 0 // higher version first
		}
		if (a.Date == "") != (b.Date == "") {
			return a.Date == "" // undated (alias) over dated snapshot
		}
		if a.Date != b.Date {
			return a.Date > b.Date // later snapshot first
		}
		return a.ID < b.ID // deterministic tie-break
	})

	return &Resolution{
		Input:      pattern,
		Resolved:   candidates[0].ID,
		Candidates: candidates,
	}, nil
}

// compileModelPattern converts a wildcard pattern into an anchored regexp
// where "*" matches any sequence of characters.
func compileModelPattern(pattern string) (*regexp.Regexp, error) {
	parts := strings.Split(pattern, "*")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return regexp.Compile("^" + strings.Join(parts, ".*") + "$")
}

// parseModelID classifies the tokens of a model ID (split on "-", ".", "/",
// and "@") into name tokens (lowercased), numeric version components, and a
// snapshot date ("YYYYMMDD").
func parseModelID(id string) (names []string, version []int, date string) {
	tokens := splitModelTokens(id)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !isDigits(tok) {
			names = append(names, strings.ToLower(tok))
			continue
		}
		if isCompactDate(tok) {
			date = tok
			continue
		}
		if d, consumed, ok := tryDashedDate(tokens, i); ok {
			date = d
			i += consumed - 1
			continue
		}
		n, _ := strconv.Atoi(tok)
		version = append(version, n)
	}
	return names, version, date
}

// splitModelTokens splits a model ID into tokens on "-", ".", "/", and "@".
func splitModelTokens(id string) []string {
	return strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '.' || r == '/' || r == '@'
	})
}

// isDigits reports whether s is non-empty and consists only of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isCompactDate reports whether tok looks like a compact snapshot date,
// e.g. "20250929".
func isCompactDate(tok string) bool {
	return len(tok) == 8 && (strings.HasPrefix(tok, "19") || strings.HasPrefix(tok, "20"))
}

// tryDashedDate detects a dashed snapshot date starting at tokens[i],
// e.g. "2025", "04", "16" (from "gpt-5-2025-04-16"). It returns the
// normalized "YYYYMMDD" date, the number of tokens consumed, and whether
// a date was found.
func tryDashedDate(tokens []string, i int) (string, int, bool) {
	if i+2 >= len(tokens) {
		return "", 0, false
	}
	year, month, day := tokens[i], tokens[i+1], tokens[i+2]
	if len(year) != 4 || !isDigits(year) {
		return "", 0, false
	}
	y, _ := strconv.Atoi(year)
	if y < 2015 || y > 2100 {
		return "", 0, false
	}
	if len(month) != 2 || !isDigits(month) || len(day) != 2 || !isDigits(day) {
		return "", 0, false
	}
	return year + month + day, 3, true
}

// compareVersionVectors compares two version vectors numerically and
// lexicographically. It returns -1, 0, or 1. When one vector is a prefix of
// the other, the longer vector is greater ([4,6] > [4]).
func compareVersionVectors(a, b []int) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}
