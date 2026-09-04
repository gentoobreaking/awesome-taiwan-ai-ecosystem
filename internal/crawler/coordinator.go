// Package crawler implements the crawl pipeline coordinator.
package crawler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/classify"
	"github.com/david/awesome-taiwan-mcp/internal/dedupe"
	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
	"github.com/david/awesome-taiwan-mcp/internal/scoring"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
	"github.com/david/awesome-taiwan-mcp/internal/storage"
	"github.com/david/awesome-taiwan-mcp/internal/crawler/run"
)

// CrawlCoordinator orchestrates the full crawl pipeline (§31).
type CrawlCoordinator struct {
	sources    []sources.SourceAdapter
	normalizer normalize.Normalizer
	dedupEngine *dedupe.DedupEngine
	scorer     *scoring.QualityScorer
	store      *storage.Store
	metrics    *metrics.CrawlMetrics
	logger     *metrics.Logger
	workers    int
}

// CrawlOptions configures a crawl run.
type CrawlOptions struct {
	Source    string // "github", "all", or specific source name
	FullCrawl bool
	Workers   int
}

// NewCrawlCoordinator creates a new coordinator.
func NewCrawlCoordinator(
	store *storage.Store,
	normalizer normalize.Normalizer,
	sourceAdapters []sources.SourceAdapter,
	logger *metrics.Logger,
) *CrawlCoordinator {
	return &CrawlCoordinator{
		sources:    sourceAdapters,
		normalizer: normalizer,
		dedupEngine: dedupe.New(),
		scorer:     scoring.New(),
		store:      store,
		metrics:    metrics.NewCrawlMetrics(),
		logger:     logger,
		workers:    4,
	}
}

// Run executes the full crawl pipeline (§31, §29).
func (c *CrawlCoordinator) Run(ctx context.Context, opts CrawlOptions) error {
	crawlID := time.Now().UTC().Format("20060102T150405Z")
	runMgr := run.New(c.store, crawlID)
	if err := runMgr.Start(ctx); err != nil {
		return fmt.Errorf("start crawl run: %w", err)
	}

	c.metrics.StartStage("crawl")
	c.logger.Info(ctx, crawlID, "crawl", "started", "full", opts.FullCrawl)

	// Filter sources
	activeSources := c.filterSources(opts.Source)

	// Stage 1: Discover + Fetch from each source
	allRawRecords, err := c.discoverAndFetch(ctx, crawlID, activeSources, runMgr)
	if err != nil {
		return fmt.Errorf("discover/fetch: %w", ctx.Err())
	}

	// Stage 2: Normalize
	servers, err := c.normalizeServers(ctx, allRawRecords)
	if err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	runMgr.RecordNormalized(len(servers))

	// Stage 3: Score Taiwan relevance
	for i := range servers {
		result := classify.Score(servers[i])
		servers[i].TaiwanRelevance = models.TaiwanRelevance{
			Level:      result.Level,
			Score:      result.Score,
			Confidence: 1.0,
			Evidence:   result.Evidence,
		}
	}

	// Stage 4: Score quality
	for i := range servers {
		servers[i].Quality = c.scorer.Score(servers[i])
	}

	// Stage 5: Apply identity (CanonicalID)
	for i := range servers {
		id := dedupe.ServerID(servers[i])
		servers[i].ID = id
	}

	// Stage 6: Dedup
	serverSources := make([]*dedupe.ServerSource, len(servers))
	for i, s := range servers {
		trust := models.SourceTrustScores[s.Sources[0].Source]
		if trust == 0 {
			trust = 0.5
		}
		serverSources[i] = &dedupe.ServerSource{Server: s, TrustScore: trust}
	}

	deduped, err := c.dedupEngine.Deduplicate(serverSources)
	if err != nil {
		return fmt.Errorf("dedup: %w", err)
	}
	runMgr.RecordDuplicates(len(servers) - len(deduped))

	// Count Taiwan candidates
	taiwanCount := 0
	for _, s := range deduped {
		if s.TaiwanRelevance.Level != "" && s.TaiwanRelevance.Level != "T0" {
			taiwanCount++
		}
	}
	runMgr.RecordTaiwanCandidates(taiwanCount)

	// Stage 7: Persist
	for _, s := range deduped {
		if err := c.store.SaveServer(ctx, s, crawlID); err != nil {
			runMgr.AddError(err)
			c.logger.Warn(ctx, crawlID, "persist", "server_save_failed", "id", s.ID, "error", err.Error())
		} else {
			runMgr.RecordVerified(1)
		}
	}

	// Stage 8: Finish
	if err := runMgr.Finish(ctx); err != nil {
		return fmt.Errorf("finish crawl run: %w", err)
	}

	c.logger.Info(ctx, crawlID, "crawl", "completed",
		"candidates", runMgr.Run().CandidatesFound,
		"normalized", runMgr.Run().CandidatesNorm,
		"deduped", runMgr.Run().DuplicatesRemoved,
		"taiwan", runMgr.Run().TaiwanCandidates,
	)

	return nil
}

