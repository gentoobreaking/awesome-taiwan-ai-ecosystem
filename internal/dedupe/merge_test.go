package dedupe

import (
	"testing"

	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func makeServerSource(id, name, repoURL string, trust float64, tools []models.Tool, endpoints []models.Endpoint) *ServerSource {
	return &ServerSource{
		Server: &models.MCPServer{
			ID:   id,
			Name: name,
			Repository: models.RepositoryInfo{
				URL: repoURL,
			},
			Tools:     tools,
			Endpoints: endpoints,
		},
		TrustScore: trust,
	}
}

func TestDeduplicate_Empty(t *testing.T) {
	de := New()
	result, err := de.Deduplicate(nil)
	if err != nil {
		t.Fatalf("Deduplicate error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(result))
	}
}

func TestDeduplicate_SameID(t *testing.T) {
	id := "same-id-123"
	s1 := makeServerSource(id, "name1", "https://github.com/foo/bar", 0.95, nil, nil)
	s2 := makeServerSource(id, "name2", "https://github.com/foo/bar", 0.85, nil, nil)
	s2.Server.Category = []string{"cat1"}

	de := New()
	result, err := de.Deduplicate([]*ServerSource{s1, s2})
	if err != nil {
		t.Fatalf("Deduplicate error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(result))
	}

	// Higher trust wins for name
	if result[0].Name != "name1" {
		t.Errorf("Expected 'name1' (higher trust), got '%s'", result[0].Name)
	}
}

func TestDeduplicate_MergeTools(t *testing.T) {
	id := "same-id-456"
	s1 := makeServerSource(id, "server", "https://github.com/foo/bar", 0.95,
		[]models.Tool{{Name: "tool1"}}, nil)
	s2 := makeServerSource(id, "server", "https://github.com/foo/bar", 0.85,
		[]models.Tool{{Name: "tool2"}}, nil)

	de := New()
	result, err := de.Deduplicate([]*ServerSource{s1, s2})
	if err != nil {
		t.Fatalf("Deduplicate error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(result))
	}
	if len(result[0].Tools) != 2 {
		t.Errorf("Expected 2 merged tools, got %d", len(result[0].Tools))
	}
}

func TestMergeEndpoints_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{Endpoints: []models.Endpoint{{URL: "https://a.com/mcp"}, {URL: "https://b.com/mcp"}}},
		{Endpoints: []models.Endpoint{{URL: "https://a.com/mcp"}, {URL: "https://c.com/mcp"}}},
	}
	result := mergeEndpoints(servers)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique endpoints, got %d", len(result))
	}
}

func TestMergeTools_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{Tools: []models.Tool{{Name: "tool1"}, {Name: "tool2"}}},
		{Tools: []models.Tool{{Name: "tool1"}, {Name: "tool3"}}},
	}
	result := mergeTools(servers)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique tools, got %d", len(result))
	}
}

func TestMergeResources_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{Resources: []models.Resource{{URI: "test://a"}, {URI: "test://b"}}},
		{Resources: []models.Resource{{URI: "test://a"}, {URI: "test://c"}}},
	}
	result := mergeResources(servers)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique resources, got %d", len(result))
	}
}

func TestMergePrompts_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{Prompts: []models.Prompt{{Name: "p1"}, {Name: "p2"}}},
		{Prompts: []models.Prompt{{Name: "p1"}, {Name: "p3"}}},
	}
	result := mergePrompts(servers)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique prompts, got %d", len(result))
	}
}

func TestMergeDataSources_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{DataSources: []models.DataSource{{Name: "twse.com.tw"}, {Name: "cwa.gov.tw"}}},
		{DataSources: []models.DataSource{{Name: "twse.com.tw"}, {Name: "moi.gov.tw"}}},
	}
	result := mergeDataSources(servers)
	if len(result) != 3 {
		t.Errorf("Expected 3 unique data sources, got %d", len(result))
	}
}


func TestMergeTimeLatest(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	servers := []*models.MCPServer{
		{LastSeen: t1},
		{LastSeen: t2},
		{LastSeen: t3},
	}
	result := mergeTimeLatest(servers)
	if !result.Equal(t2) {
		t.Errorf("Expected latest time t2, got %v", result)
	}
}

func TestCanonicalIdentity_EdgeCases(t *testing.T) {
	// Test with empty repo URL
	s := &models.MCPServer{ID: "test-id"}
	_ = s
	// CanonicalIdentity needs a server with repository URL
	// This is tested via identity_test.go
}

func TestMergeStrings_Empty(t *testing.T) {
	servers := []*models.MCPServer{{}, {}}
	result := mergeStrings(servers, func(s *models.MCPServer) []string { return s.Transport })
	if len(result) != 0 {
		t.Errorf("Expected 0 strings, got %d", len(result))
	}
}

func TestMergeStrings_Dedup(t *testing.T) {
	servers := []*models.MCPServer{
		{Transport: []string{"stdio", "http"}},
		{Transport: []string{"stdio", "sse"}},
	}
	result := mergeStrings(servers, func(s *models.MCPServer) []string { return s.Transport })
	if len(result) != 3 {
		t.Errorf("Expected 3 unique strings, got %d", len(result))
	}
}

func TestMergeTime_NotExists(t *testing.T) {
	servers := []*models.MCPServer{
		{FirstSeen: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	result := mergeTime(servers, func(s *models.MCPServer) bool { return false }, func(s *models.MCPServer) time.Time { return s.FirstSeen })
	if !result.IsZero() {
		t.Error("Expected zero time when exists=false")
	}
}

func TestMergeTime_NoValidTimes(t *testing.T) {
	servers := []*models.MCPServer{
		{FirstSeen: time.Time{}},
		{FirstSeen: time.Time{}},
	}
	result := mergeTime(servers, func(s *models.MCPServer) bool { return !s.FirstSeen.IsZero() }, func(s *models.MCPServer) time.Time { return s.FirstSeen })
	if !result.IsZero() {
		t.Error("Expected zero time when all are zero")
	}
}
