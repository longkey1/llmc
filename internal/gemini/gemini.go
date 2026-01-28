package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName   = "gemini"
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultModel   = "gemini-2.0-flash"
)

// Supported models for Gemini (fallback list)
var supportedModels = []llmc.ModelInfo{
	{ID: "gemini-2.0-flash", Description: "Fast and efficient Gemini 2.0", IsDefault: true},
	{ID: "gemini-2.0-pro", Description: "Advanced Gemini 2.0 for complex tasks", IsDefault: false},
	{ID: "gemini-1.5-pro", Description: "Previous generation pro model", IsDefault: false},
	{ID: "gemini-1.5-flash", Description: "Previous generation flash model", IsDefault: false},
}

// ModelsAPIResponse represents the response from Gemini's models endpoint
type ModelsAPIResponse struct {
	Models []GeminiModelData `json:"models"`
}

// GeminiModelData represents a single model in the API response
type GeminiModelData struct {
	Name                      string   `json:"name"`
	DisplayName               string   `json:"displayName"`
	Description               string   `json:"description"`
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
	Text string `json:"text"`
}

// GeminiTool represents a tool configuration for Gemini
type GeminiTool struct {
	GoogleSearch *GeminiGoogleSearch `json:"google_search,omitempty"`
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
	Text string `json:"text"`
}

// GeminiGroundingMetadata contains grounding information
type GeminiGroundingMetadata struct {
	SearchEntryPoint   *GeminiSearchEntryPoint `json:"searchEntryPoint,omitempty"`
	WebSearchQueries   []string                `json:"webSearchQueries,omitempty"`
	GroundingChunks    []GeminiGroundingChunk  `json:"groundingChunks,omitempty"`
	GroundingSupports  []GeminiGroundingSupport `json:"groundingSupports,omitempty"`
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
	Segment        *GeminiSegment `json:"segment,omitempty"`
	GroundingChunkIndices []int   `json:"groundingChunkIndices,omitempty"`
}

// GeminiSegment represents a text segment
type GeminiSegment struct {
	StartIndex int `json:"startIndex,omitempty"`
	EndIndex   int `json:"endIndex,omitempty"`
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
	config                 Config
	webSearchEnabled       bool
	ignoreWebSearchErrors  bool
	debug                  bool
}

// NewProvider creates a new Gemini provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config:                config,
		webSearchEnabled:      false,
		ignoreWebSearchErrors: false,
		debug:                 false,
	}
}

// SetWebSearch enables or disables web search
func (p *Provider) SetWebSearch(enabled bool) {
	p.webSearchEnabled = enabled
}

// SetIgnoreWebSearchErrors enables or disables ignoring web search errors (auto-retry without web search)
func (p *Provider) SetIgnoreWebSearchErrors(enabled bool) {
	p.ignoreWebSearchErrors = enabled
}

// SetDebug enables or disables debug mode
func (p *Provider) SetDebug(enabled bool) {
	p.debug = enabled
}

// ListModels returns the list of supported models from the API
func (p *Provider) ListModels() ([]llmc.ModelInfo, error) {
	// Get token for Gemini
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for Gemini
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return nil, fmt.Errorf("failed to get base URL: %w", err)
	}

	// Build URL with API key
	url := baseURL + "/models?key=" + token

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

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

	for _, model := range result.Models {
		// Extract model ID from name (remove "models/" prefix)
		id := strings.TrimPrefix(model.Name, "models/")

		// Only include models that support generateContent
		if !contains(model.SupportedGenerationMethods, "generateContent") {
			continue
		}

		// Use API-provided description or displayName
		description := model.Description
		if description == "" {
			description = model.DisplayName
		}
		// If no description available, leave empty

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

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Chat sends a message to Gemini's API and returns the response
func (p *Provider) Chat(message string) (string, error) {
	response, retry, err := p.sendRequest(message, p.webSearchEnabled)

	// If web search was enabled but returned empty response
	if retry && p.webSearchEnabled {
		// If ignoreWebSearchErrors is enabled, retry without web search
		if p.ignoreWebSearchErrors {
			if p.debug {
				fmt.Fprintf(os.Stderr, "Web search returned empty response, retrying without web search...\n")
			}
			response, _, err = p.sendRequest(message, false)
		} else {
			// Otherwise, return an error
			return "", fmt.Errorf("web search did not return a text response (known Gemini API issue). Use --ignore-web-search-errors to automatically retry without web search")
		}
	}

	return response, err
}

// sendRequest sends a request to Gemini's API and returns the response
// Returns: (response text, should retry without web search, error)
func (p *Provider) sendRequest(message string, enableWebSearch bool) (string, bool, error) {
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

	// Add Google Search tool if enabled
	if enableWebSearch {
		reqBody.Tools = []GeminiTool{
			{
				GoogleSearch: &GeminiGoogleSearch{},
			},
		}
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", false, fmt.Errorf("error marshaling request: %v", err)
	}

	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", false, fmt.Errorf("invalid model format: %w", err)
	}

	// Get token for Gemini
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", false, fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for Gemini
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", false, fmt.Errorf("failed to get base URL: %w", err)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, modelName, token)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", false, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("error reading response: %v", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		if p.debug {
			return "", false, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", false, fmt.Errorf("API error: %s", string(body))
	}

	// Debug: print raw response
	if p.debug {
		fmt.Fprintf(os.Stderr, "Raw API response: %s\n", string(body))
	}

	// Parse response
	var result GeminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if p.debug {
			return "", false, fmt.Errorf("error parsing response: %v\nRaw response: %s", err, string(body))
		}
		return "", false, fmt.Errorf("error parsing response: %v", err)
	}

	// Debug: print parsed response structure
	if p.debug {
		fmt.Fprintf(os.Stderr, "Candidates count: %d\n", len(result.Candidates))
		if len(result.Candidates) > 0 {
			fmt.Fprintf(os.Stderr, "Parts count in first candidate: %d\n", len(result.Candidates[0].Content.Parts))
		}
	}

	if len(result.Candidates) == 0 {
		if p.debug {
			return "", false, fmt.Errorf("no response from API (empty candidates)\nRaw response: %s", string(body))
		}
		return "", false, fmt.Errorf("no response from API")
	}

	var responseText string
	shouldRetry := false

	// Check if there's text content in parts
	if len(result.Candidates[0].Content.Parts) > 0 {
		responseText = result.Candidates[0].Content.Parts[0].Text
	}

	// If no text content but grounding metadata exists, mark for retry
	if responseText == "" && result.GroundingMetadata != nil && enableWebSearch {
		shouldRetry = true
		if p.debug {
			fmt.Fprintf(os.Stderr, "Empty response with grounding metadata detected (known Gemini API issue)\n")
		}
	}

	// If still no content and shouldn't retry, return error
	if responseText == "" && !shouldRetry {
		if p.debug {
			return "", false, fmt.Errorf("no response from API (empty parts)\nRaw response: %s", string(body))
		}
		return "", false, fmt.Errorf("no response from API")
	}

	// Format citations if grounding metadata is present
	if responseText != "" && result.GroundingMetadata != nil && len(result.GroundingMetadata.GroundingChunks) > 0 {
		citations := extractGroundingCitations(result.GroundingMetadata)
		if citations != "" {
			responseText += "\n\n---\nSources:\n" + citations
		}
	}

	return responseText, shouldRetry, nil
}

