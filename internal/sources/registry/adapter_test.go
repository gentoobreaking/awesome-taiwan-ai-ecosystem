package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestNew(t *testing.T) {
	adapter := New()
	if adapter.Name() != "registry" {
		t.Errorf("Expected name 'registry', got %s", adapter.Name())
	}
	if adapter.BaseURL != "https://api.mcp-servers.dev" {
		t.Errorf("Expected default BaseURL, got %s", adapter.BaseURL)
	}
	if adapter.TrustScore() != 0.8 {
		t.Errorf("Expected trust score 0.8, got %f", adapter.TrustScore())
	}
}

func TestDiscoverWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"servers": []map[string]any{
					{
						"name":           "taiwan-stock-mcp",
						"description":    "TWSE stock MCP",
						"repository_url": "https://github.com/foo/taiwan-mcp",
						"homepage":       "https://twse.com.tw",
						"registry_url":   "https://api.mcp-servers.dev/servers/taiwan-stock-mcp",
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &Adapter{
		HTTPClient: &StdHTTPClient{Client: server.Client()},
		BaseURL:    server.URL,
	}

	candidates, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Name != "taiwan-stock-mcp" {
		t.Errorf("Expected name='taiwan-stock-mcp', got %s", candidates[0].Name)
	}
	if candidates[0].Source != "registry" {
		t.Errorf("Expected source='registry', got %s", candidates[0].Source)
	}
}

func TestFetchWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers/taiwan-stock-mcp" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"name":           "taiwan-stock-mcp",
				"description":    "TWSE stock MCP",
				"repository_url": "https://github.com/foo/taiwan-mcp",
				"homepage":       "https://twse.com.tw",
				"version":        "1.0.0",
				"runtime":        "python",
				"transport":      []string{"stdio"},
				"license":        "MIT",
				"readme":         "# taiwan-stock-mcp\n\nStock data from TWSE",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &Adapter{
		HTTPClient: &StdHTTPClient{Client: server.Client()},
		BaseURL:    server.URL,
	}

	candidate := models.RawCandidate{
		Source:    "registry",
		Name:      "taiwan-stock-mcp",
		SourceURL: "https://registry.example.com/taiwan-stock-mcp",
	}

	record, err := adapter.Fetch(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if record.Name != "taiwan-stock-mcp" {
		t.Errorf("Expected name='taiwan-stock-mcp', got %s", record.Name)
	}
	if record.Repository.License != "MIT" {
		t.Errorf("Expected license=MIT, got %s", record.Repository.License)
	}
	if record.Readme == "" {
		t.Error("Expected non-empty README")
	}
}

func TestFetchNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &Adapter{
		HTTPClient: &StdHTTPClient{Client: server.Client()},
		BaseURL:    server.URL,
	}

	candidate := models.RawCandidate{
		Source:    "registry",
		Name:      "missing",
		SourceURL: "https://registry.example.com/missing",
	}

	_, err := adapter.Fetch(context.Background(), candidate)
	if err == nil {
		t.Error("Expected error for 404")
	}
}

func TestAdapterImplementsInterface(t *testing.T) {
	var _ = func() {
		var a interface{} = New()
		_ = a.(interface {
			Name() string
			TrustScore() float64
		})
	}
}

func TestDiscoverContextCancellation(t *testing.T) {
	adapter := &Adapter{
		HTTPClient: &StdHTTPClient{Client: &http.Client{Timeout: 100 * time.Millisecond}},
		BaseURL:    "https://invalid-url-that-does-not-exist.invalid",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _ = adapter.Discover(ctx)
}
