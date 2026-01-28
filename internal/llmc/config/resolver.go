package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// expandEnvVar expands environment variable references in the given value
// Supports both $VAR and ${VAR} syntax
// Returns the expanded value. If the environment variable is not set, returns empty string.
func expandEnvVar(value string) (string, error) {
	// Check if it's an environment variable reference
	if !strings.HasPrefix(value, "$") {
		// Not an environment variable reference, return as-is
		return value, nil
	}

	var envVarName string
	// Support both $VAR and ${VAR} syntax
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		// Extract variable name from ${VAR} format
		envVarName = value[2 : len(value)-1]
	} else {
		// Extract variable name from $VAR format
		envVarName = strings.TrimPrefix(value, "$")
	}

	// Get environment variable value
	// If not set, return empty string (no error)
	envValue := os.Getenv(envVarName)
	return envValue, nil
}

// GetBaseURL returns the base URL for the specified provider
// Environment variables are already expanded during LoadConfig()
func (c *Config) GetBaseURL(provider string) (string, error) {
	var baseURLValue string
	switch provider {
	case "openai":
		baseURLValue = c.OpenAIBaseURL
	case "gemini":
		baseURLValue = c.GeminiBaseURL
	case "anthropic":
		baseURLValue = c.AnthropicBaseURL
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Validate that base URL is not empty
	if baseURLValue == "" {
		return "", fmt.Errorf("%s base URL is not configured. Set it in config file (%s_base_url) or environment variable (LLMC_%s_BASE_URL)", provider, provider, strings.ToUpper(provider))
	}

	return baseURLValue, nil
}

// GetToken returns the token for the specified provider
// Environment variables are already expanded during LoadConfig()
func (c *Config) GetToken(provider string) (string, error) {
	var tokenValue string
	switch provider {
	case "openai":
		tokenValue = c.OpenAIToken
	case "gemini":
		tokenValue = c.GeminiToken
	case "anthropic":
		tokenValue = c.AnthropicToken
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Validate that token is not empty
	if tokenValue == "" {
		return "", fmt.Errorf("%s token is not configured. Set it in config file (%s_token) or environment variable (LLMC_%s_TOKEN)", provider, provider, strings.ToUpper(provider))
	}

	return tokenValue, nil
}

// ResolvePath converts a relative path to absolute path if needed
func ResolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Get config file directory as base directory
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		// If no config file is used, fall back to current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current working directory: %v", err)
		}
		return filepath.Join(cwd, path), nil
	}

	// Use config file directory as base
	configDir := filepath.Dir(configFile)

	// If configDir is relative, make it absolute
	if !filepath.IsAbs(configDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("error getting current working directory: %v", err)
		}
		configDir = filepath.Join(cwd, configDir)
	}

	resolvedPath := filepath.Join(configDir, path)
	return resolvedPath, nil
}
