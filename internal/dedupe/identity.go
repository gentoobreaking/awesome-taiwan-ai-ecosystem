package dedupe

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Identity holds canonical identification for an MCP server (§21, §22).
type Identity struct {
	CanonicalID   string
	GitHubURL     string
	PackageName   string
	RegistryName  string
	Fingerprints  []string
}

// CanonicalIdentity computes the canonical identity for an MCPServer (§21, §22).
// Priority:
//  1. repository URL (sha256 of normalized URL)
//  2. package identifier (sha256 of package name)
//  3. official MCP registry name (sha256 of name)
//  4. canonical endpoint (sha256 of normalized endpoint URL)
//  5. fingerprint fallback (sha256 of name + author + endpoints + sorted tools)
func CanonicalIdentity(server *models.MCPServer) Identity {
	var id Identity

	id.GitHubURL = normalizeURL(server.Repository.URL)
	id.RegistryName = server.Name
	for _, ep := range server.Endpoints {
		if ep.URL != "" {
			id.Fingerprints = append(id.Fingerprints, normalizeURL(ep.URL))
		}
	}
	for _, t := range server.Tools {
		id.Fingerprints = append(id.Fingerprints, t.Name)
	}

	switch {
	case id.GitHubURL != "":
		id.CanonicalID = sha256Hex(id.GitHubURL)
		id.PackageName = server.Name
	case id.RegistryName != "":
		id.CanonicalID = sha256Hex(id.RegistryName)
		id.PackageName = server.Name
	case len(id.Fingerprints) > 0:
		sort.Strings(id.Fingerprints)
		parts := []string{server.Name, server.Repository.Owner}
		parts = append(parts, id.Fingerprints...)
		id.CanonicalID = sha256Hex(strings.Join(parts, "|"))
	default:
		id.CanonicalID = sha256Hex(server.Name + server.Repository.Owner)
	}

	return id
}

// ServerID returns the canonical ID (alias for CanonicalIdentity().CanonicalID).
func ServerID(server *models.MCPServer) string {
	return CanonicalIdentity(server).CanonicalID
}

// normalizeURL normalizes a URL for identity comparison (§21):
// https://github.com/foo/bar/ → github.com/foo/bar
// https://github.com/foo/bar.git → github.com/foo/bar
// Lowercase host.
func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return s
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
