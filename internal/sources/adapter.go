package sources

import (
	"context"
	"errors"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// SourceAdapter defines the interface for all MCP discovery sources.
type SourceAdapter interface {
	Name() string
	Discover(ctx context.Context) ([]models.RawCandidate, error)
	Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error)
}

// ErrNotAvailable indicates the source is unavailable (e.g., blocked by WAF).
var ErrNotAvailable = errors.New("source not available")

// RateLimitConfig defines per-source rate limiting (§40).
type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	MaxConcurrency    int
}
