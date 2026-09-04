package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestCountServers(t *testing.T) {
	store := testStore(t)
	if err := store.UpsertServer(context.Background(), testServer()); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
	count, err := store.CountServers(context.Background())
	if err != nil {
		t.Fatalf("CountServers error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 server, got %d", count)
	}
}

func TestGetServers(t *testing.T) {
	store := testStore(t)
	s1 := testServer()
	s1.ID = "srv-1"
	s1.Slug = "srv-1"
	s2 := testServer()
	s2.ID = "srv-2"
	s2.Slug = "srv-2"
	if err := store.UpsertServer(context.Background(), s1); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
	if err := store.UpsertServer(context.Background(), s2); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}
	servers, err := store.GetServers(context.Background())
	if err != nil {
		t.Fatalf("GetServers error: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}
}

func TestGetServer_NotFound(t *testing.T) {
	store := testStore(t)
	server, err := store.GetServer(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}
	if server != nil {
		t.Error("Expected nil for nonexistent server")
	}
}

func TestSaveServer(t *testing.T) {
	store := testStore(t)
	server := testServer()
	if err := store.SaveServer(context.Background(), server, "20260905T120000Z"); err != nil {
		t.Fatalf("SaveServer error: %v", err)
	}
}

func TestGetTaiwanServersByLevel(t *testing.T) {
	store := testStore(t)
	server := testServer()
	if err := store.UpsertServer(context.Background(), server); err != nil {
		t.Fatalf("UpsertServer error: %v", err)
	}

	servers, err := store.GetTaiwanServers(context.Background(), "T5")
	if err != nil {
		t.Fatalf("GetTaiwanServers error: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("Expected 1 T5 server, got %d", len(servers))
	}
}

func TestCreateCrawlRun(t *testing.T) {
	store := testStore(t)
	if err := store.CreateCrawlRun(context.Background(), "20260905T120000Z"); err != nil {
		t.Fatalf("CreateCrawlRun error: %v", err)
	}
}

func TestUpsertCrawlRun_Finish(t *testing.T) {
	store := testStore(t)
	run := &models.CrawlRun{
		CrawlID:    "test-001",
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
	}
	if err := store.UpsertCrawlRun(context.Background(), run); err != nil {
		t.Fatalf("UpsertCrawlRun error: %v", err)
	}
}

func TestInsertSecurityFinding_Empty(t *testing.T) {
	store := testStore(t)
	if err := store.InsertSecurityFinding(context.Background(), "srv-1", &models.SecurityFinding{
		Type:     "test",
		Severity: models.SeverityLow,
	}); err != nil {
		t.Fatalf("InsertSecurityFinding error: %v", err)
	}
}

func TestInsertServerSnapshot_Empty(t *testing.T) {
	store := testStore(t)
	server := testServer()
	if err := store.InsertServerSnapshot(context.Background(), server.ID, "crawl-1", server); err != nil {
		t.Fatalf("InsertServerSnapshot error: %v", err)
	}
}

func TestSaveServer_WithToolsAndEndpoints(t *testing.T) {
	store := testStore(t)
	server := testServer()
	server.ID = "srv-complex"
	server.Slug = "srv-complex"
	server.Tools = []models.Tool{
		{Name: "tool1", Description: "desc1"},
		{Name: "tool2", Description: "desc2"},
	}
	server.Endpoints = []models.Endpoint{
		{URL: "https://example.com/mcp", Transport: "sse"},
	}
	server.Resources = []models.Resource{
		{URI: "test://resource", Name: "resource1"},
	}
	server.Prompts = []models.Prompt{
		{Name: "prompt1", Description: "desc"},
	}
	server.DataSources = []models.DataSource{
		{Name: "twse.com.tw", Type: models.DataSourceOfficialGovAPI},
	}
	if err := store.SaveServer(context.Background(), server, "crawl-1"); err != nil {
		t.Fatalf("SaveServer error: %v", err)
	}
}
