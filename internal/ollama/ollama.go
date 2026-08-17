package ollama

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName   = "ollama"
	DefaultBaseURL = "http://localhost:11434/v1"
)

// ModelsAPIResponse represents the response from Ollama's OpenAI-compatible
// models endpoint
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

// ChatCompletionsRequest represents the request body for Ollama's
// OpenAI-compatible Chat Completions API
type ChatCompletionsRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []ChatTool    `json:"tools,omitempty"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role       string         `json:"role"`    // "system", "user", "assistant" or "tool"
	Content    string         `json:"content"` // Message content
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"` // role "tool" only
}

// ChatTool represents a function tool definition in the standard Chat
// Completions format
type ChatTool struct {
	Type     string           `json:"type"` // "function"
	Function ChatToolFunction `json:"function"`
}

// ChatToolFunction describes the function inside a ChatTool
type ChatToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ChatToolCall represents a tool call in an assistant message
type ChatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ChatCallFunction `json:"function"`
}

// ChatCallFunction carries the function name and raw JSON arguments
type ChatCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionsResponse represents the response from the Chat Completions API
type ChatCompletionsResponse struct {
	ID      string       `json:"id"`
	Choices []ChatChoice `json:"choices"`
}

// ChatChoice represents a single choice in the response
type ChatChoice struct {
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Config defines the configuration interface for Ollama provider
type Config interface {
	GetModel() string
	GetBaseURL(provider string) (string, error)
	GetToken(provider string) (string, error)
}

// Provider implements the llmc.Provider interface for Ollama
type Provider struct {
	config Config
	debug  bool
}

// NewProvider creates a new Ollama provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config: config,
		debug:  false,
	}
}

// SetWebSearch is a no-op for Ollama (web search is not supported)
func (p *Provider) SetWebSearch(enabled bool) {
	// Not applicable for Ollama
}

// SetIgnoreWebSearchErrors is a no-op for Ollama (not applicable)
func (p *Provider) SetIgnoreWebSearchErrors(enabled bool) {
	// Not applicable for Ollama
}

// SetDebug enables or disables debug mode
func (p *Provider) SetDebug(enabled bool) {
	p.debug = enabled
}

// endpoint resolves the base URL and returns the full URL for path along with
// the request headers. The token is optional: a local Ollama server requires
// no authentication, so the Authorization header is sent only when a token is
// configured (e.g., for a remote server behind an authenticating proxy).
func (p *Provider) endpoint(path string) (string, map[string]string, error) {
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get base URL: %w", err)
	}

	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get token: %w", err)
	}
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}

	return baseURL + path, headers, nil
}

// ListModels returns the list of locally installed models from the API
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
		// Ollama reports the model's local modification time as "created"
		createdTime := time.Unix(model.Created, 0).In(jst)
		models = append(models, llmc.ModelInfo{
			ID:          model.ID,
			Description: fmt.Sprintf("Modified: %s", createdTime.Format("2006-01-02 15:04:05 JST")),
			IsDefault:   false, // Set by caller
		})
	}

	// Sort models by ID (descending order)
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID > models[j].ID
	})

	return models, nil
}

// Chat sends a single message to Ollama's Chat Completions API and returns the response
func (p *Provider) Chat(ctx context.Context, message string) (string, error) {
	return p.ChatWithHistory(ctx, "", nil, message)
}

// ChatWithHistory sends a conversation history with a new message to Ollama's
// Chat Completions API
func (p *Provider) ChatWithHistory(ctx context.Context, systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Build the message array: system prompt first, then history, then the new message
	chatMessages := make([]ChatMessage, 0, len(messages)+2)
	if systemPrompt != "" {
		chatMessages = append(chatMessages, ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	for _, msg := range messages {
		chatMessages = append(chatMessages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	chatMessages = append(chatMessages, ChatMessage{
		Role:    "user",
		Content: newMessage,
	})

	reqBody := ChatCompletionsRequest{
		Model:    modelName,
		Messages: chatMessages,
	}

	url, headers, err := p.endpoint("/chat/completions")
	if err != nil {
		return "", err
	}

	var result ChatCompletionsResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, headers, reqBody, &result, p.debug)
	if err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		if p.debug {
			return "", fmt.Errorf("API returned empty response (id=%s)\nRaw response: %s",
				result.ID, string(body))
		}
		return "", fmt.Errorf("API returned empty response. Use --verbose for details")
	}

	return result.Choices[0].Message.Content, nil
}

// ChatWithTools sends the conversation history with function tools available
// and returns the model's text and/or requested tool calls.
func (p *Provider) ChatWithTools(ctx context.Context, systemPrompt string, messages []llmc.Message, tools []llmc.ToolDef) (*llmc.TurnResult, error) {
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return nil, fmt.Errorf("invalid model format: %w", err)
	}

	chatTools := make([]ChatTool, 0, len(tools))
	for _, tool := range tools {
		chatTools = append(chatTools, ChatTool{
			Type: "function",
			Function: ChatToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	reqBody := ChatCompletionsRequest{
		Model:    modelName,
		Messages: buildChatMessages(systemPrompt, messages),
		Tools:    chatTools,
	}

	url, headers, err := p.endpoint("/chat/completions")
	if err != nil {
		return nil, err
	}

	var result ChatCompletionsResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, headers, reqBody, &result, p.debug)
	if err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		if p.debug {
			return nil, fmt.Errorf("API returned empty response (id=%s)\nRaw response: %s",
				result.ID, string(body))
		}
		return nil, fmt.Errorf("API returned empty response. Use --verbose for details")
	}

	message := result.Choices[0].Message
	turn := &llmc.TurnResult{Text: message.Content, ToolCalls: nil}
	for _, call := range message.ToolCalls {
		turn.ToolCalls = append(turn.ToolCalls, llmc.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: call.Function.Arguments,
		})
	}
	return turn, nil
}

// buildChatMessages converts neutral history messages into Chat Completions
// messages, prepending the system prompt as the first message.
func buildChatMessages(systemPrompt string, messages []llmc.Message) []ChatMessage {
	chatMessages := make([]ChatMessage, 0, len(messages)+1)
	if systemPrompt != "" {
		chatMessages = append(chatMessages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	for _, msg := range messages {
		chatMsg := ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "tool" {
			chatMsg.ToolCallID = msg.ToolCallID
		}
		for _, call := range msg.ToolCalls {
			chatMsg.ToolCalls = append(chatMsg.ToolCalls, ChatToolCall{
				ID:   call.ID,
				Type: "function",
				Function: ChatCallFunction{
					Name:      call.Name,
					Arguments: call.Arguments,
				},
			})
		}
		chatMessages = append(chatMessages, chatMsg)
	}
	return chatMessages
}