// ChatWithHistory sends a conversation history with a new message to Gemini's API
func (p *Provider) ChatWithHistory(systemPrompt string, messages []llmc.Message, newMessage string) (string, error) {
	// Convert messages to GeminiContent array
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

	// Add new user message
	contents = append(contents, GeminiContent{
		Role:  "user",
		Parts: []GeminiPart{{Text: newMessage}},
	})

	// Prepare the request body
	reqBody := GeminiRequest{
		Contents: contents,
	}

	// Add system instruction if provided
	if systemPrompt != "" {
		reqBody.SystemInstruction = &GeminiSystemInstruction{
			Parts: []GeminiPart{{Text: systemPrompt}},
		}
	}

	// Add Google Search tool if enabled
	if p.webSearchEnabled {
		reqBody.Tools = []GeminiTool{
			{
				GoogleSearch: &GeminiGoogleSearch{},
			},
		}
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	// Extract model name from provider:model format
	_, modelName, err := llmc.ParseModelString(p.config.GetModel())
	if err != nil {
		return "", fmt.Errorf("invalid model format: %w", err)
	}

	// Get token for Gemini
	token, err := p.config.GetToken(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Get base URL for Gemini
	baseURL, err := p.config.GetBaseURL(ProviderName)
	if err != nil {
		return "", fmt.Errorf("failed to get base URL: %w", err)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, modelName, token)
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
		if p.debug {
			return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("API error: %s", string(body))
	}

	// Debug: print raw response
	if p.debug {
		fmt.Fprintf(os.Stderr, "Raw API response: %s\n", string(body))
	}

	// Parse response
	var result GeminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if p.debug {
			return "", fmt.Errorf("error parsing response: %v\nRaw response: %s", err, string(body))
		}
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	// Debug: print parsed response structure
	if p.debug {
		fmt.Fprintf(os.Stderr, "Candidates count: %d\n", len(result.Candidates))
		if len(result.Candidates) > 0 {
			fmt.Fprintf(os.Stderr, "Parts count in first candidate: %d\n", len(result.Candidates[0].Content.Parts))
		}
	}

	if len(result.Candidates) == 0 {
		if p.debug {
			return "", fmt.Errorf("no response from API (empty candidates)\nRaw response: %s", string(body))
		}
		return "", fmt.Errorf("no response from API")
	}

	var responseText string
	shouldRetry := false

	// Check if there's text content in parts
	if len(result.Candidates[0].Content.Parts) > 0 {
		responseText = result.Candidates[0].Content.Parts[0].Text
	}

	// If no text content but grounding metadata exists, mark for retry
	if responseText == "" && result.GroundingMetadata != nil && p.webSearchEnabled {
		shouldRetry = true
		if p.debug {
			fmt.Fprintf(os.Stderr, "Empty response with grounding metadata detected (known Gemini API issue)\n")
		}
	}

	// If still no content and should retry, retry without web search
	if shouldRetry && p.ignoreWebSearchErrors {
		if p.debug {
			fmt.Fprintf(os.Stderr, "Web search returned empty response, retrying without web search...\n")
		}
		// Recursive call without web search (temporarily disable it)
		originalWebSearch := p.webSearchEnabled
		p.webSearchEnabled = false
		response, err := p.ChatWithHistory(systemPrompt, messages, newMessage)
		p.webSearchEnabled = originalWebSearch
		return response, err
	}

	// If still no content and shouldn't retry, return error
	if responseText == "" {
		if shouldRetry {
			return "", fmt.Errorf("web search did not return a text response (known Gemini API issue). Use --ignore-web-search-errors to automatically retry without web search")
		}
		if p.debug {
			return "", fmt.Errorf("no response from API (empty parts)\nRaw response: %s", string(body))
		}
		return "", fmt.Errorf("no response from API")
	}

	// Format citations if grounding metadata is present
	if result.GroundingMetadata != nil && len(result.GroundingMetadata.GroundingChunks) > 0 {
		citations := extractGroundingCitations(result.GroundingMetadata)
		if citations != "" {
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
