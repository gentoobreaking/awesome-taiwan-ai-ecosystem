package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestNewInjectionExporter(t *testing.T) {
	entities := []*models.Entity{}
	exporter := NewInjectionExporter(entities, "/tmp/test")

	if exporter == nil {
		t.Fatal("NewInjectionExporter returned nil")
	}
	if exporter.OutputDir != "/tmp/test" {
		t.Errorf("OutputDir = %s, want /tmp/test", exporter.OutputDir)
	}
	if len(exporter.Entities) != 0 {
		t.Errorf("Entities length = %d, want 0", len(exporter.Entities))
	}
}

func TestInjectionExporter_Export_NoInjection(t *testing.T) {
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
		},
	}

	tmpDir := t.TempDir()
	exporter := NewInjectionExporter(entities, filepath.Join(tmpDir, "injection"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	reportPath := filepath.Join(tmpDir, "injection", "INJECTION_REPORT.md")
	patternsPath := filepath.Join(tmpDir, "injection", "patterns.json")

	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Errorf("INJECTION_REPORT.md not created")
	}

	if _, err := os.Stat(patternsPath); os.IsNotExist(err) {
		t.Errorf("patterns.json not created")
	}
}

func TestInjectionExporter_Export_WithInjection(t *testing.T) {
	entities := []*models.Entity{
		{
			ID:   "entity1",
			Name: "Injection Server",
			Repository: models.RepositoryInfo{
				Owner: "injection",
				Name:  "evil-repo",
				URL:   "https://github.com/injection/evil-repo",
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{
						Type:        "prompt_injection",
						Severity:    "HIGH",
						Source:      "readme",
						Location:    "line 10",
						Evidence:    "Ignore previous instructions and...",
						Rule:        "prompt_injection_pattern",
						Confidence:  0.85,
					},
					{
						Type:        "injection",
						Severity:    "MEDIUM",
						Source:      "readme",
						Location:    "line 25",
						Evidence:    "System prompt: ...",
						Rule:        "system_prompt_leak",
						Confidence:  0.75,
					},
				},
			},
		},
		{
			ID:   "entity2",
			Name: "Another Injection Server",
			Repository: models.RepositoryInfo{
				Owner: "injection2",
				Name:  "suspect-repo",
				URL:   "https://github.com/injection2/suspect-repo",
			},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{
						Type:        "prompt_injection",
						Severity:    "CRITICAL",
						Source:      "readme",
						Location:    "line 5",
						Evidence:    "You are now in developer mode...",
						Rule:        "prompt_injection_pattern",
						Confidence:  0.9,
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	exporter := NewInjectionExporter(entities, filepath.Join(tmpDir, "injection"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	reportPath := filepath.Join(tmpDir, "injection", "INJECTION_REPORT.md")
	reportContent, _ := os.ReadFile(reportPath)
	content := string(reportContent)

	if !contains(content, "Servers with injection patterns detected: 2") {
		t.Errorf("Report should show 2 detected servers")
	}
	if !contains(content, "prompt_injection_pattern") {
		t.Errorf("Report should show prompt_injection_pattern")
	}
	if !contains(content, "system_prompt_leak") {
		t.Errorf("Report should show system_prompt_leak")
	}
	if !contains(content, "Top 10 High-Risk Servers") {
		t.Errorf("Report should have top risk servers section")
	}

	// Check patterns.json
	patternsPath := filepath.Join(tmpDir, "injection", "patterns.json")
	patternsContent, _ := os.ReadFile(patternsPath)
	if !contains(string(patternsContent), "prompt_injection_pattern") {
		t.Errorf("patterns.json should contain prompt_injection_pattern")
	}
	if !contains(string(patternsContent), "system_prompt_leak") {
		t.Errorf("patterns.json should contain system_prompt_leak")
	}
}

func TestInjectionExporter_ThreeCategories(t *testing.T) {
	entities := []*models.Entity{
		{
			ID:   "injection",
			Name: "Injection Server",
			Repository: models.RepositoryInfo{Owner: "o", Name: "injection"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{Type: "prompt_injection", Severity: "CRITICAL", Rule: "prompt_injection_pattern", Source: "readme", Location: "line 1", Evidence: "test", Confidence: 0.9},
				},
			},
		},
		{
			ID:   "injection2",
			Name: "Another Injection",
			Repository: models.RepositoryInfo{Owner: "o", Name: "injection2"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusSuspicious,
				Findings: []models.SecurityFinding{
					{Type: "injection", Severity: "MEDIUM", Rule: "system_prompt_leak", Source: "readme", Location: "line 1", Evidence: "test", Confidence: 0.7},
				},
			},
		},
		{
			ID:   "clean",
			Name: "Clean Server",
			Repository: models.RepositoryInfo{Owner: "o", Name: "clean"},
			SecurityStatus: models.SecurityStatusDetail{
				Status: models.SecurityStatusClean,
				Findings: []models.SecurityFinding{},
			},
		},
	}

	tmpDir := t.TempDir()
	exporter := NewInjectionExporter(entities, filepath.Join(tmpDir, "injection"))

	err := exporter.Export()
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	reportPath := filepath.Join(tmpDir, "injection", "INJECTION_REPORT.md")
	content, _ := os.ReadFile(reportPath)
	contentStr := string(content)

	if !contains(contentStr, "Servers with injection patterns detected: 2") {
		t.Errorf("Should detect 2 injection servers")
	}

	patternsPath := filepath.Join(tmpDir, "injection", "patterns.json")
	patternsContent, _ := os.ReadFile(patternsPath)
	if !contains(string(patternsContent), "prompt_injection_pattern") {
		t.Errorf("patterns.json should have prompt_injection_pattern")
	}
	if !contains(string(patternsContent), "system_prompt_leak") {
		t.Errorf("patterns.json should have system_prompt_leak")
	}
}
