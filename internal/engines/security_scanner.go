// Package engines implements security scanning for discovered entities.
// This scanner detects obfuscation, credential extraction, remote binary
// download, shell injection, persistence mechanisms, network beaconing,
// and filesystem abuse in source code and manifests.
//
// Implements spec §12, §56 Test 12, §61 Phase 8, §64 Definition of Done.
package engines

import (
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// SecurityScanResult holds the result of a security scan (spec §12, §56 Test 12).
type SecurityScanResult struct {
	Status          models.SecurityStatus `json:"status"`
	Findings        []models.SecurityFinding `json:"findings"`
	ScannedAt       time.Time             `json:"scanned_at"`
	ScannerVersion  string                `json:"scanner_version"`
	Confidence      float64               `json:"confidence"`
}

// SecurityScanner scans entities for security threats.
// Security is independent from classification, MCP identity, Taiwan/AI
// relevance, and quality score (spec §45).
type SecurityScanner struct {
	version string
}

// NewSecurityScanner creates a new security scanner.
func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{
		version: "2.0.0",
	}
}

// ScannerVersion returns the scanner version string.
func (s *SecurityScanner) ScannerVersion() string {
	return s.version
}

// Scan performs security scanning on an entity (spec §12, §56 Test 12).
// Scans source code, package manifests, scripts, README, and endpoints.
func (s *SecurityScanner) Scan(entity *models.Entity) SecurityScanResult {
	findings := []models.SecurityFinding{}
	scannedAt := time.Now().UTC()

	// Aggregate all content to scan
	content := s.collectContent(entity)

	// 1. Obfuscation detection
	findings = append(findings, s.scanObfuscation(content)...)

	// 2. Credential extraction
	findings = append(findings, s.scanCredentials(content)...)

	// 3. Remote binary download
	findings = append(findings, s.scanRemoteBinaryDownloads(content)...)

	// 4. Shell injection
	findings = append(findings, s.scanShellInjection(content)...)

	// 5. Persistence mechanisms
	findings = append(findings, s.scanPersistence(content)...)

	// 6. Network beaconing
	findings = append(findings, s.scanNetworkBeaconing(content)...)

	// 7. Filesystem abuse
	findings = append(findings, s.scanFilesystemAbuse(content)...)

	// 8. Endpoint security checks
	findings = append(findings, s.scanEndpoints(entity)...)

	// Determine overall status
	status := s.determineStatus(findings)

	// Calculate confidence
	confidence := s.calculateConfidence(findings)

	return SecurityScanResult{
		Status:          status,
		Findings:        findings,
		ScannedAt:       scannedAt,
		ScannerVersion:  s.version,
		Confidence:      confidence,
	}
}

// collectContent gathers all text content from an entity for scanning.
func (s *SecurityScanner) collectContent(entity *models.Entity) string {
	var parts []string

	if entity.RawContent != "" {
		parts = append(parts, entity.RawContent)
	}
	if entity.Description != "" {
		parts = append(parts, entity.Description)
	}
	if entity.Slug != "" {
		parts = append(parts, entity.Slug)
	}
	// Tools and resources descriptions
	for _, tool := range entity.Tools {
		if tool.Description != "" {
			parts = append(parts, tool.Description)
		}
	}
	for _, res := range entity.Resources {
		if res.Description != "" {
			parts = append(parts, res.Description)
		}
	}
	// Data source info
	for _, ds := range entity.DataSources {
		if ds.URL != "" {
			parts = append(parts, ds.URL)
		}
	}

	return strings.Join(parts, "\n\n")
}

// --- Detection Rules ---

// obfuscationPattern defines patterns indicating obfuscated code.
var obfuscationPatterns = []struct {
	rule        string
	pattern     *regexp.Regexp
	description string
}{
	{
		"base64_encoded_payload",
		regexp.MustCompile(`eval\s*\(\s*atob\s*\(`),
		"eval(atob()) pattern indicating base64-encoded payload execution",
	},
	{
		"base64_decode_execution",
		regexp.MustCompile(`(Buffer\.from|base64_decode|b64decode|atob)\s*\([^)]*\)\s*\)\s*[,;]?`),
		"base64 decode followed by execution",
	},
	{
		"minified_no_sourcemap",
		regexp.MustCompile(`(document\.write|innerHTML)\s*\([^)]{100,}\)`),
		"document.write/innerHTML with very long string (minified, no sourcemap)",
	},
	{
		"string_concatenation_obfuscation",
		regexp.MustCompile(`(["'][^"']{50,}["'])\s*\+(\s*["'][^"']{50,}["'])`),
		"suspicious string concatenation pattern",
	},
}

// scanObfuscation detects obfuscated or encoded code (spec §12, §56 Test 12).
func (s *SecurityScanner) scanObfuscation(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, op := range obfuscationPatterns {
		matches := op.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-50)
			ctxEnd := minInt(len(content), end+50)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "obfuscation",
				Severity:   "HIGH",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       op.rule,
				Confidence: 0.8,
			})
		}
	}

	return findings
}

