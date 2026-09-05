package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
)

// InjectionExporter generates injection detection reports.
type InjectionExporter struct{}

// NewInjectionExporter creates a new InjectionExporter.
func NewInjectionExporter() *InjectionExporter {
	return &InjectionExporter{}
}

// InjectionReportData holds data for the injection report.
type InjectionReportData struct {
	GeneratedAt     string
	CrawlerVersion  string
	TotalScanned    int
	TotalFlagged    int
	PatternCounts   map[string]int
	Servers         []InjectionServerEntry
}

// InjectionServerEntry represents a server with injection findings.
type InjectionServerEntry struct {
	Name           string
	Repository     string
	MatchedPatterns []InjectionMatchEntry
	ScannedAt      string
}

// InjectionMatchEntry represents a single pattern match.
type InjectionMatchEntry struct {
	Pattern      string
	Description  string
	MatchedText  string
	Context      string
	LineNumber   int
}

// InjectionPatternsJSON represents the JSON output for patterns.
type InjectionPatternsJSON struct {
	GeneratedAt   string         `json:"generated_at"`
	TotalPatterns int            `json:"total_patterns"`
	TotalMatches  int            `json:"total_matches"`
	Patterns      []PatternStats `json:"patterns"`
}

// PatternStats holds statistics for a single injection pattern.
type PatternStats struct {
	Pattern      string `json:"pattern"`
	Description  string `json:"description"`
	MatchCount   int    `json:"match_count"`
	AffectedRepos int   `json:"affected_repos"`
}

// ExportInjectionReport generates INJECTION_REPORT.md and patterns.json.
func (ie *InjectionExporter) ExportInjectionReport(dir string, servers []models.MCPServer) error {
	injectionDir := filepath.Join(dir, "security", "injection")
	if err := os.MkdirAll(injectionDir, 0755); err != nil {
		return fmt.Errorf("create injection dir: %w", err)
	}

	// Scan all servers for injection patterns
	var flagged []InjectionServerEntry
	patternCounts := make(map[string]int)
	patternRepos := make(map[string]map[string]bool) // pattern -> repo -> bool

	for _, s := range servers {
		matches := scanForInjectionPatterns(s)
		if len(matches) > 0 {
			flagged = append(flagged, InjectionServerEntry{
				Name:            s.Name,
				Repository:      s.Repository.URL,
				MatchedPatterns: matches,
				ScannedAt:       time.Now().UTC().Format(time.RFC3339),
			})
			for _, m := range matches {
				patternCounts[m.Pattern]++
				if patternRepos[m.Pattern] == nil {
					patternRepos[m.Pattern] = make(map[string]bool)
				}
				patternRepos[m.Pattern][s.Repository.URL] = true
			}
		}
	}

	// Sort by number of matches (descending)
	sort.Slice(flagged, func(i, j int) bool {
		return len(flagged[i].MatchedPatterns) > len(flagged[j].MatchedPatterns)
	})

	// Generate markdown report
	reportPath := filepath.Join(injectionDir, "INJECTION_REPORT.md")
	if err := generateInjectionMarkdown(reportPath, InjectionReportData{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		CrawlerVersion: "v1.0.0",
		TotalScanned:   len(servers),
		TotalFlagged:   len(flagged),
		PatternCounts:  patternCounts,
		Servers:        flagged,
	}); err != nil {
		return fmt.Errorf("generate markdown: %w", err)
	}

	// Generate patterns.json
	patternsJSON := InjectionPatternsJSON{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		TotalPatterns: len(normalize.GetInjectionPatterns()),
		TotalMatches:  sumMapValues(patternCounts),
		Patterns:      buildPatternStats(normalize.GetInjectionPatterns(), patternCounts, patternRepos),
	}
	jsonPath := filepath.Join(injectionDir, "patterns.json")
	if err := writeJSON(jsonPath, patternsJSON, true); err != nil {
		return fmt.Errorf("write patterns.json: %w", err)
	}

	return nil
}

