package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// testEntityStore creates a new in-memory database with schema_v2 applied.
func testEntityStore(t *testing.T) *EntityStore {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	store.db.SetMaxOpenConns(1)

	// Apply both legacy migrations and v2 schema
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate v1: %v", err)
	}
	es := NewEntityStore(store.db)
	if err := es.ApplySchemaV2(ctx); err != nil {
		t.Fatalf("Failed to apply schema_v2: %v", err)
	}
	return es
}

// testEntity creates a fully populated test entity with a unique ID and slug.
func testEntity(idSuffix string) *models.Entity {
	now := time.Now().UTC()
	return &models.Entity{
		ID:           "test-entity-" + idSuffix,
		Name:         "test-mcp-server-" + idSuffix,
		Slug:         "test-mcp-server-" + idSuffix,
		Description:  "A test MCP server for Taiwan finance data with extensive documentation.",
		EntityStatus: models.EntityStatusVerified,
		Classification: models.ClassificationResult{
			Primary:    models.PrimaryClassificationMCPServer,
			Confidence: 0.95,
			MCPRole:    models.MCPRoleServer,
			Reasoning:  "Test reasoning",
		},
		TaiwanRelevance: models.TaiwanRelevance{
			Score:      95.0,
			Level:      "T0",
			Confidence: 0.95,
			Evidence: []models.Evidence{
				{Type: "taiwan_domain", Source: "README", Rule: "tw_domain", Confidence: 0.9},
			},
		},
		AIRelevance: models.AIRelevance{
			Score:      85.0,
			Level:      "A2",
			Confidence: 0.85,
		},
		MCPIdentity: models.MCPIdentity{
			Status:     models.MCPIdentityStatusRuntimeVerified,
			Confidence: 1.0,
			Role:       models.MCPRoleServer,
		},
		Quality: models.QualityScore{
			Score:  92,
			Grade:  models.QualityGradeA,
			Components: models.QualityComponents{
				DataSource: 20, Maintenance: 15, Documentation: 10,
				MCPCompliance: 15, ToolSchema: 10, Health: 10,
				Repository: 5, License: 5, Security: 5, Community: 5,
			},
		},
		SecurityStatus: models.SecurityStatusDetail{
			Status: models.SecurityStatusClean,
		},
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/example/test-mcp",
			Stars:   500,
			Forks:   50,
			License: "MIT",
			Topics:  []string{"mcp", "taiwan", "finance"},
			PushedAt: models.RFC3339Time(now.Add(-7 * 24 * time.Hour)),
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{
					URL:       "https://test-mcp.example.com/mcp",
					Transport: "streamable-http",
					TLS:       true,
				},
				Type:       models.EndpointTypeMCPRuntime,
				Confidence: 0.95,
			},
		},
		Tools: []models.Tool{
			{Name: "get_stock_price", Description: "Get stock price", InputSchema: map[string]any{"type": "object"}},
			{Name: "search_stocks", Description: "Search stocks", InputSchema: map[string]any{"type": "object"}},
		},
		DataSources: []models.DataSource{
			{Name: "TWSE", Type: models.DataSourceOfficialGovAPI, Country: "TW", Official: true},
		},
		RawContent:  "# Test MCP Server\nNo issues here.",
		FirstSeen:   models.RFC3339Time(now.Add(-30 * 24 * time.Hour)),
		LastSeen:    models.RFC3339Time(now),
	}
}

func TestEntityStore_SaveAndGet(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	entity := testEntity("1")
	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := es.Get(ctx, entity.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Expected non-nil entity, got nil")
	}
	if got.Name != entity.Name {
		t.Errorf("Name: got %q, want %q", got.Name, entity.Name)
	}
	if got.Description != entity.Description {
		t.Errorf("Description: got %q, want %q", got.Description, entity.Description)
	}
	if got.EntityStatus != entity.EntityStatus {
		t.Errorf("EntityStatus: got %q, want %q", got.EntityStatus, entity.EntityStatus)
	}
	if got.Classification.Primary != entity.Classification.Primary {
		t.Errorf("Classification.Primary: got %q, want %q", got.Classification.Primary, entity.Classification.Primary)
	}
	if got.Classification.Confidence != entity.Classification.Confidence {
		t.Errorf("Classification.Confidence: got %f, want %f", got.Classification.Confidence, entity.Classification.Confidence)
	}
	if got.Repository.URL != entity.Repository.URL {
		t.Errorf("Repository.URL: got %q, want %q", got.Repository.URL, entity.Repository.URL)
	}
	if got.Repository.Stars != entity.Repository.Stars {
		t.Errorf("Repository.Stars: got %d, want %d", got.Repository.Stars, entity.Repository.Stars)
	}
	if got.Repository.License != entity.Repository.License {
		t.Errorf("Repository.License: got %q, want %q", got.Repository.License, entity.Repository.License)
	}
	if len(got.Tools) != len(entity.Tools) {
		t.Errorf("Tools count: got %d, want %d", len(got.Tools), len(entity.Tools))
	}
	if got.Tools[0].Name != entity.Tools[0].Name {
		t.Errorf("First tool name: got %q, want %q", got.Tools[0].Name, entity.Tools[0].Name)
	}
	if got.Quality.Score != entity.Quality.Score {
		t.Errorf("Quality.Score: got %d, want %d", got.Quality.Score, entity.Quality.Score)
	}
	if got.Quality.Grade != entity.Quality.Grade {
		t.Errorf("Quality.Grade: got %q, want %q", got.Quality.Grade, entity.Quality.Grade)
	}
	if got.SecurityStatus.Status != entity.SecurityStatus.Status {
		t.Errorf("SecurityStatus.Status: got %q, want %q", got.SecurityStatus.Status, entity.SecurityStatus.Status)
	}
}