func (c *CrawlCoordinator) filterSources(source string) []sources.SourceAdapter {
	if source == "" || source == "all" {
		return c.sources
	}
	for _, s := range c.sources {
		if s.Name() == source {
			return []sources.SourceAdapter{s}
		}
	}
	return nil
}

func (c *CrawlCoordinator) discoverAndFetch(
	ctx context.Context,
	crawlID string,
	activeSources []sources.SourceAdapter,
	runMgr *run.Manager,
) ([]*sources.RawRecord, error) {
	var mu sync.Mutex
	var allRecords []*sources.RawRecord

	var wg sync.WaitGroup
	for _, src := range activeSources {
		wg.Add(1)
		go func(s sources.SourceAdapter) {
			defer wg.Done()

			c.metrics.StartStage("discover:" + s.Name())
			runMgr.IncSourcesScanned()
			c.logger.Info(ctx, crawlID, "discover", "source_started", "source", s.Name())

			candidates, err := s.Discover(ctx)
			if err != nil {
				runMgr.AddError(fmt.Errorf("source %s discover: %w", s.Name(), err))
				c.logger.Warn(ctx, crawlID, "discover", "source_degraded", "source", s.Name(), "error", err.Error())
				return
			}

			c.logger.Info(ctx, crawlID, "discover", "candidates_found", "source", s.Name(), "count", len(candidates))
			runMgr.RecordCandidate(len(candidates))

			// Fetch each candidate with worker pool
			workerCount := c.workers
			if workerCount < 1 {
				workerCount = 1
			}
			sem := make(chan struct{}, workerCount)

			for _, cand := range candidates {
				select {
				case <-ctx.Done():
					return
				case sem <- struct{}{}:
				}

				go func(cand models.RawCandidate) {
					defer func() { <-sem }()
					record, err := s.Fetch(ctx, cand)
					if err != nil {
						mu.Lock()
						runMgr.AddError(fmt.Errorf("source %s fetch %s: %w", s.Name(), cand.RepositoryURL, err))
						mu.Unlock()
						return
					}
					mu.Lock()
					allRecords = append(allRecords, record)
					mu.Unlock()
				}(cand)
			}

			c.metrics.FinishStage("discover:"+s.Name(), len(candidates), 0)
		}(src)
	}

	wg.Wait()

	return allRecords, nil
}

func (c *CrawlCoordinator) normalizeServers(
	ctx context.Context,
	records []*sources.RawRecord,
) ([]*models.MCPServer, error) {
	servers := make([]*models.MCPServer, 0, len(records))
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.workers)

	for _, record := range records {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(rec *sources.RawRecord) {
			defer wg.Done()
			defer func() { <-sem }()

			server, err := c.normalizer.Normalize(rec)
			if err != nil {
				return
			}
			mu.Lock()
			servers = append(servers, server)
			mu.Unlock()
		}(record)
	}

	wg.Wait()
	return servers, nil
}
