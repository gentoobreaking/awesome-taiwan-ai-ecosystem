package security

import (
	"testing"
	"time"
)

func TestNewMaliciousDetector(t *testing.T) {
	d := NewMaliciousDetector()
	if d == nil {
		t.Fatal("NewMaliciousDetector returned nil")
	}
	if d.EntropyThreshold != 7.0 {
		t.Errorf("EntropyThreshold = %f, want 7.0", d.EntropyThreshold)
	}
	if d.ReadmeSizeThreshold != 100*1024 {
		t.Errorf("ReadmeSizeThreshold = %d, want %d", d.ReadmeSizeThreshold, 100*1024)
	}
	if d.NonTextRatioThreshold != 0.30 {
		t.Errorf("NonTextRatioThreshold = %f, want 0.30", d.NonTextRatioThreshold)
	}
	if d.AccountAgeThreshold != 90*24*time.Hour {
		t.Errorf("AccountAgeThreshold = %v, want %v", d.AccountAgeThreshold, 90*24*time.Hour)
	}
	if d.MinFollowers != 0 {
		t.Errorf("MinFollowers = %d, want 0", d.MinFollowers)
	}
	if d.MinRepos != 5 {
		t.Errorf("MinRepos = %d, want 5", d.MinRepos)
	}
}

