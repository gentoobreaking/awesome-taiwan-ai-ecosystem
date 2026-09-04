package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/retry"
)

func TestNew(t *testing.T) {
	adapter := New()
	if adapter.Name() != "official-registry" {
		t.Errorf("Expected name 'official-registry', got %s", adapter.Name())
	}
	if adapter.BaseURL != "https://api.mcp-servers.dev" {
		t.Errorf("Expected default BaseURL, got %s", adapter.BaseURL)
	}
}

func TestDiscoverWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"servers": [
				{"name": "taiwan-stock-mcp", "description": "TWSE stock MCP", "repository_url": "https://github.com/foo/taiwan-mcp", "homepage": "https://twse.com.tw", "registry_url": "https://github.com/foo/taiwan-mcp"}
			]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &OfficialRegistryAdapter{
		HTTPClient: retry.NewClient(retry.Config{
			MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond,
		}).WithHTTPClient(server.Client()),
		BaseURL: server.URL,
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
	if candidates[0].Source != "official-registry" {
		t.Errorf("Expected source='official-registry', got %s", candidates[0].Source)
	}
}

func TestFetchWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/servers/taiwan-stock-mcp" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name": "taiwan-stock-mcp", "description": "TWSE stock MCP", "repository_url": "https://github.com/foo/taiwan-mcp", "homepage": "https://twse.com.tw", "version": "1.0.0", "runtime": "python", "transport": "stdio", "license": "MIT", "readme": "# taiwan-stock-mcp\n\nStock data from TWSE"}))`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &OfficialRegistryAdapter{
		HTTPClient: retry.NewClient(retry.Config{
			MaxRetries: 3, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond,
		}).WithHTTPClient(server.Client()),
		BaseURL: server.URL,
	}

	candidate := models.RawCandidate{
		Source:    "official-registry",
		Name:      "taiwan-stock-mcp",
		SourceURL: "https://registry.example.com/taiwan-stock-mcp",
	}

	record, err := adapter.Fetch(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if record.Candidate.Name != "taiwan-stock-mcp" {
		t.Errorf("Expected name='taiwan-stock-mcp', got %s", record.Candidate.Name)
	}
	if v, ok := record.Candidate.RawMetadata["license"]; !ok || v != "MIT" {
		t.Errorf("Expected license=MIT, got %v", record.Candidate.RawMetadata["license"])
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

	adapter := &OfficialRegistryAdapter{
		HTTPClient: retry.NewClient(retry.Config{
			MaxRetries: 0, BaseDelay: 10 * time.Millisecond, MaxDelay: 50 * time.Millisecond,
		}).WithHTTPClient(server.Client()),
		BaseURL: server.URL,
	}

	candidate := models.RawCandidate{
		Source:    "official-registry",
		SourceURL: "https://registry.example.com/missing",
	}

	_, err := adapter.Fetch(context.Background(), candidate)
	if err == nil {
		t.Error("Expected error for 404")
	}
}

func TestInterfaceImplementation(t *testing.T) {
	adapter := New()
	var _ interface {
		Discover(context.Context) ([]models.RawCandidate, error)
	} = adapter
}
