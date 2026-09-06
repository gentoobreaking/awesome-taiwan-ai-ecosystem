package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"log/slog"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	// Create temp database
	dbPath := filepath.Join(t.TempDir(), "test.db")
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	return New(DefaultConfig(), store, logger)
}

func seedTestEntities(t *testing.T, s *Server, entities ...*models.Entity) {
	t.Helper()
	ctx := context.Background()
	es := storage.NewEntityStore(s.store.DB())
	for _, e := range entities {
		if err := es.Save(ctx, e); err != nil {
			t.Fatalf("Failed to seed entity: %v", err)
		}
	}
}

func testEntity(id, name, level string) *models.Entity {
	return &models.Entity{
		ID:          id,
		Name:        name,
		Slug:        name,
		Description: "Test entity " + name,
		Classification: models.ClassificationResult{
			Primary:    models.PrimaryClassificationMCPServer,
		},
		TaiwanRelevance: models.TaiwanRelevance{
			Level: models.TaiwanRelevanceLevel(level),
			Score: 50,
		},
		MCPIdentity: models.MCPIdentity{
			Status:     models.MCPIdentityStatusRuntimeVerified,
		},
		EntityStatus: models.EntityStatusVerified,
		Quality: models.QualityScore{
			Score: 75,
			Grade: models.QualityGradeB,
		},
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s, testEntity("1", "test-server", "T1"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %s", resp.Status)
	}
	if resp.Version != "v0.1" {
		t.Errorf("expected version 'v0.1', got %s", resp.Version)
	}
	if resp.DBCount != 1 {
		t.Errorf("expected db_count 1, got %d", resp.DBCount)
	}
}

func TestServersEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s,
		testEntity("1", "server-1", "T1"),
		testEntity("2", "server-2", "T2"),
		testEntity("3", "server-3", "T3"),
	)

	// Test pagination
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers?page=1&limit=2", nil)
	w := httptest.NewRecorder()

	s.handleServers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ServersResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if len(resp.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(resp.Servers))
	}
	if resp.Pagination.Total != 3 {
		t.Errorf("expected total 3, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Pagination.Page)
	}
	if resp.Pagination.Limit != 2 {
		t.Errorf("expected limit 2, got %d", resp.Pagination.Limit)
	}
}

func TestServersEndpointLevelFilter(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s,
		testEntity("1", "t1-server", "T1"),
		testEntity("2", "t3-server", "T3"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers?level=T3", nil)
	w := httptest.NewRecorder()

	s.handleServers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ServersResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if len(resp.Servers) != 1 {
		t.Errorf("expected 1 server (T3), got %d", len(resp.Servers))
	}
	if resp.Servers[0].Name != "t3-server" {
		t.Errorf("expected t3-server, got %s", resp.Servers[0].Name)
	}
}

func TestServerByIDEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s, testEntity("srv-123", "my-server", "T2"))

	t.Run("existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/servers/srv-123", nil)
		w := httptest.NewRecorder()
		s.handleServerByID(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp ServerResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode: %v", err)
		}
		if resp.Server.Name != "my-server" {
			t.Errorf("expected my-server, got %s", resp.Server.Name)
		}
	})

	t.Run("non-existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/servers/nonexistent", nil)
		w := httptest.NewRecorder()
		s.handleServerByID(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestSearchEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s,
		testEntity("1", "Financial Data MCP", "T3"),
		testEntity("2", "Taiwan Weather Tool", "T2"),
		testEntity("3", "Non-matching Server", "T1"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=taiwan", nil)
	w := httptest.NewRecorder()

	s.handleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if resp.Query != "taiwan" {
		t.Errorf("expected query 'taiwan', got %s", resp.Query)
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
}

func TestRegistryEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s,
		testEntity("1", "server-1", "T1"),
		testEntity("2", "server-2", "T2"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/registry", nil)
	w := httptest.NewRecorder()

	s.handleRegistry(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp RegistryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if resp.TotalServers != 2 {
		t.Errorf("expected 2 servers, got %d", resp.TotalServers)
	}
	if resp.TaiwanRelevant != 2 {
		t.Errorf("expected 2 taiwan relevant, got %d", resp.TaiwanRelevant)
	}
}

func TestStatisticsEndpoint(t *testing.T) {
	s := newTestServer(t)
	seedTestEntities(t, s,
		testEntity("1", "server-1", "T1"),
		testEntity("2", "server-2", "T3"),
		testEntity("3", "server-3", "T0"),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/statistics", nil)
	w := httptest.NewRecorder()

	s.handleStatistics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if stats["total_servers"] != float64(3) {
		t.Errorf("expected total_servers 3, got %v", stats["total_servers"])
	}
	if stats["taiwan_relevant"] != float64(2) {
		t.Errorf("expected taiwan_relevant 2, got %v", stats["taiwan_relevant"])
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/health"},
		{http.MethodPost, "/api/v1/servers"},
		{http.MethodDelete, "/api/v1/servers/123"},
		{http.MethodPost, "/api/v1/search"},
		{http.MethodPost, "/api/v1/registry"},
		{http.MethodPost, "/api/v1/statistics"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Route to the appropriate handler
			switch {
			case tt.path == "/health":
				s.handleHealth(w, req)
			case tt.path == "/api/v1/servers":
				s.handleServers(w, req)
			case tt.path == "/api/v1/statistics":
				s.handleStatistics(w, req)
			case tt.path == "/api/v1/registry":
				s.handleRegistry(w, req)
			default:
				s.handleServerByID(w, req)
			}

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", w.Code)
			}
		})
	}
}

func TestPaginationDefaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	page, limit, err := parsePagination(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page != 1 {
		t.Errorf("expected default page 1, got %d", page)
	}
	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
}

func TestPaginationMaxLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers?limit=999", nil)
	page, limit, err := parsePagination(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 200 {
		t.Errorf("expected capped limit 200, got %d", limit)
	}
	_ = page
}
