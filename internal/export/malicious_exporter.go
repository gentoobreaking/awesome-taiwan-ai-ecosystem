package export

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/security"
)

// MaliciousExporter generates malicious repository reports.
type MaliciousExporter struct{}

// NewMaliciousExporter creates a new MaliciousExporter.
func NewMaliciousExporter() *MaliciousExporter {
	return &MaliciousExporter{}
}

// MaliciousReportData holds data for the malicious report.
type MaliciousReportData struct {
	GeneratedAt      string
	CrawlerVersion   string
	TotalScanned     int
	TotalFlagged     int
	CriticalCount    int
	HighCount        int
	MediumCount      int
	LowCount         int
	Servers          []MaliciousServerEntry
}

// MaliciousServerEntry represents a flagged server in the report.
type MaliciousServerEntry struct {
	Name        string
	Repository  string
	RiskLevel   string
	Score       float64
	Signals     []MaliciousSignalEntry
	Recommend   string
	ScannedAt   string
	ReportURL   string
}

// MaliciousSignalEntry represents a malicious signal in the report.
type MaliciousSignalEntry struct {
	Name        string
	Description string
	Confidence  float64
	Evidence    string
}

// ExportMaliciousReport generates MALICIOUS_REPORT.md and blocklist.txt.
func (me *MaliciousExporter) ExportMaliciousReport(dir string, servers []models.MCPServer, threshold string) error {
	maliciousDir := filepath.Join(dir, "malicious")
	if err := os.MkdirAll(maliciousDir, 0755); err != nil {
		return fmt.Errorf("create malicious dir: %w", err)
	}

	// Filter servers with malicious findings
	var flagged []MaliciousServerEntry
	thresholdLevel := parseThreshold(threshold)

	for _, s := range servers {
		entries := extractMaliciousEntries(s)
		for _, e := range entries {
			if riskMeetsThreshold(e.RiskLevel, thresholdLevel) {
				flagged = append(flagged, e)
			}
		}
	}

	// Sort by risk level (CRITICAL first) then score
	sort.Slice(flagged, func(i, j int) bool {
		riskOrder := map[string]int{"CRITICAL": 4, "HIGH": 3, "MEDIUM": 2, "LOW": 1}
		if riskOrder[flagged[i].RiskLevel] != riskOrder[flagged[j].RiskLevel] {
			return riskOrder[flagged[i].RiskLevel] > riskOrder[flagged[j].RiskLevel]
		}
		return flagged[i].Score > flagged[j].Score
	})

	// Count by risk level
	counts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}
	for _, f := range flagged {
		counts[f.RiskLevel]++
	}

	// Generate report data
	reportData := MaliciousReportData{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		CrawlerVersion: "v1.0.0",
		TotalScanned:   len(servers),
		TotalFlagged:   len(flagged),
		CriticalCount:  counts["CRITICAL"],
		HighCount:      counts["HIGH"],
		MediumCount:    counts["MEDIUM"],
		LowCount:       counts["LOW"],
		Servers:        flagged,
	}

	// Generate markdown report
	reportPath := filepath.Join(maliciousDir, "MALICIOUS_REPORT.md")
	if err := generateMaliciousMarkdown(reportPath, reportData); err != nil {
		return fmt.Errorf("generate markdown: %w", err)
	}

	// Generate blocklist
	blocklistPath := filepath.Join(maliciousDir, "blocklist.txt")
	if err := generateBlocklist(blocklistPath, flagged); err != nil {
		return fmt.Errorf("generate blocklist: %w", err)
	}

	return nil
}

// extractMaliciousEntries extracts malicious findings from a server.
func extractMaliciousEntries(server models.MCPServer) []MaliciousServerEntry {
	var entries []MaliciousServerEntry

	for _, finding := range server.Security {
		if finding.Type == security.MaliciousType {
			// Parse the finding evidence to reconstruct signals
			signals := parseMaliciousEvidence(finding.Evidence)
			entries = append(entries, MaliciousServerEntry{
				Name:       server.Name,
				Repository: server.Repository.URL,
				RiskLevel:  string(finding.Severity),
				Score:      calculateRiskScore(finding.Severity),
				Signals:    signals,
				Recommend:  getRecommendation(finding.Severity),
				ScannedAt:  time.Now().UTC().Format(time.RFC3339),
				ReportURL:  fmt.Sprintf("https://github.com/%s", strings.TrimPrefix(server.Repository.URL, "https://github.com/")),
			})
		}
	}
	return entries
}

// parseMaliciousEvidence parses the finding evidence into signal entries.
func parseMaliciousEvidence(evidence string) []MaliciousSignalEntry {
	var signals []MaliciousSignalEntry
	parts := strings.Split(evidence, "; ")
	for _, part := range parts {
		kv := strings.SplitN(part, ": ", 2)
		if len(kv) == 2 {
			signals = append(signals, MaliciousSignalEntry{
				Name:        kv[0],
				Description: getSignalDescription(kv[0]),
				Confidence:  0.8, // default
				Evidence:    kv[1],
			})
		}
	}
	return signals
}

