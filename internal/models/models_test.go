package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMCPServerJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := MCPServer{
		ID:          "sha256:abc123",
		Name:        "TWStock MCP",
		Slug:        "twstock-mcp",
		Description: "Taiwan stock market MCP",
		Region:      []string{"TW"},
		Category:    []string{"finance", "stock"},
		TaiwanRelevance: TaiwanRelevance{
			Level:      "T5",
			Score:      75,
			Confidence: 1.0,
			Evidence: []Evidence{
				{
					Type:        "official_domain",
					Source:      "README",
					Location:    "https://github.com/example/twstock-mcp",
					ContentHash: "sha256:def456",
					MatchedText: "twse.com.tw",
					Rule:        "official_taiwan_domain",
					Score:       40,
				},
			},
		},
		Repository: RepositoryInfo{
			URL:      "https://github.com/example/twstock-mcp",
			Host:     "github.com",
			Owner:    "example",
			Name:     "twstock-mcp",
			Stars:    100,
			Archived: false,
			License:  "MIT",
			Topics:   []string{"mcp", "taiwan"},
			PushedAt: now,
		},
		Endpoints: []Endpoint{
			{URL: "https://twstock-mcp.example.com/mcp", Transport: "streamable-http", TLS: true},
		},
		Transport: []string{"streamable-http"},
		Tools: []Tool{
			{Name: "get_stock_price", Description: "Get Taiwan stock price", InputSchema: map[string]any{"type": "object"}},
		},
		DataSources: []DataSource{
			{Name: "TWSE", Type: DataSourceOfficialGovAPI, URL: "https://www.twse.com.tw", Country: "TW", Official: true},
		},
		License:  "MIT",
		Status:   StatusActive,
		Health:   HealthHealthy,
		Quality:  QualityScore{Score: 90, Grade: "A"},
		Sources:  []SourceReference{{Source: "github", URL: "https://github.com/example/twstock-mcp", TrustScore: 0.95}},
	}

	data, err := json.Marshal(server)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored MCPServer
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ID != server.ID {
		t.Errorf("ID mismatch: %s != %s", restored.ID, server.ID)
	}
	if restored.TaiwanRelevance.Level != "T5" {
		t.Errorf("Level mismatch: %s", restored.TaiwanRelevance.Level)
	}
	if len(restored.TaiwanRelevance.Evidence) != 1 {
		t.Errorf("Evidence count: %d", len(restored.TaiwanRelevance.Evidence))
	}
	if restored.Repository.Stars != 100 {
		t.Errorf("Stars mismatch: %d", restored.Repository.Stars)
	}
	if restored.Quality.Grade != "A" {
		t.Errorf("Grade mismatch: %s", restored.Quality.Grade)
	}
}

func TestRawCandidateJSON(t *testing.T) {
	c := RawCandidate{
		Source:        "github",
		SourceURL:     "https://github.com/example/mcp",
		Name:          "mcp",
		RepositoryURL: "https://github.com/example/mcp",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var restored RawCandidate
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Source != "github" {
		t.Error("Source mismatch")
	}
}

func TestRawRecordJSON(t *testing.T) {
	r := RawRecord{
		Source:        "github",
		Name:          "mcp",
		RepositoryURL: "https://github.com/example/mcp",
		Readme:        "# My MCP",
		License:       "MIT",
	}
	data, _ := json.Marshal(r)
	var restored RawRecord
	json.Unmarshal(data, &restored)
	if restored.License != "MIT" {
		t.Error("License mismatch")
	}
}

func TestScoreToLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{75, "T5"},
		{60, "T4"},
		{50, "T3"},
		{30, "T2"},
		{10, "T1"},
		{3, "T0"},
	}
	for _, tt := range tests {
		got := ScoreToLevel(tt.score)
		if got != tt.expected {
			t.Errorf("ScoreToLevel(%f) = %s, want %s", tt.score, got, tt.expected)
		}
	}
}

func TestIsValidCategory(t *testing.T) {
	if !IsValidCategory("finance") {
		t.Error("finance should be valid")
	}
	if IsValidCategory("invalid-category") {
		t.Error("invalid-category should be invalid")
	}
}

func TestIsValidLevel(t *testing.T) {
	if !IsValidLevel("T5") {
		t.Error("T5 should be valid")
	}
	if IsValidLevel("T6") {
		t.Error("T6 should be invalid")
	}
}

func TestGradeForScore(t *testing.T) {
	if GradeForScore(95) != "A" {
		t.Error("95 should be A")
	}
	if GradeForScore(75) != "C" {
		t.Error("75 should be C")
	}
	if GradeForScore(30) != "F" {
		t.Error("30 should be F")
	}
}
