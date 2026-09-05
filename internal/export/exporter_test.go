package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestExport(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID:          "abc123",
			Name:        "taiwan-mcp",
			Slug:        "taiwan-mcp",
			Description: "Taiwan stock MCP server",
			Category:    []string{"finance"},
			Repository: models.RepositoryInfo{
				URL:      "https://github.com/foo/taiwan-mcp",
				Stars:    100,
				License:  "MIT",
			},
			Transport: []string{"stdio"},
			Health:    models.HealthHealthy,
			Quality:   models.QualityScore{Score: 85, Grade: "B"},
			TaiwanRelevance: models.TaiwanRelevance{
				Level: "T5",
				Score: 65,
			},
			Sources: []models.SourceReference{
				{Source: "github", URL: "https://github.com/foo/taiwan-mcp", TrustScore: 0.95},
			},
		},
		{
			ID:          "def456",
			Name:        "non-taiwan-mcp",
			Slug:        "non-taiwan-mcp",
			Description: "Global search MCP",
			Category:    []string{"search"},
			Repository: models.RepositoryInfo{
				URL:   "https://github.com/foo/global-mcp",
				Stars: 5000,
			},
			Transport: []string{"stdio"},
			Health:    models.HealthUnavailable,
			Quality:   models.QualityScore{Score: 70, Grade: "C"},
			TaiwanRelevance: models.TaiwanRelevance{
				Level: "T0",
				Score: 0,
			},
			Sources: []models.SourceReference{
				{Source: "github", URL: "https://github.com/foo/global-mcp", TrustScore: 0.95},
			},
		},
	}

	dir := t.TempDir()
	re := New()
	if err := re.Export(dir, servers); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	files := []string{"registry.json", "registry.min.json", "categories.json", "sources.json", "statistics.json", "health.json"}
	for _, f := range files {
		path := filepath.Join(dir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Expected file %s to exist: %v", f, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Expected file %s to have content", f)
		}
	}
}

func TestExport_EmptyServers(t *testing.T) {
	dir := t.TempDir()
	re := New()
	if err := re.Export(dir, nil); err != nil {
		t.Fatalf("Export with empty servers failed: %v", err)
	}

	// All files should still be created
	for _, f := range []string{"registry.json", "categories.json", "statistics.json"} {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", f)
		}
	}
}

func TestComputeStatistics(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID:   "s1",
			Name: "taiwan-1",
			TaiwanRelevance: models.TaiwanRelevance{Level: "T5", Score: 70},
			Health: models.HealthHealthy,
			Quality: models.QualityScore{Score: 90, Grade: "A"},
			Sources: []models.SourceReference{{Source: "github"}},
		},
		{
			ID:   "s2",
			Name: "taiwan-2",
			TaiwanRelevance: models.TaiwanRelevance{Level: "T3", Score: 40},
			Health: models.HealthDegraded,
			Quality: models.QualityScore{Score: 65, Grade: "D"},
			Sources: []models.SourceReference{{Source: "global"}},
		},
		{
			ID:   "s3",
			Name: "non-taiwan-1",
			TaiwanRelevance: models.TaiwanRelevance{Level: "T0", Score: 0},
			Health: models.HealthUnavailable,
			Quality: models.QualityScore{Score: 30, Grade: "F"},
		},
	}

	stats := computeStatistics(servers)
	if stats.TotalServers != 3 {
		t.Errorf("Expected 3 total servers, got %d", stats.TotalServers)
	}
	if stats.TaiwanRelevant != 2 {
		t.Errorf("Expected 2 Taiwan relevant, got %d", stats.TaiwanRelevant)
	}
	if stats.ByLevel["T5"] != 1 {
		t.Errorf("Expected 1 T5 server, got %d", stats.ByLevel["T5"])
	}
	if stats.ByLevel["T3"] != 1 {
		t.Errorf("Expected 1 T3 server, got %d", stats.ByLevel["T3"])
	}
	if stats.ByLevel["T0"] != 1 {
		t.Errorf("Expected 1 T0 server, got %d", stats.ByLevel["T0"])
	}
	if stats.ByHealth["HEALTHY"] != 1 {
		t.Errorf("Expected 1 HEALTHY, got %d", stats.ByHealth["HEALTHY"])
	}
	if stats.QualityDist["A"] != 1 {
		t.Errorf("Expected 1 A grade, got %d", stats.QualityDist["A"])
	}
}

func TestExport_MkdirAllError(t *testing.T) {
	re := New()
	// Try to create a directory where a file exists (will fail)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "blocker")
	if err := os.WriteFile(filePath, []byte("block"), 0644); err != nil {
		t.Fatal(err)
	}
	err := re.Export(filepath.Join(filePath, "subdir"), nil)
	if err == nil {
		t.Error("Expected error when directory creation fails")
	}
}

func TestWriteJSON_NoIndent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_noident.json")
	err := writeJSON(path, map[string]string{"key": "value"}, false)
	if err != nil {
		t.Fatalf("writeJSON error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty file")
	}
}

func TestExport_InvalidDir(t *testing.T) {
	re := New()
	err := re.Export("", nil)
	_ = err
}

