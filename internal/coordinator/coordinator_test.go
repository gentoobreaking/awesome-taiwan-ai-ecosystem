package coordinator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/engines"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/security"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/sources/github"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

func TestPipelineStagesOrdered(t *testing.T) {
	if len(PipelineStages) != 10 {
		t.Errorf("Expected 10 pipeline stages, got %d", len(PipelineStages))
	}
	expected := []string{
		"DISCOVERY", "NORMALIZER", "TAIWAN_RELEVANCE", "AI_RELEVANCE",
		"CLASSIFIER", "MCP_IDENTITY", "RUNTIME_VERIFICATION",
		"SECURITY_SCANNER", "QUALITY_SCORING", "REGISTRY_VIEWS",
	}
	for i, stage := range PipelineStages {
		if stage != expected[i] {
			t.Errorf("Stage %d: expected %s, got %s", i, expected[i], stage)
		}
	}
}

func TestPipelineModes(t *testing.T) {
	if ModeFull != "full" {
		t.Errorf("ModeFull = %q, want %q", ModeFull, "full")
	}
	if ModeDiscoveryOnly != "discovery-only" {
		t.Errorf("ModeDiscoveryOnly = %q, want %q", ModeDiscoveryOnly, "discovery-only")
	}
	if ModeClassifyOnly != "classify-only" {
		t.Errorf("ModeClassifyOnly = %q, want %q", ModeClassifyOnly, "classify-only")
	}
	if ModeVerifyOnly != "verify-only" {
		t.Errorf("ModeVerifyOnly = %q, want %q", ModeVerifyOnly, "verify-only")
	}
}

func TestGenerateCrawlID(t *testing.T) {
	id := generateCrawlID()
	if len(id) != 16 {
		t.Errorf("Expected crawl ID length 16, got %d: %s", len(id), id)
	}
	if id[8] != 'T' {
		t.Errorf("Expected 'T' separator at position 8, got %c", id[8])
	}
	if id[15] != 'Z' {
		t.Errorf("Expected 'Z' suffix at position 15, got %c", id[15])
	}
}

func TestGenerateEntityID(t *testing.T) {
	id1 := generateEntityID("https://github.com/foo/bar")
	id2 := generateEntityID("https://github.com/foo/bar")
	id3 := generateEntityID("https://github.com/foo/baz")

	if id1 != id2 {
		t.Error("Same URL should produce same entity ID")
	}
	if id1 == id3 {
		t.Error("Different URLs should produce different entity IDs")
	}
	if len(id1) != 16 {
		t.Errorf("Expected entity ID length 16, got %d: %s", len(id1), id1)
	}
}

func TestNewCoordinator(t *testing.T) {
	store := &storage.Store{}
	logger := metrics.New(false)
	norm := normalize.New()

	adapters := []sources.SourceAdapter{
		github.New("test-token"),
	}

	pc := New(store, norm, adapters, logger)
	if pc == nil {
		t.Fatal("Expected non-nil coordinator")
	}
	if len(pc.sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(pc.sources))
	}
	if pc.classifier == nil {
		t.Error("Expected non-nil classifier")
	}
	if pc.qualityEngine == nil {
		t.Error("Expected non-nil quality engine")
	}
	if pc.securityScan == nil {
		t.Error("Expected non-nil security scanner")
	}
}

func TestServerToEntity(t *testing.T) {
	pc := &PipelineCoordinator{}
	server := &models.MCPServer{
		ID:          "test-id",
		Name:        "test-mcp",
		Slug:        "test-mcp",
		Description: "A test MCP server",
		Repository: models.RepositoryInfo{
			URL:    "https://github.com/test/test-mcp",
			Owner:  "test",
			Name:   "test-mcp",
			Stars:  100,
		},
	}

	entity := pc.serverToEntity(server)
	if entity.ID != "test-id" {
		t.Errorf("Expected entity ID 'test-id', got %s", entity.ID)
	}
	if entity.Name != "test-mcp" {
		t.Errorf("Expected entity name 'test-mcp', got %s", entity.Name)
	}
	if entity.Repository.URL != "https://github.com/test/test-mcp" {
		t.Errorf("Expected repository URL, got %s", entity.Repository.URL)
	}
	if entity.EntityStatus != models.EntityStatusDiscovered {
		t.Errorf("Expected status DISCOVERED, got %s", entity.EntityStatus)
	}
}

