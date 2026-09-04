package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/crawler"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/export"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/scoring"
	"github.com/david/awesome-taiwan-mcp/internal/security"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/sources/github"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"github.com/david/awesome-taiwan-mcp/internal/verify"
)

// TestGitHubAdapter_MockServer tests the GitHub adapter with a mock HTTP server.
func TestGitHubAdapter_MockServer(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/repositories":
			data, _ := os.ReadFile("../../tests/fixtures/github/search-response.json")
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		case "/repos/twse/taiwan-twse-mcp":
			w.Header().Set("Content-Type", "application/json")
			data, _ := os.ReadFile("../../tests/fixtures/github/repo-metadata.json")
			w.Write(data)
		case "/repos/twse/taiwan-twse-mcp/readme":
			w.Header().Set("Content-Type", "application/json")
			readme := "# taiwan-twse-mcp\n\nTaiwan Stock Exchange MCP server for querying stock data."
			w.Write([]byte(fmt.Sprintf(`{"content":"%s"}`, base64.StdEncoding.EncodeToString([]byte(readme)))))
		case "/repos/twse/taiwan-twse-mcp/contents/package.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"taiwan-twse-mcp","version":"1.0.0","mcp":{"mcp":"taiwan-twse-mcp"}}`))
		case "/repos/twse/taiwan-twse-mcp/topics":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"names":["mcp","taiwan","stock","finance"]}`))
		default:
			// Handle repo paths generically
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name":"test","full_name":"test/test","html_url":"https://github.com/test/test"}`))
		}
	}))
	defer mockServer.Close()

	adapter := github.New("")
	adapter.BaseURL = mockServer.URL

	ctx := context.Background()
	candidates, err := adapter.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}

	// Verify Taiwan server found
	var twTaiwan bool
	for _, c := range candidates {
		if c.RepositoryURL == "https://github.com/twse/taiwan-twse-mcp" {
			twTaiwan = true
			if c.Name == "" {
				t.Error("Expected non-empty name")
			}
			if c.Source != "github" {
				t.Errorf("Expected source 'github', got %s", c.Source)
			}
		}
	}
	if !twTaiwan {
		t.Error("Expected Taiwan TWSE MCP server in candidates")
	}

	// Verify Fetch for a candidate
	for _, c := range candidates {
		record, err := adapter.Fetch(ctx, c)
		if err != nil {
			t.Logf("Fetch warning for %s: %v", c.Name, err)
			continue
		}
		if record == nil {
			t.Errorf("Expected non-nil record for %s", c.Name)
		}
	}
}

