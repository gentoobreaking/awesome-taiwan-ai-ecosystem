package storage

import (
	"context"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}
	return store
}

func testServer() *models.MCPServer {
	now := time.Now().UTC()
	return &models.MCPServer{
		ID:          "sha256:test123",
		Name:        "Test MCP",
		Slug:        "test-mcp",
		Description: "A test MCP server",
		Region:      []string{"TW"},
		Category:    []string{"finance", "stock"},
		TaiwanRelevance: models.TaiwanRelevance{
			Level:      "T5",
			Score:      75,
			Confidence: 1.0,
			Evidence: []models.Evidence{
				{Type: "official_domain", Source: "README", Location: "/", Rule: "test"},
			},
		},
		Repository: models.RepositoryInfo{
			URL:         "https://github.com/test/mcp",
			Host:        "github.com",
			Owner:       "test",
			Name:        "mcp",
			Stars:       50,
			License:     "MIT",
			Archived:    false,
			Topics:      []string{"mcp", "taiwan"},
			PushedAt:    now,
			LastCommitAt: now,
		},
		Endpoints: []models.Endpoint{
			{URL: "https://test-mcp.example.com/mcp", Transport: "streamable-http", TLS: true, Status: "unknown"},
		},
		Transport: []string{"streamable-http"},
		Tools: []models.Tool{
			{Name: "get_stock_price", Description: "Get stock price", InputSchema: map[string]any{"type": "object"}},
		},
		DataSources: []models.DataSource{
			{Name: "TWSE", Type: models.DataSourceOfficialGovAPI, URL: "https://www.twse.com.tw", Country: "TW", Official: true},
		},
		License:   "MIT",
		Status:    models.StatusActive,
		Health:    models.HealthHealthy,
		Quality:   models.QualityScore{Score: 90, Grade: "A"},
		Sources: []models.SourceReference{
			{Source: "github", URL: "https://github.com/test/mcp", TrustScore: 0.95},
		},
		FirstSeen:    now,
		LastSeen:     now,
		LastVerified: now,
	}
}

func TestMigrateIdempotent(t *testing.T) {
	store := testStore(t)

	// Run migration a second time — should not error
	if err := store.Migrate(context.Background()); err != nil {
		t.Errorf("Second migration should not error: %v", err)
	}
}

func TestUpsertServer(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	count, err := store.CountServers(context.Background())
	if err != nil {
		t.Fatalf("CountServers failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 server, got %d", count)
	}
}

func TestUpsertServerIdempotent(t *testing.T) {
	store := testStore(t)
	server := testServer()

	// First insert
	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("First upsert failed: %v", err)
	}
	// Second insert (same server)
	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("Second upsert failed: %v", err)
	}

	count, err := store.CountServers(context.Background())
	if err != nil {
		t.Fatalf("CountServers failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 server after duplicate upsert, got %d", count)
	}
}

func TestGetServer(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	restored, err := store.GetServer(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}

	if restored.ID != server.ID {
		t.Errorf("ID mismatch: %s != %s", restored.ID, server.ID)
	}
	if restored.Name != server.Name {
		t.Errorf("Name mismatch: %s != %s", restored.Name, server.Name)
	}
	if restored.TaiwanRelevance.Level != "T5" {
		t.Errorf("Level mismatch: %s", restored.TaiwanRelevance.Level)
	}
	if len(restored.Sources) != 1 {
		t.Errorf("Sources count: %d", len(restored.Sources))
	}
	if restored.Sources[0].Source != "github" {
		t.Errorf("Source mismatch: %s", restored.Sources[0].Source)
	}
}

func TestGetServerIDs(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	ids, err := store.GetServerIDs(context.Background())
	if err != nil {
		t.Fatalf("GetServerIDs failed: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("Expected 1 ID, got %d", len(ids))
	}
	if ids[0] != server.ID {
		t.Errorf("ID mismatch: %s != %s", ids[0], server.ID)
	}
}

func TestGetTaiwanServers(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	servers, err := store.GetTaiwanServers(context.Background(), "T5")
	if err != nil {
		t.Fatalf("GetTaiwanServers failed: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("Expected 1 T5 server, got %d", len(servers))
	}
}

func TestUpsertCrawlRun(t *testing.T) {
	store := testStore(t)
	run := &models.CrawlRun{
		CrawlID:           "20260905T120000Z",
		StartedAt:         time.Now(),
		CandidatesFound:   100,
		DuplicatesRemoved: 5,
	}

	if err := store.CreateCrawlRun(context.Background(), run.CrawlID); err != nil {
		t.Fatalf("CreateCrawlRun failed: %v", err)
	}
	if err := store.UpsertCrawlRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertCrawlRun failed: %v", err)
	}
}

func TestInsertEvidence(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	ev := &models.Evidence{
		Type:   "official_domain",
		Source: "README",
		Location: "https://github.com/test/mcp",
		Rule:   "official_taiwan_domain",
		Score:  40,
	}
	if err := store.InsertEvidence(context.Background(), server.ID, ev); err != nil {
		t.Errorf("InsertEvidence failed: %v", err)
	}
}

func TestInsertSecurityFinding(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	f := &models.SecurityFinding{
		Type:     "code_execution",
		Severity: models.SeverityHigh,
		Source:   "README",
		Location: "https://github.com/test/mcp",
		Evidence: "os.system('rm -rf /')",
	}
	if err := store.InsertSecurityFinding(context.Background(), server.ID, f); err != nil {
		t.Errorf("InsertSecurityFinding failed: %v", err)
	}
}

func TestInsertServerSnapshot(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	if err := store.InsertServerSnapshot(context.Background(), server.ID, "20260905T120000Z", server); err != nil {
		t.Errorf("InsertServerSnapshot failed: %v", err)
	}
}
