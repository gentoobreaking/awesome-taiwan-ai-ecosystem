package classify

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestLoadConfig(t *testing.T) {
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(keywordCfg.Government) == 0 {
		t.Error("Expected government keywords to be loaded")
	}
	if len(domainCfg.Domains) == 0 {
		t.Error("Expected domains to be loaded")
	}
}

func TestMatchTaiwanKeywords(t *testing.T) {
	_ = LoadConfig()
	texts := []string{"TWSE stock data MCP server", "uses data.gov.tw API"}
	matches := MatchTaiwanKeywords(texts)
	if len(matches) == 0 {
		t.Error("Expected at least one keyword match")
	}
	found := false
	for _, m := range matches {
		if m.Keyword == "TWSE" || m.Keyword == "data.gov.tw" {
			found = true
		}
	}
	if !found {
		t.Error("Expected to find TWSE or data.gov.tw keywords")
	}
}

func TestMatchOfficialDomains(t *testing.T) {
	_ = LoadConfig()
	urls := []string{
		"https://twse.com.tw/api",
		"https://github.com/user/repo",
		"https://cwa.gov.tw/weather",
		"https://example.com",
	}
	matched := MatchOfficialDomains(urls)
	if len(matched) != 2 {
		t.Errorf("Expected 2 domain matches, got %d: %v", len(matched), matched)
	}
}

func TestMatchDataSource(t *testing.T) {
	_ = LoadConfig()
	urls := []string{
		"https://twse.com.tw/api",
		"https://data.gov.tw/dataset",
	}
	matched := MatchDataSource(urls)
	if len(matched) != 2 {
		t.Errorf("Expected 2 data source matches, got %d: %v", len(matched), matched)
	}
}

func TestScoreTaiwanServer(t *testing.T) {
	_ = LoadConfig()
	server := &models.MCPServer{
		Name: "Taiwan Stock MCP",
		Repository: models.RepositoryInfo{
			URL:    "https://github.com/foo/taiwan-stock-mcp",
			Owner:  "foo",
			Name:   "taiwan-stock-mcp",
			Topics: []string{"mcp", "taiwan"},
		},
		Endpoints: []models.Endpoint{
			{URL: "https://twse.com.tw/api/mcp"},
		},
		Description: "TWSE stock data MCP server for Taiwan stock market",
	}

	result := Score(server)
	if result.Level != "T5" {
		t.Errorf("Expected T5, got %s (score: %f)", result.Level, result.Score)
	}
	if len(result.Evidence) == 0 {
		t.Error("Expected evidence to be generated")
	}
	if result.Score < 70 {
		t.Errorf("Expected score >= 70 for T5, got %f", result.Score)
	}
}

func TestScoreNonTaiwanServer(t *testing.T) {
	_ = LoadConfig()
	server := &models.MCPServer{
		Name: "US Stock MCP",
		Repository: models.RepositoryInfo{
			URL:    "https://github.com/foo/us-stock-mcp",
			Owner:  "foo",
			Name:   "us-stock-mcp",
		},
		Description: "US stock market data",
	}

	result := Score(server)
	if result.Level == "T5" {
		t.Error("Expected non-T5 level for non-Taiwan server")
	}
}

func TestThresholdToLevel(t *testing.T) {
	tests := []struct {
		score     float64
		expected  string
	}{
		{0, "T0"},
		{5, "T1"},
		{20, "T2"},
		{40, "T3"},
		{55, "T4"},
		{70, "T5"},
		{100, "T5"},
	}
	for _, tt := range tests {
		got := ThresholdToLevel(tt.score)
		if got != tt.expected {
			t.Errorf("Score %f: expected %s, got %s", tt.score, tt.expected, got)
		}
	}
}

func TestScoreDeterministic(t *testing.T) {
	_ = LoadConfig()
	server := &models.MCPServer{
		Name: "Taiwan Stock MCP",
		Repository: models.RepositoryInfo{
			URL:    "https://github.com/foo/taiwan-stock-mcp",
			Owner:  "foo",
		},
		Endpoints: []models.Endpoint{
			{URL: "https://twse.com.tw/api"},
		},
	}

	scores := make(map[float64]int)
	for i := 0; i < 100; i++ {
		result := Score(server)
		scores[result.Score]++
	}

	if len(scores) != 1 {
		t.Errorf("Expected deterministic score (1 unique), got %d unique scores", len(scores))
	}
}
