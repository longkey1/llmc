package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	ProviderName   = "openai"
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4.1"
)

// Supported models for Responses API
var responsesAPISupportedModels = []string{
	"gpt-4o",
	"gpt-4.1",
	"o3",
	"o4-mini",
	"gpt-5",
}

// ResponsesAPIRequest represents the request body for OpenAI's Responses API
type ResponsesAPIRequest struct {
	Model  string              `json:"model"`
	Input  string              `json:"input"`
	Tools  []ResponsesAPITool  `json:"tools,omitempty"`
}

// ResponsesAPITool represents a tool configuration
type ResponsesAPITool struct {
	Type string `json:"type"`
}

// ResponsesAPIResponse represents the response from OpenAI's Responses API
type ResponsesAPIResponse struct {
	Output []ResponsesAPIOutput `json:"output"`
}

// ResponsesAPIOutput represents an output element
type ResponsesAPIOutput struct {
	Content []ResponsesAPIContent `json:"content"`
}

// ResponsesAPIContent represents content with text and annotations
type ResponsesAPIContent struct {
	Text        string                    `json:"text"`
	Annotations []ResponsesAPIAnnotation  `json:"annotations,omitempty"`
}

// ResponsesAPIAnnotation represents a citation annotation
type ResponsesAPIAnnotation struct {
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Config defines the configuration interface for OpenAI provider
type Config interface {
	GetModel() string
	GetBaseURL() string
	GetToken() string
}

// Provider implements the llmc.Provider interface for OpenAI
type Provider struct {
	config        Config
	webSearchEnabled bool
}

// NewProvider creates a new OpenAI provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config:        config,
		webSearchEnabled: false,
	}
}

// SetWebSearch enables or disables web search
func (p *Provider) SetWebSearch(enabled bool) {
	p.webSearchEnabled = enabled
}

// isResponsesAPISupported checks if the model is supported by Responses API
func isResponsesAPISupported(model string) bool {
	for _, supported := range responsesAPISupportedModels {
		if strings.HasPrefix(model, supported) {
			return true
		}
	}
	return false
}

// Chat sends a message to OpenAI's Responses API and returns the response
func (p *Provider) Chat(message string) (string, error) {
	model := p.config.GetModel()

	// Check model compatibility
	if !isResponsesAPISupported(model) {
		return "", fmt.Errorf(`Model '%s' is not supported with Responses API.

Supported models: gpt-4o, gpt-4.1, o3, o4-mini, gpt-5 series

Please change your model with --model flag or in config file.
Example: llmc chat --model gpt-4o "your question"`, model)
	}

	// Prepare the request body
	reqBody := ResponsesAPIRequest{
		Model: model,
		Input: message,
	}

	// Add web_search tool if enabled
	if p.webSearchEnabled {
		reqBody.Tools = []ResponsesAPITool{
			{Type: "web_search"},
		}
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", p.config.GetBaseURL()+"/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.GetToken())

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
	var result ResponsesAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(result.Output) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	if len(result.Output[0].Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Extract text and citations
	content := result.Output[0].Content[0]
	responseText := content.Text

	// Format citations if present
	if len(content.Annotations) > 0 {
		citations := extractCitations(content.Annotations)
		if citations != "" {
			responseText += "\n\n---\nSources:\n" + citations
		}
	}

	return responseText, nil
}

// extractCitations formats annotations into a citation list
func extractCitations(annotations []ResponsesAPIAnnotation) string {
	var citations []string
	seenURLs := make(map[string]bool)
	index := 1

	for _, annotation := range annotations {
		if annotation.Type == "url_citation" && annotation.URL != "" {
			// Skip duplicate URLs
			if seenURLs[annotation.URL] {
				continue
			}
			seenURLs[annotation.URL] = true

			title := annotation.Title
			if title == "" {
				title = "Source"
			}
			citations = append(citations, fmt.Sprintf("[%d] %s - %s", index, title, annotation.URL))
			index++
		}
	}

	return strings.Join(citations, "\n")
}
