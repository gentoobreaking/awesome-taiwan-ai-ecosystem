package export

import (
	"os"
	"path/filepath"
	"testing"

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
				{Source: "github", URL: "https://github.com/foo/taiwan-mcp"},
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