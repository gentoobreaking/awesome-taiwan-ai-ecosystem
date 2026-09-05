package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// InjectionExporter handles export of injection detection reports.
type InjectionExporter struct {
	GeneratedAt time.Time
	OutputDir   string
	Entities    []*models.Entity
}

// InjectionEntry represents an injection detection entry for reporting.
type InjectionEntry struct {
	Entity     *models.Entity
	Findings   []models.SecurityFinding
	HitCount   int
	RiskLevel  string
	Patterns   []string
}

// InjectionPatternStats holds statistics for a detection pattern.
type InjectionPatternStats struct {
	Pattern     string `json:"pattern"`
	Count       int    `json:"count"`
	Description string `json:"description"`
}

// InjectionReportData holds all data for the injection report.
type InjectionReportData struct {
	GeneratedAt    time.Time               `json:"generated_at"`
	TotalScanned   int                     `json:"total_scanned"`
	TotalDetected  int                     `json:"total_detected"`
	PatternStats   []InjectionPatternStats `json:"pattern_stats"`
	TopRiskServers []InjectionServerEntry  `json:"top_risk_servers"`
	Details        []InjectionDetailEntry  `json:"details"`
}

// InjectionServerEntry represents a server entry in the report.
type InjectionServerEntry struct {
	Owner      string  `json:"owner"`
	Repo       string  `json:"repo"`
	URL        string  `json:"url"`
	HitCount   int     `json:"hit_count"`
	RiskLevel  string  `json:"risk_level"`
	Patterns   []string `json:"patterns"`
}

// InjectionDetailEntry represents a detailed finding.
type InjectionDetailEntry struct {
	Owner         string  `json:"owner"`
	Repo          string  `json:"repo"`
	URL           string  `json:"url"`
	Pattern       string  `json:"pattern"`
	MatchedText   string  `json:"matched_text"`
	Location      string  `json:"location"`
	Severity      string  `json:"severity"`
	Confidence    float64 `json:"confidence"`
}

// NewInjectionExporter creates a new injection exporter.
func NewInjectionExporter(entities []*models.Entity, outputDir string) *InjectionExporter {
	return &InjectionExporter{
		GeneratedAt: time.Now().UTC(),
		OutputDir:   outputDir,
		Entities:    entities,
	}
}

// Export generates injection detection reports.
func (e *InjectionExporter) Export() error {
	if err := os.MkdirAll(e.OutputDir, 0755); err != nil {
		return err
	}

	// Collect injection findings
	var entries []InjectionEntry
	patternCounts := make(map[string]int)

	for _, entity := range e.Entities {
		if entity.SecurityStatus.Findings == nil {
			continue
		}

		var injectionFindings []models.SecurityFinding
		for _, f := range entity.SecurityStatus.Findings {
			// Check for injection-related findings
			if isInjectionFinding(f) {
				injectionFindings = append(injectionFindings, f)
				patternCounts[f.Rule]++
			}
		}

		if len(injectionFindings) > 0 {
			// Determine highest severity
			riskLevel := "LOW"
			for _, f := range injectionFindings {
				sev := strings.ToUpper(f.Severity)
				if severityRank(sev) > severityRank(riskLevel) {
					riskLevel = sev
				}
			}

			// Collect unique patterns
			patterns := make(map[string]bool)
			for _, f := range injectionFindings {
				patterns[f.Rule] = true
			}
			patternList := make([]string, 0, len(patterns))
			for p := range patterns {
				patternList = append(patternList, p)
			}

			entries = append(entries, InjectionEntry{
				Entity:    entity,
				Findings:  injectionFindings,
				HitCount:  len(injectionFindings),
				RiskLevel: riskLevel,
				Patterns:  patternList,
			})
		}
	}

	// Generate INJECTION_REPORT.md
	if err := e.generateMarkdownReport(entries, patternCounts); err != nil {
		return err
	}

	// Generate patterns.json
	if err := e.generatePatternsJSON(entries, patternCounts); err != nil {
		return err
	}

	return nil
}

// isInjectionFinding checks if a finding is injection-related.
func isInjectionFinding(f models.SecurityFinding) bool {
	t := strings.ToLower(f.Type)
	return strings.Contains(t, "injection") ||
		strings.Contains(t, "prompt") ||
		strings.Contains(strings.ToLower(f.Rule), "injection") ||
		strings.Contains(strings.ToLower(f.Rule), "prompt")
}

