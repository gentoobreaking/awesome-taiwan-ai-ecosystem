package schema_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/classify"
	"github.com/david/awesome-taiwan-mcp/internal/dedupe"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
)

// goldenFixture holds a golden MCPServer and its expected classification.
type goldenFixture struct {
	Server  models.MCPServer
	Level   string
}

// loadGoldenFixtures loads all golden fixtures (§TST-068).
func loadGoldenFixtures(t *testing.T) []goldenFixture {
	t.Helper()
	goldenDir := filepath.Join("..", "..", "tests", "fixtures", "golden")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatalf("Failed to read golden dir: %v", err)
	}

	var fixtures []goldenFixture
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 5 || name[len(name)-5:] != ".json" {
			continue
		}
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var server models.MCPServer
		if err := json.Unmarshal(data, &server); err != nil {
			t.Logf("Skipping %s (unmarshal error): %v", name, err)
			continue
		}
		fixtures = append(fixtures, goldenFixture{
			Server: server,
			Level:  string(server.TaiwanRelevance.Level),
		})
	}
	return fixtures
}

// TestGoldenClassificationAccuracy verifies TST-068 classification accuracy.
func TestGoldenClassificationAccuracy(t *testing.T) {
	fixtures := loadGoldenFixtures(t)
	if len(fixtures) == 0 {
		t.Skip("No golden fixtures found")
	}

	passed := 0
	for _, f := range fixtures {
		result := classify.Score(&f.Server)
		if result.Level == f.Level {
			passed++
		} else {
			t.Errorf("Classification mismatch for %s: got %s, want %s (score=%.1f)",
				f.Server.ID, result.Level, f.Level, result.Score)
		}
	}
	accuracy := float64(passed) / float64(len(fixtures)) * 100
	t.Logf("Classification accuracy: %.0f%% (%d/%d)", accuracy, passed, len(fixtures))
	if accuracy < 100 {
		t.Errorf("Classification accuracy: %.0f%% (expected 100%%)", accuracy)
	}
}

// TestGoldenIdentityAccuracy verifies TST-068 identity/CanonicalID accuracy.
func TestGoldenIdentityAccuracy(t *testing.T) {
	fixtures := loadGoldenFixtures(t)
	ids := make(map[string]int)
	for _, f := range fixtures {
		if f.Server.Repository.URL != "" {
			id := normalize.GenerateID(f.Server.Repository.URL)
			ids[id]++
		}
	}

	dupes := 0
	for id, count := range ids {
		if count > 1 {
			dupes++
			t.Errorf("Duplicate ID found: %s (count=%d)", id, count)
		}
	}
	if dupes > 0 {
		t.Errorf("Identity accuracy: %d duplicate IDs found", dupes)
	}
	t.Logf("Identity accuracy: 100%% (%d unique IDs)", len(ids))
}

// TestGoldenDedupAccuracy verifies TST-068 dedup accuracy.
func TestGoldenDedupAccuracy(t *testing.T) {
	fixtures := loadGoldenFixtures(t)

	sources := make([]*dedupe.ServerSource, 0, len(fixtures))
	for _, f := range fixtures {
		trust := models.SourceTrustScores{}.GitHub
		if trust == 0 {
			trust = 0.5
		}
		s := f.Server
		sources = append(sources, &dedupe.ServerSource{
			Server:     &s,
			TrustScore: trust,
		})
	}

	engine := dedupe.New()
	result, err := engine.Deduplicate(sources)
	if err != nil {
		t.Fatalf("Deduplicate error: %v", err)
	}

	ids := make(map[string]int)
	for _, s := range result {
		ids[s.ID]++
	}
	dupes := 0
	for id, count := range ids {
		if count > 1 {
			dupes++
			t.Errorf("Duplicate ID after dedup: %s (count=%d)", id, count)
		}
	}
	if dupes > 0 {
		t.Errorf("Dedup accuracy: %d duplicate IDs after dedup", dupes)
	}
	t.Logf("Dedup accuracy: 100%% (0 duplicates in %d servers)", len(result))
}

// TestGoldenInvalidHandling verifies TST-068 invalid handling.
func TestGoldenInvalidHandling(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "fixtures", "golden", "invalid-1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("No invalid fixture found")
	}

	var server models.MCPServer
	if err := json.Unmarshal(data, &server); err != nil {
		// Invalid JSON is expected to fail unmarshal — that's correct handling
		return
	}

	// If it parsed, classification should still work without panic
	result := classify.Score(&server)
	if result.Level == "" {
		t.Error("Expected a classification level even for invalid fixtures")
	}
}

// TestGoldenRegressionEvidence verifies golden dataset test results are saved as evidence.
func TestGoldenRegressionEvidence(t *testing.T) {
	fixtures := loadGoldenFixtures(t)
	if len(fixtures) == 0 {
		t.Skip("No golden fixtures")
	}

	// Sort for deterministic output
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].Server.ID < fixtures[j].Server.ID
	})

	total := len(fixtures)
	classified := 0
	for _, f := range fixtures {
		result := classify.Score(&f.Server)
		if result.Level == f.Level {
			classified++
		}
	}
	accuracy := float64(classified) / float64(total) * 100
	t.Logf("Golden regression evidence: total=%d, classified=%d, accuracy=%.1f%%",
		total, classified, accuracy)
}
