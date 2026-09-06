package normalize

import (
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestNormalizeNameFromRepo(t *testing.T) {
	if got := normalizeName("", "https://github.com/foo/my-repo"); got != "my-repo" {
		t.Errorf("Expected 'my-repo', got %q", got)
	}
}

func TestNormalizeDescriptionWithReadme(t *testing.T) {
	desc := normalizeDescription("fallback", "# My MCP\nThis is a description.")
	if desc != "This is a description." {
		t.Errorf("Expected 'This is a description.', got %q", desc)
	}
}

func TestFirstParagraphEmpty(t *testing.T) {
	if got := firstParagraph(""); got != "" {
		t.Errorf("Expected empty string, got %q", got)
	}
}

func TestExtractOwner(t *testing.T) {
	if got := extractOwner("https://github.com/foo/bar"); got != "foo" {
		t.Errorf("Expected 'foo', got %q", got)
	}
}

func TestExtractRepoName(t *testing.T) {
	if got := extractRepoName("https://github.com/foo/bar.git"); got != "bar.git" {
		t.Errorf("Expected \"bar.git\", got %q", got)
	}
}

func TestExtractURLs(t *testing.T) {
	text := "Check https://example.com/mcp and http://test.com"
	urls := extractURLs(text)
	if len(urls) != 2 {
		t.Errorf("Expected 2 URLs, got %d", len(urls))
	}
}

func TestIsMCPEndpoint(t *testing.T) {
	if !isMCPEndpoint("https://example.com/mcp") {
		t.Error("Expected true for /mcp suffix")
	}
	if isMCPEndpoint("https://example.com/api") {
		t.Error("Expected false for non-MCP URL")
	}
}

func TestDetectTransportFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"sse://localhost:8080", "sse"},
		{"wss://example.com", "websocket"},
		{"https://example.com/sse", "sse"},
		{"https://example.com/mcp", "http"},
		{"stdio", "stdio"},
	}
	for _, tt := range tests {
		if got := detectTransportFromURL(tt.url); got != tt.want {
			t.Errorf("detectTransportFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestExtractEndpoints(t *testing.T) {
	endpoints := extractEndpoints(
		"Check https://example.com/mcp",
		map[string]any{"endpoint": "https://alt.example.com/mcp"},
		map[string]any{"endpoint": "https://raw.example.com/mcp"},
	)
	if len(endpoints) < 2 {
		t.Errorf("Expected at least 2 endpoints, got %d", len(endpoints))
	}
}

func TestParsePackageFiles_Nil(t *testing.T) {
	result := parsePackageFiles(nil, nil)
	if result != nil {
		t.Error("Expected nil for nil package files and manifest")
	}
}

func TestParsePackageFiles_ManifestMap(t *testing.T) {
	result := parsePackageFiles(nil, map[string]any{
		"content": `{"tools":[{"name":"test","description":"test"}]}`,
	})
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestParsePackageFiles_InvalidJSON(t *testing.T) {
	result := parsePackageFiles(map[string]string{
		"package.json": "not valid json",
	}, nil)
	if result != nil {
		t.Error("Expected nil for invalid JSON")
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"package.json", "package.json"},
		{"pyproject.toml", "pyproject.toml"},
		{"go.mod", "go.mod"},
		{"Cargo.toml", "cargo.toml"},
		{"server.json", "server.json"},
		{"mcp.json", "mcp.json"},
		{"manifest.json", "manifest.json"},
		{"unknown.txt", "json"},
	}
	for _, tt := range tests {
		if got := detectFileType(tt.filename); got != tt.want {
			t.Errorf("detectFileType(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("https://github.com/foo/bar")
	id2 := GenerateID("https://github.com/foo/bar/")
	if id1 != id2 {
		t.Error("GenerateID should be deterministic for URL variations")
	}
	if len(id1) != 64 {
		t.Errorf("Expected 64-char hex ID, got %d chars", len(id1))
	}
}

func TestSafeInt(t *testing.T) {
	tests := []struct {
		input any
		want  int
	}{
		{int(42), 42},
		{int32(42), 42},
		{int64(42), 42},
		{float64(42.7), 42},
		{"string", 0},
	}
	for _, tt := range tests {
		if got := safeInt(tt.input); got != tt.want {
			t.Errorf("safeInt(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNormalize_FullRecord(t *testing.T) {
	n := New()
	record := &models.RawRecord{
		RawCandidate: models.RawCandidate{
			Source:        "github",
			SourceURL:     "https://github.com/foo/taiwan",
			Name:          "taiwan-mcp",
			Description:   "Taiwan server",
			RepositoryURL: "https://github.com/foo/taiwan-mcp",
			HomepageURL:   "https://taiwan.example.com",
			RawMetadata:   map[string]any{"topics": []string{"mcp", "taiwan"}},
			DiscoveredAt:  time.Time{},
		},
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/foo/taiwan-mcp",
			Stars:   100,
			License: "MIT",
		},
		Transport: []string{"stdio"},
	}

	server, err := n.Normalize(record)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if server.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if server.Name != "taiwan-mcp" {
		t.Errorf("Expected name 'taiwan-mcp', got '%s'", server.Name)
	}
	if server.Repository.Stars != 100 {
		t.Errorf("Expected 100 stars, got %d", server.Repository.Stars)
	}
}
