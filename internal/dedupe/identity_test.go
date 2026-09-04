package dedupe

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func makeServer(name, repoURL, author string) *models.MCPServer {
	return &models.MCPServer{
		Name: name,
		Repository: models.RepositoryInfo{
			URL:   repoURL,
			Name:  repoURL,
			Owner: author,
		},
	}
}

func TestCanonicalIdentitySameURL(t *testing.T) {
	server := makeServer("test-mcp", "https://github.com/foo/bar", "foo")
	id1 := CanonicalIdentity(server)

	server2 := makeServer("test-mcp", "https://github.com/foo/bar/", "foo")
	id2 := CanonicalIdentity(server2)

	if id1.CanonicalID != id2.CanonicalID {
		t.Errorf("Expected same CanonicalID for same URL with/without trailing slash")
	}
}

func TestCanonicalIdentityURLVariations(t *testing.T) {
	base := "https://github.com/foo/bar"
	urls := []string{
		base,
		base + "/",
		base + ".git",
		base + ".git/",
		"https://github.com/foo/bar",
	}
	ids := make(map[string]bool)
	for _, u := range urls {
		s := makeServer("test", u, "foo")
		id := CanonicalIdentity(s)
		ids[id.CanonicalID] = true
	}
	if len(ids) != 1 {
		t.Errorf("Expected 1 unique CanonicalID for URL variations, got %d", len(ids))
	}
}

func TestCanonicalIdentityDifferentURLs(t *testing.T) {
	s1 := makeServer("test1", "https://github.com/foo/bar", "foo")
	s2 := makeServer("test2", "https://github.com/foo/baz", "foo")
	id1 := CanonicalIdentity(s1)
	id2 := CanonicalIdentity(s2)
	if id1.CanonicalID == id2.CanonicalID {
		t.Error("Expected different CanonicalIDs for different URLs")
	}
}

func TestCanonicalIDDeterminism(t *testing.T) {
	server := makeServer("test-mcp", "https://github.com/foo/bar", "foo")
	ids := make(map[string]int)
	for i := 0; i < 100; i++ {
		id := CanonicalIdentity(server)
		ids[id.CanonicalID]++
	}
	if len(ids) != 1 {
		t.Errorf("Expected 1 unique CanonicalID (100 runs), got %d", len(ids))
	}
}

func TestCanonicalIDIs64Chars(t *testing.T) {
	server := makeServer("test", "https://github.com/foo/bar", "foo")
	id := CanonicalIdentity(server)
	if len(id.CanonicalID) != 64 {
		t.Errorf("Expected 64-char sha256 hex, got %d", len(id.CanonicalID))
	}
}

func TestDedupSameServer(t *testing.T) {
	engine := New()
	server := makeServer("taiwan-mcp", "https://github.com/foo/taiwan-mcp", "foo")

	servers := []*ServerSource{
		{Server: server, TrustScore: 0.95},
		{Server: server, TrustScore: 0.85},
	}

	result, err := engine.Deduplicate(servers)
	if err != nil {
		t.Fatalf("Dedup failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 server after dedup, got %d", len(result))
	}
}

func TestDedupDifferentServers(t *testing.T) {
	engine := New()
	s1 := makeServer("mcp1", "https://github.com/foo/mcp1", "foo")
	s2 := makeServer("mcp2", "https://github.com/foo/mcp2", "foo")

	servers := []*ServerSource{
		{Server: s1, TrustScore: 0.95},
		{Server: s2, TrustScore: 0.85},
	}

	result, err := engine.Deduplicate(servers)
	if err != nil {
		t.Fatalf("Dedup failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 servers after dedup, got %d", len(result))
	}
}

func TestDedupMergesSources(t *testing.T) {
	engine := New()
	server := makeServer("taiwan-mcp", "https://github.com/foo/taiwan-mcp", "foo")
	server.Sources = []models.SourceReference{
		{Source: "github", TrustScore: 0.95},
	}

	server2 := makeServer("taiwan-mcp", "https://github.com/foo/taiwan-mcp", "foo")
	server2.Sources = []models.SourceReference{
		{Source: "glama", TrustScore: 0.85},
	}

	servers := []*ServerSource{
		{Server: server, TrustScore: 0.95},
		{Server: server2, TrustScore: 0.85},
	}

	result, err := engine.Deduplicate(servers)
	if err != nil {
		t.Fatalf("Dedup failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("Expected 1 server after dedup, got %d", len(result))
	}
	if len(result[0].Sources) < 2 {
		t.Errorf("Expected >=2 merged sources, got %d", len(result[0].Sources))
	}
}
