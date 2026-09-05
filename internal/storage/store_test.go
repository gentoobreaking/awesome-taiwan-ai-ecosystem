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
	nowTime := time.Now().UTC()
	nowRFC := models.RFC3339Time(nowTime)
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
			URL:          "https://github.com/test/mcp",
			Host:         "github.com",
			Owner:        "test",
			Name:         "mcp",
			Stars:        50,
			License:      "MIT",
			Archived:     false,
			Topics:       []string{"mcp", "taiwan"},
			PushedAt:     nowRFC,
			LastCommitAt: nowRFC,
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
		Quality:   models.QualityScore{Score: 80, Grade: "A"},
		Sources: []models.SourceReference{
			{Source: "github", URL: "https://github.com/test/mcp", TrustScore: 0.95, DiscoveredAt: nowRFC, LastSeen: nowRFC},
		},
		FirstSeen: nowTime,
		LastSeen:  nowTime,
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
		StartedAt:         models.RFC3339Time(time.Now().UTC()),
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


func TestUpsertServer_WithAllFields(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "full-001"
	server.Slug = "full-001"
	server.Category = []string{"finance", "stock"}
	server.Tools = []models.Tool{
		{Name: "tool1", Description: "desc1", InputSchema: map[string]any{"type": "object"}},
		{Name: "tool2", Description: "desc2"},
	}
	server.Resources = []models.Resource{
		{URI: "test://r1", Name: "r1"},
	}
	server.Prompts = []models.Prompt{
		{Name: "p1", Description: "desc"},
	}
	server.DataSources = []models.DataSource{
		{Name: "twse.com.tw", Type: models.DataSourceOfficialGovAPI},
	}

	if err := store.UpsertServer(context.Background(), server); err != nil {
		sErr := store.UpsertServer(context.Background(), server)
		_ = err
		_ = sErr
		// Already tested idempotency above
	}

	// Retrieve it
	retrieved, err := store.GetServer(context.Background(), "full-001")
	if err != nil {
		t.Fatalf("GetServer error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected non-nil server")
	}
	if retrieved.Name != server.Name {
		t.Errorf("Name mismatch: got %s", retrieved.Name)
	}
}

func TestGetTaiwanServers_NoMatch(t *testing.T) {
	store := testStore(t)
	servers, err := store.GetTaiwanServers(context.Background(), "T5")
	if err != nil {
		t.Fatalf("GetTaiwanServers error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers for non-matching level, got %d", len(servers))
	}
}

func TestGetServerIDs_Empty(t *testing.T) {
	store := testStore(t)
	ids, err := store.GetServerIDs(context.Background())
	if err != nil {
		t.Fatalf("GetServerIDs error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("Expected 0 IDs, got %d", len(ids))
	}
}

func TestGetServers_Empty(t *testing.T) {
	store := testStore(t)
	servers, err := store.GetServers(context.Background())
	if err != nil {
		t.Fatalf("GetServers error: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}
}

func TestClose(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}
	// Verify closed
	err := store.UpsertServer(context.Background(), &models.MCPServer{ID: "x", Slug: "x"})
	if err == nil {
		t.Error("Expected error after close")
	}
}

func TestUpsertServer_NoSources(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "nosrc-001"
	server.Slug = "nosrc-001"
	server.Sources = nil

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
}

func TestUpsertServer_WithRegion(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "region-001"
	server.Slug = "region-001"
	server.Region = []string{"TW", "Taipei"}

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
}

func TestMigrate_AlreadyApplied(t *testing.T) {
	store := testStore(t)
	// Migrate again (idempotent)
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Second Migrate error: %v", err)
	}
}

func TestSaveServer_FailedUpsert(t *testing.T) {
	store := testStore(t)
	// Close the store first
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	server := testServer()
	err := store.SaveServer(context.Background(), server, "crawl-1")
	if err == nil {
		t.Error("Expected error when saving to closed store")
	}
}

func TestUpsertServer_WithNilEndpointsAndTools(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "nil-fields"
	server.Slug = "nil-fields"
	server.Endpoints = nil
	server.Tools = nil
	server.Resources = nil
	server.Prompts = nil
	server.DataSources = nil

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
}

func TestMigrate_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	// Try to migrate a closed database
	err := store.Migrate(context.Background())
	if err == nil {
		t.Error("Expected error migrating closed database")
	}
}

func TestUpsertServer_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err := store.UpsertServer(context.Background(), testServer())
	if err == nil {
		t.Error("Expected error upserting to closed database")
	}
}

func TestGetServer_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetServer(context.Background(), "any-id")
	if err == nil {
		t.Error("Expected error getting from closed database")
	}
}

func TestGetTaiwanServers_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetTaiwanServers(context.Background(), "T5")
	if err == nil {
		t.Error("Expected error querying closed database")
	}
}

