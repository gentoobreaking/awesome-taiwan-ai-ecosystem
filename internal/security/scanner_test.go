package security

import (
	"strings"
	"testing"
	"time"

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
func TestMaliciousDetector_NormalReadme(t *testing.T) {
	detector := NewMaliciousDetector()
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	readme := "# Test Server\n\nThis is a normal MCP server for Taiwan finance data.\n\n## Features\n\n- Stock data\n- Real-time quotes\n"

	result := detector.Detect(server, readme, nil, 10, 5, 3)
	if result.RiskLevel != "LOW" {
		t.Errorf("Expected LOW risk for normal README, got %s", result.RiskLevel)
	}
	if len(result.Signals) > 0 {
		t.Errorf("Expected no signals for normal README, got %d", len(result.Signals))
	}
}

func TestMaliciousDetector_HighEntropy(t *testing.T) {
	detector := NewMaliciousDetectorWithConfig(MaliciousDetectorConfig{
		EntropyThreshold: 2.0, // Very low threshold for test with pseudo-random data
	})
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	randData := make([]byte, 5000)
	for i := range randData {
		randData[i] = byte((i * 13 + 17) % 256)
	}
	readme := string(randData)

	result := detector.Detect(server, readme, nil, 10, 5, 3)
	if result.RiskLevel == "LOW" {
		t.Errorf("Expected non-LOW risk for high entropy README, got %s", result.RiskLevel)
	}
	found := false
	for _, s := range result.Signals {
		if s.Name == "high_entropy" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected high_entropy signal")
	}
}

func TestMaliciousDetector_LuaBytecode(t *testing.T) {
	detector := NewMaliciousDetector()
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	// Simulate clearsdunker-create/ez style Lua bytecode
	readme := `return({dQ=function(W,W)while W[26]do W[0xB]=0x9D__;return-0x2,W[0X25];end;return nil;end,qd=function(W,J,d)(d)[8200]=0X18+(((W.Ta((W.Fa((W.za(W.D[0X5],(d[28798])))-d[18500]))))<d[15506]and W.D[0X5__]or d[19990])+d[15506]);J=(-1272528919+(W.Ta((W.Ra(((W.ya(d[0X7301],(d[0X602F])))~=d[0X36d3]and W.D[5]or W.D[0B1000])-d[4034],(d[24623])))-d[12396])));d[0X760d_]=(J);return J;end}`

	result := detector.Detect(server, readme, nil, 10, 5, 3)
	if result.RiskLevel != "CRITICAL" && result.RiskLevel != "HIGH" {
		t.Errorf("Expected CRITICAL/HIGH risk for Lua bytecode, got %s", result.RiskLevel)
	}
	found := false
	for _, s := range result.Signals {
		if s.Name == "obfuscation_pattern" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected obfuscation_pattern signal for Lua bytecode")
	}
}

func TestMaliciousDetector_ThrowawayAccount(t *testing.T) {
	detector := NewMaliciousDetector()
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	readme := "# Normal README\n\nThis is a test."
	created := time.Now().Add(-30 * 24 * time.Hour) // 30 days old
	followers := 0
	profileFields := 0
	repos := 10

	result := detector.Detect(server, readme, &created, followers, profileFields, repos)
	if result.RiskLevel == "LOW" {
		t.Errorf("Expected non-LOW risk for throwaway account, got %s", result.RiskLevel)
	}
	found := false
	for _, s := range result.Signals {
		if s.Name == "throwaway_account" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected throwaway_account signal")
	}
}

func TestMaliciousDetector_OversizedReadme(t *testing.T) {
	detector := NewMaliciousDetector()
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	readme := strings.Repeat("x", 200*1024) // 200 KB

	result := detector.Detect(server, readme, nil, 10, 5, 3)
	found := false
	for _, s := range result.Signals {
		if s.Name == "oversized_readme" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected oversized_readme signal")
	}
}

func TestMaliciousDetector_ToSecurityFinding(t *testing.T) {
	detector := NewMaliciousDetector()
	server := &models.MCPServer{Name: "test", Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}
	readme := `return({dQ=function(W,W)while W[26]do W[0xB]=0x9D__;end;end}`
	result := detector.Detect(server, readme, nil, 10, 5, 3)

	finding := result.ToSecurityFinding(server.Repository.URL)
	if finding.Type != MaliciousType {
		t.Errorf("Expected type %s, got %s", MaliciousType, finding.Type)
	}
	if finding.Severity != models.SecuritySeverity(result.RiskLevel) {
		t.Errorf("Expected severity %s, got %s", result.RiskLevel, finding.Severity)
	}
	if finding.Source != "malicious_detector" {
		t.Errorf("Expected source malicious_detector, got %s", finding.Source)
	}
	if finding.Location != server.Repository.URL {
		t.Errorf("Expected location %s, got %s", server.Repository.URL, finding.Location)
	}
}
