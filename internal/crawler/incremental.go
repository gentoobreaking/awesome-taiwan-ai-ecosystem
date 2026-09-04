// Package crawler implements the crawl pipeline coordinator.
package crawler

import (
	"context"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// IncrementalCrawler provides incremental crawl functionality (§38, T032).
type IncrementalCrawler struct {
	coordinator *CrawlCoordinator
}

// NewIncrementalCrawler creates an IncrementalCrawler.
func NewIncrementalCrawler(coord *CrawlCoordinator) *IncrementalCrawler {
	return &IncrementalCrawler{coordinator: coord}
}

// CheckForUpdates determines if a server needs to be re-fetched (§38).
// Returns true if the server has changed since the last crawl.
func (ic *IncrementalCrawler) CheckForUpdates(ctx context.Context, server *models.MCPServer) (bool, error) {
	// If PushedAt is available from GitHub, compare with LastSeen
	if !server.Repository.PushedAt.IsZero() && !server.LastSeen.IsZero() {
		if server.Repository.PushedAt.After(server.LastSeen) {
			return true, nil // pushed after last seen — changed
		}
		return false, nil // not pushed since last crawl — skip
	}

	// Fallback: always re-fetch if we don't have timestamps
	return true, nil
}

// IsChanged checks if a candidate differs from the stored server (§38).
func (ic *IncrementalCrawler) IsChanged(server *models.MCPServer, candidate models.RawCandidate) bool {
	// Compare repository URL
	if server.Repository.URL != candidate.RepositoryURL {
		return true
	}

	// Compare name
	if server.Name != "" && server.Name != candidate.Name {
		return true
	}

	// If no comparison possible, assume changed
	return true
}

// FullCrawl returns true if the crawl options specify a full crawl.
func (ic *IncrementalCrawler) FullCrawl(opts CrawlOptions) bool {
	return opts.FullCrawl
}

// ShouldCrawl determines if a source should be crawled based on schedule (§38).
func (ic *IncrementalCrawler) ShouldCrawl(source string, lastCrawl time.Time) bool {
	interval := 24 * time.Hour
	switch source {
	case "github":
		interval = 24 * time.Hour
	case "registry":
		interval = 24 * time.Hour
	case "full":
		interval = 168 * time.Hour // weekly
	}

	return time.Since(lastCrawl) >= interval
}

// CrawlIDFromTime generates a crawl ID from the current time (§37).
func CrawlIDFromTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// RunIncremental runs an incremental crawl for a specific source.
func (ic *IncrementalCrawler) RunIncremental(ctx context.Context, source string) error {
	opts := CrawlOptions{
		Source:    source,
		FullCrawl: false,
		Workers:   4,
	}
	return ic.coordinator.Run(ctx, opts)
}

// RunFullCrawl runs a full crawl of all sources.
func (ic *IncrementalCrawler) RunFullCrawl(ctx context.Context) error {
	opts := CrawlOptions{
		Source:    "all",
		FullCrawl: true,
		Workers:   4,
	}
	return ic.coordinator.Run(ctx, opts)
}
