package dedupe

import (
	"sort"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// ServerSource holds a server with its source trust score for conflict resolution.
type ServerSource struct {
	Server     *models.MCPServer
	TrustScore float64
}

// DedupEngine merges candidates from multiple sources into a single MCPServer.
type DedupEngine struct{}

// New creates a new DedupEngine.
func New() *DedupEngine {
	return &DedupEngine{}
}

// Deduplicate groups servers by CanonicalID and merges them (§20, §24).
func (de *DedupEngine) Deduplicate(servers []*ServerSource) ([]*models.MCPServer, error) {
	groups := make(map[string][]*ServerSource)

	for _, ss := range servers {
		id := ServerID(ss.Server)
		groups[id] = append(groups[id], ss)
	}

	var result []*models.MCPServer
	for _, group := range groups {
		merged := mergeGroup(group)
		result = append(result, merged)
	}

	// Sort by name for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func mergeGroup(group []*ServerSource) *models.MCPServer {
	// Sort by trust score descending — highest trust first
	sort.Slice(group, func(i, j int) bool {
		return group[i].TrustScore > group[j].TrustScore
	})

	base := group[0].Server

	var servers []*models.MCPServer
	for _, ss := range group {
		servers = append(servers, ss.Server)
	}

	result := &models.MCPServer{
		ID:               base.ID,
		Name:             base.Name,
		Slug:             base.Slug,
		Description:      base.Description,
		Region:           base.Region,
		Category:         base.Category,
		TaiwanRelevance:  base.TaiwanRelevance,
		Repository:       base.Repository,
		Endpoints:        mergeEndpoints(servers),
		Transport:        mergeStrings(servers, func(s *models.MCPServer) []string { return s.Transport }),
		Tools:            mergeTools(servers),
		Resources:        mergeResources(servers),
		Prompts:          mergePrompts(servers),
		DataSources:      mergeDataSources(servers),
		License:          base.License,
		Status:           base.Status,
		Health:           base.Health,
		Quality:          base.Quality,
		Sources:          mergeSourceRefs(group),
		FirstSeen:        mergeTime(servers, func(s *models.MCPServer) bool { return !s.FirstSeen.IsZero() }, func(s *models.MCPServer) time.Time { return s.FirstSeen }),
		LastSeen:         mergeTimeLatest(servers),
		LastVerified:     base.LastVerified,
	}

	return result
}

func mergeEndpoints(servers []*models.MCPServer) []models.Endpoint {
	seen := make(map[string]bool)
	var result []models.Endpoint
	for _, s := range servers {
		for _, ep := range s.Endpoints {
			if !seen[ep.URL] {
				seen[ep.URL] = true
				result = append(result, ep)
			}
		}
	}
	return result
}

func mergeTools(servers []*models.MCPServer) []models.Tool {
	seen := make(map[string]bool)
	var result []models.Tool
	for _, s := range servers {
		for _, t := range s.Tools {
			if !seen[t.Name] {
				seen[t.Name] = true
				result = append(result, t)
			}
		}
	}
	return result
}

func mergeResources(servers []*models.MCPServer) []models.Resource {
	seen := make(map[string]bool)
	var result []models.Resource
	for _, s := range servers {
		for _, r := range s.Resources {
			if !seen[r.URI] {
				seen[r.URI] = true
				result = append(result, r)
			}
		}
	}
	return result
}

func mergePrompts(servers []*models.MCPServer) []models.Prompt {
	seen := make(map[string]bool)
	var result []models.Prompt
	for _, s := range servers {
		for _, p := range s.Prompts {
			if !seen[p.Name] {
				seen[p.Name] = true
				result = append(result, p)
			}
		}
	}
	return result
}

func mergeDataSources(servers []*models.MCPServer) []models.DataSource {
	seen := make(map[string]bool)
	var result []models.DataSource
	for _, s := range servers {
		for _, ds := range s.DataSources {
			if !seen[ds.Name] {
				seen[ds.Name] = true
				result = append(result, ds)
			}
		}
	}
	return result
}

func mergeStrings(servers []*models.MCPServer, getter func(s *models.MCPServer) []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range servers {
		for _, v := range getter(s) {
			if !seen[v] {
				seen[v] = true
				result = append(result, v)
			}
		}
	}
	return result
}

func mergeSourceRefs(group []*ServerSource) []models.SourceReference {
	seen := make(map[string]bool)
	var result []models.SourceReference
	for _, ss := range group {
		for _, sr := range ss.Server.Sources {
			key := sr.Source + "|" + sr.URL
			if !seen[key] {
				seen[key] = true
				result = append(result, sr)
			}
		}
	}
	return result
}

func mergeTime(servers []*models.MCPServer, exists func(s *models.MCPServer) bool, getter func(s *models.MCPServer) time.Time) time.Time {
	for _, s := range servers {
		if exists(s) {
			return getter(s)
		}
	}
	return time.Time{}
}

func mergeTimeLatest(servers []*models.MCPServer) time.Time {
	var latest time.Time
	for _, s := range servers {
		if s.LastSeen.After(latest) {
			latest = s.LastSeen
		}
	}
	return latest
}

var _ = strings.TrimSpace // keep import if used later
