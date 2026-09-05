package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/security"
)
// MaliciousExporter handles export of malicious detection reports.
type MaliciousExporter struct {
	GeneratedAt time.Time
	OutputDir   string
	Entities    []*models.Entity
}

// MaliciousEntry represents a malicious detection entry for reporting.
type MaliciousEntry struct {
	Entity       *models.Entity
	Result       security.MaliciousResult
	RiskLevel    security.RiskLevel
	HighestSev   string
}

// NewMaliciousExporter creates a new malicious exporter.
func NewMaliciousExporter(entities []*models.Entity, outputDir string) *MaliciousExporter {
	return &MaliciousExporter{
		GeneratedAt: time.Now().UTC(),
		OutputDir:   outputDir,
		Entities:    entities,
	}
}

// Export generates malicious reports (MALICIOUS_REPORT.md and blocklist.txt).
func (e *MaliciousExporter) Export() error {
	if err := os.MkdirAll(e.OutputDir, 0755); err != nil {
		return err
	}

	// Collect malicious findings
	var entries []MaliciousEntry
	for _, entity := range e.Entities {
		if entity.SecurityStatus.Findings == nil {
			continue
		}

		hasMalicious := false
		highestSev := ""
		for _, f := range entity.SecurityStatus.Findings {
			if f.Type == "malicious_repository" {
				hasMalicious = true
				sev := strings.ToUpper(f.Severity)
				if highestSev == "" || severityRank(sev) > severityRank(highestSev) {
					highestSev = sev
				}
			}
		}

		if hasMalicious {
			// Re-run detection to get full result
			detector := security.NewMaliciousDetector()
			result := detector.Detect(entity.RawContent, security.RepositoryInfo{
				OwnerCreatedAt: toTimePtr(entity.Repository.CreatedAt),
				OwnerFollowers: nil,
				OwnerBio:       nil,
				OwnerRepos:     nil,
			})
			entries = append(entries, MaliciousEntry{
				Entity:    entity,
				Result:    result,
				RiskLevel: result.RiskLevel,
				HighestSev: highestSev,
			})
		}
	}

	// Generate MALICIOUS_REPORT.md
	if err := e.generateMarkdownReport(entries); err != nil {
		return err
	}

	// Generate blocklist.txt
	if err := e.generateBlocklist(entries); err != nil {
		return err
	}

	return nil
}

// generateMarkdownReport generates MALICIOUS_REPORT.md.
func (e *MaliciousExporter) generateMarkdownReport(entries []MaliciousEntry) error {
	var sb strings.Builder

	sb.WriteString("# Malicious Repository Detection Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", e.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total entities scanned: %d\n", len(e.Entities)))
	sb.WriteString(fmt.Sprintf("Malicious repositories detected: %d\n\n", len(entries)))

	// Summary by risk level
	riskCounts := make(map[security.RiskLevel]int)
	for _, entry := range entries {
		riskCounts[entry.RiskLevel]++
	}

	sb.WriteString("## Summary by Risk Level\n\n")
	for _, level := range []security.RiskLevel{security.RiskLevelCritical, security.RiskLevelHigh, security.RiskLevelMedium, security.RiskLevelLow} {
		count := riskCounts[level]
		if count > 0 {
			sb.WriteString(fmt.Sprintf("- **%s**: %d\n", level, count))
		}
	}
	sb.WriteString("\n")

	// Detailed entries
	sb.WriteString("## Detailed Findings\n\n")

	for _, entry := range entries {
		repoURL := entry.Entity.Repository.URL
		if repoURL == "" {
			repoURL = fmt.Sprintf("https://github.com/%s/%s", entry.Entity.Repository.Owner, entry.Entity.Repository.Name)
		}

		sb.WriteString(fmt.Sprintf("### %s/%s\n\n", entry.Entity.Repository.Owner, entry.Entity.Repository.Name))
		sb.WriteString(fmt.Sprintf("- **Repository**: %s\n", repoURL))
		sb.WriteString(fmt.Sprintf("- **Risk Level**: %s\n", entry.RiskLevel))
		sb.WriteString(fmt.Sprintf("- **Overall Confidence**: %.2f\n", entry.Result.Confidence))
		sb.WriteString(fmt.Sprintf("- **Highest Signal Severity**: %s\n", entry.HighestSev))
		sb.WriteString("\n")

		// Signals table
		if len(entry.Result.Signals) > 0 {
			sb.WriteString("#### Detection Signals\n\n")
			sb.WriteString("| Signal Type | Severity | Confidence | Evidence |\n")
			sb.WriteString("|-------------|----------|------------|----------|\n")
			for _, signal := range entry.Result.Signals {
				evidence := truncateForMarkdown(signal.Evidence, 100)
				sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %s |\n",
					signal.Type, signal.Severity, signal.Confidence, evidence))
			}
			sb.WriteString("\n")
		}

		// Recommended action
		action := e.recommendedAction(entry.RiskLevel)
		sb.WriteString(fmt.Sprintf("#### Recommended Action\n\n%s\n\n", action))

		// GitHub report template
		sb.WriteString("#### GitHub Report Template\n\n")
		sb.WriteString("```\n")
		sb.WriteString(e.githubReportTemplate(entry.Entity, entry))
		sb.WriteString("```\n\n")

		sb.WriteString("---\n\n")
	}

	// Sanitize and write
	markdown := SanitizeString(sb.String())
	path := filepath.Join(e.OutputDir, "MALICIOUS_REPORT.md")
	return os.WriteFile(path, []byte(markdown), 0644)
}