// credentialPatterns are regex patterns for common credential formats.
var credentialPatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"github_token",
		regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`),
	},
	{
		"aws_access_key",
		regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	},
	{
		"aws_secret_key",
		regexp.MustCompile(`(?i)aws_secret_access_key\s*[:=]\s*["']?[a-zA-Z0-9/+=]{40}["']?`),
	},
	{
		"google_api_key",
		regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	},
	{
		"slack_token",
		regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]+`),
	},
	{
		"database_url",
		regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:\s]+:[^@\s]+@`),
	},
	{
		"private_key",
		regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
	},
	{
		"generic_api_key",
		regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*["']?[a-zA-Z0-9_\-]{20,}["']?`),
	},
	{
		"password_field",
		regexp.MustCompile(`(?i)password\s*[:=]\s*["'][^"'\s]{8,}["']?`),
	},
	{
		"env_var_theft",
		regexp.MustCompile(`(process\.env\.[A-Z_]+|os\.environ\[|os\.getenv\()`),
	},
}

// scanCredentials checks for hardcoded secrets and credential extraction (spec §12).
func (s *SecurityScanner) scanCredentials(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, cp := range credentialPatterns {
		matches := cp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-30)
			ctxEnd := minInt(len(content), end+30)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "credential_extraction",
				Severity:   "CRITICAL",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       cp.rule,
				Confidence: 0.85,
			})
		}
	}

	return findings
}

// remoteBinaryPatterns detect curl|wget piped to shell (spec §12).
var remoteBinaryPatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"curl_pipe_bash",
		regexp.MustCompile(`(?i)curl\s+[^|]*\|\s*(bash|sh)`),
	},
	{
		"wget_pipe_sh",
		regexp.MustCompile(`(?i)wget\s+[^|]*\-\-?\S*\s+(?i)\|?\s*(sh|bash)|wget\s+[^|]*\|\s*(bash|sh)`),
	},
	{
		"remote_binary_exec",
		regexp.MustCompile(`(?i)(curl|wget|fetch).*[^\n]*\s*(?:>>\s*/dev/stdin\s+|tee\s+>)`),
	},
}

// scanRemoteBinaryDownloads detects download-and-execute patterns (spec §12).
func (s *SecurityScanner) scanRemoteBinaryDownloads(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, rp := range remoteBinaryPatterns {
		matches := rp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-50)
			ctxEnd := minInt(len(content), end+50)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "remote_binary_download",
				Severity:   "CRITICAL",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       rp.rule,
				Confidence: 0.9,
			})
		}
	}

	return findings
}

// shellInjectionPatterns detect unsanitized command execution (spec §12).
var shellInjectionPatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"os_system",
		regexp.MustCompile(`os\.system\s*\(`),
	},
	{
		"subprocess_run",
		regexp.MustCompile(`subprocess\.(run|Popen|call)\s*\(`),
	},
	{
		"child_process_exec",
		regexp.MustCompile(`child_process\.(exec|execSync|spawn)\s*\(`),
	},
	{
		"shell_js",
		regexp.MustCompile(`shell\.exec\s*\(|execa\s*\(|\bexec\s*\(`),
	},
}

// scanShellInjection detects shell injection vulnerabilities (spec §12).
func (s *SecurityScanner) scanShellInjection(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, sp := range shellInjectionPatterns {
		matches := sp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-40)
			ctxEnd := minInt(len(content), end+40)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "shell_injection",
				Severity:   "HIGH",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       sp.rule,
				Confidence: 0.75,
			})
		}
	}

	return findings
}

// persistencePatterns detect persistence mechanisms (spec §12).
var persistencePatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"cron_job",
		regexp.MustCompile(`crontab\s+|/etc/cron\.(d|daily|hourly|weekly|monthly)`),
	},
	{
		"systemd_service",
		regexp.MustCompile(`/etc/systemd/system/|\.service\s*\[`),
	},
	{
		"startup_file",
		regexp.MustCompile(`/etc/rc\.local|~/.bashrc|~/.profile|~/.ssh/rc|~/.config/autostart`),
	},
	{
		"windows_startup",
		regexp.MustCompile(`HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run|HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run`),
	},
}

// scanPersistence detects persistence mechanism abuse (spec §12).
func (s *SecurityScanner) scanPersistence(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, pp := range persistencePatterns {
		matches := pp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-30)
			ctxEnd := minInt(len(content), end+30)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "persistence_mechanism",
				Severity:   "HIGH",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       pp.rule,
				Confidence: 0.8,
			})
		}
	}

	return findings
}

