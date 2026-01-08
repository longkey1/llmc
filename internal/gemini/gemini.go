package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	ProviderName   = "gemini"
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultModel   = "gemini-2.0-flash"
)

// Supported models for Gemini
var supportedModels = []llmc.ModelInfo{
	{ID: "gemini-2.0-flash", Description: "Fast and efficient Gemini 2.0", IsDefault: true},
	{ID: "gemini-2.0-pro", Description: "Advanced Gemini 2.0 for complex tasks", IsDefault: false},
	{ID: "gemini-1.5-pro", Description: "Previous generation pro model", IsDefault: false},
	{ID: "gemini-1.5-flash", Description: "Previous generation flash model", IsDefault: false},
}

// GeminiRequest represents the request body for Gemini's generate content API
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
	Tools    []GeminiTool    `json:"tools,omitempty"`
}

// GeminiContent represents a content item in the Gemini request format
type GeminiContent struct {
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
	WebSearchQueries   []string                `json:"webSearchQueries,omitempty"`
	GroundingChunks    []GeminiGroundingChunk  `json:"groundingChunks,omitempty"`
	GroundingSupports  []GeminiGroundingSupport `json:"groundingSupports,omitempty"`
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
	GetBaseURL() string
	GetToken() string
}

// Provider implements the llmc.Provider interface for Gemini
type Provider struct {
	config           Config
	webSearchEnabled bool
}

// NewProvider creates a new Gemini provider instance
func NewProvider(config Config) *Provider {
	return &Provider{
		config:           config,
		webSearchEnabled: false,
	}
}

// SetWebSearch enables or disables web search
func (p *Provider) SetWebSearch(enabled bool) {
	p.webSearchEnabled = enabled
}

// ListModels returns the list of supported models
func (p *Provider) ListModels() []llmc.ModelInfo {
	return supportedModels
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
	var result GeminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	responseText := result.Candidates[0].Content.Parts[0].Text

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
