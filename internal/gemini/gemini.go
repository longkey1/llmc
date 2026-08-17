package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName   = "gemini"
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultModel   = "gemini-2.0-flash"
)

// ModelsAPIResponse represents the response from Gemini's models endpoint
type ModelsAPIResponse struct {
	Models []GeminiModelData `json:"models"`
}

// GeminiModelData represents a single model in the API response
type GeminiModelData struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// GeminiRequest represents the request body for Gemini's generate content API
type GeminiRequest struct {
	Contents          []GeminiContent          `json:"contents"`
	SystemInstruction *GeminiSystemInstruction `json:"system_instruction,omitempty"`
	Tools             []GeminiTool             `json:"tools,omitempty"`
}

// GeminiSystemInstruction represents system instruction for Gemini
type GeminiSystemInstruction struct {
	Parts []GeminiPart `json:"parts"`
}

// GeminiContent represents a content item in the Gemini request format
type GeminiContent struct {
	Role  string       `json:"role,omitempty"` // "user" or "model"
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content in the Gemini request format
type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
}

// GeminiFunctionCall represents a function call requested by the model
type GeminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// GeminiFunctionResponse represents a function execution result sent back to
// the model. Gemini matches responses to calls by function name.
type GeminiFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// GeminiTool represents a tool configuration for Gemini
type GeminiTool struct {
	GoogleSearch         *GeminiGoogleSearch         `json:"google_search,omitempty"`
	FunctionDeclarations []GeminiFunctionDeclaration `json:"function_declarations,omitempty"`
}

// GeminiFunctionDeclaration represents a function tool definition
type GeminiFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// GeminiGoogleSearch represents Google Search grounding configuration
type GeminiGoogleSearch struct {
	// Empty struct as per API specification
}

// GeminiResponse represents the full response from Gemini API
type GeminiResponse struct {
	Candidates        []GeminiCandidate        `json:"candidates"`
	GroundingMetadata *GeminiGroundingMetadata `json:"groundingMetadata,omitempty"`
}

// GeminiCandidate represents a candidate response
type GeminiCandidate struct {
	Content GeminiResponseContent `json:"content"`
}

// GeminiResponseContent represents the content of a response
type GeminiResponseContent struct {
	Parts []GeminiResponsePart `json:"parts"`
}

