package sources

import (
	"context"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// MockAdapter is a test double for SourceAdapter.
type MockAdapter struct {
	Candidates  []models.RawCandidate
	Records     map[string]models.RawRecord
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
		Records:    make(map[string]models.RawRecord),
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
		return &record, nil
	}
	return &models.RawRecord{
		RawCandidate: candidate,
		Repository:   models.RepositoryInfo{URL: candidate.RepositoryURL, Name: candidate.Name},
		Readme:       "# Test",
	}, nil
}

var _ SourceAdapter = (*MockAdapter)(nil)

func TestMockAdapterDiscover(t *testing.T) {
	mock := NewMockAdapter()
	candidates, err := mock.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(candidates) < 3 {
		t.Errorf("Expected >= 3 candidates, got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.Source != "mock" {
			t.Errorf("Expected source 'mock', got '%s'", c.Source)
		}
	}
}

func TestMockAdapterDiscoverContextCancellation(t *testing.T) {
	mock := &MockAdapter{
		Candidates: []models.RawCandidate{{Source: "mock", Name: "test"}},
		Delay:      100 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := mock.Discover(ctx)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestMockAdapterShouldFail(t *testing.T) {
	mock := &MockAdapter{
		Candidates: []models.RawCandidate{{Source: "mock", Name: "test"}},
		ShouldFail: true,
	}

	_, err := mock.Discover(context.Background())
	if err == nil {
		t.Error("Expected error when ShouldFail=true")
	}
}

func TestMockAdapterFetch(t *testing.T) {
	mock := NewMockAdapter()
	candidates, _ := mock.Discover(context.Background())

	for _, c := range candidates {
		record, err := mock.Fetch(context.Background(), c)
		if err != nil {
			t.Errorf("Fetch failed for %s: %v", c.Name, err)
			continue
		}
		if record.Name != c.Name {
			t.Errorf("Record name mismatch: %s != %s", record.Name, c.Name)
		}
		if record.Repository.URL == "" {
			t.Error("Repository URL should not be empty")
		}
	}
}

func TestMockAdapterFetchFromRecords(t *testing.T) {
	candidate := models.RawCandidate{
		Source:        "mock",
		Name:          "custom-mcp",
		RepositoryURL: "https://github.com/mock/custom-mcp",
	}
	record := models.RawRecord{
		RawCandidate: candidate,
		Repository:   models.RepositoryInfo{URL: candidate.RepositoryURL, Name: "custom-mcp"},
		Readme:       "# Custom MCP\nTaiwan data.",
	}
	mock := &MockAdapter{
		Candidates: []models.RawCandidate{candidate},
		Records:    map[string]models.RawRecord{"custom-mcp": record},
	}

	result, err := mock.Fetch(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if result.Readme != "# Custom MCP\nTaiwan data." {
		t.Errorf("Unexpected readme: %s", result.Readme)
	}
}

func TestMockAdapterName(t *testing.T) {
	mock := NewMockAdapter()
	if mock.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", mock.Name())
	}
}

func TestMockAdapterTrustScore(t *testing.T) {
	mock := NewMockAdapter()
	if mock.TrustScore() != 0.5 {
		t.Errorf("Expected trust score 0.5, got %f", mock.TrustScore())
	}
}
