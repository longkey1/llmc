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
	Model        string             `json:"model"`
	Instructions string             `json:"instructions,omitempty"` // System-level instructions (optional)
	Input        interface{}        `json:"input"`                  // string or []InputMessage
	Tools        []ResponsesAPITool `json:"tools,omitempty"`
}

// InputMessage represents a message in the conversation history
type InputMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message content
}

// ResponsesAPITool represents a tool configuration
type ResponsesAPITool struct {
	Type string `json:"type"`
}

// ResponsesAPIResponse represents the response from OpenAI's Responses API
type ResponsesAPIResponse struct {
	ID     string               `json:"id"`
	Status string               `json:"status"`
	Error  *ResponsesAPIError   `json:"error,omitempty"`
	Output []ResponsesAPIOutput `json:"output"`
}

// ResponsesAPIError represents an error in the API response
type ResponsesAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponsesAPIOutput represents an output element
type ResponsesAPIOutput struct {
	Type    string                `json:"type"`
	Content []ResponsesAPIContent `json:"content,omitempty"`
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
	GetBaseURL(provider string) (string, error)
	GetToken(provider string) (string, error)
}

// Provider implements the llmc.Provider interface for OpenAI
type Provider struct {
	config           Config
	webSearchEnabled bool
	debug            bool
}

// NewProvider creates a new OpenAI provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config:           config,
		webSearchEnabled: false,
		debug:            false,
	}
}

// SetWebSearch enables or disables web search
func (p *Provider) SetWebSearch(enabled bool) {
	p.webSearchEnabled = enabled
}

// SetIgnoreWebSearchErrors is a no-op for OpenAI (not applicable)
func (p *Provider) SetIgnoreWebSearchErrors(enabled bool) {
	// Not applicable for OpenAI
}

// SetDebug enables or disables debug mode
func (p *Provider) SetDebug(enabled bool) {
	p.debug = enabled
}