func TestExportMarkdown(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID:          "md-test-1",
			Name:        "台灣金融 MCP",
			Description: "A Taiwan financial MCP server",
			Category:    []string{"finance"},
			Repository: models.RepositoryInfo{
				Name:     "taiwan-finance",
				URL:      "https://github.com/test/taiwan-finance",
				Stars:    100,
				Language: "Go",
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Level:      "T5",
				Score:      85,
				Confidence: 1.0,
			},
			Health:  models.HealthHealthy,
			Quality: models.QualityScore{Grade: "A", Score: 90},
			License: "MIT",
			Tools:   []models.Tool{{Name: "get_stock_price"}},
			Endpoints: []models.Endpoint{{URL: "https://api.test.com/mcp", Transport: "http"}},
		},
		{
			ID:   "md-test-2",
			Name: "Global Server",
			Category: []string{"search"},
			Repository: models.RepositoryInfo{
				Name: "server",
				URL:  "https://github.com/global/server",
			},
			TaiwanRelevance: models.TaiwanRelevance{Level: "T0", Score: 5},
			Health:           models.HealthDegraded,
			Quality:          models.QualityScore{Grade: "C", Score: 40},
		},
	}

	re := New()
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "REGISTRY.md")

	if err := re.ExportMarkdown(mdPath, servers); err != nil {
		t.Fatalf("ExportMarkdown error: %v", err)
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	md := string(content)

	// Verify markdown structure
	for _, expected := range []string{
		"# Awesome Taiwan MCP Registry",
		"## Statistics",
		"## MCP Servers",
		"### 🇹🇼 Taiwan-relevant",
		"### 🌍 International",
		"**台灣金融 MCP**",
		"https://github.com/test/taiwan-finance",
		"**Taiwan**: T5 (score: 85)",
		"**Global Server**",
	} {
		if !strings.Contains(md, expected) {
			t.Errorf("Markdown missing: %s", expected)
		}
	}

}

func TestExportMarkdown_LevelDescriptions(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID:   "md-test-1",
			Name: "Test Taiwan Server",
			Category: []string{"finance"},
			Repository: models.RepositoryInfo{
				Name:     "server",
				URL:      "https://github.com/test/server",
				Stars:    10,
				Language: "Go",
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Level:      "T3",
				Score:      55,
				Confidence: 0.9,
				Evidence: []models.Evidence{
					{Type: "keyword", Rule: "taiwan_keyword", Score: 40, MatchedText: "taiwan"},
					{Type: "domain", Rule: "gov_tw", Score: 15},
				},
			},
			Health:  models.HealthHealthy,
			Quality: models.QualityScore{Grade: "A", Score: 85},
		},
	}

	re := New()
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "REGISTRY.md")

	if err := re.ExportMarkdown(mdPath, servers); err != nil {
		t.Fatalf("ExportMarkdown error: %v", err)
	}

	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	md := string(content)

	// Verify Taiwan relevance with score
	if !strings.Contains(md, "**Taiwan**: T3 (score: 55)") {
		t.Error("Markdown missing Taiwan relevance with score")
	}

	// Verify classification evidence is shown
	if !strings.Contains(md, "+40 taiwan (taiwan_keyword)") {
		t.Error("Markdown missing keyword evidence")
	}
	if !strings.Contains(md, "+15 ") {
		t.Error("Markdown missing domain evidence")
	}

	// Verify language link is correct GitHub search URL
	expectedLink := "https://github.com/search?q=server+language:Go&type=repositories"
	if !strings.Contains(md, expectedLink) {
		t.Errorf("Markdown missing language link: %s", expectedLink)
	}

	// Verify language text is linked
	if !strings.Contains(md, "[Go](https://github.com/search") {
		t.Error("Markdown missing linked language text")
	}

	// Verify functional category grouping
	if !strings.Contains(md, "💰 Finance & Fintech") {
		t.Error("Markdown missing functional category")
	}
}

func TestExportMarkdown_UTF8Sanitization(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID:          "utf8-test",
			Name:        "Test UTF8",
			Category:    []string{"finance"},
			Description: "測試台灣法律 RAG \xff\xfe非法字元",
			Repository: models.RepositoryInfo{
				Name: "test-utf8",
				URL:  "https://github.com/test/utf8",
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Level:      "T3",
				Score:      60,
				Confidence: 0.9,
			},
			Health:  models.HealthHealthy,
			Quality: models.QualityScore{Grade: "B", Score: 75},
		},
	}

	re := New()
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "REGISTRY.md")

	if err := re.ExportMarkdown(mdPath, servers); err != nil {
		t.Fatalf("ExportMarkdown error: %v", err)
	}

	// Verify the file is valid UTF-8
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	// Invalid UTF-8 bytes should be stripped, valid UTF-8 preserved
	if !utf8.Valid(data) {
		t.Error("Markdown file contains invalid UTF-8")
	}

	// The Chinese text should be preserved
	if !strings.Contains(string(data), "測試台灣") {
		t.Error("Markdown missing UTF-8 content")
	}

	// Invalid bytes should be gone
	if strings.Contains(string(data), "\xff\xfe") {
		t.Error("Markdown contains invalid UTF-8 bytes")
	}
}