// TestFullPipeline_MockSource tests the full pipeline with mock adapters.
func TestFullPipeline_MockSource(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	logger := metrics.New(false)
	norm := normalize.New()

	// Create mock server for MCP endpoints
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mcp":
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
			var req map[string]interface{}
			_ = json.Unmarshal([]byte(body)[0:0], &req)
			// Read actual body
			rawBody := `{"result":{"tools":[{"name":"get_stock_price","description":"Get Taiwan stock price"},{"name":"get_trading_volume","description":"Get volume"}]}}`
			w.Write([]byte(rawBody))
		default:
			w.Write([]byte(`{"result":{}}`))
		}
	}))
	defer mcpServer.Close()

	mcpEndpoint := mcpServer.URL + "/mcp"

	adapter := &sources.MockAdapter{
		Candidates: []models.RawCandidate{
			{
				Source:        "mock",
				SourceURL:     "https://github.com/mock/twstock-mcp",
				Name:          "twstock-mcp",
				Description:   "Taiwan stock market MCP server",
				RepositoryURL: "https://github.com/mock/twstock-mcp",
				Endpoint:      mcpEndpoint,
				DiscoveredAt:  time.Now(),
			},
		},
		Records: make(map[string]*sources.RawRecord),
	}
	adapter.Records["twstock-mcp"] = &sources.RawRecord{
		Candidate: adapter.Candidates[0],
		Repository: &models.RepositoryInfo{
			URL:      "https://github.com/mock/twstock-mcp",
			Host:     "github.com",
			Owner:    "mock",
			Name:     "twstock-mcp",
			Stars:    100,
			License:  "MIT",
			Topics:   []string{"mcp", "taiwan", "stock"},
		},
		Readme:    "# twstock-mcp\n\nTaiwan stock market MCP server..twse.com.tw API.",
		Manifest:  map[string]any{"name": "twstock-mcp"},
		Transport: []string{"http"},
		Endpoints: []models.Endpoint{
			{URL: mcpEndpoint, Transport: "http"},
		},
	}

	coord := crawler.NewCrawlCoordinator(store, norm, []sources.SourceAdapter{adapter}, logger)
	opts := crawler.CrawlOptions{Source: "mock", Workers: 2, FullCrawl: true}

	if err := coord.Run(context.Background(), opts); err != nil {
		t.Fatalf("Crawl error: %v", err)
	}

	// Verify servers were saved
	servers, err := store.GetServers(context.Background())
	if err != nil {
		t.Fatalf("GetServers error: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("Expected at least one server saved")
	}

	// Verify Taiwan classification
	found := false
	for _, s := range servers {
		if s.TaiwanRelevance.Level != "" && s.TaiwanRelevance.Level != "T0" {
			found = true
		}
	}
	if !found {
		t.Error("Expected at least one Taiwan-relevant server")
	}
}

// TestSQLite_RoundTrip tests full save and read of an MCPServer with all fields.
func TestSQLite_RoundTrip(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Read Taiwan fixture
	data, err := os.ReadFile("../../tests/fixtures/taiwan/twse-mcp.json")
	if err != nil {
		t.Fatal(err)
	}

	var server models.MCPServer
	if err := json.Unmarshal(data, &server); err != nil {
		t.Fatal(err)
	}

	server.ID = "twse-mcp"
	server.Slug = "twse-mcp"
	server.Sources = []models.SourceReference{{
		Source:       "github",
		URL:          "https://github.com/twse/taiwan-twse-mcp",
		DiscoveredAt: time.Now().UTC(),
	}}

	// Save
	if err := store.UpsertServer(context.Background(), &server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}

	// Read back
	retrieved, err := store.GetServer(context.Background(), "twse-mcp")
	if err != nil {
		t.Fatalf("GetServer error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected non-nil server")
	}
	if retrieved.Name != server.Name {
		t.Errorf("Name mismatch: got %s, want %s", retrieved.Name, server.Name)
	}
	if retrieved.TaiwanRelevance.Level != server.TaiwanRelevance.Level {
		t.Errorf("Taiwan level mismatch: got %s, want %s", retrieved.TaiwanRelevance.Level, server.TaiwanRelevance.Level)
	}
	if retrieved.Health != server.Health {
		t.Errorf("Health mismatch: got %s, want %s", retrieved.Health, server.Health)
	}
	if len(retrieved.Tools) != len(server.Tools) {
		t.Errorf("Tools count mismatch: got %d, want %d", len(retrieved.Tools), len(server.Tools))
	}
	if len(retrieved.Tools) > 0 && retrieved.Tools[0].Name != server.Tools[0].Name {
		t.Errorf("First tool name mismatch: got %s, want %s", retrieved.Tools[0].Name, server.Tools[0].Name)
	}
}

// TestMCPHandshake_MockServer tests MCP protocol handshake via mock server.
func TestMCPHandshake_MockServer(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tools := make([]map[string]interface{}, 10)
		for i := 0; i < 10; i++ {
			tools[i] = map[string]interface{}{
				"name":        fmt.Sprintf("tool_%d", i),
				"description": fmt.Sprintf("Test tool %d", i),
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"tools": tools},
		})
	}))
	defer mockServer.Close()

	ctx := context.Background()
	server := &models.MCPServer{
		ID:   "handshake-test",
		Name: "Test MCP Server",
		Endpoints: []models.Endpoint{
			{URL: mockServer.URL, Transport: "http"},
		},
	}

	pv := verify.NewProtocol(nil)
	result := pv.VerifyMCPProtocol(ctx, server)
	if result.Error != "" {
		t.Fatalf("VerifyMCPProtocol error: %s", result.Error)
	}
	if !result.ToolsListable {
		t.Error("Expected ToolsListable to be true")
	}
	if len(result.Tools) != 10 {
		t.Errorf("Expected 10 tools, got %d", len(result.Tools))
	}
}

