package tools

import (
	"context"
	"strings"
	"testing"
)

// envNames returns the variable names present in an environment slice.
func envNames(env []string) map[string]bool {
	names := make(map[string]bool, len(env))
	for _, entry := range env {
		if name, _, ok := strings.Cut(entry, "="); ok {
			names[name] = true
		}
	}
	return names
}

func TestBuildEnvFilteredStripsSecrets(t *testing.T) {
	t.Setenv("LLMC_OPENAI_TOKEN", "secret")
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("MY_PASSWORD", "secret")
	t.Setenv("HARMLESS_VAR", "ok")

	names := envNames(buildEnv(&Options{EnvMode: EnvModeFiltered}))

	for _, stripped := range []string{"LLMC_OPENAI_TOKEN", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "MY_PASSWORD"} {
		if names[stripped] {
			t.Errorf("%s should be stripped in filtered mode", stripped)
		}
	}
	if !names["HARMLESS_VAR"] {
		t.Error("HARMLESS_VAR should be passed through in filtered mode")
	}
	if !names["PATH"] {
		t.Error("PATH should be passed through in filtered mode")
	}
}

func TestBuildEnvDefaultsToFiltered(t *testing.T) {
	t.Setenv("LLMC_ANTHROPIC_TOKEN", "secret")

	// A nil Options and an empty EnvMode both mean filtered.
	for _, opts := range []*Options{nil, {}} {
		if envNames(buildEnv(opts))["LLMC_ANTHROPIC_TOKEN"] {
			t.Errorf("token leaked with opts %+v", opts)
		}
	}
}

func TestBuildEnvPassthroughReAllowsSecret(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret")

	names := envNames(buildEnv(&Options{
		EnvMode:        EnvModeFiltered,
		EnvPassthrough: []string{"GITHUB_TOKEN"},
	}))
	if !names["GITHUB_TOKEN"] {
		t.Error("GITHUB_TOKEN should be passed through when listed")
	}
}

func TestBuildEnvMinimal(t *testing.T) {
	t.Setenv("HARMLESS_VAR", "ok")
	t.Setenv("KEEP_ME", "ok")

	names := envNames(buildEnv(&Options{
		EnvMode:        EnvModeMinimal,
		EnvPassthrough: []string{"KEEP_ME"},
	}))
	if names["HARMLESS_VAR"] {
		t.Error("HARMLESS_VAR should be dropped in minimal mode")
	}
	if !names["KEEP_ME"] {
		t.Error("KEEP_ME should be passed through when listed")
	}
	if !names["PATH"] {
		t.Error("PATH should be kept in minimal mode")
	}
}

func TestBuildEnvAllInheritsEverything(t *testing.T) {
	t.Setenv("LLMC_OPENAI_TOKEN", "secret")

	// nil means os/exec inherits the parent environment unchanged.
	if env := buildEnv(&Options{EnvMode: EnvModeAll}); env != nil {
		t.Errorf("buildEnv(all) = %v, want nil (inherit)", env)
	}
}

func TestExecCommandDoesNotLeakTokens(t *testing.T) {
	t.Setenv("LLMC_OPENAI_TOKEN", "sk-should-not-appear")

	got, err := runExecCommand(context.Background(), `{"command":"env"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got, "sk-should-not-appear") {
		t.Error("exec_command output leaked the provider token")
	}
}

func TestIsSecretEnvName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"LLMC_MODEL", true},
		{"OPENAI_API_KEY", true},
		{"github_token", true},
		{"AWS_ACCESS_KEY_ID", true},
		{"DB_PASSWORD", true},
		{"PATH", false},
		{"HOME", false},
		{"EDITOR", false},
	}
	for _, tt := range tests {
		if got := isSecretEnvName(tt.name); got != tt.want {
			t.Errorf("isSecretEnvName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