// generateMarkdownReport generates INJECTION_REPORT.md.
func (e *InjectionExporter) generateMarkdownReport(entries []InjectionEntry, patternCounts map[string]int) error {
	var sb strings.Builder

	sb.WriteString("# Prompt Injection Detection Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", e.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total entities scanned: %d\n", len(e.Entities)))
	sb.WriteString(fmt.Sprintf("Servers with injection patterns detected: %d\n\n", len(entries)))

	// Summary by pattern
	if len(patternCounts) > 0 {
		sb.WriteString("## Pattern Hit Statistics\n\n")
		sb.WriteString("| Pattern | Hits |\n")
		sb.WriteString("|---------|------|\n")
		for pattern, count := range patternCounts {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", pattern, count))
		}
		sb.WriteString("\n")
	}

	// Top 10 risk servers
	if len(entries) > 0 {
		// Sort by hit count descending
		sortedEntries := make([]InjectionEntry, len(entries))
		copy(sortedEntries, entries)
		for i := 0; i < len(sortedEntries)-1; i++ {
			for j := i + 1; j < len(sortedEntries); j++ {
				if sortedEntries[j].HitCount > sortedEntries[i].HitCount {
					sortedEntries[i], sortedEntries[j] = sortedEntries[j], sortedEntries[i]
				}
			}
		}

		sb.WriteString("## Top 10 High-Risk Servers\n\n")
		sb.WriteString("| Server | Hits | Risk Level | Patterns |\n")
		sb.WriteString("|--------|------|------------|----------|\n")
		for i, entry := range sortedEntries {
			if i >= 10 {
				break
			}
			repoURL := entry.Entity.Repository.URL
			if repoURL == "" {
				repoURL = fmt.Sprintf("https://github.com/%s/%s", entry.Entity.Repository.Owner, entry.Entity.Repository.Name)
			}
			patternsStr := strings.Join(entry.Patterns, ", ")
			sb.WriteString(fmt.Sprintf("| [%s/%s](%s) | %d | %s | %s |\n",
				entry.Entity.Repository.Owner, entry.Entity.Repository.Name, repoURL,
				entry.HitCount, entry.RiskLevel, truncateForMarkdown(patternsStr, 100)))
		}
		sb.WriteString("\n")
	}

	// Detailed findings
	sb.WriteString("## Detailed Findings\n\n")

	for _, entry := range entries {
		repoURL := entry.Entity.Repository.URL
		if repoURL == "" {
			repoURL = fmt.Sprintf("https://github.com/%s/%s", entry.Entity.Repository.Owner, entry.Entity.Repository.Name)
		}

		sb.WriteString(fmt.Sprintf("### %s/%s\n\n", entry.Entity.Repository.Owner, entry.Entity.Repository.Name))
		sb.WriteString(fmt.Sprintf("- **Repository**: %s\n", repoURL))
		sb.WriteString(fmt.Sprintf("- **Hit Count**: %d\n", entry.HitCount))
		sb.WriteString(fmt.Sprintf("- **Risk Level**: %s\n", entry.RiskLevel))
		sb.WriteString(fmt.Sprintf("- **Patterns**: %s\n\n", strings.Join(entry.Patterns, ", ")))

		if len(entry.Findings) > 0 {
			sb.WriteString("#### Matches\n\n")
			sb.WriteString("| Pattern | Severity | Confidence | Evidence | Location |\n")
			sb.WriteString("|---------|----------|------------|----------|----------|\n")
			for _, f := range entry.Findings {
				evidence := truncateForMarkdown(f.Evidence, 100)
				sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %s | %s |\n",
					f.Rule, f.Severity, f.Confidence, evidence, f.Location))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	// Sanitize and write
	markdown := SanitizeString(sb.String())
	path := filepath.Join(e.OutputDir, "INJECTION_REPORT.md")
	return os.WriteFile(path, []byte(markdown), 0644)
}

// generatePatternsJSON generates patterns.json for CI/CD automation.
func (e *InjectionExporter) generatePatternsJSON(entries []InjectionEntry, patternCounts map[string]int) error {
	data := InjectionReportData{
		GeneratedAt:   e.GeneratedAt,
		TotalScanned:  len(e.Entities),
		TotalDetected: len(entries),
	}

	// Pattern stats
	for pattern, count := range patternCounts {
		data.PatternStats = append(data.PatternStats, InjectionPatternStats{
			Pattern: pattern,
			Count:   count,
		})
	}

	// Top risk servers
	for _, entry := range entries {
		repoURL := entry.Entity.Repository.URL
		if repoURL == "" {
			repoURL = fmt.Sprintf("https://github.com/%s/%s", entry.Entity.Repository.Owner, entry.Entity.Repository.Name)
		}
		data.TopRiskServers = append(data.TopRiskServers, InjectionServerEntry{
			Owner:     entry.Entity.Repository.Owner,
			Repo:      entry.Entity.Repository.Name,
			URL:       repoURL,
			HitCount:  entry.HitCount,
			RiskLevel: entry.RiskLevel,
			Patterns:  entry.Patterns,
		})
	}

	// Details
	for _, entry := range entries {
		for _, f := range entry.Findings {
			data.Details = append(data.Details, InjectionDetailEntry{
				Owner:       entry.Entity.Repository.Owner,
				Repo:        entry.Entity.Repository.Name,
				URL:         entry.Entity.Repository.URL,
				Pattern:     f.Rule,
				MatchedText: f.Evidence,
				Location:    f.Location,
				Severity:    f.Severity,
				Confidence:  f.Confidence,
			})
		}
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(e.OutputDir, "patterns.json")
	return os.WriteFile(path, jsonData, 0644)
}
