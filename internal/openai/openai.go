package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName   = "openai"
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4.1"
)

// ModelsAPIResponse represents the response from OpenAI's models endpoint
type ModelsAPIResponse struct {
	Data []ModelData `json:"data"`
}

// ModelData represents a single model in the API response
type ModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
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

// ListModels returns the list of supported models from the API
func (p *Provider) ListModels() []llmc.ModelInfo {
	models, err := p.fetchModelsFromAPI()
	if err != nil {
		// Return empty list on error (caller should handle)
		return nil
	}
	return models
}

// fetchModelsFromAPI retrieves the list of available models from OpenAI API
func (p *Provider) fetchModelsFromAPI() ([]llmc.ModelInfo, error) {
	// Create HTTP request
	req, err := http.NewRequest("GET", p.config.GetBaseURL()+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+p.config.GetToken())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	// Parse response
	var result ModelsAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	// Convert to ModelInfo format
	models := make([]llmc.ModelInfo, 0)
	defaultModel := p.config.GetModel()

	for _, model := range result.Data {
		id := model.ID
		isDefault := (id == defaultModel)

		// Convert created timestamp to JST and use as description
		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		createdTime := time.Unix(model.Created, 0).In(jst)
		description := fmt.Sprintf("Created: %s", createdTime.Format("2006-01-02 15:04:05 JST"))

		models = append(models, llmc.ModelInfo{
			ID:          id,
			Description: description,
			IsDefault:   isDefault,
		})
	}

	// Sort models by ID (descending order)
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID > models[j].ID
	})

	return models, nil
}

// Chat sends a message to OpenAI's Responses API and returns the response
func (p *Provider) Chat(message string) (string, error) {
	model := p.config.GetModel()

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
