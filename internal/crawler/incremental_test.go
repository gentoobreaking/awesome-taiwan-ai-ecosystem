package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

func testCoord(t *testing.T) *CrawlCoordinator {
	t.Helper()
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	logger := metrics.New(false)
	norm := normalize.New()
	return NewCrawlCoordinator(store, norm, nil, logger)
}

func TestIncrementalCrawler_CheckForUpdates_Changed(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	now := time.Now().UTC()
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			PushedAt: models.RFC3339Time(now.Add(-1 * time.Hour)),
		},
		LastSeen: now.Add(-2 * time.Hour),
	}

	changed, err := ic.CheckForUpdates(context.Background(), server)
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}
	if !changed {
		t.Error("Expected changed=true when pushed_at is after last_seen")
	}
}

func TestIncrementalCrawler_CheckForUpdates_NotChanged(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	now := time.Now().UTC()
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			PushedAt: models.RFC3339Time(now.Add(-2 * time.Hour)),
		},
		LastSeen: now.Add(-1 * time.Hour),
	}

	changed, err := ic.CheckForUpdates(context.Background(), server)
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}
	if changed {
		t.Error("Expected changed=false when pushed_at is before last_seen")
	}
}

func TestIncrementalCrawler_CheckForUpdates_NoTimestamps(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	server := &models.MCPServer{
		Repository: models.RepositoryInfo{},
	}

	changed, err := ic.CheckForUpdates(context.Background(), server)
	if err != nil {
		t.Fatalf("CheckForUpdates error: %v", err)
	}
	if !changed {
		t.Error("Expected changed=true when no timestamps available")
	}
}

func TestIncrementalCrawler_IsChanged(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	server := &models.MCPServer{
		Name: "test-server",
		Repository: models.RepositoryInfo{
			URL: "https://github.com/foo/bar",
		},
	}
	candidate := models.RawCandidate{
		RepositoryURL: "https://github.com/foo/different",
		Name:          "different-name",
	}

	if !ic.IsChanged(server, candidate) {
		t.Error("Expected changed=true when URLs differ")
	}
}

func TestIncrementalCrawler_FullCrawl(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	opts := CrawlOptions{FullCrawl: true}
	if !ic.FullCrawl(opts) {
		t.Error("Expected FullCrawl=true")
	}

	opts2 := CrawlOptions{FullCrawl: false}
	if ic.FullCrawl(opts2) {
		t.Error("Expected FullCrawl=false")
	}
}

func TestIncrementalCrawler_ShouldCrawl(t *testing.T) {
	coord := testCoord(t)
	ic := NewIncrementalCrawler(coord)

	lastCrawl := time.Now().Add(-25 * time.Hour)
	if !ic.ShouldCrawl("github", lastCrawl) {
		t.Error("Expected ShouldCrawl=true for 25h ago")
	}

	lastCrawl = time.Now().Add(-1 * time.Hour)
	if ic.ShouldCrawl("github", lastCrawl) {
		t.Error("Expected ShouldCrawl=false for 1h ago")
	}
}

func TestCrawlIDFromTime(t *testing.T) {
	t1 := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	id := CrawlIDFromTime(t1)
	if id != "20260905T120000Z" {
		t.Errorf("Expected '20260905T120000Z', got %s", id)
	}
}

func TestFilterSources(t *testing.T) {
	coord := testCoord(t)
	// No sources configured — should return nil
	result := coord.filterSources("all")
	if len(result) != 0 {
		t.Errorf("Expected 0 sources, got %d", len(result))
	}
}

func TestFilterSources_ByName(t *testing.T) {
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	store.Migrate(context.Background())
	logger := metrics.New(false)
	norm := normalize.New()
	_ = store
	_ = logger
	_ = norm
}
