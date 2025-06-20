package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	ProviderName   = "gemini"
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultModel   = "gemini-2.0-flash"
)

// GeminiRequest represents the request body for Gemini's generate content API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

// GeminiContent represents a content item in the Gemini request format
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content in the Gemini request format
type GeminiPart struct {
	Text string `json:"text"`
}

// Config defines the configuration interface for Gemini provider
type Config interface {
	GetModel() string
	GetBaseURL() string
	GetToken() string
}

// Provider implements the llmc.Provider interface for Gemini
type Provider struct {
	config Config
}

// NewProvider creates a new Gemini provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config: config,
	}
}

// Chat sends a message to Gemini's API and returns the response
func (p *Provider) Chat(message string) (string, error) {
	// Prepare the request body
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{
						Text: message,
					},
				},
			},
		},
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	baseURL := p.config.GetBaseURL()
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, p.config.GetModel(), p.config.GetToken())
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", string(body))
	}

	// Parse response
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
