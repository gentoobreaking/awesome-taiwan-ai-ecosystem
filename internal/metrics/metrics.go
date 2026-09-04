// Package metrics provides structured logging and metrics for crawl runs.
package metrics

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Logger is a thread-safe structured logger for crawl pipeline stages.
type Logger struct {
	mu     sync.Mutex
	logger *slog.Logger
}

// New creates a new Logger with human-readable or JSON formatting.
func New(jsonFormat bool) *Logger {
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	return &Logger{
		logger: slog.New(handler),
	}
}

// Info logs a structured info message with crawl context.
func (l *Logger) Info(ctx context.Context, crawlID, stage, event string, attrs ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.InfoContext(ctx, event,
		append([]any{"crawl_id", crawlID, "stage", stage}, attrs...)...)
}

// Warn logs a structured warning.
func (l *Logger) Warn(ctx context.Context, crawlID, stage, event string, attrs ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.WarnContext(ctx, event,
		append([]any{"crawl_id", crawlID, "stage", stage}, attrs...)...)
}

// Error logs a structured error.
func (l *Logger) Error(ctx context.Context, crawlID, stage, event string, attrs ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logger.ErrorContext(ctx, event,
		append([]any{"crawl_id", crawlID, "stage", stage}, attrs...)...)
}

// StageMetrics tracks metrics for a pipeline stage.
type StageMetrics struct {
	Stage       string
	StartedAt   time.Time
	FinishedAt  time.Time
	ItemCount   int
	ErrorCount  int
}

// CrawlMetrics collects metrics across all crawl stages.
type CrawlMetrics struct {
	mu        sync.Mutex
	stages    map[string]*StageMetrics
}

// NewCrawlMetrics creates a new CrawlMetrics.
func NewCrawlMetrics() *CrawlMetrics {
	return &CrawlMetrics{
		stages: make(map[string]*StageMetrics),
	}
}

// StartStage records the start time for a stage.
func (cm *CrawlMetrics) StartStage(stage string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.stages[stage] = &StageMetrics{
		Stage:     stage,
		StartedAt: time.Now().UTC(),
	}
}

// FinishStage records the completion of a stage.
func (cm *CrawlMetrics) FinishStage(stage string, itemCount, errorCount int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if m, ok := cm.stages[stage]; ok {
		m.FinishedAt = time.Now().UTC()
		m.ItemCount = itemCount
		m.ErrorCount = errorCount
	}
}

// GetStage returns metrics for a specific stage.
func (cm *CrawlMetrics) GetStage(stage string) *StageMetrics {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.stages[stage]
}

// All returns all stage metrics.
func (cm *CrawlMetrics) All() map[string]*StageMetrics {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	dup := make(map[string]*StageMetrics, len(cm.stages))
	for k, v := range cm.stages {
		dup[k] = v
	}
	return dup
}
