package scoring

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestScore(t *testing.T) {
	scorer := New()
	server := &models.MCPServer{
		Name: "test-mcp",
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/foo/test-mcp",
			Stars:    150,
			Topics:   []string{"mcp"},
			License:  "MIT",
		},
		Transport: []string{"stdio"},
		Tools: []models.Tool{
			{Name: "tool1", Description: "desc", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool2", Description: "desc", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool3", Description: "desc", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool4", Description: "desc", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool5", Description: "desc", InputSchema: map[string]any{"type": "object"}},
		},
		Health: models.HealthHealthy,
		DataSources: []models.DataSource{
			{Name: "twse.com.tw", Type: models.DataSourceOfficialGovAPI},
		},
	}

	score := scorer.Score(server)
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("Score %d out of range [0, 100]", score.Score)
	}
	if score.Grade == "" {
		t.Error("Expected non-empty grade")
	}
}

func TestScoreDeterministic(t *testing.T) {
	scorer := New()
	server := &models.MCPServer{
		Name: "test-mcp",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/foo/test-mcp",
			Stars:   50,
			License: "MIT",
		},
		Transport: []string{"stdio"},
		Health:    models.HealthHealthy,
	}

	scores := make(map[int]int)
	for i := 0; i < 100; i++ {
		result := scorer.Score(server)
		scores[result.Score]++
	}

	if len(scores) != 1 {
		t.Errorf("Expected deterministic score (1 unique), got %d unique", len(scores))
	}
}

func TestScoreRange(t *testing.T) {
	scorer := New()
	server := &models.MCPServer{
		Name: "empty",
	}

	score := scorer.Score(server)
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("Score %d out of range [0, 100]", score.Score)
	}
	if score.Grade != "F" {
		t.Errorf("Expected grade F for low score, got %s", score.Grade)
	}
}
