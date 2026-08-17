package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunFetchURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello from server"))
	}))
	defer server.Close()

	got, err := runFetchURL(context.Background(), `{"url":"`+server.URL+`"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello from server" {
		t.Errorf("got %q, want body", got)
	}
}

func TestRunFetchURLTruncates(t *testing.T) {
	big := strings.Repeat("a", fetchURLMaxSize+100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(big))
	}))
	defer server.Close()

	got, err := runFetchURL(context.Background(), `{"url":"`+server.URL+`"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, "(truncated at 256KB)") {
		t.Errorf("expected truncation note, got tail %q", got[len(got)-40:])
	}
}

func TestRunFetchURLBinaryContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0x00, 0x01, 0x02})
	}))
	defer server.Close()

	got, err := runFetchURL(context.Background(), `{"url":"`+server.URL+`"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "binary content") {
		t.Errorf("got %q, want binary content summary", got)
	}
}

func TestRunFetchURLNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	got, err := runFetchURL(context.Background(), `{"url":"`+server.URL+`"}`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "HTTP 404") {
		t.Errorf("got %q, want HTTP 404 prefix", got)
	}
}

func TestRunFetchURLRejectsScheme(t *testing.T) {
	_, err := runFetchURL(context.Background(), `{"url":"file:///etc/passwd"}`, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported URL scheme") {
		t.Errorf("err = %v, want unsupported scheme error", err)
	}
}

func TestRunFetchURLContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runFetchURL(ctx, `{"url":"`+server.URL+`"}`, nil)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestIsTextContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		{"", true},
		{"text/html; charset=utf-8", true},
		{"application/json", true},
		{"application/ld+json", true},
		{"application/octet-stream", false},
		{"image/png", false},
	}
	for _, tt := range tests {
		if got := isTextContentType(tt.contentType); got != tt.want {
			t.Errorf("isTextContentType(%q) = %v, want %v", tt.contentType, got, tt.want)
		}
	}
}
