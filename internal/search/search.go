// Package search implements registry search engine (§22, T036, T037).
package search

import (
	"sort"
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// SearchQuery configures a registry search (§22).
type SearchQuery struct {
	Text      string
	Level     string // T0-T5
	Category  []string
	MinScore  int
	Health    []models.HealthStatus
	Transport []string
	Official  bool
	Limit     int
	Offset    int
}

// SearchResult wraps a server with a relevance score.
type SearchResult struct {
	Server models.MCPServer
	Score  float64
}

// SearchEngine is the in-memory registry search engine.
type SearchEngine struct {
	servers []models.MCPServer
}

// New creates a SearchEngine from the given servers.
func New(servers []models.MCPServer) *SearchEngine {
	return &SearchEngine{servers: servers}
}

// Search executes a registry search (§22, T036).
func (se *SearchEngine) Search(query SearchQuery) ([]SearchResult, error) {
	results := make([]SearchResult, 0, len(se.servers))

	for _, s := range se.servers {
		if !se.matches(s, query) {
			continue
		}
		relevance := se.score(s, query)
		results = append(results, SearchResult{
			Server: s,
			Score:  relevance,
		})
	}

	// Sort by relevance (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply offset and limit
	start := query.Offset
	if start > len(results) {
		start = len(results)
	}
	end := start + query.Limit
	if query.Limit == 0 || end > len(results) {
		end = len(results)
	}

	return results[start:end], nil
}

func (se *SearchEngine) matches(s models.MCPServer, q SearchQuery) bool {
	// Level filter
	if q.Level != "" && s.TaiwanRelevance.Level != q.Level {
		return false
	}

	// Category filter
	if len(q.Category) > 0 {
		matched := false
		for _, c := range s.Category {
			for _, qc := range q.Category {
				if strings.EqualFold(c, qc) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// MinScore filter
	if q.MinScore > 0 && s.Quality.Score < q.MinScore {
		return false
	}

	// Health filter
	if len(q.Health) > 0 {
		matched := false
		for _, h := range q.Health {
			if s.Health == h {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Transport filter
	if len(q.Transport) > 0 {
		matched := false
		for _, t := range s.Transport {
			for _, qt := range q.Transport {
				if strings.EqualFold(t, qt) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Official source filter
	if q.Official {
		matched := false
		for _, src := range s.Sources {
			if strings.Contains(strings.ToLower(src.Source), "official") ||
				strings.Contains(strings.ToLower(src.Source), "registry") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Text search (case-insensitive)
	if q.Text != "" {
		if !se.textMatch(s, q.Text) {
			return false
		}
	}

	return true
}

func (se *SearchEngine) textMatch(s models.MCPServer, text string) bool {
	q := strings.ToLower(text)

	if strings.Contains(strings.ToLower(s.Name), q) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), q) {
		return true
	}
	for _, c := range s.Category {
		if strings.Contains(strings.ToLower(c), q) {
			return true
		}
	}
	for _, t := range s.Tools {
		if strings.Contains(strings.ToLower(t.Name), q) {
			return true
		}
		if t.Description != "" && strings.Contains(strings.ToLower(t.Description), q) {
			return true
		}
	}
	for _, r := range s.Resources {
		if strings.Contains(strings.ToLower(r.Name), q) {
			return true
		}
	}
	for _, ds := range s.DataSources {
		if strings.Contains(strings.ToLower(ds.Name), q) {
			return true
		}
	}

	return false
}

// score computes a relevance score (§59: capability + Taiwan relevance + health + quality).
func (se *SearchEngine) score(s models.MCPServer, q SearchQuery) float64 {
	var score float64

	// Taiwan relevance (T5=50, T4=40, ..., T1=10, T0=0)
	levelScores := map[string]float64{
		"T5": 50, "T4": 40, "T3": 30, "T2": 20, "T1": 10, "T0": 0,
	}
	score += levelScores[s.TaiwanRelevance.Level]
	score += s.TaiwanRelevance.Score * 0.3

	// Quality score contribution
	score += float64(s.Quality.Score) * 0.2

	// Health contribution (§59: HEALTHY=15, DEGRADED=7, UNAVAILABLE=0)
	healthScores := map[models.HealthStatus]float64{
		models.HealthHealthy:     15,
		models.HealthDegraded:    7,
		models.HealthUnavailable: 0,
		models.HealthInvalid:     0,
		models.HealthUnknown:     0,
	}
	score += healthScores[s.Health]

	// Capability match bonus (§59)
	if q.Text != "" {
		capMatch := se.capabilityMatch(s, q.Text)
		score += capMatch * 15
	}

	return score
}

// capabilityMatch returns a 0-1 score for how well the server's tools/resources/data match the query (§22).
func (se *SearchEngine) capabilityMatch(s models.MCPServer, text string) float64 {
	q := strings.ToLower(text)
	var matches int
	var total int

	// Tool name + description match
	for _, t := range s.Tools {
		total++
		desc := strings.ToLower(t.Name + " " + t.Description)
		if strings.Contains(desc, q) {
			matches++
		}
	}

	// Resource match
	for _, r := range s.Resources {
		total++
		uri := strings.ToLower(r.URI + " " + r.Name + " " + r.Description)
		if strings.Contains(uri, q) {
			matches++
		}
	}

	// Data source match
	for _, ds := range s.DataSources {
		total++
		if strings.Contains(strings.ToLower(ds.Name+" "+ds.URL), q) {
			matches++
		}
	}

	if total == 0 {
		return 0
	}
	return float64(matches) / float64(total)
}

// SearchByCapability searches for servers matching a capability query (§22, T037).
type CapabilitySearcher = SearchEngine

// SearchByCapability finds MCP servers by capability keywords (§22, T037).
func (se *SearchEngine) SearchByCapability(query string) ([]SearchResult, error) {
	return se.Search(SearchQuery{
		Text:  query,
		Limit: 100,
	})
}
