package metrics

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	logger := New(false)
	if logger == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestNewJSON(t *testing.T) {
	logger := New(true)
	if logger == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestLogger_Info(t *testing.T) {
	logger := New(false)
	logger.Info(context.Background(), "crawl-123", "discover", "source_started", "source", "github")
}

func TestLogger_Warn(t *testing.T) {
	logger := New(false)
	logger.Warn(context.Background(), "crawl-123", "discover", "source_degraded", "source", "github", "error", "timeout")
}

func TestLogger_Error(t *testing.T) {
	logger := New(false)
	logger.Error(context.Background(), "crawl-123", "persist", "server_save_failed", "id", "srv-1", "error", "db error")
}

func TestCrawlMetrics_StartFinishStage(t *testing.T) {
	cm := NewCrawlMetrics()
	cm.StartStage("discover")
	cm.FinishStage("discover", 100, 0)

	stage := cm.GetStage("discover")
	if stage == nil {
		t.Fatal("Expected non-nil stage")
	}
	if stage.Stage != "discover" {
		t.Errorf("Expected stage 'discover', got %s", stage.Stage)
	}
	if stage.ItemCount != 100 {
		t.Errorf("Expected 100 items, got %d", stage.ItemCount)
	}
}

func TestCrawlMetrics_GetStage_NotFound(t *testing.T) {
	cm := NewCrawlMetrics()
	stage := cm.GetStage("nonexistent")
	if stage != nil {
		t.Error("Expected nil for nonexistent stage")
	}
}

func TestCrawlMetrics_FinishStage_NotStarted(t *testing.T) {
	cm := NewCrawlMetrics()
	// Finish a stage that was never started — should be a no-op
	cm.FinishStage("unknown", 0, 0)
}

func TestCrawlMetrics_All(t *testing.T) {
	cm := NewCrawlMetrics()
	cm.StartStage("discover")
	cm.FinishStage("discover", 10, 1)
	cm.StartStage("normalize")
	cm.FinishStage("normalize", 8, 0)

	all := cm.All()
	if len(all) != 2 {
		t.Errorf("Expected 2 stages, got %d", len(all))
	}
}
