// Package run provides CrawlRun lifecycle management.
package run

import (
	"context"
	"sync"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
)

// Manager orchestrates a single crawl run's lifecycle and counters.
type Manager struct {
	store   *storage.Store
	crawlID string
	run     *models.CrawlRun
	mu      sync.Mutex
}

// New creates a new crawl run manager.
func New(store *storage.Store, crawlID string) *Manager {
	return &Manager{
		store:   store,
		crawlID: crawlID,
	}
}

// Start initializes and begins a crawl run.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.store.CreateCrawlRun(ctx, m.crawlID); err != nil {
		return err
	}
	m.run = &models.CrawlRun{
		CrawlID:    m.crawlID,
		StartedAt:  time.Now().UTC(),
	}
	return nil
}

// Finish finalizes the crawl run and persists it.
func (m *Manager) Finish(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.FinishedAt = time.Now().UTC()
	return m.store.UpsertCrawlRun(ctx, m.run)
}

// RecordCandidate increments the candidates found counter.
func (m *Manager) RecordCandidate(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.CandidatesFound += count
}

// RecordNormalized increments the normalized counter.
func (m *Manager) RecordNormalized(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.CandidatesNorm += count
}

// RecordDuplicates increments the duplicates removed counter.
func (m *Manager) RecordDuplicates(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.DuplicatesRemoved += count
}

// RecordTaiwanCandidates sets the taiwan candidates count.
func (m *Manager) RecordTaiwanCandidates(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.TaiwanCandidates = count
}

// RecordVerified increments the verified counter.
func (m *Manager) RecordVerified(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.Verified += count
}

// RecordFailed increments the failed counter.
func (m *Manager) RecordFailed(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.Failed += count
}

// AddError appends an error to the run.
func (m *Manager) AddError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.run.Errors == nil {
		m.run.Errors = []string{}
	}
	m.run.Errors = append(m.run.Errors, err.Error())
}

// IncSourcesScanned increments the sources scanned counter.
func (m *Manager) IncSourcesScanned() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.run.SourcesScanned++
}

// CrawlID returns the crawl run ID.
func (m *Manager) CrawlID() string {
	return m.crawlID
}

// Run returns the current CrawlRun metadata.
func (m *Manager) Run() *models.CrawlRun {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.run
}