// GeminiResponsePart represents a part of the response content
type GeminiResponsePart struct {
	Text         string              `json:"text"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
}

// GeminiGroundingMetadata contains grounding information
type GeminiGroundingMetadata struct {
	SearchEntryPoint  *GeminiSearchEntryPoint  `json:"searchEntryPoint,omitempty"`
	WebSearchQueries  []string                 `json:"webSearchQueries,omitempty"`
	GroundingChunks   []GeminiGroundingChunk   `json:"groundingChunks,omitempty"`
	GroundingSupports []GeminiGroundingSupport `json:"groundingSupports,omitempty"`
}

// GeminiSearchEntryPoint contains search entry point information
type GeminiSearchEntryPoint struct {
	RenderedContent string `json:"renderedContent,omitempty"`
}

// GeminiGroundingChunk represents a grounding source
type GeminiGroundingChunk struct {
	Web *GeminiWebChunk `json:"web,omitempty"`
}

// GeminiWebChunk contains web source information
type GeminiWebChunk struct {
	URI   string `json:"uri"`
	Title string `json:"title,omitempty"`
}

// GeminiGroundingSupport represents grounding support information
type GeminiGroundingSupport struct {
	Segment               *GeminiSegment `json:"segment,omitempty"`
	GroundingChunkIndices []int          `json:"groundingChunkIndices,omitempty"`
}

// GeminiSegment represents a text segment
type GeminiSegment struct {
	StartIndex int    `json:"startIndex,omitempty"`
	EndIndex   int    `json:"endIndex,omitempty"`
	Text       string `json:"text,omitempty"`
}

// Config defines the configuration interface for Gemini provider
type Config interface {
	GetModel() string
	GetBaseURL(provider string) (string, error)
	GetToken(provider string) (string, error)
}

// Provider implements the llmc.Provider interface for Gemini
type Provider struct {
	config           Config
	webSearchEnabled bool
	debug            bool
}

// NewProvider creates a new Gemini provider instance
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

// SetIgnoreWebSearchErrors is kept for interface compatibility (no-op for Gemini)
func (p *Provider) SetIgnoreWebSearchErrors(enabled bool) {
	// No-op: auto-retry feature has been removed
}

// SetDebug enables or disables debug mode
func (p *Provider) SetDebug(enabled bool) {
	p.debug = enabled
}

// endpoint resolves the token and base URL and returns the full URL for path.
// Gemini authenticates via the key query parameter instead of a header.
func (p *Provider) endpoint(path string) (string, error) {
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %w", err)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return baseURL + path + "?key=" + token, nil
}

// ListModels returns the list of supported models from the API
func (p *Provider) ListModels(ctx context.Context) ([]llmc.ModelInfo, error) {
	url, err := p.endpoint("/models")
	if err != nil {
		return nil, err
	}

	var result ModelsAPIResponse
	if _, err := llmc.DoJSON(ctx, http.MethodGet, url, nil, nil, &result, p.debug); err != nil {
		return nil, err
	}

	models := make([]llmc.ModelInfo, 0, len(result.Models))
	for _, model := range result.Models {
		// Only include models that support generateContent
		if !slices.Contains(model.SupportedGenerationMethods, "generateContent") {
			continue
		}

		// Use API-provided description or displayName
		description := model.Description
		if description == "" {
			description = model.DisplayName
		}

		models = append(models, llmc.ModelInfo{
			ID:          strings.TrimPrefix(model.Name, "models/"),
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

// Chat sends a single message to Gemini's API and returns the response
func (p *Provider) Chat(ctx context.Context, message string) (string, error) {
	return p.ChatWithHistory(ctx, "", nil, message)
}

// ChatWithHistory sends a conversation history with a new message to Gemini's API
func (p *Provider) ChatWithHistory(ctx context.Context, systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Convert messages to GeminiContent array and append the new message
	contents := make([]GeminiContent, 0, len(messages)+1)
	for _, msg := range messages {
		role := msg.Role
		// Gemini uses "model" instead of "assistant"
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}
	contents = append(contents, GeminiContent{
		Role:  "user",
		Parts: []GeminiPart{{Text: newMessage}},
	})

	reqBody := GeminiRequest{
		Contents: contents,
	}
	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}
	if p.webSearchEnabled {
		reqBody.Tools = []GeminiTool{
			{GoogleSearch: &GeminiGoogleSearch{}},
		}
	}

	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	url, err := p.endpoint(fmt.Sprintf("/models/%s:generateContent", modelName))
	if err != nil {
		return "", err
	}

	var result GeminiResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, nil, reqBody, &result, p.debug)
	if err != nil {
		return "", err
	}

	if p.debug {
		fmt.Fprintf(os.Stderr, "Raw API response: %s\n", string(body))
	}

	return p.extractText(&result, body)
}

// ChatWithTools sends the conversation history with function tools available
// and returns the model's text and/or requested tool calls.
func (p *Provider) ChatWithTools(ctx context.Context, systemPrompt string, messages []llmc.Message, tools []llmc.ToolDef) (*llmc.TurnResult, error) {
	reqBody := GeminiRequest{
		Contents:          buildContents(messages),
		SystemInstruction: nil,
		Tools:             p.buildTools(tools),
	}
	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return nil, fmt.Errorf("invalid model format: %w", err)
	}

	url, err := p.endpoint(fmt.Sprintf("/models/%s:generateContent", modelName))
	if err != nil {
		return nil, err
	}

	var result GeminiResponse
	body, err := llmc.DoJSON(ctx, http.MethodPost, url, nil, reqBody, &result, p.debug)
	if err != nil {
		return nil, err
	}

	if p.debug {
		fmt.Fprintf(os.Stderr, "Raw API response: %s\n", string(body))
	}

	return p.extractTurn(&result, body)
}

// buildContents converts neutral history messages into Gemini contents,
// expanding assistant tool calls into functionCall parts and tool results
// into user functionResponse parts.
func buildContents(messages []llmc.Message) []GeminiContent {
	contents := make([]GeminiContent, 0, len(messages))
	for _, msg := range messages {
		switch {
		case msg.Role == "tool":
			responseKey := "output"
			if msg.ToolIsError {
				responseKey = "error"
			}
			contents = append(contents, GeminiContent{
				Role: "user",
				Parts: []GeminiPart{{
					FunctionResponse: &GeminiFunctionResponse{
						Name:     msg.ToolName,
						Response: map[string]any{responseKey: msg.Content},
					},
				}},
			})
		case len(msg.ToolCalls) > 0:
			var parts []GeminiPart
			if msg.Content != "" {
				parts = append(parts, GeminiPart{Text: msg.Content})
			}
			for _, call := range msg.ToolCalls {
				args := json.RawMessage(call.Arguments)
				if call.Arguments == "" {
					args = json.RawMessage("{}")
				}
				parts = append(parts, GeminiPart{
					FunctionCall: &GeminiFunctionCall{Name: call.Name, Args: args},
				})
			}
			contents = append(contents, GeminiContent{Role: "model", Parts: parts})
		default:
			role := msg.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, GeminiContent{
				Role:  role,
				Parts: []GeminiPart{{Text: msg.Content}},
			})
		}
	}
	return contents
}

// buildTools assembles the tools array, combining Google Search grounding
// (when enabled) with the given function declarations. Some models reject
// the combination; the API error is surfaced as-is.
func (p *Provider) buildTools(tools []llmc.ToolDef) []GeminiTool {
	var out []GeminiTool
	if p.webSearchEnabled {
		out = append(out, GeminiTool{GoogleSearch: &GeminiGoogleSearch{}})
	}
	if len(tools) > 0 {
		declarations := make([]GeminiFunctionDeclaration, 0, len(tools))
		for _, tool := range tools {
			declarations = append(declarations, GeminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			})
		}
		out = append(out, GeminiTool{FunctionDeclarations: declarations})
	}
	return out
}

// extractTurn pulls text and function calls out of a Gemini result. Gemini
// doesn't issue call IDs, so sequential IDs are synthesized; responses are
// matched by function name on the way back.
func (p *Provider) extractTurn(result *GeminiResponse, body []byte) (*llmc.TurnResult, error) {
	if len(result.Candidates) == 0 {
		if p.debug {
			return nil, fmt.Errorf("no response from API (empty candidates)\nRaw response: %s", string(body))
		}
		return nil, fmt.Errorf("no response from API")
	}

	turn := &llmc.TurnResult{Text: "", ToolCalls: nil}
	var textParts []string
	for _, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			turn.ToolCalls = append(turn.ToolCalls, llmc.ToolCall{
				ID:        fmt.Sprintf("call-%d", len(turn.ToolCalls)),
				Name:      part.FunctionCall.Name,
				Arguments: string(part.FunctionCall.Args),
			})
			continue
		}
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}
	turn.Text = strings.Join(textParts, "\n")

	if turn.Text == "" && len(turn.ToolCalls) == 0 {
		if p.debug {
			return nil, fmt.Errorf("no text or function calls in API response\nRaw response: %s", string(body))
		}
		return nil, fmt.Errorf("no text or function calls in API response. Use --verbose for details")
	}

	if turn.Text != "" && result.GroundingMetadata != nil && len(result.GroundingMetadata.GroundingChunks) > 0 {
		if citations := extractGroundingCitations(result.GroundingMetadata); citations != "" {
			turn.Text += "\n\n---\nSources:\n" + citations
		}
	}

	return turn, nil
}

// extractText pulls the response text (and citations) out of a Gemini result.
// body is the raw response, included in debug error output.
func (p *Provider) extractText(result *GeminiResponse, body []byte) (string, error) {
	if len(result.Candidates) == 0 {
		if p.debug {
			return "", fmt.Errorf("no response from API (empty candidates)\nRaw response: %s", string(body))
		}
		return "", fmt.Errorf("no response from API")
	}

	var responseText string
	if len(result.Candidates[0].Content.Parts) > 0 {
		responseText = result.Candidates[0].Content.Parts[0].Text
	}

	if responseText == "" {
		// Known Gemini API issue: web search can return grounding metadata
		// without any text parts
		if result.GroundingMetadata != nil && p.webSearchEnabled {
			return "", fmt.Errorf("web search returned empty response. Try again without --web-search flag")
		}
		if p.debug {
			return "", fmt.Errorf("no response from API (empty parts)\nRaw response: %s", string(body))
		}
		return "", fmt.Errorf("no response from API")
	}

	// Format citations if grounding metadata is present
	if result.GroundingMetadata != nil && len(result.GroundingMetadata.GroundingChunks) > 0 {
		if citations := extractGroundingCitations(result.GroundingMetadata); citations != "" {
			responseText += "\n\n---\nSources:\n" + citations
		}
	}

	return responseText, nil
}

// extractGroundingCitations formats grounding chunks into a citation list
func extractGroundingCitations(metadata *GeminiGroundingMetadata) string {
	var citations []string
	seenURIs := make(map[string]bool)

	for i, chunk := range metadata.GroundingChunks {
		if chunk.Web != nil && chunk.Web.URI != "" {
			// Skip duplicate URIs
			if seenURIs[chunk.Web.URI] {
				continue
			}
			seenURIs[chunk.Web.URI] = true

			title := chunk.Web.Title
			if title == "" {
				title = "Source"
			}
			citations = append(citations, fmt.Sprintf("[%d] %s - %s", i+1, title, chunk.Web.URI))
		}
	}

	return strings.Join(citations, "\n")
}
