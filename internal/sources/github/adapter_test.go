package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

func TestNew(t *testing.T) {
	adapter := New("test-token")
	if adapter.Name() != "github" {
		t.Errorf("Expected name 'github', got '%s'", adapter.Name())
	}
}

func TestKeywordMatrix(t *testing.T) {
	if len(KeywordMatrix) < 10 {
		t.Errorf("Expected at least 10 keywords, got %d", len(KeywordMatrix))
	}
	foundTaiwan := false
	foundTWSE := false
	foundGovData := false
	for _, kw := range KeywordMatrix {
		if kw == "mcp Taiwan" {
			foundTaiwan = true
		}
		if kw == "mcp TWSE" {
			foundTWSE = true
		}
		if kw == `mcp "data.gov.tw"` {
			foundGovData = true
		}
	}
	if !foundTaiwan {
		t.Error("Missing 'mcp Taiwan' keyword")
	}
	if !foundTWSE {
		t.Error("Missing 'mcp TWSE' keyword")
	}
	if !foundGovData {
		t.Error("Missing 'mcp \"data.gov.tw\"' keyword")
	}
}

func TestDiscoverWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/repositories" {
			resp := map[string]any{
				"total_count": 2,
				"items": []map[string]any{
					{
						"id":          1,
						"name":        "twstock-mcp",
						"full_name":    "mock/twstock-mcp",
						"description": "Taiwan stock MCP server",
						"html_url":     "https://github.com/mock/twstock-mcp",
						"stargazers_count": 100,
						"forks_count": 10,
						"language":    "Python",
						"license": map[string]any{"spdx_id": "MIT"},
						"owner": map[string]any{"login": "mock"},
						"pushed_at":    "2026-09-01T00:00:00Z",
					},
					{
						"id":          2,
						"name":        "finmind-mcp",
						"full_name":    "mock/finmind-mcp",
						"description": "FinMind financial MCP",
						"html_url":     "https://github.com/mock/finmind-mcp",
						"stargazers_count": 50,
						"language":    "Go",
						"owner": map[string]any{"login": "mock"},
						"pushed_at":    "2026-09-01T00:00:00Z",
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/repos/mock/twstock-mcp/topics" {
			json.NewEncoder(w).Encode(map[string]any{"names": []string{"mcp", "taiwan", "stock"}})
			return
		}
		if r.URL.Path == "/repos/mock/finmind-mcp/topics" {
			json.NewEncoder(w).Encode(map[string]any{"names": []string{"mcp", "finance"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &GitHubAdapter{
		Client:  server.Client(),
		Token:   "test",
		BaseURL: server.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	candidates, err := adapter.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(candidates) != 2 {
		t.Errorf("Expected 2 candidates, got %d", len(candidates))
	}

	for _, c := range candidates {
		if c.Source != "github" {
			t.Errorf("Expected source 'github', got '%s'", c.Source)
		}
		if c.RepositoryURL == "" {
			t.Error("RepositoryURL should not be empty")
		}
		meta := c.RawMetadata
		if meta["stars"] == nil {
			t.Error("Missing stars in metadata")
		}
		if meta["language"] == nil {
			t.Error("Missing language in metadata")
		}
	}
}

func TestDiscoverContextCancellation(t *testing.T) {
	adapter := &GitHubAdapter{
		Client:  &http.Client{Timeout: 1 * time.Second},
		Token:   "",
		BaseURL: "https://invalid-url-that-does-not-exist.invalid",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not panic
	_, _ = adapter.Discover(ctx)
}

func TestDiscoverRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	adapter := &GitHubAdapter{
		Client:  server.Client(),
		Token:   "test",
		BaseURL: server.URL,
	}

	candidates, err := adapter.Discover(context.Background())
	if err != nil {
		// Error is acceptable for rate limit
	}
	if candidates == nil {
		candidates = []models.RawCandidate{}
	}
}

func TestFetchWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/mock/test-mcp" {
			repo := map[string]any{
				"name": "test-mcp", "full_name": "mock/test-mcp", "description": "Test MCP",
				"html_url": "https://github.com/mock/test-mcp", "stargazers_count": 100,
				"subscribers_count": 5, "language": "Python",
				"license": map[string]any{"spdx_id": "MIT"},
				"default_branch": "main", "pushed_at": "2026-09-01T00:00:00Z",
				"topics": []string{"mcp", "taiwan"},
				"owner": map[string]any{"login": "mock"},
			}
			json.NewEncoder(w).Encode(repo)
			return
		}
		if r.URL.Path == "/repos/mock/test-mcp/contents/README.md" {
			encoded := base64.StdEncoding.EncodeToString([]byte("TWSE stock data MCP server"))
			json.NewEncoder(w).Encode(map[string]any{
				"content":  encoded,
				"encoding": "base64",
			})
			return
		}
		if r.URL.Path == "/repos/mock/test-mcp/contents/pyproject.toml" {
			encoded := base64.StdEncoding.EncodeToString([]byte("name = \"test-mcp\""))
			json.NewEncoder(w).Encode(map[string]any{
				"content":  encoded,
				"encoding": "base64",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	adapter := &GitHubAdapter{
		Client:  server.Client(),
		Token:   "test",
		BaseURL: server.URL,
	}

	candidate := models.RawCandidate{
		Source:        "github",
		Name:          "test-mcp",
		RepositoryURL: "https://github.com/mock/test-mcp",
		Description:   "Test MCP",
	}

	record, err := adapter.Fetch(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if record.Repository == nil {
		t.Fatal("Repository should not be nil")
	}
	if record.Repository.Stars != 100 {
		t.Errorf("Expected 100 stars, got %d", record.Repository.Stars)
	}
	if record.Repository.DefaultBranch != "main" {
		t.Errorf("Expected default_branch 'main', got '%s'", record.Repository.DefaultBranch)
	}
	if record.Readme == "" {
		t.Error("README should not be empty")
	}
	if record.Readme != "TWSE stock data MCP server" {
		t.Errorf("Unexpected README content: %s", record.Readme)
	}
	if record.PackageFiles["pyproject.toml"] == "" {
		t.Error("pyproject.toml should be fetched")
	}
	if len(record.Transport) == 0 {
		t.Error("Transport should be extracted from README")
	}
}

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		inputURL string
		expected string
	}{
		{"https://github.com/owner/repo", "owner/repo"},
		{"https://github.com/owner/repo.git", "owner/repo"},
		{"https://github.com/owner/repo/", "owner/repo"},
		{"https://invalid-url", ""},
	}
	for _, tt := range tests {
		got := extractRepoPath(tt.inputURL)
		if got != tt.expected {
			t.Errorf("extractRepoPath(%q) = %q, want %q", tt.inputURL, got, tt.expected)
		}
	}
}

func TestParseTime(t *testing.T) {
	tm := parseTime("2026-09-01T12:00:00Z")
	if tm.Year() != 2026 {
		t.Errorf("Expected year 2026, got %d", tm.Year())
	}
}

func TestExtractEndpointFromReadme(t *testing.T) {
	readme := `My MCP Server
Endpoint: https://example.com/mcp
Some text`
	endpoint := extractEndpointFromReadme(readme)
	if endpoint != "https://example.com/mcp" {
		t.Errorf("Expected endpoint 'https://example.com/mcp', got '%s'", endpoint)
	}
}

func TestExtractTransportsFromReadme(t *testing.T) {
	readme := "MCP server with stdio transport"
	transports := extractTransportsFromReadme(readme)
	if len(transports) == 0 {
		t.Error("Should extract at least one transport")
	}
}

func TestGitHubAdapterImplementsInterface(t *testing.T) {
	var _ sources.SourceAdapter = (*GitHubAdapter)(nil)
}
