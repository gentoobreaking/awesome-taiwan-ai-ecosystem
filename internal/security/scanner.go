package security

import (
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Scanner performs security scanning on discovered entities.
type Scanner struct {
	maliciousDetector *MaliciousDetector
	version           string
}

// NewScanner creates a new security scanner.
func NewScanner() *Scanner {
	return &Scanner{
		maliciousDetector: NewMaliciousDetector(),
		version:           "1.0.0",
	}
}

// Scan performs security scanning on an entity.
func (s *Scanner) Scan(entity *models.Entity) *models.SecurityStatusDetail {
	findings := []models.SecurityFinding{}
	scannedAt := time.Now().UTC()

	// 1. Static analysis patterns (from spec §33)
	findings = append(findings, s.staticAnalysis(entity)...)

	// 2. Hardcoded secrets detection
	findings = append(findings, s.detectSecrets(entity)...)

	// 3. Malicious repository detection (T060)
	maliciousResult := s.maliciousDetector.Detect(entity.RawContent, RepositoryInfo{
		OwnerCreatedAt: toTimePtr(entity.Repository.CreatedAt),
		OwnerFollowers: nil, // Not available in RepositoryInfo
		OwnerBio:       nil, // Not available in RepositoryInfo
		OwnerRepos:     nil, // Not available in RepositoryInfo
	})
	findings = append(findings, s.maliciousResultToFindings(maliciousResult, entity)...)

	// Determine overall security status
	status := s.determineStatus(findings)

	// Calculate confidence
	confidence := s.calculateConfidence(findings)

	return &models.SecurityStatusDetail{
		Status:          status,
		Findings:        findings,
		ScannedAt:       models.RFC3339Time(scannedAt),
		ScannerVersion:  s.version,
		Confidence:      confidence,
	}
}
// staticAnalysis checks for dangerous code patterns.
func (s *Scanner) staticAnalysis(entity *models.Entity) []models.SecurityFinding {
	var findings []models.SecurityFinding

	// Patterns from spec §33
	patterns := []struct {
		findingType string
		pattern     string
		severity    string
		source      string
		rule        string
	}{
		// Shell execution
		{"shell_execution", `exec\s*\(|shell\s*\(|subprocess\s*\(|child_process\s*\(|os\.system\s*\(`, "HIGH", "source_code", "shell_exec_pattern"},
		// Filesystem write
		{"filesystem_write", `os\.WriteFile|ioutil\.WriteFile|fs\.writeFile|open\(.*O_WRONLY|open\(.*O_CREATE`, "HIGH", "source_code", "fs_write_pattern"},
		// Credential collection
		{"credential_extraction", `password|secret|token|api[_-]?key|access[_-]?key|private[_-]?key`, "CRITICAL", "source_code", "credential_pattern"},
		// Arbitrary URL fetch
		{"arbitrary_url_fetch", `http\.Get\(|requests\.get\(|fetch\(|axios\.get\(|curl\s+`, "MEDIUM", "source_code", "url_fetch_pattern"},
		// Browser automation
		{"browser_automation", `chromedp|puppeteer|playwright|selenium`, "MEDIUM", "source_code", "browser_automation_pattern"},
		// RCE patterns
		{"rce_pattern", `eval\s*\(|Function\s*\(|exec\s*\(|subprocess\.run|os\.popen`, "CRITICAL", "source_code", "rce_pattern"},
	}

	content := entity.RawContent

	for _, p := range patterns {
		re := regexp.MustCompile(`(?i)` + p.pattern)
		matches := re.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start := match[0]
			end := match[1]
			// Get context around match
			contextStart := max(0, start-50)
			contextEnd := min(len(content), end+50)
			evidence := content[contextStart:contextEnd]

			findings = append(findings, models.SecurityFinding{
				Type:        p.findingType,
				Severity:    p.severity,
				Source:      p.source,
				Location:    "readme/source",
				Evidence:    evidence,
				Rule:        p.rule,
				Confidence:  0.7,
			})
		}
	}

	return findings
}