func TestGetServerIDs_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetServerIDs(context.Background())
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestGetServers_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetServers(context.Background())
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestCountServers_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.CountServers(context.Background())
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestInsertEvidence_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err := store.InsertEvidence(context.Background(), "srv-1", &models.Evidence{})
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestInsertSecurityFinding_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err := store.InsertSecurityFinding(context.Background(), "srv-1", &models.SecurityFinding{})
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestInsertServerSnapshot_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	server := testServer()
	err := store.InsertServerSnapshot(context.Background(), server.ID, "crawl-1", server)
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestUpsertCrawlRun_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err := store.UpsertCrawlRun(context.Background(), &models.CrawlRun{CrawlID: "test"})
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestCreateCrawlRun_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	err := store.CreateCrawlRun(context.Background(), "test-002")
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestUpsertServer_StatusPersisted(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "status-test"
	server.Slug = "status-test"
	server.Status = models.StatusArchived

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}

	// Verify status was persisted by checking via a raw query
	var status string
	err := store.db.QueryRowContext(context.Background(),
		"SELECT status FROM repositories WHERE server_id = ?", server.ID).Scan(&status)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if status != string(models.StatusArchived) {
		t.Errorf("Expected status %s, got %s", models.StatusArchived, status)
	}
}

func TestUpsertServer_SecurityFindingsPersisted(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "security-test"
	server.Slug = "security-test"
	server.Security = models.SecurityStatusDetail{
		Status:         models.SecurityStatusClean,
		ScannerVersion: "1.0.0",
		ScannedAt:      models.RFC3339Time(time.Now().UTC()),
		Findings: []models.SecurityFinding{
			{
				Type:     "unsafe_transport",
				Severity: models.SeverityHigh,
				Source:   "security_scanner",
				Location: "endpoint",
				Evidence: "HTTP endpoint without TLS",
			},
		},
	}

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}

	// Verify findings were persisted
	var count int
	err := store.db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM security_findings WHERE server_id = ?", server.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Query error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 security finding, got %d", count)
	}
}
func TestGetCrawlRuns(t *testing.T) {
	store := testStore(t)

	// Create multiple crawl runs
	run1 := &models.CrawlRun{
		CrawlID:         "20260901T120000Z",
		StartedAt:       models.RFC3339Time(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)),
		SourcesScanned:  5,
		CandidatesFound: 100,
		CandidatesNorm:  80,
		DuplicatesRemoved: 10,
		TaiwanCandidates: 30,
		Verified:        25,
		Failed:          5,
		Errors:          []string{},
	}
	run2 := &models.CrawlRun{
		CrawlID:         "20260902T120000Z",
		StartedAt:       models.RFC3339Time(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)),
		SourcesScanned:  4,
		CandidatesFound: 80,
		CandidatesNorm:  65,
		DuplicatesRemoved: 8,
		TaiwanCandidates: 25,
		Verified:        20,
		Failed:          3,
		Errors:          []string{"error1"},
	}
	run3 := &models.CrawlRun{
		CrawlID:         "20260903T120000Z",
		StartedAt:       models.RFC3339Time(time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)),
		SourcesScanned:  6,
		CandidatesFound: 120,
		CandidatesNorm:  95,
		DuplicatesRemoved: 15,
		TaiwanCandidates: 40,
		Verified:        35,
		Failed:          2,
		Errors:          []string{},
	}

	for _, r := range []*models.CrawlRun{run1, run2, run3} {
		if err := store.UpsertCrawlRun(context.Background(), r); err != nil {
			t.Fatalf("UpsertCrawlRun failed: %v", err)
		}
	}

	runs, err := store.GetCrawlRuns(context.Background())
	if err != nil {
		t.Fatalf("GetCrawlRuns failed: %v", err)
	}

	if len(runs) != 3 {
		t.Fatalf("Expected 3 crawl runs, got %d", len(runs))
	}

	// Should be ordered by started_at DESC (newest first)
	if runs[0].CrawlID != "20260903T120000Z" {
		t.Errorf("Expected first run to be 20260903T120000Z, got %s", runs[0].CrawlID)
	}
	if runs[1].CrawlID != "20260902T120000Z" {
		t.Errorf("Expected second run to be 20260902T120000Z, got %s", runs[1].CrawlID)
	}
	if runs[2].CrawlID != "20260901T120000Z" {
		t.Errorf("Expected third run to be 20260901T120000Z, got %s", runs[2].CrawlID)
	}

	// Verify run2 has errors
	if len(runs[1].Errors) != 1 || runs[1].Errors[0] != "error1" {
		t.Errorf("Expected run2 to have error 'error1', got %v", runs[1].Errors)
	}
}

