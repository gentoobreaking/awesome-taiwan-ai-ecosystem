// Package export generates registry JSON export files.
package export

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// RegistryExporter generates registry JSON export files (§28).
type RegistryExporter struct{}

// New creates a new RegistryExporter.
func New() *RegistryExporter {
	return &RegistryExporter{}
}

// Export generates all registry JSON files in the given directory.
func (re *RegistryExporter) Export(dir string, servers []models.MCPServer) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create export dir: %w", err)
	}

	stats := computeStatistics(servers)

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	crawlerVersion := "v1.0.0"

	// registry.json
	registry := Registry{
		SchemaVersion:    "0.1",
		RegistryVersion:  "v1." + time.Now().UTC().Format("2006.01.02"),
		GeneratedAt:      generatedAt,
		CrawlerVersion:   crawlerVersion,
		TotalServers:     len(servers),
		TaiwanRelevant:   stats.TaiwanRelevant,
		Statistics:       stats,
		Servers:          servers,
	}
	if err := writeJSON(filepath.Join(dir, "registry.json"), &registry, true); err != nil {
		return err
	}

	// registry.min.json
	minServers := make([]map[string]any, len(servers))
	for i, s := range servers {
		minServers[i] = map[string]any{
			"id":          s.ID,
			"name":        s.Name,
			"description": s.Description,
			"category":    s.Category,
			"transport":   s.Transport,
		}
	}
	minRegistry := MinRegistry{
		SchemaVersion:  "0.1",
		GeneratedAt:    generatedAt,
		Servers:        minServers,
	}
	if err := writeJSON(filepath.Join(dir, "registry.min.json"), &minRegistry, true); err != nil {
		return err
	}

	// categories.json
	categories := make(map[string]int)
	for _, s := range servers {
		for _, c := range s.Category {
			categories[c]++
		}
		if len(s.Category) == 0 {
			categories["uncategorized"]++
		}
	}
	if err := writeJSON(filepath.Join(dir, "categories.json"), categories, true); err != nil {
		return err
	}

	// sources.json
	srcMap := make(map[string]int)
	for _, s := range servers {
		for _, src := range s.Sources {
			srcMap[src.Source]++
		}
	}
	if err := writeJSON(filepath.Join(dir, "sources.json"), srcMap, true); err != nil {
		return err
	}

	// statistics.json
	if err := writeJSON(filepath.Join(dir, "statistics.json"), stats, true); err != nil {
		return err
	}

	// health.json
	health := make([]HealthEntry, len(servers))
	for i, s := range servers {
		health[i] = HealthEntry{
			ID:     s.ID,
			Health: string(s.Health),
		}
	}
	if err := writeJSON(filepath.Join(dir, "health.json"), health, true); err != nil {
		return err
	}

	return nil
}

type Registry struct {
	SchemaVersion    string             `json:"schema_version"`
	RegistryVersion  string             `json:"registry_version"`
	GeneratedAt      string             `json:"generated_at"`
	CrawlerVersion   string             `json:"crawler_version"`
	TotalServers     int                `json:"total_servers"`
	TaiwanRelevant   int                `json:"taiwan_relevant"`
	Statistics       Statistics         `json:"statistics"`
	Servers          []models.MCPServer `json:"servers"`
}

type MinRegistry struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Servers       []map[string]any `json:"servers"`
}

type Statistics struct {
	TotalServers   int            `json:"total_servers"`
	TaiwanRelevant int            `json:"taiwan_relevant"`
	ByLevel        map[string]int `json:"by_level"`
	ByHealth       map[string]int `json:"by_health"`
	QualityDist    map[string]int `json:"quality_distribution"`
}

type HealthEntry struct {
	ID     string `json:"id"`
	Health string `json:"health"`
}