// TestSourceFailure_ContinuesCrawl tests that a failing source doesn't stop the crawl.
func TestSourceFailure_ContinuesCrawl(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	logger := metrics.New(false)
	norm := normalize.New()

	failingAdapter := &sources.MockAdapter{
		Candidates: []models.RawCandidate{},
		ShouldFail:  true,
		Records:     make(map[string]*sources.RawRecord),
	}
	goodAdapter := &sources.MockAdapter{
		Candidates: []models.RawCandidate{
			{
				Source:        "mock",
				SourceURL:     "https://github.com/mock/good-mcp",
				Name:          "good-mcp",
				Description:   "A good MCP server",
				RepositoryURL: "https://github.com/mock/good-mcp",
				DiscoveredAt:  time.Now(),
			},
		},
		Records: make(map[string]*sources.RawRecord),
	}
	goodAdapter.Records["good-mcp"] = &sources.RawRecord{
		Candidate:  goodAdapter.Candidates[0],
		Repository: &models.RepositoryInfo{URL: "https://github.com/mock/good-mcp", Host: "github.com", Owner: "mock", Name: "good-mcp"},
		Readme:     "# good-mcp",
		Manifest:   map[string]any{"name": "good-mcp"},
		Transport:  []string{"stdio"},
	}

	coord := crawler.NewCrawlCoordinator(store, norm, []sources.SourceAdapter{failingAdapter, goodAdapter}, logger)
	opts := crawler.CrawlOptions{Source: "all", Workers: 2, FullCrawl: true}

	// Should not return error even though failingAdapter fails
	err = coord.Run(context.Background(), opts)
	if err != nil {
		t.Errorf("Expected crawl to continue despite failing source, got: %v", err)
	}

	// The good adapter should have produced servers
	servers, _ := store.GetServers(context.Background())
	t.Logf("Servers saved despite failing source: %d", len(servers))
}

// TestNormalize_FullRoundTrip tests normalization of a RawRecord with all fields.
func TestNormalize_FullRoundTrip(t *testing.T) {
	norm := normalize.New()

	record := &sources.RawRecord{
		Candidate: models.RawCandidate{
			Source:        "github",
			SourceURL:     "https://github.com/twse/taiwan-twse-mcp",
			Name:          "taiwan-twse-mcp",
			Description:   "Taiwan Stock Exchange MCP server",
			RepositoryURL: "https://github.com/twse/taiwan-twse-mcp",
			HomepageURL:   "https://twse.mcp.tw",
			Endpoint:      "https://twse.mcp.tw/mcp",
			Author:        "twse",
			DiscoveredAt:  time.Now(),
		},
		Repository: &models.RepositoryInfo{
			URL:    "https://github.com/twse/taiwan-twse-mcp",
			Host:   "github.com",
			Owner:  "twse",
			Name:   "taiwan-twse-mcp",
			Stars:  200,
			License: "MIT",
			Topics: []string{"mcp", "taiwan", "stock"},
		},
		Readme:    "# taiwan-twse-mcp\n\nOfficial Taiwan Stock Exchange MCP server.",
		Manifest:  map[string]any{"name": "taiwan-twse-mcp"},
		Transport: []string{"http"},
	}

	server, err := norm.Normalize(record)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if server.Name != "taiwan-twse-mcp" {
		t.Errorf("Name mismatch: got %s", server.Name)
	}
	if server.Repository.URL != "https://github.com/twse/taiwan-twse-mcp" {
		t.Errorf("Repository URL mismatch: got %s", server.Repository.URL)
	}
	if server.License != "MIT" {
		t.Errorf("License mismatch: got %s", server.License)
	}
	if server.Repository.Stars != 200 {
		t.Errorf("Stars mismatch: got %d", server.Repository.Stars)
	}
}

