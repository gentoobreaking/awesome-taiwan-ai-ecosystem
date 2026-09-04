package sources

import (
	"context"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

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
		if record.Candidate.Name != c.Name {
			t.Errorf("Record name mismatch: %s != %s", record.Candidate.Name, c.Name)
		}
		if record.Repository == nil {
			t.Error("Repository should not be nil")
		}
	}
}

func TestMockAdapterFetchFromRecords(t *testing.T) {
	candidate := models.RawCandidate{
		Source:        "mock",
		Name:          "custom-mcp",
		RepositoryURL: "https://github.com/mock/custom-mcp",
	}
	mock := &MockAdapter{
		Candidates: []models.RawCandidate{candidate},
		Records: map[string]*RawRecord{
			"custom-mcp": {
				Candidate: candidate,
				Readme:    "# Custom MCP\nTaiwan data.",
			},
		},
	}

	record, err := mock.Fetch(context.Background(), candidate)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if record.Readme != "# Custom MCP\nTaiwan data." {
		t.Errorf("Unexpected readme: %s", record.Readme)
	}
}

func TestRawRecordJSON(t *testing.T) {
	candidate := models.RawCandidate{
		Source:        "mock",
		Name:          "test-mcp",
		RepositoryURL: "https://github.com/mock/test-mcp",
		DiscoveredAt:  time.Now(),
	}

	record := &RawRecord{
		Candidate: candidate,
		Repository: &models.RepositoryInfo{
			URL:     candidate.RepositoryURL,
			Name:    "test-mcp",
			Stars:   50,
		},
		Readme:   "# Test MCP",
		Transport: []string{"stdio"},
	}

	// Verify the record is usable
	if record.Candidate.Name != "test-mcp" {
		t.Errorf("Candidate name mismatch: %s", record.Candidate.Name)
	}
	if record.Repository.Stars != 50 {
		t.Errorf("Stars mismatch: %d", record.Repository.Stars)
	}
}

func TestMockAdapterName(t *testing.T) {
	mock := NewMockAdapter()
	if mock.Name() != "mock" {
		t.Errorf("Expected name 'mock', got '%s'", mock.Name())
	}
}