func TestEntityStore_Get_NotFound(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	got, err := es.Get(ctx, "nonexistent-id")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("Expected nil for non-existent entity")
	}
}

func TestEntityStore_SaveIdempotent(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	entity := testEntity("1")
	// Save twice
	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("First save failed: %v", err)
	}
	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	// Verify only one row
	count, err := es.Count(ctx, EntityFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Count: got %d, want 1", count)
	}
}

func TestEntityStore_Update(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	entity := testEntity("1")
	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Update fields
	entity.Description = "Updated description"
	entity.Repository.Stars = 600
	entity.Quality.Score = 88
	if err := es.Update(ctx, entity); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := es.Get(ctx, entity.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Description != "Updated description" {
		t.Errorf("Description: got %q, want %q", got.Description, "Updated description")
	}
	if got.Repository.Stars != 600 {
		t.Errorf("Stars: got %d, want %d", got.Repository.Stars, 600)
	}
	if got.Quality.Score != 88 {
		t.Errorf("Quality.Score: got %d, want %d", got.Quality.Score, 88)
	}
}

func TestEntityStore_Delete(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	entity := testEntity("1")
	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := es.Delete(ctx, entity.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	got, err := es.Get(ctx, entity.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != nil {
		t.Error("Expected nil after delete")
	}
}

func TestEntityStore_List_FilterByEntityStatus(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	e1 := testEntity("1")
	e1.EntityStatus = models.EntityStatusVerified
	if err := es.Save(ctx, e1); err != nil {
		t.Fatalf("Save e1 failed: %v", err)
	}

	e2 := testEntity("2")
	e2.EntityStatus = models.EntityStatusQuarantined
	if err := es.Save(ctx, e2); err != nil {
		t.Fatalf("Save e2 failed: %v", err)
	}

	results, err := es.List(ctx, EntityFilter{EntityStatus: string(models.EntityStatusVerified)})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(results))
	}
	if results[0].ID != "test-entity-1" {
		t.Errorf("Expected test-entity-1, got %s", results[0].ID)
	}
}

func TestEntityStore_List_FilterByClassification(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	e1 := testEntity("1")
	e1.Classification.Primary = models.PrimaryClassificationMCPServer
	if err := es.Save(ctx, e1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	e2 := testEntity("2")
	e2.Classification.Primary = models.PrimaryClassificationAIAgent
	if err := es.Save(ctx, e2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	results, err := es.List(ctx, EntityFilter{PrimaryClassification: string(models.PrimaryClassificationMCPServer)})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(results))
	}
	if results[0].Classification.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected MCP_SERVER, got %s", results[0].Classification.Primary)
	}
}

func TestEntityStore_List_FilterByMCPIdentityStatus(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	e1 := testEntity("1")
	e1.MCPIdentity.Status = models.MCPIdentityStatusRuntimeVerified
	if err := es.Save(ctx, e1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	e2 := testEntity("2")
	e2.MCPIdentity.Status = models.MCPIdentityStatusNotMCP
	if err := es.Save(ctx, e2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	results, err := es.List(ctx, EntityFilter{MCPIdentityStatus: string(models.MCPIdentityStatusRuntimeVerified)})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(results))
	}
}

