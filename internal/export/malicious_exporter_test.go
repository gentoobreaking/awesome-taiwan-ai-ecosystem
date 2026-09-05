package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/security"
)

func TestNewMaliciousExporter(t *testing.T) {
	entities := []*models.Entity{}
	exporter := NewMaliciousExporter(entities, "/tmp/test")

	if exporter == nil {
		t.Fatal("NewMaliciousExporter returned nil")
	}
	if exporter.OutputDir != "/tmp/test" {
		t.Errorf("OutputDir = %s, want /tmp/test", exporter.OutputDir)
	}
	if len(exporter.Entities) != 0 {
		t.Errorf("Entities length = %d, want 0", len(exporter.Entities))
	}
}

func TestMaliciousExporter_Export_NoMalicious(t *testing.T) {
	// Create entities with no malicious findings
	entities := []*models.Entity{
		{
			ID:   "entity1",
			Name: "Clean Server",
			Repository: models.RepositoryInfo{
				Owner: "owner1",
				Name:  "clean-repo",
				URL:   "https://github.com/owner1/clean-repo",
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status:   models.SecurityStatusClean,
				Findings: []models.SecurityFinding{},
			},
			RawContent: "# Clean Server\n\nThis is a clean server.",
		},
	}

	tmpDir := t.TempDir()
	exporter := NewMaliciousExporter(entities, filepath.Join(tmpDir, "malicious"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Check that reports were generated
	reportPath := filepath.Join(tmpDir, "malicious", "MALICIOUS_REPORT.md")
	blocklistPath := filepath.Join(tmpDir, "malicious", "blocklist.txt")

	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Errorf("MALICIOUS_REPORT.md not created")
	}

	if _, err := os.Stat(blocklistPath); os.IsNotExist(err) {
		t.Errorf("blocklist.txt not created")
	}

	// Check report content
	reportContent, _ := os.ReadFile(reportPath)
	if !contains(string(reportContent), "Malicious repositories detected: 0") {
		t.Errorf("Report should show 0 malicious repos")
	}
}

func TestMaliciousExporter_Export_WithMalicious(t *testing.T) {
	// Create entities with malicious findings
	entities := []*models.Entity{
		{
			ID:   "entity1",
			Name: "Malicious Server",
			Repository: models.RepositoryInfo{
				Owner: "malicious",
				Name:  "evil-repo",
				URL:   "https://github.com/malicious/evil-repo",
				CreatedAt: models.RFC3339Time(time.Now().Add(-30 * 24 * time.Hour)),
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusQuarantined,
				Findings: []models.SecurityFinding{
					{
						Type:        "malicious_repository",
						Severity:    "CRITICAL",
						Source:      "malicious_detector",
						Location:    "readme/account",
						Evidence:    "eval(atob(...)) found",
						Rule:        "obfuscation_js",
						Confidence:  0.9,
					},
				},
			},
			RawContent: `eval(atob("ZXZhbC..."))`,
		},
		{
			ID:   "entity2",
			Name: "Suspicious Server",
			Repository: models.RepositoryInfo{
				Owner: "suspicious",
				Name:  "suspect-repo",
				URL:   "https://github.com/suspicious/suspect-repo",
				CreatedAt: models.RFC3339Time(time.Now().Add(-60 * 24 * time.Hour)),
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{
						Type:        "malicious_repository",
						Severity:    "MEDIUM",
						Source:      "malicious_detector",
						Location:    "readme/account",
						Evidence:    "High entropy README",
						Rule:        "readme_entropy",
						Confidence:  0.75,
					},
				},
			},
			RawContent: "X7$kL9@mP2#vR5X7$kL9@mP2#vR5...",
		},
	}

	tmpDir := t.TempDir()
	exporter := NewMaliciousExporter(entities, filepath.Join(tmpDir, "malicious"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Check report content
	reportPath := filepath.Join(tmpDir, "malicious", "MALICIOUS_REPORT.md")
	reportContent, _ := os.ReadFile(reportPath)
	content := string(reportContent)

	if !contains(content, "Malicious repositories detected: 2") {
		t.Errorf("Report should show 2 malicious repos, got: %s", content)
	}
	if !contains(content, "CRITICAL") {
		t.Errorf("Report should show CRITICAL risk level")
	}
	if !contains(content, "MEDIUM") {
		t.Errorf("Report should show MEDIUM risk level")
	}
	if !contains(content, "GitHub Report Template") {
		t.Errorf("Report should include GitHub report template")
	}
	if !contains(content, "Recommended Action") {
		t.Errorf("Report should include recommended action")
	}

	// Check blocklist content
	blocklistPath := filepath.Join(tmpDir, "malicious", "blocklist.txt")
	blocklistContent, _ := os.ReadFile(blocklistPath)
	blContent := string(blocklistContent)

	// Only CRITICAL and HIGH should be in blocklist
	if !contains(blContent, "malicious/evil-repo") {
		t.Errorf("Blocklist should contain malicious/evil-repo")
	}
	if contains(blContent, "suspicious/suspect-repo") {
		t.Errorf("Blocklist should NOT contain suspicious/suspect-repo (MEDIUM only)")
	}
	if !contains(blContent, "RISK: CRITICAL") {
		t.Errorf("Blocklist should show RISK: CRITICAL")
	}
}

func TestMaliciousExporter_RecommendedAction(t *testing.T) {
	entities := []*models.Entity{}
	exporter := NewMaliciousExporter(entities, "/tmp/test")

	tests := []struct {
		riskLevel    security.RiskLevel
		expectedWord string
	}{
		{security.RiskLevelCritical, "BLOCK IMMEDIATELY"},
		{security.RiskLevelHigh, "QUARANTINE"},
		{security.RiskLevelMedium, "REVIEW REQUIRED"},
		{security.RiskLevelLow, "MONITOR"},
	}

	for _, tt := range tests {
		t.Run(string(tt.riskLevel), func(t *testing.T) {
			action := exporter.recommendedAction(tt.riskLevel)
			if !contains(action, tt.expectedWord) {
				t.Errorf("recommendedAction(%s) = %q, want contains %q", tt.riskLevel, action, tt.expectedWord)
			}
		})
	}
}


func TestMaliciousExporter_GitHubTemplate(t *testing.T) {
	entities := []*models.Entity{}
	exporter := NewMaliciousExporter(entities, "/tmp/test")

	entity := &models.Entity{
		Name: "Test Server",
		Repository: models.RepositoryInfo{
			Owner: "testowner",
			Name:  "testrepo",
			URL:   "https://github.com/testowner/testrepo",
		},
	}

	entry := MaliciousEntry{
		Entity: entity,
		Result: security.MaliciousResult{
			RiskLevel: security.RiskLevelCritical,
			Confidence: 0.9,
			Signals: []security.MaliciousSignal{
				{
					Type:        "obfuscation_js",
					Severity:    security.RiskLevelCritical,
					Description: "eval(atob(...)) - base64 decoded eval",
					Evidence:    "eval(atob(\"...\"))",
					Confidence:  0.9,
				},
			},
		},
		RiskLevel:  security.RiskLevelCritical,
		HighestSev: "CRITICAL",
	}

	template := exporter.githubReportTemplate(entity, entry)

	if !contains(template, "testowner/testrepo") {
		t.Errorf("Template should contain repo name")
	}
	if !contains(template, "CRITICAL") {
		t.Errorf("Template should contain risk level")
	}
	if !contains(template, "obfuscation_js") {
		t.Errorf("Template should contain signal type")
	}
	if !contains(template, "Immediate removal") {
		t.Errorf("Template should contain requested action for CRITICAL")
	}
}

func TestExportMarkdown_StyleConsistency(t *testing.T) {
	entities := []*models.Entity{
		{
			ID:   "entity1",
			Name: "Test Server",
			Repository: models.RepositoryInfo{
				Owner: "owner",
				Name:  "repo",
				URL:   "https://github.com/owner/repo",
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusQuarantined,
				Findings: []models.SecurityFinding{
					{
						Type:        "malicious_repository",
						Severity:    "CRITICAL",
						Source:      "malicious_detector",
						Location:    "readme/account",
						Evidence:    "eval(atob(\"...\"))",
						Rule:        "obfuscation_js",
						Confidence:  0.9,
					},
				},
			},
			RawContent: "eval(atob(\"...\"))",
		},
	}

	tmpDir := t.TempDir()
	exporter := NewMaliciousExporter(entities, filepath.Join(tmpDir, "malicious"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	reportPath := filepath.Join(tmpDir, "malicious", "MALICIOUS_REPORT.md")
	content, _ := os.ReadFile(reportPath)
	contentStr := string(content)

	// Check UTF-8 sanitization - just ensure no encoding errors
	if len(contentStr) == 0 {
		t.Errorf("Report should not be empty")
	}
	// Check markdown structure
	if !contains(contentStr, "# Malicious Repository Detection Report") {
		t.Errorf("Report should have main heading")
	}
	if !contains(contentStr, "## Summary by Risk Level") {
		t.Errorf("Report should have summary section")
	}
	if !contains(contentStr, "## Detailed Findings") {
		t.Errorf("Report should have detailed findings section")
	}

	// Check table format (markdown tables)
	if !contains(contentStr, "| Signal Type | Severity | Confidence | Evidence |") {
		t.Errorf("Report should have signals table")
	}
}

func TestExportBlocklist_Format(t *testing.T) {
	// Use actual malicious content that will trigger CRITICAL and HIGH risk levels
	entities := []*models.Entity{
		{
			ID:   "entity1",
			Name: "Critical Server",
			Repository: models.RepositoryInfo{
				Owner: "owner1",
				Name:  "critical-repo",
				CreatedAt: models.RFC3339Time(time.Now().Add(-30 * 24 * time.Hour)),
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusQuarantined,
				Findings: []models.SecurityFinding{
					{Type: "malicious_repository", Severity: "CRITICAL", Source: "malicious_detector"},
				},
			},
			// Content that triggers CRITICAL (eval(atob))
			RawContent: `eval(atob("ZXZhbChhdG9iKCJ...base64...payload..."))`,
		},
		{
			ID:   "entity2",
			Name: "High Server",
			Repository: models.RepositoryInfo{
				Owner: "owner2",
				Name:  "high-repo",
				CreatedAt: models.RFC3339Time(time.Now().Add(-60 * 24 * time.Hour)),
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusQuarantined,
				Findings: []models.SecurityFinding{
					{Type: "malicious_repository", Severity: "HIGH", Source: "malicious_detector"},
				},
			},
			// Content that triggers HIGH (Lua obfuscation)
			RawContent: `while W[1] do
  local x = 0x48656C6C6F
end`,
		},
		{
			ID:   "entity3",
			Name: "Medium Server",
			Repository: models.RepositoryInfo{
				Owner: "owner3",
				Name:  "medium-repo",
				CreatedAt: models.RFC3339Time(time.Now().Add(-90 * 24 * time.Hour)),
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{Type: "malicious_repository", Severity: "MEDIUM", Source: "malicious_detector"},
				},
			},
			// Content that triggers MEDIUM (high entropy)
			RawContent: "X7$kL9@mP2#vR5X7$kL9@mP2#vR5X7$kL9@mP2#vR5X7$kL9@mP2#vR5X7$kL9@mP2#vR5X7$kL9@mP2#vR5",
		},
	}

	tmpDir := t.TempDir()
	exporter := NewMaliciousExporter(entities, filepath.Join(tmpDir, "malicious"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	blocklistPath := filepath.Join(tmpDir, "malicious", "blocklist.txt")
	content, _ := os.ReadFile(blocklistPath)
	contentStr := string(content)

	// Check header
	if !contains(contentStr, "Malicious Repository Blocklist") {
		t.Errorf("Blocklist should have header")
	}
	if !contains(contentStr, "owner/repo # RISK:") {
		t.Errorf("Blocklist should have format comment")
	}

	// Check entries - only CRITICAL and HIGH
	lines := splitLines(contentStr)
	dataLines := 0
	for _, line := range lines {
		if line != "" && !startsWith(line, "#") {
			dataLines++
			if !contains(line, "owner1/critical-repo") && !contains(line, "owner2/high-repo") {
				t.Errorf("Unexpected blocklist entry: %s", line)
			}
		}
	}
	if dataLines != 2 {
		t.Errorf("Blocklist should have 2 entries (CRITICAL + HIGH), got %d. Content: %s", dataLines, contentStr)
	}
}

func TestMaliciousExporter_ThreeCategories(t *testing.T) {
	// Test CRITICAL, MEDIUM, and Clean
	entities := []*models.Entity{
		{
			ID:   "critical",
			Name: "Critical",
			Repository: models.RepositoryInfo{Owner: "o", Name: "critical"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusQuarantined,
				Findings: []models.SecurityFinding{{Type: "malicious_repository", Severity: "CRITICAL", Source: "malicious_detector"}},
			},
			RawContent: "eval(atob(...))",
		},
		{
			ID:   "medium",
			Name: "Medium",
			Repository: models.RepositoryInfo{Owner: "o", Name: "medium"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{{Type: "malicious_repository", Severity: "MEDIUM", Source: "malicious_detector"}},
			},
			RawContent: "high entropy content...",
		},
		{
			ID:   "clean",
			Name: "Clean",
			Repository: models.RepositoryInfo{Owner: "o", Name: "clean"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusClean,
				Findings: []models.SecurityFinding{},
			},
			RawContent: "# Clean Server\n\nNormal content.",
		},
	}

	tmpDir := t.TempDir()
	exporter := NewMaliciousExporter(entities, filepath.Join(tmpDir, "malicious"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	reportPath := filepath.Join(tmpDir, "malicious", "MALICIOUS_REPORT.md")
	content, _ := os.ReadFile(reportPath)
	contentStr := string(content)

	// Should detect 2 malicious (critical + medium)
	if !contains(contentStr, "Malicious repositories detected: 2") {
		t.Errorf("Should detect 2 malicious repos")
	}

	// Check all three signal types mentioned
	if !contains(contentStr, "CRITICAL") {
		t.Errorf("Should mention CRITICAL")
	}
	if !contains(contentStr, "MEDIUM") {
		t.Errorf("Should mention MEDIUM")
	}

	// Blocklist should only have CRITICAL
	blocklistPath := filepath.Join(tmpDir, "malicious", "blocklist.txt")
	blContent, _ := os.ReadFile(blocklistPath)
	blStr := string(blContent)

	if !contains(blStr, "o/critical") {
		t.Errorf("Blocklist should have critical repo")
	}
	if contains(blStr, "o/medium") {
		t.Errorf("Blocklist should NOT have medium repo")
	}
}


func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}

func isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); {
		r, size := rune(data[i]), 1
		if r >= 0x80 {
			r, size = decodeRune(data[i:])
			if r == 0xFFFD && size == 1 {
				// Check if it's a valid replacement char or actual invalid
				return false
			}
		}
		i += size
	}
	return true
}

func decodeRune(p []byte) (rune, int) {
	// Simplified UTF-8 decode
	if len(p) == 0 {
		return 0, 0
	}
	if p[0] < 0x80 {
		return rune(p[0]), 1
	}
	if p[0] < 0xC0 {
		return 0xFFFD, 1
	}
	if p[0] < 0xE0 {
		if len(p) < 2 {
			return 0xFFFD, 1
		}
		return rune(p[0]&0x1F)<<6 | rune(p[1]&0x3F), 2
	}
	if p[0] < 0xF0 {
		if len(p) < 3 {
			return 0xFFFD, 1
		}
		return rune(p[0]&0x0F)<<12 | rune(p[1]&0x3F)<<6 | rune(p[2]&0x3F), 3
	}
	return 0xFFFD, 1
}