// generateBlocklist generates blocklist.txt.
func (e *MaliciousExporter) generateBlocklist(entries []MaliciousEntry) error {
	var sb strings.Builder

	sb.WriteString("# Malicious Repository Blocklist\n")
	sb.WriteString(fmt.Sprintf("# Generated at: %s\n", e.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString("# Format: owner/repo # RISK: LEVEL - PRIMARY_SIGNAL\n\n")

	for _, entry := range entries {
		// Only include HIGH and CRITICAL in blocklist
		if entry.RiskLevel == security.RiskLevelHigh || entry.RiskLevel == security.RiskLevelCritical {
			primarySignal := ""
			if len(entry.Result.Signals) > 0 {
				primarySignal = entry.Result.Signals[0].Type
			}
			line := fmt.Sprintf("%s/%s # RISK: %s - %s\n",
				entry.Entity.Repository.Owner,
				entry.Entity.Repository.Name,
				entry.RiskLevel,
				primarySignal)
			sb.WriteString(line)
		}
	}

	path := filepath.Join(e.OutputDir, "blocklist.txt")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// recommendedAction returns recommended action based on risk level.
func (e *MaliciousExporter) recommendedAction(riskLevel security.RiskLevel) string {
	switch riskLevel {
	case security.RiskLevelCritical:
		return "🚫 **BLOCK IMMEDIATELY** - Do not use. Report to GitHub Security. Add to blocklist."
	case security.RiskLevelHigh:
		return "⚠️ **QUARANTINE** - Isolate for manual review. Do not integrate until cleared. Report to GitHub if confirmed malicious."
	case security.RiskLevelMedium:
		return "🔍 **REVIEW REQUIRED** - Investigate signals. May be false positive. Monitor for escalation."
	case security.RiskLevelLow:
		return "📝 **MONITOR** - Low risk indicators. Track for pattern changes. No immediate action needed."
	default:
		return "✅ **CLEAN** - No action needed."
	}
}

// githubReportTemplate generates a GitHub report template.
func (e *MaliciousExporter) githubReportTemplate(entity *models.Entity, entry MaliciousEntry) string {
	var sb strings.Builder

	repoURL := entity.Repository.URL
	if repoURL == "" {
		repoURL = fmt.Sprintf("https://github.com/%s/%s", entity.Repository.Owner, entity.Repository.Name)
	}

	sb.WriteString("### GitHub Security Report\n\n")
	sb.WriteString(fmt.Sprintf("**Repository**: %s\n", repoURL))
	sb.WriteString(fmt.Sprintf("**Risk Level**: %s\n", entry.RiskLevel))
	sb.WriteString(fmt.Sprintf("**Detection Time**: %s\n\n", e.GeneratedAt.Format(time.RFC3339)))

	sb.WriteString("**Description**:\n")
	sb.WriteString("This repository has been flagged by the Taiwan AI Ecosystem Crawler's malicious repository detector ")
	sb.WriteString("as potentially containing supply chain attack vectors.\n\n")

	if len(entry.Result.Signals) > 0 {
		sb.WriteString("**Detected Signals**:\n")
		for _, signal := range entry.Result.Signals {
			sb.WriteString(fmt.Sprintf("- **%s** (%s, confidence: %.0f%%): %s\n",
				signal.Type, signal.Severity, signal.Confidence*100, signal.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("**Evidence**:\n")
	for _, signal := range entry.Result.Signals {
		if signal.Evidence != "" {
			sb.WriteString(fmt.Sprintf("- %s: `%s`\n", signal.Type, truncateForMarkdown(signal.Evidence, 200)))
		}
	}

	sb.WriteString("\n**Requested Action**: ")
	switch entry.RiskLevel {
	case security.RiskLevelCritical:
		sb.WriteString("Immediate removal/blocking from GitHub. This repository shows signs of active supply chain attack (e.g., obfuscated malware payloads).")
	case security.RiskLevelHigh:
		sb.WriteString("Quarantine and investigation. High-confidence indicators of malicious behavior detected.")
	case security.RiskLevelMedium:
		sb.WriteString("Review for policy violations. Suspicious patterns detected requiring manual verification.")
	default:
		sb.WriteString("Monitor. Low-risk indicators detected.")
	}

	return sb.String()
}


// toTimePtr converts RFC3339Time to *time.Time
func toTimePtr(t models.RFC3339Time) *time.Time {
	tm := time.Time(t)
	return &tm
}