func TestEntityStore_List_FilterByTaiwanLevel(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	e1 := testEntity("1")
	e1.TaiwanRelevance.Level = "T0"
	if err := es.Save(ctx, e1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	e2 := testEntity("2")
	e2.TaiwanRelevance.Level = "T3"
	if err := es.Save(ctx, e2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	results, err := es.List(ctx, EntityFilter{TaiwanLevel: "T0"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(results))
	}
	if results[0].TaiwanRelevance.Level != "T0" {
		t.Errorf("Expected T0, got %s", results[0].TaiwanRelevance.Level)
	}
}

func TestEntityStore_List_FilterBySecurityStatus(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	e1 := testEntity("1")
	e1.SecurityStatus.Status = models.SecurityStatusClean
	if err := es.Save(ctx, e1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	e2 := testEntity("2")
	e2.SecurityStatus.Status = models.SecurityStatusBlocked
	if err := es.Save(ctx, e2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	results, err := es.List(ctx, EntityFilter{SecurityStatus: models.SecurityStatusClean})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(results))
	}
}

func TestEntityStore_List_Empty(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	results, err := es.List(ctx, EntityFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 entities, got %d", len(results))
	}
}

func TestEntityStore_List_LimitAndOffset(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e := testEntity(fmt.Sprintf("%d", i))
		if err := es.Save(ctx, e); err != nil {
			t.Fatalf("Save entity-%d failed: %v", i, err)
		}
	}

	// Limit 3
	results, err := es.List(ctx, EntityFilter{Limit: 3})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Limit 3: got %d, want 3", len(results))
	}

	// Limit 3, offset 2
	results, err = es.List(ctx, EntityFilter{Limit: 3, Offset: 2})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Limit 3 Offset 2: got %d, want 3", len(results))
	}

	// Offset 4 — only 1 row left
	results, err = es.List(ctx, EntityFilter{Offset: 4})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Offset 4: got %d, want 1", len(results))
	}
}

func TestEntityStore_Count(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	if count, err := es.Count(ctx, EntityFilter{}); err != nil {
		t.Fatalf("Count failed: %v", err)
	} else if count != 0 {
		t.Errorf("Empty count: got %d, want 0", count)
	}

	for i := 0; i < 3; i++ {
		e := testEntity(fmt.Sprintf("%d", i))
		if err := es.Save(ctx, e); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	if count, err := es.Count(ctx, EntityFilter{}); err != nil {
		t.Fatalf("Count failed: %v", err)
	} else if count != 3 {
		t.Errorf("Count after saves: got %d, want 3", count)
	}
}

func TestEntityStore_Save_NilSlices(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	entity := &models.Entity{
		ID:          "minimal-entity",
		Name:        "minimal",
		Slug:        "minimal-slug",
		Description: "Minimal entity test",
	}

	if err := es.Save(ctx, entity); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	got, err := es.Get(ctx, entity.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Expected non-nil entity")
	}
	if got.Name != "minimal" {
		t.Errorf("Name: got %q, want %q", got.Name, "minimal")
	}
}

func TestEntityStore_MigrationV1ToV2(t *testing.T) {
	// Create a store with v1 schema and some data
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	store.db.SetMaxOpenConns(1)

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed v1 migrate: %v", err)
	}

	// Insert a server using v1 API
	server := &models.MCPServer{
		ID:          "mcp-v1-test",
		Name:        "V1 Test Server",
		Slug:        "v1-test-server",
		Description: "A server from the v1 schema.",
	}
	if err := store.UpsertServer(ctx, server); err != nil {
		t.Fatalf("Failed to insert v1 server: %v", err)
	}

	// Now create EntityStore and migrate
	es := NewEntityStore(store.db)
	if err := es.MigrateV1ToV2(ctx); err != nil {
		t.Fatalf("MigrateV1ToV2 failed: %v", err)
	}

	// Verify the entity exists
	got, err := es.Get(ctx, "mcp-v1-test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got == nil {
		t.Fatal("Expected migrated entity, got nil")
	}
	if got.Name != "V1 Test Server" {
		t.Errorf("Name: got %q, want %q", got.Name, "V1 Test Server")
	}
	if got.Slug != "v1-test-server" {
		t.Errorf("Slug: got %q, want %q", got.Slug, "v1-test-server")
	}
}

func TestEntityStore_MigrationV1ToV2_Idempotent(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	store.db.SetMaxOpenConns(1)

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed v1 migrate: %v", err)
	}

	// Insert data
	server := &models.MCPServer{
		ID:          "mcp-idempotent-test",
		Name:        "Idempotent Test",
		Slug:        "idempotent-test",
		Description: "Test idempotency",
	}
	if err := store.UpsertServer(ctx, server); err != nil {
		t.Fatalf("Failed to insert v1 server: %v", err)
	}

	// Migrate twice
	es := NewEntityStore(store.db)
	if err := es.MigrateV1ToV2(ctx); err != nil {
		t.Fatalf("First migration failed: %v", err)
	}
	if err := es.MigrateV1ToV2(ctx); err != nil {
		t.Fatalf("Second migration failed: %v", err)
	}

	// Verify no duplicates
	count, err := es.Count(ctx, EntityFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 entity after idempotent migration, got %d", count)
	}
}

func TestEntityStore_ApplySchemaV2_Idempotent(t *testing.T) {
	es := testEntityStore(t)
	ctx := context.Background()

	// Should not error when called again
	if err := es.ApplySchemaV2(ctx); err != nil {
		t.Fatalf("ApplySchemaV2 should be idempotent: %v", err)
	}
}