// networkBeaconingPatterns detect network beaconing/C2 patterns (spec §12).
var networkBeaconingPatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"periodic_http_callback",
		regexp.MustCompile(`setInterval\s*\(\s*(?:async\s*)?\(\)\s*=>\s*\{\s*\n[^}]*fetch\s*\(`),
	},
	{
		"hardcoded_c2_domain",
		regexp.MustCompile(`(?i)(?:c2|command-and-control|beac[eo]n)[.\w-]*\.(?:com|net|org|io|ru)`),
	},
	{
		"websocket_reconnect",
		regexp.MustCompile(`(?i)WebSocket\s*\([^)]*\)\s*[^}]*onclose[^}]*reconnect|onclose\s*=\s*\(\)\s*=>\s*(?:location\.reload|connect)`),
	},
}

// scanNetworkBeaconing detects network beaconing to suspicious domains (spec §12).
func (s *SecurityScanner) scanNetworkBeaconing(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, bp := range networkBeaconingPatterns {
		matches := bp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-40)
			ctxEnd := minInt(len(content), end+40)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "network_beaconing",
				Severity:   "HIGH",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       bp.rule,
				Confidence: 0.8,
			})
		}
	}

	return findings
}

// filesystemAbusePatterns detect writes to sensitive directories (spec §12).
var filesystemAbusePatterns = []struct {
	rule   string
	pattern *regexp.Regexp
}{
	{
		"etc_write",
		regexp.MustCompile(`/etc/(passwd|shadow|hosts|sudoers)`),
	},
	{
		"ssh_dir_write",
		regexp.MustCompile(`~/.ssh/(authorized_keys|id_rsa|known_hosts)`),
	},
	{
		"root_write",
		regexp.MustCompile(`/root/|\bsudo\b\s+(?:rm\s+-rf|chmod|chown|mv)`),
	},
	{
		"authorized_keys_inject",
		regexp.MustCompile(`(?i)\.ssh/authorized_keys`),
	},
}

// scanFilesystemAbuse detects writes to sensitive filesystem locations (spec §12).
func (s *SecurityScanner) scanFilesystemAbuse(content string) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, fp := range filesystemAbusePatterns {
		matches := fp.pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			ctxStart := maxInt(0, start-40)
			ctxEnd := minInt(len(content), end+40)
			evidence := content[ctxStart:ctxEnd]

			findings = append(findings, models.SecurityFinding{
				Type:       "filesystem_abuse",
				Severity:   "CRITICAL",
				Source:     "source_code",
				Location:   "readme/source",
				Evidence:   evidence,
				Rule:       fp.rule,
				Confidence: 0.85,
			})
		}
	}

	return findings
}

// scanEndpoints checks endpoints for security issues (insecure transport, localhost).
func (s *SecurityScanner) scanEndpoints(entity *models.Entity) []models.SecurityFinding {
	var findings []models.SecurityFinding

	for _, ep := range entity.Endpoints {
		// Check URL for insecure transport
		if !strings.HasPrefix(ep.Endpoint.URL, "https://") &&
			strings.HasPrefix(ep.Endpoint.URL, "http://") {
			findings = append(findings, models.SecurityFinding{
				Type:       "insecure_transport",
				Severity:   "MEDIUM",
				Source:     "endpoint",
				Location:   ep.Endpoint.URL,
				Evidence:   ep.Endpoint.URL,
				Rule:       "http_url_pattern",
				Confidence: 0.9,
			})
		}

		// Check for localhost exposure
		if s.isLocalhost(ep.Endpoint.URL) {
			findings = append(findings, models.SecurityFinding{
				Type:       "localhost_exposure",
				Severity:   "MEDIUM",
				Source:     "endpoint",
				Location:   ep.Endpoint.URL,
				Evidence:   ep.Endpoint.URL,
				Rule:       "localhost_exposed_pattern",
				Confidence: 0.85,
			})
		}
	}

	return findings
}

// isLocalhost checks if a URL points to localhost.
func (s *SecurityScanner) isLocalhost(url string) bool {
	lower := strings.ToLower(url)
	return strings.Contains(lower, "127.0.0.1") ||
		strings.Contains(lower, "localhost") ||
		strings.Contains(lower, "0.0.0.0") ||
		strings.Contains(lower, "169.254.") ||
		strings.Contains(lower, "local")
}

// --- Status & Confidence Determination ---

// determineStatus determines overall security status from findings.
func (s *SecurityScanner) determineStatus(findings []models.SecurityFinding) models.SecurityStatus {
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
		return models.SecurityStatusQuarantined
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
func (s *SecurityScanner) calculateConfidence(findings []models.SecurityFinding) float64 {
	if len(findings) == 0 {
		return 1.0
	}

	total := 0.0
	for _, f := range findings {
		total += f.Confidence
	}
	return total / float64(len(findings))
}

// ToSecurityStatusDetail converts a SecurityScanResult to models.SecurityStatusDetail
// for backward compatibility with the Entity model.
func (r SecurityScanResult) ToSecurityStatusDetail() models.SecurityStatusDetail {
	return models.SecurityStatusDetail{
		Status:         r.Status,
		Findings:       r.Findings,
		ScannedAt:      models.RFC3339Time(r.ScannedAt),
		ScannerVersion: r.ScannerVersion,
		Confidence:     r.Confidence,
	}
}

// maxInt returns the larger of two integers.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