func TestCalculateEntropy(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
		delta    float64
	}{
		{"empty string", "", 0, 0},
		{"all same char", "aaaaa", 0, 0.001},
		{"two chars equal", "ababab", 1.0, 0.01},
		{"uniform distribution", "abcdefghijklmnopqrstuvwxyz", 4.7, 0.1},
		{"high entropy random", "X7$kL9@mP2#vR5", 4.0, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateEntropy(tt.input)
			if result < tt.expected-tt.delta || result > tt.expected+tt.delta {
				t.Errorf("calculateEntropy(%q) = %f, want ~%f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDetect_NormalReadme(t *testing.T) {
	d := NewMaliciousDetector()
	readme := "# Awesome MCP Server\n\nThis is a normal MCP server for Taiwan weather data.\n\n## Features\n- Real-time weather\n- Historical data\n- API access\n\n## Installation\n```bash\ngo install github.com/example/weather-mcp@latest\n```"

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	if result.RiskLevel != RiskLevelLow {
		t.Errorf("RiskLevel = %s, want LOW", result.RiskLevel)
	}
	if len(result.Signals) > 0 {
		t.Errorf("Expected no signals, got %d: %v", len(result.Signals), result.Signals)
	}
	if result.Confidence < 0.9 {
		t.Errorf("Confidence = %f, want >0.9", result.Confidence)
	}
}

func TestDetect_HighEntropyReadme(t *testing.T) {
	d := NewMaliciousDetector()
	// High entropy content without markdown structure (simulating binary/encoded)
	// Using latin-1 bytes (0-255) to achieve >7.0 entropy
	readme := ""
	for i := 0; i < 2; i++ {
		for b := 0; b < 256; b++ {
			readme += string(rune(b))
		}
	}
	// 512 chars, entropy ~8.0

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "readme_entropy" {
			found = true
			if s.Severity != RiskLevelHigh {
				t.Errorf("Severity = %s, want HIGH", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected readme_entropy signal, got: %v", result.Signals)
	}
	if result.RiskLevel != RiskLevelHigh && result.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %s, want HIGH or CRITICAL", result.RiskLevel)
	}
}

func TestDetect_LargeReadmeNoStructure(t *testing.T) {
	d := NewMaliciousDetector()
	// 150KB of repetitive non-markdown content
	readme := ""
	for i := 0; i < 4000; i++ {
		readme += "abcdefghijklmnopqrstuvwxyz\n"
	}

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "readme_size_anomaly" {
			found = true
			if s.Severity != RiskLevelMedium {
				t.Errorf("Severity = %s, want MEDIUM", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected readme_size_anomaly signal, got: %v", result.Signals)
	}
}

func TestDetect_ObfuscatedLua(t *testing.T) {
	d := NewMaliciousDetector()
	readme := `# MCP Server

while W[1] do
  local x = 0x48656C6C6F
end

loadstring("malicious")()`

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	luaCount := 0
	for _, s := range result.Signals {
		if s.Type == "obfuscation_lua" {
			luaCount++
			if s.Severity != RiskLevelHigh && s.Severity != RiskLevelMedium {
				t.Errorf("Severity = %s, want HIGH or MEDIUM", s.Severity)
			}
		}
	}
	if luaCount == 0 {
		t.Errorf("Expected obfuscation_lua signals, got: %v", result.Signals)
	}
	if result.RiskLevel != RiskLevelHigh && result.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %s, want HIGH or CRITICAL", result.RiskLevel)
	}
}

func TestDetect_ObfuscatedJS(t *testing.T) {
	d := NewMaliciousDetector()
	readme := `# MCP Server

eval(atob("ZXZhbChhdG9iKCJ...base64...payload..."))`

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "obfuscation_js" {
			found = true
			if s.Severity != RiskLevelCritical {
				t.Errorf("Severity = %s, want CRITICAL", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected obfuscation_js signal, got: %v", result.Signals)
	}
	if result.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %s, want CRITICAL", result.RiskLevel)
	}
}

func TestDetect_LargeBase64(t *testing.T) {
	d := NewMaliciousDetector()
	// Large base64 content
	base64 := "SGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQgSGVsbG8gV29ybGQ="
	readme := `# MCP Server

` + base64 + base64 + base64 + base64

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "obfuscation_base64" {
			found = true
			if s.Severity != RiskLevelMedium {
				t.Errorf("Severity = %s, want MEDIUM", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected obfuscation_base64 signal, got: %v", result.Signals)
	}
}

func TestDetect_AccountAnomaly(t *testing.T) {
	d := NewMaliciousDetector()
	readme := `# Normal MCP Server

This is a normal server.`

	// New account (<90 days), 0 followers, no bio, <5 repos
	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-30 * 24 * time.Hour)),
		OwnerFollowers: intPtr(0),
		OwnerBio:       strPtr(""),
		OwnerRepos:     intPtr(2),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "account_anomaly" {
			found = true
			if s.Severity != RiskLevelCritical {
				t.Errorf("Severity = %s, want CRITICAL (4 anomalies)", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected account_anomaly signal, got: %v", result.Signals)
	}
	if result.RiskLevel != RiskLevelHigh && result.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %s, want HIGH or CRITICAL", result.RiskLevel)
	}
}

func TestDetect_AccountAnomaly_NewButEstablished(t *testing.T) {
	d := NewMaliciousDetector()
	readme := `# Normal MCP Server`

	// New account but with followers and repos
	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-30 * 24 * time.Hour)),
		OwnerFollowers: intPtr(500),
		OwnerBio:       strPtr("Active developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "account_anomaly" {
			found = true
			break
		}
	}
	// Should not trigger because only age is anomalous
	if found {
		t.Errorf("Did not expect account_anomaly signal, got: %v", result.Signals)
	}
}

func TestDetect_HighNonTextRatio(t *testing.T) {
	d := NewMaliciousDetector()
	// Content with many non-printable characters
	readme := "Normal text\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f"
	readme += readme + readme + readme + readme + readme + readme // Make it long enough

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-365 * 24 * time.Hour)),
		OwnerFollowers: intPtr(100),
		OwnerBio:       strPtr("MCP developer"),
		OwnerRepos:     intPtr(20),
	}

	result := d.Detect(readme, repoInfo)

	found := false
	for _, s := range result.Signals {
		if s.Type == "non_text_ratio" {
			found = true
			if s.Severity != RiskLevelHigh {
				t.Errorf("Severity = %s, want HIGH", s.Severity)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected non_text_ratio signal, got: %v", result.Signals)
	}
}

func TestDetect_CombinedSignals(t *testing.T) {
	d := NewMaliciousDetector()
	// Combination of Lua obfuscation + account anomaly
	readme := `while W[1] do
  local x = 0x48656C6C6F
end`

	repoInfo := RepositoryInfo{
		OwnerCreatedAt: timePtr(time.Now().Add(-30 * 24 * time.Hour)),
		OwnerFollowers: intPtr(0),
		OwnerBio:       strPtr(""),
		OwnerRepos:     intPtr(1),
	}

	result := d.Detect(readme, repoInfo)

	// Should have both lua obfuscation and account anomaly
	hasLua := false
	hasAccount := false
	for _, s := range result.Signals {
		if s.Type == "obfuscation_lua" {
			hasLua = true
		}
		if s.Type == "account_anomaly" {
			hasAccount = true
		}
	}
	if !hasLua {
		t.Errorf("Expected obfuscation_lua signal")
	}
	if !hasAccount {
		t.Errorf("Expected account_anomaly signal")
	}
	// Combined should be CRITICAL
	if result.RiskLevel != RiskLevelCritical {
		t.Errorf("RiskLevel = %s, want CRITICAL (combined)", result.RiskLevel)
	}
}

func TestHasMarkdownStructure(t *testing.T) {
	d := NewMaliciousDetector()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"has heading and paragraph", "# Title\n\nContent", true},
		{"has list and paragraph", "- item 1\n- item 2\n\nMore content", true},
		{"has code block and paragraph", "```go\ncode\n```\n\nMore content", true},
		{"has link and paragraph", "[link](url)\n\nMore content", true},
		{"has heading and list", "# Title\n\n- item 1\n- item 2", true},
		{"no structure", "just plain text without any markdown", false},
		{"only one element", "# Title", false},
		{"two elements", "# Title\n\nContent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.hasMarkdownStructure(tt.input)
			if result != tt.expected {
				t.Errorf("hasMarkdownStructure(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAggregateRisk(t *testing.T) {
	d := NewMaliciousDetector()

	tests := []struct {
		name         string
		signals      []MaliciousSignal
		expectedRisk RiskLevel
	}{
		{
			name:         "no signals",
			signals:      []MaliciousSignal{},
			expectedRisk: RiskLevelLow,
		},
		{
			name: "one critical",
			signals: []MaliciousSignal{
				{Severity: RiskLevelCritical, Confidence: 0.9},
			},
			expectedRisk: RiskLevelCritical,
		},
		{
			name: "two high",
			signals: []MaliciousSignal{
				{Severity: RiskLevelHigh, Confidence: 0.8},
				{Severity: RiskLevelHigh, Confidence: 0.8},
			},
			expectedRisk: RiskLevelCritical,
		},
		{
			name: "one high",
			signals: []MaliciousSignal{
				{Severity: RiskLevelHigh, Confidence: 0.8},
			},
			expectedRisk: RiskLevelHigh,
		},
		{
			name: "three medium",
			signals: []MaliciousSignal{
				{Severity: RiskLevelMedium, Confidence: 0.7},
				{Severity: RiskLevelMedium, Confidence: 0.7},
				{Severity: RiskLevelMedium, Confidence: 0.7},
			},
			expectedRisk: RiskLevelHigh,
		},
		{
			name: "one medium",
			signals: []MaliciousSignal{
				{Severity: RiskLevelMedium, Confidence: 0.7},
			},
			expectedRisk: RiskLevelMedium,
		},
		{
			name: "three low",
			signals: []MaliciousSignal{
				{Severity: RiskLevelLow, Confidence: 0.6},
				{Severity: RiskLevelLow, Confidence: 0.6},
				{Severity: RiskLevelLow, Confidence: 0.6},
			},
			expectedRisk: RiskLevelMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			risk, conf := d.aggregateRisk(tt.signals)
			if risk != tt.expectedRisk {
				t.Errorf("aggregateRisk() risk = %s, want %s", risk, tt.expectedRisk)
			}
			if conf < 0 || conf > 1 {
				t.Errorf("confidence = %f, want 0-1", conf)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		prec     int
		expected string
	}{
		{3.14159, 2, "3.14"},
		{7.0, 2, "7.00"},
		{0.12345, 3, "0.123"},
		{100.0, 1, "100.0"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatFloat(tt.input, tt.prec)
			if result != tt.expected {
				t.Errorf("formatFloat(%f, %d) = %s, want %s", tt.input, tt.prec, result, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"long", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

// Helper functions
func timePtr(t time.Time) *time.Time {
	return &t
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}