// detectSecrets checks for hardcoded secrets.
func (s *Scanner) detectSecrets(entity *models.Entity) []models.SecurityFinding {
	var findings []models.SecurityFinding

	content := entity.RawContent

	// Secret patterns
	secretPatterns := []struct {
		pattern string
		rule    string
	}{
		{`(?i)(api[_-]?key|apikey)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{20,}["']?`, "api_key_pattern"},
		{`(?i)(password|passwd)\s*[:=]\s*["']?[^"'\s]{8,}["']?`, "password_pattern"},
		{`(?i)(token|access[_-]?token)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{20,}["']?`, "token_pattern"},
		{`(?i)(secret|secret[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{20,}["']?`, "secret_pattern"},
		{`(?i)(private[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{20,}["']?`, "private_key_pattern"},
		{`(?:^|\s)(sk|pk)_(live|test)_[a-zA-Z0-9]{24,}(?:\s|$)`, "stripe_key_pattern"},
		{`(?:^|\s)gh[pousr]_[a-zA-Z0-9]{36,}(?:\s|$)`, "github_token_pattern"},
		{`(?:^|\s)AIza[0-9A-Za-z\-_]{35}(?:\s|$)`, "google_api_key_pattern"},
	}

	for _, p := range secretPatterns {
		re := regexp.MustCompile(p.pattern)
		matches := re.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start := match[0]
			end := match[1]
			contextStart := max(0, start-30)
			contextEnd := min(len(content), end+30)
			evidence := content[contextStart:contextEnd]

			findings = append(findings, models.SecurityFinding{
				Type:        "hardcoded_secret",
				Severity:    "CRITICAL",
				Source:      "source_code",
				Location:    "readme/source",
				Evidence:    evidence,
				Rule:        p.rule,
				Confidence:  0.8,
			})
		}
	}

	return findings
}

// maliciousResultToFindings converts MaliciousResult to SecurityFinding.
func (s *Scanner) maliciousResultToFindings(result MaliciousResult, entity *models.Entity) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, signal := range result.Signals {
		severity := string(signal.Severity)
		// Map malicious risk levels to security severity
		switch signal.Severity {
		case RiskLevelCritical:
			severity = "CRITICAL"
		case RiskLevelHigh:
			severity = "HIGH"
		case RiskLevelMedium:
			severity = "MEDIUM"
		case RiskLevelLow:
			severity = "LOW"
		}

		findings = append(findings, models.SecurityFinding{
			Type:        "malicious_repository",
			Severity:    severity,
			Source:      "malicious_detector",
			Location:    "readme/account",
			Evidence:    signal.Evidence,
			Rule:        signal.Type,
			Confidence:  signal.Confidence,
		})
	}

	return findings
}

// determineStatus determines overall security status from findings.
func (s *Scanner) determineStatus(findings []models.SecurityFinding) models.SecurityStatus {
	if len(findings) == 0 {
		return models.SecurityStatusClean
	}

	hasCritical := false
	hasHigh := false
	hasMedium := false

	for _, f := range findings {
		switch strings.ToUpper(f.Severity) {
		case "CRITICAL":
			hasCritical = true
		case "HIGH":
			hasHigh = true
		case "MEDIUM":
			hasMedium = true
		}
	}

	if hasCritical {
		return models.SecurityStatusQuarantined // or Blocked for confirmed
	}
	if hasHigh {
		return models.SecurityStatusQuarantined
	}
	if hasMedium {
		return models.SecurityStatusSuspicious
	}

	return models.SecurityStatusClean
}

// calculateConfidence calculates overall confidence from findings.
func (s *Scanner) calculateConfidence(findings []models.SecurityFinding) float64 {
	if len(findings) == 0 {
		return 1.0
	}

	total := 0.0
	for _, f := range findings {
		total += f.Confidence
	}
	return total / float64(len(findings))
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// toTimePtr converts RFC3339Time to *time.Time
func toTimePtr(t models.RFC3339Time) *time.Time {
	tm := time.Time(t)
	return &tm
}