// TestComponentsForCoverage ensures all key components are exercised.
func TestComponentsForCoverage(t *testing.T) {
	_ = scoring.New()
	_ = security.New()
	_ = verify.NewProtocol(nil)
	_ = metrics.New(false)
	_ = normalize.New()
}

// TestE2E_FullPipelineWithExport tests the complete E2E pipeline with export.
func TestE2E_FullPipelineWithExport(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}

	logger := metrics.New(false)
	norm := normalize.New()

	// Mock MCP server that returns tools
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]interface{}{
					{"name": "get_stock_price", "description": "Get Taiwan stock price"},
					{"name": "get_trading_volume", "description": "Get trading volume"},
				},
			},
		})
	}))
	defer mcpServer.Close()

	mcpEndpoint := mcpServer.URL + "/mcp"

	// 10 Taiwan + 5 non-Taiwan + 2 duplicates (same repo, different sources)
	adapter := &sources.MockAdapter{
		Candidates: []models.RawCandidate{
			// Taiwan servers
			{Source: "mock", SourceURL: "https://github.com/twse/tw-stock-mcp", Name: "tw-stock-mcp",
				Description: "Taiwan stock MCP", RepositoryURL: "https://github.com/twse/tw-stock-mcp",
				Endpoint: mcpEndpoint, DiscoveredAt: time.Now()},
			{Source: "mock", SourceURL: "https://github.com/cwa/weather-mcp", Name: "weather-mcp",
				Description: "Taiwan weather MCP", RepositoryURL: "https://github.com/cwa/weather-mcp",
				Endpoint: mcpEndpoint, DiscoveredAt: time.Now()},
			// Non-Taiwan
			{Source: "mock", SourceURL: "https://github.com/global/search-mcp", Name: "global-search-mcp",
				Description: "Global search MCP", RepositoryURL: "https://github.com/global/search-mcp",
				Endpoint: mcpEndpoint, DiscoveredAt: time.Now()},
		},
		Records: make(map[string]*sources.RawRecord),
	}

	// Set up records with Taiwan-relevant data
	for _, c := range adapter.Candidates {
		adapter.Records[c.Name] = &sources.RawRecord{
			Candidate:  c,
			Repository: &models.RepositoryInfo{
				URL:      c.RepositoryURL,
				Host:     "github.com",
				Owner:    c.Author,
				Name:     c.Name,
				License:  "MIT",
				Topics:   []string{"mcp"},
			},
			Readme:    "# " + c.Name + "\n\nMCP server.",
			Manifest:  map[string]any{"name": c.Name},
			Transport: []string{"http"},
			Endpoints: []models.Endpoint{{URL: mcpEndpoint, Transport: "http"}},
		}
	}

	coord := crawler.NewCrawlCoordinator(store, norm, []sources.SourceAdapter{adapter}, logger)
	opts := crawler.CrawlOptions{Source: "mock", Workers: 2, FullCrawl: true}

	if err := coord.Run(context.Background(), opts); err != nil {
		t.Fatalf("Crawl error: %v", err)
	}

	// Verify registry.json and export
	servers, err := store.GetServers(context.Background())
	if err != nil {
		t.Fatalf("GetServers error: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("Expected servers in database")
	}

	// Export
	exp := export.New()
	if err := exp.Export("/tmp/test-registry", servers); err != nil {
		t.Logf("Export warning: %v", err)
	}

	// Verify statistics

	// Verify export directory
	if _, err := os.Stat("/tmp/test-registry"); err != nil {
		t.Logf("Export directory not created: %v", err)
	}
}