// getSignalDescription returns human-readable description for a signal.
func getSignalDescription(name string) string {
	descriptions := map[string]string{
		"high_entropy":          "README has unusually high Shannon entropy (likely binary/obfuscated)",
		"oversized_readme":      "README exceeds typical documentation size",
		"high_nonprintable":     "README contains high ratio of non-printable characters",
		"obfuscation_pattern":   "Detected code obfuscation pattern",
		"throwaway_account":     "New account with multiple anomaly indicators",
		"lua_bytecode":          "Detected Lua VM bytecode pattern",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Unknown signal"
}

// calculateRiskScore maps severity to numeric score.
func calculateRiskScore(sev models.SecuritySeverity) float64 {
	switch sev {
	case models.SeverityCritical:
		return 90
	case models.SeverityHigh:
		return 70
	case models.SeverityMedium:
		return 45
	case models.SeverityLow:
		return 20
	default:
		return 0
	}
}

// getRecommendation returns action recommendation for severity.
func getRecommendation(sev models.SecuritySeverity) string {
	switch sev {
	case models.SeverityCritical:
		return "block,report"
	case models.SeverityHigh:
		return "block,investigate"
	case models.SeverityMedium:
		return "investigate"
	default:
		return "monitor"
	}
}

// parseThreshold parses the threshold string to severity level.
func parseThreshold(threshold string) models.SecuritySeverity {
	switch strings.ToUpper(threshold) {
	case "CRITICAL":
		return models.SeverityCritical
	case "HIGH":
		return models.SeverityHigh
	case "MEDIUM":
		return models.SeverityMedium
	case "LOW":
		return models.SeverityLow
	default:
		return models.SeverityMedium
	}
}

// riskMeetsThreshold checks if risk level meets or exceeds threshold.
func riskMeetsThreshold(riskLevel string, threshold models.SecuritySeverity) bool {
	riskOrder := map[string]int{"CRITICAL": 4, "HIGH": 3, "MEDIUM": 2, "LOW": 1}
	thresholdOrder := map[models.SecuritySeverity]int{
		models.SeverityCritical: 4,
		models.SeverityHigh:     3,
		models.SeverityMedium:   2,
		models.SeverityLow:      1,
	}
	return riskOrder[riskLevel] >= thresholdOrder[threshold]
}

// generateMaliciousMarkdown generates the MALICIOUS_REPORT.md file.
func generateMaliciousMarkdown(path string, data MaliciousReportData) error {
	var sb strings.Builder

	sb.WriteString("# Malicious Repository Detection Report\n\n")
	sb.WriteString(fmt.Sprintf("> Generated on %s\n\n", data.GeneratedAt))

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Scanned**: %d\n", data.TotalScanned))
	sb.WriteString(fmt.Sprintf("- **Total Flagged**: %d\n", data.TotalFlagged))
	sb.WriteString(fmt.Sprintf("- **CRITICAL**: %d\n", data.CriticalCount))
	sb.WriteString(fmt.Sprintf("- **HIGH**: %d\n", data.HighCount))
	sb.WriteString(fmt.Sprintf("- **MEDIUM**: %d\n", data.MediumCount))
	sb.WriteString(fmt.Sprintf("- **LOW**: %d\n\n", data.LowCount))

	if data.TotalFlagged == 0 {
		sb.WriteString("No malicious repositories detected.\n")
		return writeFile(path, []byte(sb.String()))
	}

	sb.WriteString("## Flagged Repositories\n\n")
	sb.WriteString("| Repository | Risk Level | Score | Signals | Recommendation |\n")
	sb.WriteString("|------------|------------|-------|---------|----------------|\n")

	for _, s := range data.Servers {
		signalNames := make([]string, len(s.Signals))
		for i, sig := range s.Signals {
			signalNames[i] = sig.Name
		}
		sb.WriteString(fmt.Sprintf("| [%s](%s) | %s | %.0f | %s | %s |\n",
			s.Name, s.Repository, s.RiskLevel, s.Score, strings.Join(signalNames, ", "), s.Recommend))
	}

	sb.WriteString("\n## Detailed Findings\n\n")
	for _, s := range data.Servers {
		sb.WriteString(fmt.Sprintf("### %s (%s)\n\n", s.Name, s.Repository))
		sb.WriteString(fmt.Sprintf("- **Risk Level**: %s\n", s.RiskLevel))
		sb.WriteString(fmt.Sprintf("- **Score**: %.0f\n", s.Score))
		sb.WriteString(fmt.Sprintf("- **Recommendation**: %s\n", s.Recommend))
		sb.WriteString(fmt.Sprintf("- **Scanned At**: %s\n", s.ScannedAt))
		if len(s.Signals) > 0 {
			sb.WriteString("- **Signals**:\n")
			for _, sig := range s.Signals {
				sb.WriteString(fmt.Sprintf("  - **%s** (%.0f%%): %s\n", sig.Name, sig.Confidence*100, sig.Evidence))
				sb.WriteString(fmt.Sprintf("    *%s*\n", sig.Description))
			}
		}
		sb.WriteString(fmt.Sprintf("- **Report URL**: %s\n\n", s.ReportURL))
	}

	// GitHub reporting template
	sb.WriteString("## GitHub Reporting Template\n\n")
	sb.WriteString("For each CRITICAL/HIGH risk repository, you can report to GitHub using:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("Repository: [owner/repo]\n")
	sb.WriteString("Reason: Malicious repository / Supply chain attack\n")
	sb.WriteString("Evidence: [paste relevant signals from above]\n")
	sb.WriteString("```\n\n")

	return writeFile(path, sanitizeUTF8([]byte(sb.String())))
}

// generateBlocklist generates blocklist.txt with owner/repo entries.
func generateBlocklist(path string, flagged []MaliciousServerEntry) error {
	var lines []string
	for _, s := range flagged {
		// Extract owner/repo from URL
		repoPath := strings.TrimPrefix(s.Repository, "https://github.com/")
		repoPath = strings.TrimSuffix(repoPath, "/")
		lines = append(lines, fmt.Sprintf("%s # RISK: %s - Score: %.0f - %s", repoPath, s.RiskLevel, s.Score, s.Recommend))
	}
	return writeFile(path, sanitizeUTF8([]byte(strings.Join(lines, "\n")+"\n")))
}