// scanForInjectionPatterns scans a server's README for injection patterns.
func scanForInjectionPatterns(server models.MCPServer) []InjectionMatchEntry {
	var matches []InjectionMatchEntry
	readme := server.Readme
	if readme == "" {
		return matches
	}

	lines := strings.Split(readme, "\n")
	patterns := normalize.GetInjectionPatterns()

	for lineNum, line := range lines {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
				// Extract context (surrounding text)
				start := max(0, lineNum-2)
				end := min(len(lines)-1, lineNum+2)
				context := strings.Join(lines[start:end+1], "\n")

				matches = append(matches, InjectionMatchEntry{
					Pattern:     pattern,
					Description: getInjectionDescription(pattern),
					MatchedText: strings.TrimSpace(line),
					Context:     context,
					LineNumber:  lineNum + 1,
				})
			}
		}
	}
	return matches
}

// getInjectionDescription returns description for injection pattern.
func getInjectionDescription(pattern string) string {
	descriptions := map[string]string{
		"Ignore previous instructions": "Classic prompt injection attempt to override system prompt",
		"Call this URL":               "Attempt to make the model fetch external resources",
		"Upload credentials":          "Attempt to exfiltrate sensitive credentials",
	}
	if desc, ok := descriptions[pattern]; ok {
		return desc
	}
	return "Potential prompt injection pattern"
}

// buildPatternStats builds statistics for each pattern.
func buildPatternStats(patterns []string, counts map[string]int, repos map[string]map[string]bool) []PatternStats {
	var stats []PatternStats
	for _, p := range patterns {
		stats = append(stats, PatternStats{
			Pattern:       p,
			Description:   getInjectionDescription(p),
			MatchCount:    counts[p],
			AffectedRepos: len(repos[p]),
		})
	}
	// Sort by match count descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].MatchCount > stats[j].MatchCount
	})
	return stats
}

func sumMapValues(m map[string]int) int {
	sum := 0
	for _, v := range m {
		sum += v
	}
	return sum
}

// generateInjectionMarkdown generates the INJECTION_REPORT.md file.
func generateInjectionMarkdown(path string, data InjectionReportData) error {
	var sb strings.Builder

	sb.WriteString("# Injection Detection Report\n\n")
	sb.WriteString(fmt.Sprintf("> Generated on %s\n\n", data.GeneratedAt))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Scanned**: %d\n", data.TotalScanned))
	sb.WriteString(fmt.Sprintf("- **Total Flagged**: %d\n", data.TotalFlagged))
	sb.WriteString(fmt.Sprintf("- **Total Pattern Matches**: %d\n\n", sumMapValues(data.PatternCounts)))

	if data.TotalFlagged == 0 {
		sb.WriteString("No injection patterns detected.\n")
		return writeFile(path, sanitizeUTF8([]byte(sb.String())))
	}

	sb.WriteString("## Pattern Match Distribution\n\n")
	sb.WriteString("| Pattern | Matches | Affected Repos |\n")
	sb.WriteString("|---------|---------|----------------|\n")

	// Sort patterns by count
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range data.PatternCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	for _, kv := range sorted {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n", kv.Key, kv.Value, len(data.PatternCounts)))
	}

	sb.WriteString("\n## Flagged Repositories\n\n")
	sb.WriteString("| Repository | Matches | Patterns |\n")
	sb.WriteString("|------------|---------|----------|\n")

	for _, s := range data.Servers {
		var patternNames []string
		for _, m := range s.MatchedPatterns {
			patternNames = append(patternNames, m.Pattern)
		}
		sb.WriteString(fmt.Sprintf("| [%s](%s) | %d | %s |\n",
			s.Name, s.Repository, len(s.MatchedPatterns), strings.Join(patternNames, ", ")))
	}

	sb.WriteString("\n## Detailed Findings\n\n")
	for _, s := range data.Servers {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", s.Name, s.Repository))
		sb.WriteString(fmt.Sprintf("- **Scanned At**: %s\n", s.ScannedAt))
		sb.WriteString("- **Matches**:\n")
		for _, m := range s.MatchedPatterns {
			sb.WriteString(fmt.Sprintf("  - **Pattern**: `%s`\n", m.Pattern))
			sb.WriteString(fmt.Sprintf("    **Description**: %s\n", m.Description))
			sb.WriteString(fmt.Sprintf("    **Line %d**: %s\n", m.LineNumber, m.MatchedText))
			sb.WriteString(fmt.Sprintf("    **Context**:\n    ```\n    %s\n    ```\n\n", m.Context))
		}
	}

	return writeFile(path, sanitizeUTF8([]byte(sb.String())))
}

// GetInjectionPatterns returns the list of injection patterns (for external access).
func GetInjectionPatterns() []string {
	return normalize.GetInjectionPatterns()
}