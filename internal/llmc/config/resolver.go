package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// GetBaseURL returns the base URL for the specified provider
// Resolves environment variable if value starts with "$" or "${"
func (c *Config) GetBaseURL(provider string) (string, error) {
	var baseURLValue string
	switch provider {
	case "openai":
		baseURLValue = c.OpenAIBaseURL
	case "gemini":
		baseURLValue = c.GeminiBaseURL
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if it's an environment variable reference
	if strings.HasPrefix(baseURLValue, "$") {
		var envVarName string
		// Support both $VAR and ${VAR} syntax
		if strings.HasPrefix(baseURLValue, "${") && strings.HasSuffix(baseURLValue, "}") {
			// Extract variable name from ${VAR} format
			envVarName = baseURLValue[2 : len(baseURLValue)-1]
		} else {
			// Extract variable name from $VAR format
			envVarName = strings.TrimPrefix(baseURLValue, "$")
		}

		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s is not set or empty", envVarName)
		}
		return envValue, nil
	}

	return baseURLValue, nil
}

// GetToken returns the token for the specified provider
// Resolves environment variable if value starts with "$" or "${"
func (c *Config) GetToken(provider string) (string, error) {
	var tokenValue string
	switch provider {
	case "openai":
		tokenValue = c.OpenAIToken
	case "gemini":
		tokenValue = c.GeminiToken
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check if it's an environment variable reference
	if strings.HasPrefix(tokenValue, "$") {
		var envVarName string
		// Support both $VAR and ${VAR} syntax
		if strings.HasPrefix(tokenValue, "${") && strings.HasSuffix(tokenValue, "}") {
			// Extract variable name from ${VAR} format
			envVarName = tokenValue[2 : len(tokenValue)-1]
		} else {
			// Extract variable name from $VAR format
			envVarName = strings.TrimPrefix(tokenValue, "$")
		}

		envValue := os.Getenv(envVarName)
		if envValue == "" {
			return "", fmt.Errorf("environment variable %s is not set or empty", envVarName)
		}
		return envValue, nil
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