// ListModels returns the list of supported models from the API
func (p *Provider) ListModels() ([]llmc.ModelInfo, error) {
	// Get token for OpenAI
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for OpenAI
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to get base URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("GET", baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if p.debug {
			return nil, fmt.Errorf("failed to connect to API: %v", err)
		}
		return nil, fmt.Errorf("failed to connect to API. Use --verbose for details")
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		if p.debug {
			return nil, fmt.Errorf("API request failed (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API request failed (HTTP %d). Use --verbose for details", resp.StatusCode)
	}

	// Parse response
	var result ModelsAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if p.debug {
			return nil, fmt.Errorf("failed to parse API response: %v\nRaw response: %s", err, string(body))
		}
		return nil, fmt.Errorf("failed to parse API response. Use --verbose for details")
	}

	// Convert to ModelInfo format
	models := make([]llmc.ModelInfo, 0)

	for _, model := range result.Data {
		id := model.ID

		// Convert created timestamp to JST and use as description
		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		createdTime := time.Unix(model.Created, 0).In(jst)
		description := fmt.Sprintf("Created: %s", createdTime.Format("2006-01-02 15:04:05 JST"))

		models = append(models, llmc.ModelInfo{
			ID:          id,
			Description: description,
			IsDefault:   false, // Set by caller
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
	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Prepare the request body
	reqBody := ResponsesAPIRequest{
		Model: modelName,
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

	// Get token for OpenAI
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for OpenAI
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", baseURL+"/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

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
		if p.debug {
			return "", fmt.Errorf("API request failed (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("API request failed (HTTP %d). Use --verbose for details", resp.StatusCode)
	}

	// Parse response
	var result ResponsesAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if p.debug {
			return "", fmt.Errorf("failed to parse API response: %v\nRaw response: %s", err, string(body))
		}
		return "", fmt.Errorf("failed to parse API response. Use --verbose for details")
	}

	// Check for API error in response
	if result.Error != nil {
		if p.debug {
			return "", fmt.Errorf("API error [%s]: %s (id=%s, status=%s)",
				result.Error.Code, result.Error.Message, result.ID, result.Status)
		}
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Output) == 0 {
		if p.debug {
			return "", fmt.Errorf("API returned empty response (id=%s, status=%s)\nRaw response: %s",
				result.ID, result.Status, string(body))
		}
		return "", fmt.Errorf("API returned empty response. Use --verbose for details")
	}

	// Find the message output (web_search returns multiple outputs)
	var messageOutput *ResponsesAPIOutput
	var outputTypes []string
	for i := range result.Output {
		outputTypes = append(outputTypes, result.Output[i].Type)
		if result.Output[i].Type == "message" {
			messageOutput = &result.Output[i]
			break
		}
	}

	if messageOutput == nil {
		if p.debug {
			return "", fmt.Errorf("no message found in API response (found types: %v)\nRaw response: %s",
				outputTypes, string(body))
		}
		return "", fmt.Errorf("no message found in API response (found: %v). Use --verbose for details", outputTypes)
	}

	if len(messageOutput.Content) == 0 {
		if p.debug {
			return "", fmt.Errorf("message has no content (id=%s, status=%s)\nRaw response: %s",
				result.ID, result.Status, string(body))
		}
		return "", fmt.Errorf("message has no content. Use --verbose for details")
	}

	// Extract text and citations
	content := messageOutput.Content[0]
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

// ChatWithHistory sends a conversation history with a new message to OpenAI's Responses API
func (p *Provider) ChatWithHistory(systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Convert messages to InputMessage array
	inputMessages := make([]InputMessage, 0, len(messages)+1)
	for _, msg := range messages {
		inputMessages = append(inputMessages, InputMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add new message
	inputMessages = append(inputMessages, InputMessage{
		Role:    "user",
		Content: newMessage,
	})

	// Prepare the request body
	reqBody := ResponsesAPIRequest{
		Model:        modelName,
		Instructions: systemPrompt, // Can be empty string
		Input:        inputMessages,
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

	// Get token for OpenAI
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for OpenAI
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", baseURL+"/responses", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

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
		if p.debug {
			return "", fmt.Errorf("API request failed (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("API request failed (HTTP %d). Use --verbose for details", resp.StatusCode)
	}

	// Parse response
	var result ResponsesAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if p.debug {
			return "", fmt.Errorf("failed to parse API response: %v\nRaw response: %s", err, string(body))
		}
		return "", fmt.Errorf("failed to parse API response. Use --verbose for details")
	}

	// Check for API error in response
	if result.Error != nil {
		if p.debug {
			return "", fmt.Errorf("API error [%s]: %s (id=%s, status=%s)",
				result.Error.Code, result.Error.Message, result.ID, result.Status)
		}
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Output) == 0 {
		if p.debug {
			return "", fmt.Errorf("API returned empty response (id=%s, status=%s)\nRaw response: %s",
				result.ID, result.Status, string(body))
		}
		return "", fmt.Errorf("API returned empty response. Use --verbose for details")
	}

	// Find the message output (web_search returns multiple outputs)
	var messageOutput *ResponsesAPIOutput
	var outputTypes []string
	for i := range result.Output {
		outputTypes = append(outputTypes, result.Output[i].Type)
		if result.Output[i].Type == "message" {
			messageOutput = &result.Output[i]
			break
		}
	}

	if messageOutput == nil {
		if p.debug {
			return "", fmt.Errorf("no message found in API response (found types: %v)\nRaw response: %s",
				outputTypes, string(body))
		}
		return "", fmt.Errorf("no message found in API response (found: %v). Use --verbose for details", outputTypes)
	}

	if len(messageOutput.Content) == 0 {
		if p.debug {
			return "", fmt.Errorf("message has no content (id=%s, status=%s)\nRaw response: %s",
				result.ID, result.Status, string(body))
		}
		return "", fmt.Errorf("message has no content. Use --verbose for details")
	}

	// Extract text and citations
	content := messageOutput.Content[0]
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
