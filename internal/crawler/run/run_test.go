package run

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

func testStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCrawlRunLifecycle(t *testing.T) {
	store := testStore(t)
	mgr := New(store, "20260905T120000Z")

	if err := mgr.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	mgr.RecordCandidate(100)
	mgr.RecordNormalized(90)
	mgr.RecordDuplicates(10)
	mgr.RecordTaiwanCandidates(5)
	mgr.RecordVerified(85)
	mgr.RecordFailed(2)
	mgr.AddError(context.DeadlineExceeded)

	run := mgr.Run()
	if run.CrawlID != "20260905T120000Z" {
		t.Errorf("Expected CrawlID=20260905T120000Z, got %s", run.CrawlID)
	}
	if run.CandidatesFound != 100 {
		t.Errorf("Expected CandidatesFound=100, got %d", run.CandidatesFound)
	}
	if run.CandidatesNorm != 90 {
		t.Errorf("Expected CandidatesNorm=90, got %d", run.CandidatesNorm)
	}
	if run.DuplicatesRemoved != 10 {
		t.Errorf("Expected DuplicatesRemoved=10, got %d", run.DuplicatesRemoved)
	}
	if run.TaiwanCandidates != 5 {
		t.Errorf("Expected TaiwanCandidates=5, got %d", run.TaiwanCandidates)
	}
	if run.Verified != 85 {
		t.Errorf("Expected Verified=85, got %d", run.Verified)
	}
	if run.Failed != 2 {
		t.Errorf("Expected Failed=2, got %d", run.Failed)
	}
	if len(run.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(run.Errors))
	}

	if err := mgr.Finish(context.Background()); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

func TestCounterConsistency(t *testing.T) {
	store := testStore(t)
	mgr := New(store, "20260905T130000Z")
	_ = mgr.Start(context.Background())

	mgr.RecordCandidate(100)
	mgr.RecordNormalized(90)
	mgr.RecordDuplicates(10)

	run := mgr.Run()
	if run.CandidatesFound != 100 {
		t.Errorf("Expected 100 candidates, got %d", run.CandidatesFound)
	}
	if run.CandidatesNorm != 90 {
		t.Errorf("Expected 90 normalized, got %d", run.CandidatesNorm)
	}
	if run.DuplicatesRemoved != 10 {
		t.Errorf("Expected 10 duplicates, got %d", run.DuplicatesRemoved)
	}
	if run.CandidatesFound-run.CandidatesNorm != run.DuplicatesRemoved {
		t.Error("Counters not consistent")
	}
}
