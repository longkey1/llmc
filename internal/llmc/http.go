package llmc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpClient is shared by all provider implementations. The timeout bounds
// the entire request including reading the response body, so a hung
// connection can no longer block the CLI forever.
var httpClient = &http.Client{Timeout: 5 * time.Minute}

// HTTPStatusError is returned by DoJSON for non-200 responses. It carries the
// status code and raw body so callers can extract provider-specific error
// messages; the body is included in Error() only in debug mode.
type HTTPStatusError struct {
	StatusCode int
	Body       []byte
	Debug      bool
}

func (e *HTTPStatusError) Error() string {
	if e.Debug {
		return fmt.Sprintf("API request failed (HTTP %d): %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("API request failed (HTTP %d). Use --verbose for details", e.StatusCode)
}

// DoJSON sends an HTTP request with an optional JSON body, applying the given
// headers, and unmarshals a 200 response into out (when non-nil). The raw
// response body is returned alongside so callers can include it in debug
// output. Error detail (response bodies, transport errors) is included in
// error messages only when debug is true, matching the CLI's --verbose
// behavior.
func DoJSON(ctx context.Context, method, url string, headers map[string]string, reqBody, out any, debug bool) ([]byte, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("error marshaling request: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("request canceled: %w", ctx.Err())
		}
		if debug {
			return nil, fmt.Errorf("failed to connect to API: %v", err)
		}
		return nil, fmt.Errorf("failed to connect to API. Use --verbose for details")
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return body, &HTTPStatusError{StatusCode: resp.StatusCode, Body: body, Debug: debug}
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			if debug {
				return body, fmt.Errorf("failed to parse API response: %v\nRaw response: %s", err, string(body))
			}
			return body, fmt.Errorf("failed to parse API response. Use --verbose for details")
		}
	}

	return body, nil
}
