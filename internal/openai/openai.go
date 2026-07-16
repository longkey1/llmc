package openai

import (
	"context"
	"fmt"
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
	Input        []InputMessage     `json:"input"`                  // Conversation messages
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
	Text        string                   `json:"text"`
	Annotations []ResponsesAPIAnnotation `json:"annotations,omitempty"`
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

// endpoint resolves the token and base URL and returns the full URL for path
// along with the request headers.
func (p *Provider) endpoint(path string) (string, map[string]string, error) {
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get token: %w", err)
	}
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get base URL: %w", err)
	}
	return baseURL + path, map[string]string{"Authorization": "Bearer " + token}, nil
}

// ListModels returns the list of supported models from the API
func (p *Provider) ListModels(ctx context.Context) ([]llmc.ModelInfo, error) {
	url, headers, err := p.endpoint("/models")
	if err != nil {
		return nil, err
	}

	var result ModelsAPIResponse
	if _, err := llmc.DoJSON(ctx, http.MethodGet, url, headers, nil, &result, p.debug); err != nil {
		return nil, err
	}

	models := make([]llmc.ModelInfo, 0, len(result.Data))
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	for _, model := range result.Data {
		// Convert created timestamp to JST and use as description
		createdTime := time.Unix(model.Created, 0).In(jst)
		models = append(models, llmc.ModelInfo{
			ID:          model.ID,
			Description: fmt.Sprintf("Created: %s", createdTime.Format("2006-01-02 15:04:05 JST")),
			IsDefault:   false, // Set by caller
		})
	}

	// Sort models by ID (descending order)
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID > models[j].ID
	})

	return models, nil
}

// Chat sends a single message to OpenAI's Responses API and returns the response
func (p *Provider) Chat(ctx context.Context, message string) (string, error) {
	return p.ChatWithHistory(ctx, "", nil, message)
}

// ChatWithHistory sends a conversation history with a new message to OpenAI's Responses API
func (p *Provider) ChatWithHistory(ctx context.Context, systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Convert messages to InputMessage array and append the new message
	inputMessages := make([]InputMessage, 0, len(messages)+1)
	for _, msg := range messages {
		inputMessages = append(inputMessages, InputMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	inputMessages = append(inputMessages, InputMessage{
		Role:    "user",
		Content: newMessage,
	})

	reqBody := ResponsesAPIRequest{
		Model:        modelName,
		Instructions: systemPrompt, // Can be empty string
		Input:        inputMessages,
	}
	if p.webSearchEnabled {
		reqBody.Tools = []ResponsesAPITool{
			{Type: "web_search"},
		}
	}

	url, headers, err := p.endpoint("/responses")
	if err != nil {
		return "", err
	}

	var result ResponsesAPIResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, headers, reqBody, &result, p.debug)
	if err != nil {
		return "", err
	}

	return p.extractText(&result, body)
}

// extractText pulls the message text (and citations) out of a Responses API
// result. body is the raw response, included in debug error output.
func (p *Provider) extractText(result *ResponsesAPIResponse, body []byte) (string, error) {
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
	if len(content.Annotations) > 0 {
		if citations := extractCitations(content.Annotations); citations != "" {
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
