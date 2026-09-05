// Package sources defines the SourceAdapter interface and all source adapter implementations.
package sources

import (
	"context"
	"errors"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// SourceAdapter defines the interface for all MCP discovery sources.
// Each adapter is responsible only for Discover (list candidates) and Fetch
// (retrieve full metadata). It must never make final Registry schema decisions.
type SourceAdapter interface {
	// Name returns the source identifier (e.g. "github", "glama").
	Name() string
	// Discover lists raw candidates from this source.
	Discover(ctx context.Context) ([]models.RawCandidate, error)
	// Fetch retrieves full metadata for a candidate.
	Fetch(ctx context.Context, candidate models.RawCandidate) (*RawRecord, error)
}

// RawRecord is the fully fetched candidate with all raw metadata (§7, §9).
type RawRecord struct {
	Candidate    models.RawCandidate
	Repository   *models.RepositoryInfo
	Manifest     map[string]any  // parsed server.json/mcp.json/manifest.json
	Tools        []models.Tool   // from manifest or protocol
	Resources    []models.Resource
	Prompts      []models.Prompt
	Endpoints    []models.Endpoint
	Transport    []string
	Readme       string
	PackageFiles map[string]string // "package.json": "...", "pyproject.toml": "..."
}

// RateLimitConfig defines per-source rate limiting (§40).
var ErrNotAvailable = errors.New("source not available")

type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
	MaxConcurrency    int
}

// MockAdapter is a configurable source adapter for testing the pipeline.
type MockAdapter struct {
	Candidates []models.RawCandidate
	Records    map[string]*RawRecord
	ShouldFail bool
	Delay      time.Duration
}

// NewMockAdapter creates a MockAdapter with default test candidates.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		Candidates: []models.RawCandidate{
			{
				Source:        "mock",
				SourceURL:     "https://github.com/mock/twstock-mcp",
				Name:          "twstock-mcp",
				Description:   "Taiwan stock market MCP server",
				RepositoryURL: "https://github.com/mock/twstock-mcp",
				HomepageURL:   "https://twstock-mcp.example.com",
				Endpoint:      "https://twstock-mcp.example.com/mcp",
				Author:        "mock-owner",
				DiscoveredAt:  time.Now(),
			},
			{
				Source:        "mock",
				SourceURL:     "https://github.com/mock/finmind-mcp",
				Name:          "finmind-mcp",
				Description:   "FinMind financial data MCP",
				RepositoryURL: "https://github.com/mock/finmind-mcp",
				Endpoint:      "https://finmind-mcp.example.com/mcp",
				DiscoveredAt:  time.Now(),
			},
			{
				Source:        "mock",
				SourceURL:     "https://github.com/mock/non-taiwan-mcp",
				Name:          "generic-mcp",
				Description:   "A generic MCP server",
				RepositoryURL: "https://github.com/mock/generic-mcp",
				DiscoveredAt:  time.Now(),
			},
		},
		Records: make(map[string]*RawRecord),
	}
}

func (m *MockAdapter) Name() string {
	return "mock"
}

func (m *MockAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.ShouldFail {
		return nil, &MockError{Source: m.Name(), Msg: "mock adapter configured to fail"}
	}
	return m.Candidates, nil
}

func (m *MockAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*RawRecord, error) {
	if m.ShouldFail {
		return nil, &MockError{Source: m.Name(), Msg: "mock adapter configured to fail"}
	}

	if record, ok := m.Records[candidate.Name]; ok {
		return record, nil
	}

	// Return a default record based on candidate
	return &RawRecord{
		Candidate: candidate,
		Repository: &models.RepositoryInfo{
			URL:      candidate.RepositoryURL,
			Name:     candidate.Name,
			Stars:    10,
			License:  "MIT",
			Topics:   []string{"mcp"},
		},
		Readme:   "# " + candidate.Name + "\n\nTaiwan MCP server.",
		Manifest: map[string]any{
			"name":        candidate.Name,
			"description": candidate.Description,
		},
		Transport: []string{"stdio"},
	}, nil
}

// MockError is a test error type for the MockAdapter.
type MockError struct {
	Source string
	Msg    string
}

func (e *MockError) Error() string {
	return e.Source + ": " + e.Msg
}
