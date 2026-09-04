// Package export generates registry JSON export files.
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	return os.WriteFile(path, data, 0644)
}
