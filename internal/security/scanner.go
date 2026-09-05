package security

import (
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Scanner scans for security issues in MCP server configuration (§33, §34).
type Scanner struct {
	dangerousPatterns []*regexp.Regexp
	maliciousDetector *MaliciousDetector
}

// New creates a new security Scanner.
func New() *Scanner {
	s := &Scanner{
		dangerousPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\beval\s*\(`),
			regexp.MustCompile(`(?i)\bexec(?:_|\s*)\(`),
			regexp.MustCompile(`(?i)\bchild_process\.exec`),
			regexp.MustCompile(`(?i)\brequire\s*\(\s*['"(?:child_process|fs)['"]\s*\)`),
			regexp.MustCompile(`(?i)\b(?:npm|pip|go|composer)_install`),
			regexp.MustCompile(`(?i)\bsubprocess\.(?:Popen|run|call)\(`),
			regexp.MustCompile(`(?i)\bos\.system\s*\(`),
			regexp.MustCompile(`(?i)\bspawn\s*\(`),
			regexp.MustCompile(`(?i)\bcurl\s+.*\|\s*(?:sh|bash)`),
			regexp.MustCompile(`(?i)\beval\s+.*\|\s*(?:sh|bash)`),
			regexp.MustCompile(`(?i)os_system`),
			regexp.MustCompile(`(?i)subprocess_run`),
			regexp.MustCompile(`(?i)run_exec`),
		},
		maliciousDetector: NewMaliciousDetector(),
	}
	return s
}

// SecurityScanResult holds the results of scanning an MCPServer.
type SecurityScanResult struct {
	Findings      []models.SecurityFinding
	RiskLevel     models.SecuritySeverity
	ScoreImpact   float64
	LastScanned   string
}

// ScanServer scans an MCPServer for security issues (§33).
func (s *Scanner) ScanServer(server *models.MCPServer) SecurityScanResult {
	var findings []models.SecurityFinding
	seen := make(map[string]bool)

	// Scan tool input schemas for injection patterns (§33.1: injection in input_schema)
	for _, tool := range server.Tools {
		for _, finding := range s.scanTool(tool) {
			key := finding.Type + ":" + finding.Location
			if !seen[key] {
				findings = append(findings, finding)
				seen[key] = true
			}
		}
	}

	// Scan endpoints for risky patterns (§33.2: insecure transport, no TLS)
	for _, ep := range server.Endpoints {
		for _, finding := range s.scanEndpoint(ep) {
			key := finding.Type + ":" + finding.Location
			if !seen[key] {
				findings = append(findings, finding)
				seen[key] = true
			}
		}
	}

	// Scan repository metadata (§33.3: malicious repository patterns)
	for _, finding := range s.scanRepository(&server.Repository) {
		key := finding.Type + ":" + finding.Location
		if !seen[key] {
			findings = append(findings, finding)
			seen[key] = true
		}
	}

	// Scan for malicious repository patterns (§33.4: supply chain attack detection)
	if s.maliciousDetector != nil {
		// Use server.Readme for README content
		// For account metadata, use what's available from RepositoryInfo
		var accountCreatedAt *time.Time
		if !server.Repository.CreatedAt.IsZero() {
			accountCreatedAt = &server.Repository.CreatedAt
		}
		// Follower count, profile fields, repo count not directly available
		// Pass 0 for unknown values (will be treated as suspicious if account is new)
		maliciousResult := s.maliciousDetector.Detect(
			server,
			server.Readme,
			accountCreatedAt,
			0, // followerCount - unknown
			0, // profileFieldCount - unknown
			0, // repoCount - unknown
		)
		if maliciousResult.IsMalicious() {
			finding := maliciousResult.ToSecurityFinding(server.Repository.URL)
			key := finding.Type + ":" + finding.Location
			if !seen[key] {
				findings = append(findings, finding)
				seen[key] = true
			}
		}
	}
	// Compute overall risk level and score impact
	riskLevel := models.SeverityUnknown
	if len(findings) == 0 {
		riskLevel = models.SeverityUnknown
	} else {
		for _, f := range findings {
			if severityRank(f.Severity) > severityRank(riskLevel) {
				riskLevel = f.Severity
			}
		}
	}

	scoreImpact := calculateScoreImpact(findings)

	return SecurityScanResult{
		Findings:    findings,
		RiskLevel:   riskLevel,
		ScoreImpact: scoreImpact,
	}
}

func (s *Scanner) scanTool(tool models.Tool) []models.SecurityFinding {
	var findings []models.SecurityFinding

	// Check tool name and description for suspicious patterns and keywords
	combined := strings.ToLower(tool.Name + " " + tool.Description)
	for _, pattern := range s.dangerousPatterns {
		if pattern.MatchString(combined) {
			findings = append(findings, models.SecurityFinding{
				Type:     "suspicious_tool_name",
				Severity: models.SeverityHigh,
				Source:   "security.scanner",
				Location: "tool:" + tool.Name,
				Evidence: "Potentially dangerous function call pattern detected in tool metadata",
			})
			break
		}
	}

	// Also check for dangerous keywords in tool name
	if s.hasDangerousKeyword(combined) {
		findings = append(findings, models.SecurityFinding{
			Type:     "suspicious_tool_name",
			Severity: models.SeverityMedium,
			Source:   "security.scanner",
			Location: "tool:" + tool.Name,
			Evidence: "Potentially dangerous keyword detected in tool metadata",
		})
	}

	// Check input_schema for code execution hints
	if raw, ok := tool.InputSchema["properties"]; ok {
		if propsStr, ok := raw.(string); ok {
			for _, pattern := range s.dangerousPatterns {
				if pattern.MatchString(strings.ToLower(propsStr)) {
					findings = append(findings, models.SecurityFinding{
						Type:     "unsafe_input_schema",
						Severity: models.SeverityHigh,
						Source:   "security.scanner",
						Location: "tool:" + tool.Name + ".input_schema",
						Evidence: "Unsafe deserialization or code execution pattern in input schema",
					})
					break
				}
			}
		}
	}

	return findings
}

func (s *Scanner) scanEndpoint(ep models.Endpoint) []models.SecurityFinding {
	var findings []models.SecurityFinding

	// Check for insecure HTTP (not HTTPS) endpoints
	if !ep.TLS && strings.HasPrefix(strings.ToLower(ep.URL), "http://") {
		findings = append(findings, models.SecurityFinding{
			Type:     "insecure_transport",
			Severity: models.SeverityLow,
			Source:   "security.scanner",
			Location: "endpoint:" + ep.URL,
			Evidence: "Endpoint uses HTTP without TLS",
		})
	}

	// Check for localhost/127.0.0.1 endpoints exposed publicly
	lowerURL := strings.ToLower(ep.URL)
	if strings.Contains(lowerURL, "127.0.0.1") || strings.Contains(lowerURL, "localhost") {
		findings = append(findings, models.SecurityFinding{
			Type:     "localhost_exposure",
			Severity: models.SeverityLow,
			Source:   "security.scanner",
			Location: "endpoint:" + ep.URL,
			Evidence: "Endpoint binds to localhost — may not be accessible",
		})
	}

	return findings
}

func (s *Scanner) scanRepository(repo *models.RepositoryInfo) []models.SecurityFinding {
	var findings []models.SecurityFinding

	// Check if it's a fork that might be a malicious copy
	if repo.Fork {
		findings = append(findings, models.SecurityFinding{
			Type:     "fork_repository",
			Severity: models.SeverityLow,
			Source:   "security.scanner",
			Location: "repository:" + repo.URL,
			Evidence: "Repository is a fork — verify upstream authenticity",
		})
	}

	return findings
}

func severityRank(s models.SecuritySeverity) int {
	switch s {
	case models.SeverityCritical:
		return 4
	case models.SeverityHigh:
		return 3
	case models.SeverityMedium:
		return 2
	case models.SeverityLow:
		return 1
	default:
		return 0
	}
}

func calculateScoreImpact(findings []models.SecurityFinding) float64 {
	impact := 0.0
	for _, f := range findings {
		switch f.Severity {
		case models.SeverityCritical:
			impact -= 5
		case models.SeverityHigh:
			impact -= 3
		case models.SeverityMedium:
			impact -= 1
		case models.SeverityLow:
			impact -= 0.5
		}
	}
	if impact < 0 {
		impact = 0
	}
	return impact
}

// dangerousKeywords are suspicious substrings commonly found in risky tool names.
var dangerousKeywords = []string{
	"eval", "exec", "system", "shell", "subprocess", "spawn",
	"child_process", "os.system", "os.popen", "runtime.exec",
}

func (s *Scanner) hasDangerousKeyword(s_combined string) bool {
	for _, kw := range dangerousKeywords {
		if strings.Contains(s_combined, kw) {
			return true
		}
	}
	return false
}
