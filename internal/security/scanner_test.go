package security

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestScanServer_NoFindings(t *testing.T) {
	scanner := New()
	server := &models.MCPServer{
		Name: "clean-mcp",
		Tools: []models.Tool{
			{Name: "search", Description: "Search the web"},
		},
		Endpoints: []models.Endpoint{
			{URL: "https://mcp.example.com/mcp", Transport: "sse", TLS: true},
		},
	}

	result := scanner.ScanServer(server)
	if len(result.Findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(result.Findings))
	}
	if result.RiskLevel != models.SeverityUnknown {
		t.Errorf("Expected severity UNKNOWN, got %s", result.RiskLevel)
	}
}

func TestScanServer_InsecureTransport(t *testing.T) {
	scanner := New()
	server := &models.MCPServer{
		Name: "http-mcp",
		Endpoints: []models.Endpoint{
			{URL: "http://insecure.example.com/mcp", Transport: "sse", TLS: false},
		},
	}

	result := scanner.ScanServer(server)
	if result.RiskLevel != models.SeverityLow {
		t.Errorf("Expected severity LOW, got %s", result.RiskLevel)
	}
	if len(result.Findings) == 0 {
		t.Error("Expected at least 1 finding")
	}
}

func TestScanServer_DangerousToolName(t *testing.T) {
	scanner := New()
	server := &models.MCPServer{
		Name: "exec-mcp",
		Tools: []models.Tool{
			{Name: "exec_code", Description: "Execute arbitrary code"},
		},
		Endpoints: []models.Endpoint{
			{URL: "https://example.com/mcp", TLS: true},
		},
	}

	result := scanner.ScanServer(server)
	found := false
	for _, f := range result.Findings {
		if f.Type == "suspicious_tool_name" {
			found = true
		}
	}
	if !found {
		t.Error("Expected suspicious_tool_name finding")
	}
}

func TestScanServer_ForkRepository(t *testing.T) {
	scanner := New()
	server := &models.MCPServer{
		Name: "forked-mcp",
		Repository: models.RepositoryInfo{
			URL:   "https://github.com/user/forked-mcp",
			Fork:  true,
		},
	}

	result := scanner.ScanServer(server)
	found := false
	for _, f := range result.Findings {
		if f.Type == "fork_repository" && f.Severity == models.SeverityLow {
			found = true
		}
	}
	if !found {
		t.Error("Expected fork_repository finding")
	}
}

func TestScanServer_LocalhostExposure(t *testing.T) {
	scanner := New()
	server := &models.MCPServer{
		Name: "local-mcp",
		Endpoints: []models.Endpoint{
			{URL: "http://127.0.0.1:8080/mcp", Transport: "sse"},
		},
	}

	result := scanner.ScanServer(server)
	found := false
	for _, f := range result.Findings {
		if f.Type == "localhost_exposure" {
			found = true
		}
	}
	if !found {
		t.Error("Expected localhost_exposure finding")
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity models.SecuritySeverity
		want     int
	}{
		{models.SeverityCritical, 4},
		{models.SeverityHigh, 3},
		{models.SeverityMedium, 2},
		{models.SeverityLow, 1},
		{models.SeverityUnknown, 0},
	}
	for _, tt := range tests {
		if got := severityRank(tt.severity); got != tt.want {
			t.Errorf("severityRank(%s) = %d, want %d", tt.severity, got, tt.want)
		}
	}
}

func TestCalculateScoreImpact(t *testing.T) {
	tests := []struct {
		findings []models.SecurityFinding
		want     float64
	}{
		{nil, 0},
		{[]models.SecurityFinding{
			{Severity: models.SeverityLow},
			{Severity: models.SeverityMedium},
			{Severity: models.SeverityHigh},
		}, 0}, // -0.5 - 1 - 3 = -4.5, but min is 0
		{[]models.SecurityFinding{
			{Severity: models.SeverityCritical},
			{Severity: models.SeverityHigh},
		}, 0}, // -5 - 3 = -8, min is 0
	}

	for _, tt := range tests {
		got := calculateScoreImpact(tt.findings)
		if got != tt.want {
			t.Errorf("calculateScoreImpact() = %f, want %f", got, tt.want)
		}
	}
}

func TestScanServer_PatternDetection(t *testing.T) {
	scanner := New()
	dangerousNames := []string{
		"eval_code",
		"run_exec",
		"os_system",
		"child_process",
		"subprocess_run",
	}

	for _, name := range dangerousNames {
		server := &models.MCPServer{
			Name: "test",
			Tools: []models.Tool{
				{Name: name, Description: "test tool"},
			},
			Endpoints: []models.Endpoint{
				{URL: "https://example.com/mcp", TLS: true},
			},
		}
		result := scanner.ScanServer(server)
		if len(result.Findings) == 0 {
			t.Errorf("Expected finding for dangerous name: %s", name)
		}
	}
}
