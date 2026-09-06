package sources

import (
	"context"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// MockAdapter is a test double for SourceAdapter.
type MockAdapter struct {
	Candidates  []models.RawCandidate
	Records     map[string]*models.RawRecord
	ShouldFail  bool
	Delay       time.Duration
	trustScore  float64
}

// NewMockAdapter creates a MockAdapter with default candidates.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		Candidates: []models.RawCandidate{
			{Source: "mock", Name: "server1", RepositoryURL: "https://github.com/mock/server1"},
			{Source: "mock", Name: "server2", RepositoryURL: "https://github.com/mock/server2"},
			{Source: "mock", Name: "server3", RepositoryURL: "https://github.com/mock/server3"},
		},
		Records:    make(map[string]*models.RawRecord),
		trustScore: 0.5,
	}
}

func (m *MockAdapter) Name() string { return "mock" }

func (m *MockAdapter) TrustScore() float64 {
	if m.trustScore == 0 {
		return 0.5
	}
	return m.trustScore
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
		return nil, context.DeadlineExceeded
	}
	if m.Candidates == nil {
		return []models.RawCandidate{}, nil
	}
	return m.Candidates, nil
}

func (m *MockAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	if m.ShouldFail {
		return nil, context.DeadlineExceeded
	}
	if record, ok := m.Records[candidate.Name]; ok {
		return record, nil
	}
	return &models.RawRecord{
		RawCandidate: candidate,
		Repository:   models.RepositoryInfo{URL: candidate.RepositoryURL, Name: candidate.Name},
		Readme:       "# Test",
	}, nil
}

var _ SourceAdapter = (*MockAdapter)(nil)