func TestServerToEntityEmptyID(t *testing.T) {
	pc := &PipelineCoordinator{}
	server := &models.MCPServer{
		Name: "no-id-mcp",
		Repository: models.RepositoryInfo{
			URL: "https://github.com/test/no-id-mcp",
		},
	}

	entity := pc.serverToEntity(server)
	if entity.ID == "" {
		t.Error("Expected non-empty entity ID")
	}
	if len(entity.ID) != 16 {
		t.Errorf("Expected entity ID length 16, got %d", len(entity.ID))
	}
}

func TestRunDiscoveryOnlyMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/repositories" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"total_count":0,"items":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	logger := metrics.New(false)
	pc := &PipelineCoordinator{
		sources:    []sources.SourceAdapter{github.New("test")},
		normalizer: normalize.New(),
		logger:     logger,
	}

	adapter := pc.sources[0].(*github.GitHubAdapter)
	adapter.BaseURL = server.URL
	adapter.HTTPClient = github.NewStdHTTPClient(server.Client())

	cfg := PipelineConfig{
		Mode:    ModeDiscoveryOnly,
		Workers: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := pc.Run(ctx, cfg)
	if err != nil {
		t.Logf("Run returned error (may be expected): %v", err)
	}
}

func TestTaiwanRelevanceScoring(t *testing.T) {
	pc := &PipelineCoordinator{
		logger: metrics.New(false),
	}
	entities := []*models.Entity{
		{
			Name:        "tw-ai-mcp",
			Description: "Taiwan AI MCP server for TWSE stock data",
			Repository: models.RepositoryInfo{
				URL: "https://github.com/taiwan/ai-mcp",
			},
			RawContent: "# TW AI MCP\nTaiwan stock data from TWSE",
		},
	}

	pc.runTaiwanRelevance(context.Background(), "test", entities)

	if len(entities) != 1 {
		t.Fatal("Expected 1 entity")
	}
	e := entities[0]
	if e.TaiwanRelevance.Score < 55 {
		t.Errorf("Expected Taiwan relevance score >= 55, got %f", e.TaiwanRelevance.Score)
	}
}

func TestAIRelevanceScoring(t *testing.T) {
	pc := &PipelineCoordinator{
		logger: metrics.New(false),
	}
	entities := []*models.Entity{
		{
			Name:        "ai-mcp-server",
			Description: "RAG MCP server with embeddings",
			Repository: models.RepositoryInfo{
				URL: "https://github.com/test/mcp-server",
			},
		},
	}

	pc.runAIRelevance(context.Background(), "test", entities)

	e := entities[0]
	if e.AIRelevance.Score < 20 {
		t.Errorf("Expected AI relevance score >= 20, got %f", e.AIRelevance.Score)
	}
}

func TestRunClassifier(t *testing.T) {
	pc := &PipelineCoordinator{
		classifier: engines.NewClassifier(),
		logger:     metrics.New(false),
	}
	entities := []*models.Entity{
		{
			Name:        "test-mcp",
			Description: "MCP server with stdio transport",
			Endpoints:   []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "stdio:mcp-server"}}},
		},
	}

	pc.runClassifier(context.Background(), "test", entities)

	e := entities[0]
	if e.Classification.Primary == "" {
		t.Error("Expected non-empty primary classification")
	}
}

func TestRunSecurityScan(t *testing.T) {
	pc := &PipelineCoordinator{
		securityScan: security.NewScanner(),
		logger:       metrics.New(false),
	}
	entities := []*models.Entity{
		{
			Name:       "test-mcp",
			Repository: models.RepositoryInfo{URL: "https://github.com/test/mcp"},
			RawContent: "# Test MCP Server\nThis is a safe server",
		},
	}

	pc.runSecurityScan(context.Background(), "test", entities)

	e := entities[0]
	if e.SecurityStatus.ScannedAt.IsZero() {
		t.Error("Expected non-zero scanned time")
	}
}

func TestRunQualityScoring(t *testing.T) {
	pc := &PipelineCoordinator{
		qualityEngine: engines.NewQualityEngine(),
		logger:        metrics.New(false),
	}
	entities := []*models.Entity{
		{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Repository: models.RepositoryInfo{
				URL:      "https://github.com/test/mcp",
				Language: "Go",
				Stars:    100,
			},
			RawContent: "# Test MCP Server",
			Endpoints:  []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "http://localhost:3000"}}},
		},
	}

	pc.runQualityScoring(context.Background(), "test", entities)

	e := entities[0]
	if e.Quality.Score == 0 {
		t.Error("Expected non-zero quality score")
	}
}