func computeStatistics(servers []models.MCPServer) Statistics {
	stats := Statistics{
		TotalServers:   len(servers),
		ByLevel:        make(map[string]int),
		ByHealth:       make(map[string]int),
		QualityDist:    make(map[string]int),
	}

	for _, s := range servers {
		level := s.TaiwanRelevance.Level
		if level == "" {
			level = "T0"
		}
		stats.ByLevel[level]++

		stats.ByHealth[string(s.Health)]++

		grade := s.Quality.Grade
		if grade == "" {
			grade = "F"
		}
		stats.QualityDist[grade]++

		if s.TaiwanRelevance.Level != "" && s.TaiwanRelevance.Level != "T0" {
			stats.TaiwanRelevant++
		}
	}

	return stats
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func writeJSON(path string, v interface{}, indent bool) error {
	var data []byte
	var err error
	if indent {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return writeFile(path, data)
}

// ExportMarkdown generates a human-readable markdown file from the registry.
func (re *RegistryExporter) ExportMarkdown(path string, servers []models.MCPServer) error {
	var sb strings.Builder

	sb.WriteString("# Awesome Taiwan MCP Registry\n\n")
	sb.WriteString(fmt.Sprintf("> Generated on %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	stats := computeStatistics(servers)
	sb.WriteString(fmt.Sprintf("## Statistics\n\n"))
	sb.WriteString(fmt.Sprintf("- **Total Servers**: %d\n", stats.TotalServers))
	sb.WriteString(fmt.Sprintf("- **Taiwan Relevant**: %d\n", stats.TaiwanRelevant))
	for _, level := range []string{"T5", "T4", "T3", "T2", "T1", "T0"} {
		sb.WriteString(fmt.Sprintf("- **%s**: %d\n", level, stats.ByLevel[level]))
	}
	sb.WriteString("\n### By Health\n\n")
	for _, h := range []string{"HEALTHY", "DEGRADED", "UNAVAILABLE", "UNKNOWN"} {
		sb.WriteString(fmt.Sprintf("- **%s**: %d\n", h, stats.ByHealth[h]))
	}
	sb.WriteString("\n### By Quality Grade\n\n")
	for _, grade := range []string{"A", "B", "C", "D", "F"} {
		sb.WriteString(fmt.Sprintf("- **%s**: %d\n", grade, stats.QualityDist[grade]))
	}
	sb.WriteString("\n---\n\n")

	// Group servers by level
	for _, level := range []string{"T5", "T4", "T3", "T2", "T1", "T0"} {
		levelServers := make([]models.MCPServer, 0)
		for _, s := range servers {
			if s.TaiwanRelevance.Level == level {
				levelServers = append(levelServers, s)
			}
		}
		if len(levelServers) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s — %d servers\n\n", level, len(levelServers)))
		
		// Add level description
		levelDesc := levelDescription(level)
		if levelDesc != "" {
			sb.WriteString(fmt.Sprintf("_%s_: %s\n\n", level, levelDesc))
		}
		
		for _, s := range levelServers {
			sb.WriteString(fmt.Sprintf("### %s\n\n", s.Name))
			if s.Description != "" {
				sb.WriteString(fmt.Sprintf("> %s\n\n", s.Description))
			}
			sb.WriteString(fmt.Sprintf("- **Repository**: [%s](%s)\n", s.Repository.URL, s.Repository.URL))
			if s.Repository.Stars > 0 {
				sb.WriteString(fmt.Sprintf("- **Stars**: %d\n", s.Repository.Stars))
			}
			if s.Repository.Language != "" {
				sb.WriteString(fmt.Sprintf("- **Language**: [%s](https://github.com/search?q=%s+language:%s&type=repositories)\n",
					s.Repository.Language, url.QueryEscape(s.Repository.Name), url.QueryEscape(s.Repository.Language)))
			}
			sb.WriteString(fmt.Sprintf("- **Taiwan Relevance**: %s (score: %.1f, confidence: %.2f)\n", s.TaiwanRelevance.Level, s.TaiwanRelevance.Score, s.TaiwanRelevance.Confidence))
			sb.WriteString(fmt.Sprintf("- **Health**: %s\n", s.Health))
			sb.WriteString(fmt.Sprintf("- **Quality**: %s (score: %d)\n", s.Quality.Grade, s.Quality.Score))
			if s.License != "" {
				sb.WriteString(fmt.Sprintf("- **License**: %s\n", s.License))
			}
			if len(s.Tools) > 0 {
				toolNames := make([]string, len(s.Tools))
				for i, t := range s.Tools {
					toolNames[i] = t.Name
				}
				sb.WriteString(fmt.Sprintf("- **Tools**: %s\n", strings.Join(toolNames, ", ")))
			}
			if len(s.Endpoints) > 0 {
				for _, ep := range s.Endpoints {
					sb.WriteString(fmt.Sprintf("- **Endpoint**: `%s` (transport: %s)\n", ep.URL, ep.Transport))
				}
			}
			if len(s.Security) > 0 {
				severities := make([]string, 0, len(s.Security))
				for _, f := range s.Security {
					severities = append(severities, fmt.Sprintf("%s: %s", f.Type, f.Severity))
				}
				sb.WriteString(fmt.Sprintf("- **Security**: %s\n", strings.Join(severities, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	return writeFile(path, []byte(sb.String()))
}

// levelDescription returns a human-readable description for each Taiwan relevance level.
func levelDescription(level string) string {
	switch level {
	case "T5":
		return "Definitively Taiwan-focused — official government or financial APIs with Taiwan-specific data"
	case "T4":
		return "Very strong Taiwan relevance — Taiwan data sources with clear local focus"
	case "T3":
		return "Strong Taiwan relevance — Taiwan-specific data or services (real estate, finance, etc.)"
	case "T2":
		return "Moderate Taiwan relevance — some Taiwan content or keywords detected"
	case "T1":
		return "Weak Taiwan relevance — minimal Taiwan connection"
	case "T0":
		return "No Taiwan relevance — international or general-purpose server"
	}
	return ""
}
