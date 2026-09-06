package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"log/slog"
)

// setupIntegrationServer creates a fully wired test server with routes.
func setupIntegrationServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "integration.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}
	es := storage.NewEntityStore(store.DB())
	if err := es.ApplySchemaV2(ctx); err != nil {
		t.Fatalf("Failed to apply schema v2: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(DefaultConfig(), store, logger)

	// Use the full route table via a test server
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	ts := httptest.NewServer(s.rateLim.Middleware(s.corsMiddleware(mux)))
	t.Cleanup(ts.Close)

	return s, ts
}

func seedData(t *testing.T, s *Server) {
	t.Helper()
	ctx := context.Background()
	es := storage.NewEntityStore(s.store.DB())

	entities := []*models.Entity{
		{
			ID:          "mcp-001",
			Name:        "Taiwan Financial MCP Server",
			Slug:        "taiwan-financial-mcp",
			Description: "MCP server providing Taiwan financial data APIs including TWSE, TPEx",
			Classification: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.95,
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Score:       75,
				Level:       models.TaiwanRelevanceLevelT4,
				Evidence:    []models.Evidence{{Type: "taiwan_financial_api", Score: 35}},
			},
			MCPIdentity: models.MCPIdentity{
				Status:     models.MCPIdentityStatusRuntimeVerified,
			},
			EntityStatus: models.EntityStatusVerified,
			Quality: models.QualityScore{
				Score: 92,
				Grade: models.QualityGradeA,
				Components: models.QualityComponents{
					DataSource:    20,
					Maintenance:   15,
					Documentation: 10,
					MCPCompliance: 15,
					ToolSchema:    10,
					Health:        10,
					Repository:    5,
					License:       5,
					Security:      5,
					Community:     2,
				},
			},
		},
		{
			ID:          "mcp-002",
			Name:        "Weather Data MCP",
			Slug:        "weather-data-mcp",
			Description: "OpenWeather-compatible MCP server with Taiwan region support",
			Classification: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.90,
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Score: 25,
				Level: models.TaiwanRelevanceLevelT2,
			},
			MCPIdentity: models.MCPIdentity{
				Status:     models.MCPIdentityStatusRuntimeVerified,
			},
			EntityStatus: models.EntityStatusVerified,
			Quality: models.QualityScore{
				Score: 78,
				Grade: models.QualityGradeB,
			},
		},
		{
			ID:          "ai-001",
			Name:        "Taiwan NLP Toolkit",
			Slug:        "taiwan-nlp-toolkit",
			Description: "AI toolkit for Traditional Chinese NLP with Taiwan model support",
			Classification: models.ClassificationResult{
				Primary:    models.PrimaryClassificationAITool,
				Confidence: 0.85,
			},
			TaiwanRelevance: models.TaiwanRelevance{
				Score: 55,
				Level: models.TaiwanRelevanceLevelT3,
			},
			EntityStatus: models.EntityStatusVerified,
			Quality: models.QualityScore{
				Score: 65,
				Grade: models.QualityGradeC,
			},
		},
	}

	for _, e := range entities {
		if err := es.Save(ctx, e); err != nil {
			t.Fatalf("Failed to seed entity %s: %v", e.ID, err)
		}
	}
}

// TestIntegrationHealth checks the full health endpoint flow.
func TestIntegrationHealth(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if health.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", health.Status)
	}
	if health.DBCount != 3 {
		t.Errorf("expected 3 entities in DB, got %d", health.DBCount)
	}
}

// TestIntegrationServersList checks the servers list endpoint.
func TestIntegrationServersList(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	// Only MCP_SERVER entities with RUNTIME_VERIFIED should appear
	resp, err := http.Get(ts.URL + "/api/v1/servers?page=1&limit=50")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var serversResp ServersResponse
	if err := json.NewDecoder(resp.Body).Decode(&serversResp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if len(serversResp.Servers) != 2 {
		t.Errorf("expected 2 MCP server views, got %d", len(serversResp.Servers))
	}
}

// TestIntegrationServerByID checks fetching a single server by ID.
func TestIntegrationServerByID(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	t.Run("existing server", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/servers/mcp-001")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var serverResp ServerResponse
		if err := json.NewDecoder(resp.Body).Decode(&serverResp); err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}

		if serverResp.Server.Name != "Taiwan Financial MCP Server" {
			t.Errorf("expected 'Taiwan Financial MCP Server', got %s", serverResp.Server.Name)
		}
		if serverResp.Server.ID != "mcp-001" {
			t.Errorf("expected ID 'mcp-001', got %s", serverResp.Server.ID)
		}
	})

	t.Run("non-existing server", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/servers/nonexistent")
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// TestIntegrationSearch checks the search endpoint with filters.
func TestIntegrationSearch(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	t.Run("keyword search", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/search?q=taiwan")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var searchResp SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}

		if len(searchResp.Results) != 2 {
			t.Errorf("expected 2 results for 'taiwan', got %d", len(searchResp.Results))
		}
	})

	t.Run("level filter", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/search?q=taiwan&level=T4")
		if err != nil {
			t.Fatalf("Failed: %v", err)
		}
		defer resp.Body.Close()

		var searchResp SearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}

		if len(searchResp.Results) != 1 {
			t.Errorf("expected 1 result for T4+taiwan, got %d", len(searchResp.Results))
		}
	})
}

// TestIntegrationRegistry checks the registry endpoint.
func TestIntegrationRegistry(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/registry")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var regResp RegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if regResp.TotalServers != 3 {
		t.Errorf("expected 3 total servers, got %d", regResp.TotalServers)
	}
	if regResp.TaiwanRelevant != 3 {
		t.Errorf("expected 3 taiwan relevant, got %d", regResp.TaiwanRelevant)
	}
	if regResp.SchemaVersion == "" {
		t.Error("expected non-empty schema_version")
	}
}

// TestIntegrationStatistics checks the statistics endpoint.
func TestIntegrationStatistics(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	resp, err := http.Get(ts.URL + "/api/v1/statistics")
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if stats["total_servers"].(float64) != 3 {
		t.Errorf("expected total_servers 3, got %v", stats["total_servers"])
	}
	if stats["taiwan_relevant"].(float64) != 3 {
		t.Errorf("expected taiwan_relevant 3, got %v", stats["taiwan_relevant"])
	}

	byLevel, ok := stats["by_level"].(map[string]interface{})
	if !ok {
		t.Fatal("expected by_level to be a map")
	}
	if byLevel["T4"].(float64) != 1 {
		t.Errorf("expected 1 T4 server, got %v", byLevel["T4"])
	}
}

// TestIntegrationCORS checks that CORS headers are set.
func TestIntegrationCORS(t *testing.T) {
	s, ts := setupIntegrationServer(t)
	seedData(t, s)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/servers", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %s",
			resp.Header.Get("Access-Control-Allow-Origin"))
	}
}
