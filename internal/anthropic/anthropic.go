package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName     = "anthropic"
	DefaultBaseURL   = "https://api.anthropic.com/v1"
	DefaultModel     = "claude-3-5-sonnet-20241022"
	AnthropicVersion = "2023-06-01"
)

// ModelsAPIResponse represents the response from Anthropic's models endpoint
type ModelsAPIResponse struct {
	Data []ModelData `json:"data"`
}

// ModelData represents a single model in the API response
type ModelData struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// MessagesAPIRequest represents the request body for Anthropic's Messages API
type MessagesAPIRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	System    string         `json:"system,omitempty"` // System prompt (optional)
	Messages  []MessageInput `json:"messages"`
}

// MessageInput represents a message in the conversation
type MessageInput struct {
	Role    string    `json:"role"`    // "user" or "assistant"
	Content []Content `json:"content"` // Array of content blocks
}

// Content represents a content block (text, tool_use, tool_result, etc.)
type Content struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result", etc.
	Text string `json:"text,omitempty"`
}

// MessagesAPIResponse represents the response from Anthropic's Messages API
type MessagesAPIResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Content      []ResponseContent `json:"content"`
	Model        string            `json:"model"`
	StopReason   string            `json:"stop_reason"`
	StopSequence *string           `json:"stop_sequence"`
	Usage        Usage             `json:"usage"`
	Error        *APIError         `json:"error,omitempty"`
}

// ResponseContent represents a content block in the response
type ResponseContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text,omitempty"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// APIError represents an error in the API response
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Config defines the configuration interface for Anthropic provider
type Config interface {
	GetModel() string
	GetBaseURL(provider string) (string, error)
	GetToken(provider string) (string, error)
}

// Provider implements the llmc.Provider interface for Anthropic
type Provider struct {
	config           Config
	webSearchEnabled bool
	debug            bool
}

// NewProvider creates a new Anthropic provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config:           config,
		webSearchEnabled: false,
		debug:            false,
	}
}

// SetWebSearch enables or disables web search
// Note: Anthropic Messages API doesn't directly support web search in the base API
func (p *Provider) SetWebSearch(enabled bool) {
	p.webSearchEnabled = enabled
	// Note: Web search is not natively supported in Anthropic's Messages API
	// This flag is kept for interface compatibility
}

// SetIgnoreWebSearchErrors is a no-op for Anthropic (not applicable)
func (p *Provider) SetIgnoreWebSearchErrors(enabled bool) {
	// Not applicable for Anthropic
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
	headers := map[string]string{
		"x-api-key":         token,
		"anthropic-version": AnthropicVersion,
	}
	return baseURL + path, headers, nil
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
	for _, model := range result.Data {
		// Use display name as description if available
		description := model.DisplayName
		if description == "" && !model.CreatedAt.IsZero() {
			// Convert created timestamp to JST and use as description
			jst := time.FixedZone("Asia/Tokyo", 9*60*60)
			createdTime := model.CreatedAt.In(jst)
			description = fmt.Sprintf("Created: %s", createdTime.Format("2006-01-02 15:04:05 JST"))
		}

		models = append(models, llmc.ModelInfo{
			ID:          model.ID,
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

// Chat sends a single message to Anthropic's Messages API and returns the response
func (p *Provider) Chat(ctx context.Context, message string) (string, error) {
	return p.ChatWithHistory(ctx, "", nil, message)
}

// ChatWithHistory sends a conversation history with a new message to Anthropic's Messages API
func (p *Provider) ChatWithHistory(ctx context.Context, systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Check if web search is enabled (not supported by Anthropic)
	if p.webSearchEnabled {
		return "", fmt.Errorf("web search is not supported by Anthropic provider")
	}

	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Convert messages to MessageInput array and append the new message
	inputMessages := make([]MessageInput, 0, len(messages)+1)
	for _, msg := range messages {
		inputMessages = append(inputMessages, MessageInput{
			Role:    msg.Role,
			Content: []Content{{Type: "text", Text: msg.Content}},
		})
	}
	inputMessages = append(inputMessages, MessageInput{
		Role:    "user",
		Content: []Content{{Type: "text", Text: newMessage}},
	})

	reqBody := MessagesAPIRequest{
		Model:     modelName,
		MaxTokens: 8192, // Default max tokens
		System:    systemPrompt,
		Messages:  inputMessages,
	}

	url, headers, err := p.endpoint("/messages")
	if err != nil {
		return "", err
	}

	var result MessagesAPIResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, headers, reqBody, &result, p.debug)
	if err != nil {
		// Anthropic error bodies carry a structured message; surface it when parseable
		var statusErr *llmc.HTTPStatusError
		if errors.As(err, &statusErr) {
			var errResp MessagesAPIResponse
			if json.Unmarshal(statusErr.Body, &errResp) == nil && errResp.Error != nil {
				if p.debug {
					return "", fmt.Errorf("API error [%s]: %s (HTTP %d)",
						errResp.Error.Type, errResp.Error.Message, statusErr.StatusCode)
				}
				return "", fmt.Errorf("API error: %s", errResp.Error.Message)
			}
		}
		return "", err
	}

	return p.extractText(&result, body)
}

// extractText pulls the text blocks out of a Messages API result. body is the
// raw response, included in debug error output.
func (p *Provider) extractText(result *MessagesAPIResponse, body []byte) (string, error) {
	if result.Error != nil {
		if p.debug {
			return "", fmt.Errorf("API error [%s]: %s (id=%s)",
				result.Error.Type, result.Error.Message, result.ID)
		}
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		if p.debug {
			return "", fmt.Errorf("API returned empty response (id=%s)\nRaw response: %s",
				result.ID, string(body))
		}
		return "", fmt.Errorf("API returned empty response. Use --verbose for details")
	}

	// Extract text from content blocks
	var textBlocks []string
	for _, content := range result.Content {
		if content.Type == "text" && content.Text != "" {
			textBlocks = append(textBlocks, content.Text)
		}
	}

	if len(textBlocks) == 0 {
		if p.debug {
			return "", fmt.Errorf("no text content found in API response (id=%s)\nRaw response: %s",
				result.ID, string(body))
		}
		return "", fmt.Errorf("no text content found in API response. Use --verbose for details")
	}

	return strings.Join(textBlocks, "\n"), nil
}