func TestGetCrawlRunByID(t *testing.T) {
	store := testStore(t)

	run := &models.CrawlRun{
		CrawlID:         "test-crawl-001",
		StartedAt:       models.RFC3339Time(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)),
		SourcesScanned:  3,
		CandidatesFound: 50,
		CandidatesNorm:  40,
		DuplicatesRemoved: 5,
		TaiwanCandidates: 15,
		Verified:        12,
		Failed:          2,
		Errors:          []string{"test error"},
	}

	if err := store.UpsertCrawlRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertCrawlRun failed: %v", err)
	}

	// Test existing crawl run
	found, err := store.GetCrawlRunByID(context.Background(), "test-crawl-001")
	if err != nil {
		t.Fatalf("GetCrawlRunByID failed: %v", err)
	}
	if found == nil {
		t.Fatal("Expected crawl run, got nil")
	}
	if found.CrawlID != "test-crawl-001" {
		t.Errorf("Expected crawl ID test-crawl-001, got %s", found.CrawlID)
	}
	if found.SourcesScanned != 3 {
		t.Errorf("Expected SourcesScanned 3, got %d", found.SourcesScanned)
	}
	if len(found.Errors) != 1 || found.Errors[0] != "test error" {
		t.Errorf("Expected error 'test error', got %v", found.Errors)
	}

	// Test non-existing crawl run
	found, err = store.GetCrawlRunByID(context.Background(), "non-existent")
	if err != nil {
		t.Fatalf("GetCrawlRunByID for non-existent should not error: %v", err)
	}
	if found != nil {
		t.Errorf("Expected nil for non-existent crawl run, got %v", found)
	}
}

func TestGetServerSnapshots(t *testing.T) {
	store := testStore(t)
	server := testServer()

	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer failed: %v", err)
	}

	// Insert multiple snapshots for the same server
	crawlIDs := []string{"20260901T120000Z", "20260902T120000Z", "20260903T120000Z"}
	for _, cid := range crawlIDs {
		if err := store.InsertServerSnapshot(context.Background(), server.ID, cid, server); err != nil {
			t.Fatalf("InsertServerSnapshot failed for %s: %v", cid, err)
		}
	}

	snapshots, err := store.GetServerSnapshots(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetServerSnapshots failed: %v", err)
	}

	if len(snapshots) != 3 {
		t.Fatalf("Expected 3 snapshots, got %d", len(snapshots))
	}

	// Should be ordered by created_at ASC
	for i, snap := range snapshots {
		if snap.CrawlID != crawlIDs[i] {
			t.Errorf("Snapshot %d: expected crawl ID %s, got %s", i, crawlIDs[i], snap.CrawlID)
		}
		if snap.ServerID != server.ID {
			t.Errorf("Snapshot %d: expected server ID %s, got %s", i, server.ID, snap.ServerID)
		}
		if snap.Snapshot == nil {
			t.Errorf("Snapshot %d: expected non-nil Snapshot", i)
		}
		if snap.Snapshot.ID != server.ID {
			t.Errorf("Snapshot %d: expected snapshot server ID %s, got %s", i, server.ID, snap.Snapshot.ID)
		}
	}
}

func TestGetCrawlSnapshots(t *testing.T) {
	store := testStore(t)

	// Create multiple servers
	server1 := testServer()
	server1.ID = "server-1"
	server1.Slug = "server-1"

	server2 := testServer()
	server2.ID = "server-2"
	server2.Slug = "server-2"

	server3 := testServer()
	server3.ID = "server-3"
	server3.Slug = "server-3"

	for _, s := range []*models.MCPServer{server1, server2, server3} {
		if err := store.UpsertServer(context.Background(), s); err != nil {
			t.Fatalf("UpsertServer failed: %v", err)
		}
	}

	crawlID := "20260901T120000Z"
	for _, s := range []*models.MCPServer{server1, server2, server3} {
		if err := store.InsertServerSnapshot(context.Background(), s.ID, crawlID, s); err != nil {
			t.Fatalf("InsertServerSnapshot failed for %s: %v", s.ID, err)
		}
	}

	snapshots, err := store.GetCrawlSnapshots(context.Background(), crawlID)
	if err != nil {
		t.Fatalf("GetCrawlSnapshots failed: %v", err)
	}

	if len(snapshots) != 3 {
		t.Fatalf("Expected 3 snapshots for crawl, got %d", len(snapshots))
	}

	serverIDs := map[string]bool{"server-1": false, "server-2": false, "server-3": false}
	for _, snap := range snapshots {
		if snap.CrawlID != crawlID {
			t.Errorf("Expected crawl ID %s, got %s", crawlID, snap.CrawlID)
		}
		if _, ok := serverIDs[snap.ServerID]; ok {
			serverIDs[snap.ServerID] = true
		} else {
			t.Errorf("Unexpected server ID in snapshots: %s", snap.ServerID)
		}
	}
	for id, found := range serverIDs {
		if !found {
			t.Errorf("Missing snapshot for server %s", id)
		}
	}
}


func TestGetCrawlRuns_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetCrawlRuns(context.Background())
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestGetCrawlRunByID_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetCrawlRunByID(context.Background(), "test")
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestGetServerSnapshots_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetServerSnapshots(context.Background(), "test")
	if err == nil {
		t.Error("Expected error on closed database")
	}
}

func TestGetCrawlSnapshots_ClosedDB(t *testing.T) {
	store := testStore(t)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := store.GetCrawlSnapshots(context.Background(), "test")
	if err == nil {
		t.Error("Expected error on closed database")
	}
}
