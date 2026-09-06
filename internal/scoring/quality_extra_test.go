package scoring

import (
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestScoreDataSource(t *testing.T) {
	tests := []struct {
		name   string
		source models.DataSourceType
		want   int
	}{
		{"official_gov", models.DataSourceOfficialGovAPI, 20},
		{"gov_open_data", models.DataSourceGovOpenData, 18},
		{"official_company", models.DataSourceOfficialCompany, 15},
		{"third_party", models.DataSourceThirdPartyAPI, 10},
		{"web_scraping", models.DataSourceWebScraping, 7},
		{"unknown", models.DataSourceUnknown, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreDataSource([]models.DataSource{{Type: tt.source}})
			if got != tt.want {
				t.Errorf("scoreDataSource(%s) = %d, want %d", tt.source, got, tt.want)
			}
		})
	}
}

func TestScoreMaintenance(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		pushed time.Time
		want   int
	}{
		{"<90d", now.AddDate(0, 0, -10), 15},
		{"90-180d", now.AddDate(0, 0, -100), 12},
		{"180-365d", now.AddDate(0, 0, -200), 8},
		{">365d", now.AddDate(0, 0, -400), 3},
		{"zero", time.Time{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreMaintenance(tt.pushed)
			if got != tt.want {
				t.Errorf("scoreMaintenance(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestScoreDocumentation(t *testing.T) {
	tests := []struct {
		name  string
		desc  string
		tools []models.Tool
		want  int
	}{
		{"readme>200", "a" + makeString(200), []models.Tool{}, 5},
		{"with_tools", "short", []models.Tool{{Name: "t1"}}, 3},
		{"with_tools_5", "short", []models.Tool{{}, {}, {}, {}, {}}, 5},
		{"short_no_tools", "short", []models.Tool{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreDocumentation(tt.desc, tt.tools)
			if got != tt.want {
				t.Errorf("scoreDocumentation(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func makeString(n int) string {
	return string(make([]byte, n))
}

func TestScoreMCPCompliance(t *testing.T) {
	tests := []struct {
		name       string
		transports []string
		want       int
	}{
		{"stdio", []string{"stdio"}, 8},
		{"http", []string{"http"}, 8},
		{"sse", []string{"sse"}, 8},
		{"streamable", []string{"streamable-http"}, 7},
		{"multiple", []string{"stdio", "http", "sse"}, 14},
		{"none", []string{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreMCPCompliance(tt.transports)
			if got != tt.want {
				t.Errorf("scoreMCPCompliance(%v) = %d, want %d", tt.transports, got, tt.want)
			}
		})
	}
}

func TestScoreHealth(t *testing.T) {
	tests := []struct {
		health models.HealthStatus
		want   int
	}{
		{models.HealthHealthy, 10},
		{models.HealthDegraded, 5},
		{models.HealthUnavailable, 0},
		{models.HealthInvalid, 0},
		{models.HealthUnknown, 0},
	}
	for _, tt := range tests {
		got := scoreHealth(tt.health)
		if got != tt.want {
			t.Errorf("scoreHealth(%s) = %d, want %d", tt.health, got, tt.want)
		}
	}
}

func TestScoreLicense(t *testing.T) {
	tests := []struct {
		name    string
		license string
		want    int
	}{
		{"MIT", "MIT", 5},
		{"Apache", "Apache-2.0", 5},
		{"BSD", "BSD-3-Clause", 5},
		{"GPL", "GPL-3.0", 3},
		{"empty", "", 0},
		{"unknown", "UNKNOWN", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreLicense(tt.license)
			if got != tt.want {
				t.Errorf("scoreLicense(%q) = %d, want %d", tt.license, got, tt.want)
			}
		})
	}
}

func TestScoreSecurity(t *testing.T) {
	tests := []struct {
		name     string
		findings []models.SecurityFinding
		want     int
	}{
		{"no_findings", nil, 5},
		{"low", []models.SecurityFinding{{Severity: models.SeverityLow}}, 4},
		{"medium", []models.SecurityFinding{{Severity: models.SeverityMedium}}, 3},
		{"high", []models.SecurityFinding{{Severity: models.SeverityHigh}}, 2},
		{"critical", []models.SecurityFinding{{Severity: models.SeverityCritical}}, 0},
	}
	for _, tt := range tests {
		server := &models.MCPServer{Security: models.SecurityStatusDetail{Findings: tt.findings}}
		t.Run(tt.name, func(t *testing.T) {
			got := scoreSecurity(server)
			if got != tt.want {
				t.Errorf("scoreSecurity(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestScoreCommunity(t *testing.T) {
	tests := []struct {
		name string
		repo models.RepositoryInfo
		want int
	}{
		{"stars=5", models.RepositoryInfo{Stars: 5}, 0},
		{"stars=10", models.RepositoryInfo{Stars: 10, Topics: []string{"a"}}, 2},
		{"stars=50", models.RepositoryInfo{Stars: 50, Topics: []string{"a"}}, 3},
		{"stars=100", models.RepositoryInfo{Stars: 100, Topics: []string{"a"}, OpenIssues: 5}, 5},
		{"stars=0", models.RepositoryInfo{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreCommunity(tt.repo)
			if got != tt.want {
				t.Errorf("scoreCommunity(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestScoreToolSchema(t *testing.T) {
	tests := []struct {
		name  string
		tools []models.Tool
		want  int
	}{
		{"empty", []models.Tool{}, 0},
		{"one_tool", []models.Tool{{Name: "t1", Description: "desc"}}, 6},
		{"with_schema", []models.Tool{{Name: "t1", InputSchema: map[string]any{"type": "object"}}}, 5},
		{"five_tools_with_schema", []models.Tool{
			{Name: "t1", InputSchema: map[string]any{"type": "object"}},
			{Name: "t2", InputSchema: map[string]any{"type": "object"}},
			{Name: "t3", InputSchema: map[string]any{"type": "object"}},
			{Name: "t4", InputSchema: map[string]any{"type": "object"}},
			{Name: "t5", InputSchema: map[string]any{"type": "object"}},
		}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreToolSchema(tt.tools)
			if got != tt.want {
				t.Errorf("scoreToolSchema(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestScoreRepository(t *testing.T) {
	tests := []struct {
		name string
		repo models.RepositoryInfo
		want int
	}{
		{"with_url", models.RepositoryInfo{URL: "https://github.com/foo/bar"}, 3},
		{"with_url_stars10", models.RepositoryInfo{URL: "https://github.com/foo/bar", Stars: 10}, 4},
		{"with_url_stars100", models.RepositoryInfo{URL: "https://github.com/foo/bar", Stars: 100}, 4},
		{"no_url", models.RepositoryInfo{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreRepository(tt.repo)
			if got != tt.want {
				t.Errorf("scoreRepository(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}
