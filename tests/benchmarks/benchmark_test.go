package benchmarks

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/classify"
	"github.com/david/awesome-taiwan-mcp/internal/dedupe"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/scoring"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

// BenchmarkNormalize benchmarks the normalizer (§TST-062).
func BenchmarkNormalize(b *testing.B) {
	norm := normalize.New()
	record := &models.RawRecord{
		RawCandidate: models.RawCandidate{
			Source:        "github",
			Name:          "bench-mcp",
			RepositoryURL: "https://github.com/bench/mcp-server",
			Description:   "Benchmark MCP server",
		},
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/bench/mcp-server",
			Host:    "github.com",
			Owner:   "bench",
			Name:    "mcp-server",
			Stars:   100,
			License: "MIT",
		},
		Readme:    "# bench-mcp\n\nBenchmark MCP server.",
		PackageFiles: map[string]string{},
		Transport: []string{"stdio"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := norm.Normalize(record)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkClassify benchmarks the classifier.
func BenchmarkClassify(b *testing.B) {
	server := &models.MCPServer{
		Name:        "taiwan-bench-mcp",
		Description: "Taiwan stock MCP server",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/twse/bench-mcp",
			Owner:   "twse",
			Name:    "bench-mcp",
			License: "MIT",
			Topics:  []string{"mcp", "taiwan"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classify.Score(server)
	}
}

// BenchmarkDedup benchmarks the dedup engine.
func BenchmarkDedup(b *testing.B) {
	servers := make([]*dedupe.ServerSource, 1000)
	for i := 0; i < 1000; i++ {
		servers[i] = &dedupe.ServerSource{
			Server: &models.MCPServer{
				ID:   fmt.Sprintf("server-%d", i),
				Name: fmt.Sprintf("server-%d", i),
				Repository: models.RepositoryInfo{
					URL: fmt.Sprintf("https://github.com/org/server-%d", i),
				},
				Sources: []models.SourceReference{{Source: "github"}},
			},
			TrustScore: 0.5,
		}
	}

	engine := dedupe.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Deduplicate(servers)
	}
}

// BenchmarkScoring benchmarks the quality scorer.
func BenchmarkScoring(b *testing.B) {
	server := &models.MCPServer{
		Name:        "taiwan-bench-mcp",
		Description: "Taiwan stock MCP server with good documentation",
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/twse/bench-mcp",
			Stars:    500,
			License:  "MIT",
			Forks:    50,
		},
		Tools:     []models.Tool{{Name: "tool1"}},
		Endpoints: []models.Endpoint{{URL: "https://bench.example.com/mcp", Transport: "http"}},
	}

	scorer := scoring.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scorer.Score(server)
	}
}

// BenchmarkFullPipeline benchmarks normalization of 100 candidates (§TST-062).
func BenchmarkFullPipeline(b *testing.B) {
	norm := normalize.New()
	_ = metrics.New(false)
	_ = storage.Open

	// Generate 100 mock candidates (scale down for benchmark speed)
	records := make([]*models.RawRecord, 100)
	for i := 0; i < 100; i++ {
		c := models.RawCandidate{
			Source:        "mock",
			SourceURL:     fmt.Sprintf("https://github.com/mock/server-%d", i),
			Name:          fmt.Sprintf("server-%d", i),
			Description:   "Mock MCP server",
			RepositoryURL: fmt.Sprintf("https://github.com/mock/server-%d", i),
			DiscoveredAt:  time.Now(),
		}
		records[i] = &models.RawRecord{
			RawCandidate: c,
			Repository:   models.RepositoryInfo{URL: c.RepositoryURL, Host: "github.com", Name: c.Name},
			PackageFiles: map[string]string{},
			Transport:  []string{"stdio"},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, rec := range records {
			sem <- struct{}{}
			wg.Add(1)
			go func(r *models.RawRecord) {
				defer wg.Done()
				defer func() { <-sem }()
				norm.Normalize(r)
			}(rec)
		}
		wg.Wait()
	}
}
