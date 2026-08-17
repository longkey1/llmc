package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/longkey1/llmc/internal/llmc"
)

const (
	fetchURLTimeout = 30 * time.Second
	fetchURLMaxSize = 256 * 1024
)

var fetchURLClient = &http.Client{Timeout: fetchURLTimeout}

type fetchURLArgs struct {
	URL string `json:"url"`
}

func fetchURLDef() llmc.ToolDef {
	return llmc.ToolDef{
		Name:        NameFetchURL,
		Description: "Fetch the content of a URL via HTTP GET. Returns the response body as text (truncated if large). Only http and https URLs are supported.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch (http or https)",
				},
			},
			"required": []string{"url"},
		},
		RequiresConfirmation: false,
	}
}

func runFetchURL(ctx context.Context, arguments string, _ *Options) (string, error) {
	var args fetchURLArgs
	if err := unmarshalArgs(arguments, &args); err != nil {
		return "", err
	}

	parsed, err := url.Parse(args.URL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %v", err)
	}

	resp, err := fetchURLClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching URL: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the limit so truncation is detectable.
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchURLMaxSize+1))
	if err != nil {
		return "", fmt.Errorf("reading response body: %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isTextContentType(contentType) {
		return fmt.Sprintf("binary content (%s, %d bytes)", contentType, len(body)), nil
	}

	result := string(body)
	if len(body) > fetchURLMaxSize {
		result = result[:fetchURLMaxSize] + "\n... (truncated at 256KB)"
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("HTTP %d\n\n%s", resp.StatusCode, result), nil
	}
	return result, nil
}

// isTextContentType reports whether a Content-Type is likely to be readable
// text. An empty Content-Type is treated as text.
func isTextContentType(contentType string) bool {
	if contentType == "" {
		return true
	}
	mediaType := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/xml", "application/javascript",
		"application/x-yaml", "application/yaml", "application/toml",
		"application/xhtml+xml", "application/rss+xml", "application/atom+xml":
		return true
	}
	return strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
}
