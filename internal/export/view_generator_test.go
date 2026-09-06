package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func testEntity(idSuffix string) *models.Entity {
	return &models.Entity{
		ID:        "entity-" + idSuffix,
		Name:      "Test Server " + idSuffix,
		Slug:      "test-server-" + idSuffix,
		Description: "A test MCP server for testing",
		Classification: models.ClassificationResult{
			Primary:    models.PrimaryClassificationMCPServer,
			Confidence: 0.95,
			MCPRole:    models.MCPRoleServer,
		},
		TaiwanRelevance: models.TaiwanRelevance{
			Score:      0.9,
			Level:      models.TaiwanRelevanceLevelT1,
			Confidence: 0.9,
		},
		AIRelevance: models.AIRelevance{
			Score:      0.8,
			Level:      models.AIRelevanceLevelA1,
			Confidence: 0.8,
		},
		MCPIdentity: models.MCPIdentity{
			Status:     models.MCPIdentityStatusRuntimeVerified,
			Confidence: 0.9,
		},
		SecurityStatus: models.SecurityStatusDetail{
			Status: models.SecurityStatusClean,
		},
		Quality: models.QualityScore{
			Score: 85,
			Grade: models.QualityGradeA,
			Components: models.QualityComponents{
				DataSource: 15, Maintenance: 10, Documentation: 8,
				MCPCompliance: 12, ToolSchema: 8, Health: 8,
				Repository: 4, License: 4, Security: 4, Community: 3,
			},
		},
		Repository: models.RepositoryInfo{
			URL:    "https://github.com/test/" + idSuffix,
			Owner:  "test",
			Name:   idSuffix,
			License: "MIT",
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{
					URL:       "http://localhost:8080/mcp",
					Transport: "stdio",
				},
				Type: models.EndpointTypeMCPRuntime,
			},
		},
		Tools: []models.Tool{
			{Name: "tool1", Description: "Test tool 1"},
		},
		FirstSeen:  models.RFC3339Time(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		LastSeen:   models.RFC3339Time(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
	}
}

func testViewConfig() ViewConfig {
	return ViewConfig{
		SchemaVersion:  "1.0",
		CrawlerVersion: "test-1.0",
		GeneratedAt:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestGenerateViews_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())

	entities := []*models.Entity{
		testEntity("server1"),
		testEntity("server2"),
	}

	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	expectedFiles := []string{
		"taiwan-ai-ecosystem.md", "taiwan-ai-ecosystem.json",
		"taiwan-mcp.md", "taiwan-mcp.json",
		"taiwan-mcp-candidates.md", "taiwan-mcp-candidates.json",
		"taiwan-ai-agents.md", "taiwan-ai-agents.json",
		"taiwan-ai-tools.md", "taiwan-ai-tools.json",
		"taiwan-ai-data.md", "taiwan-ai-data.json",
		"taiwan-ai-skills.md", "taiwan-ai-skills.json",
		"taiwan-ai-infrastructure.md", "taiwan-ai-infrastructure.json",
		"taiwan-ai-tutorials.md", "taiwan-ai-tutorials.json",
		"taiwan-ai-collections.md", "taiwan-ai-collections.json",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestGenerateViews_JSONHasSchemaFields(t *testing.T) {
	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())

	entities := []*models.Entity{testEntity("json-test")}

	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	jsonData, err := os.ReadFile(filepath.Join(dir, "taiwan-mcp.json"))
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	content := string(jsonData)
	if !strings.Contains(content, `"schema_version": "1.0"`) {
		t.Errorf("JSON missing schema_version")
	}
	if !strings.Contains(content, `"view_name": "taiwan-mcp"`) {
		t.Errorf("JSON missing view_name")
	}
	if !strings.Contains(content, `"generated_at"`) {
		t.Errorf("JSON missing generated_at")
	}
	if !strings.Contains(content, `"total_entities": 1`) {
		t.Errorf("JSON missing or incorrect total_entities")
	}
	if !strings.Contains(content, `"entities"`) {
		t.Errorf("JSON missing entities array")
	}
}

func TestGenerateViews_MarkdownHasHeader(t *testing.T) {
	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())

	entities := []*models.Entity{testEntity("md-test")}

	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	md, err := os.ReadFile(filepath.Join(dir, "taiwan-mcp.md"))
	if err != nil {
		t.Fatalf("read md: %v", err)
	}

	content := string(md)
	if !strings.HasPrefix(content, "# taiwan-mcp") {
		t.Errorf("markdown should start with '# taiwan-mcp', got: %s", content[:20])
	}
	if !strings.Contains(content, "Generated:") {
		t.Errorf("markdown missing Generated timestamp")
	}
	if !strings.Contains(content, "Total entities:") {
		t.Errorf("markdown missing Total entities count")
	}
}

func TestViewFilter_VerifiedMCPServers(t *testing.T) {
	entities := []*models.Entity{
		// Should be included
		testEntity("verified1"),
		// Should be excluded: not runtime verified
		func() *models.Entity {
			e := testEntity("static1")
			e.MCPIdentity.Status = models.MCPIdentityStatusStaticVerified
			return e
		}(),
		// Should be excluded: BLOCKED
		func() *models.Entity {
			e := testEntity("blocked1")
			e.SecurityStatus.Status = models.SecurityStatusBlocked
			return e
		}(),
		// Should be excluded: T0 relevance
		func() *models.Entity {
			e := testEntity("t0")
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT0
			return e
		}(),
		// Should be excluded: not MCP_SERVER
		func() *models.Entity {
			e := testEntity("agent1")
			e.Classification.Primary = models.PrimaryClassificationAIAgent
			return e
		}(),
	}

	results := FilterVerifiedMCPServers(entities)
	if len(results) != 1 {
		t.Errorf("expected 1 verified MCP server, got %d", len(results))
	}
	if results[0].ID != "entity-verified1" {
		t.Errorf("expected entity-verified1, got %s", results[0].ID)
	}
}

func TestViewFilter_MCPCandidates(t *testing.T) {
	entities := []*models.Entity{
		// Should be included: candidate
		func() *models.Entity {
			e := testEntity("cand1")
			e.MCPIdentity.Status = models.MCPIdentityStatusCandidate
			return e
		}(),
		// Should be included: static verified
		func() *models.Entity {
			e := testEntity("static1")
			e.MCPIdentity.Status = models.MCPIdentityStatusStaticVerified
			return e
		}(),
		// Should be excluded: runtime verified (not candidate/static)
		testEntity("verified1"),
		// Should be excluded: not MCP_SERVER
		func() *models.Entity {
			e := testEntity("agent1")
			e.Classification.Primary = models.PrimaryClassificationAIAgent
			e.MCPIdentity.Status = models.MCPIdentityStatusCandidate
			return e
		}(),
	}

	results := FilterMCPCandidates(entities)
	if len(results) != 2 {
		t.Errorf("expected 2 MCP candidates, got %d", len(results))
	}
}

func TestViewFilter_TaiwanRelevanceT1Plus(t *testing.T) {
	tests := []struct {
		level  models.TaiwanRelevanceLevel
		expect bool
	}{
		{models.TaiwanRelevanceLevelT0, false},
		{models.TaiwanRelevanceLevelT1, true},
		{models.TaiwanRelevanceLevelT2, true},
		{models.TaiwanRelevanceLevelT3, true},
		{models.TaiwanRelevanceLevelT4, true},
		{models.TaiwanRelevanceLevelT5, true},
	}

	for _, tt := range tests {
		got := IsTaiwanRelevantT1Plus(tt.level)
		if got != tt.expect {
			t.Errorf("TaiwanRelevanceLevel(%s): expected %v, got %v", tt.level, tt.expect, got)
		}
	}
}

func TestViewFilter_TaiwanAIEntities(t *testing.T) {
	entities := []*models.Entity{
		// T1 — included
		testEntity("t1-server"),
		// T0 — excluded
		func() *models.Entity {
			e := testEntity("t0-server")
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT0
			return e
		}(),
	}

	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())
	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	md, err := os.ReadFile(filepath.Join(dir, "taiwan-ai-ecosystem.md"))
	if err != nil {
		t.Fatalf("read md: %v", err)
	}

	content := string(md)
	if !strings.Contains(content, "Test Server t1-server") {
		t.Errorf("expected t1-server in taiwan-ai-ecosystem.md")
	}
	if strings.Contains(content, "Test Server t0-server") {
		t.Errorf("t0-server should not be in taiwan-ai-ecosystem.md")
	}
}

func TestViewFilter_AIGroups(t *testing.T) {
	entities := []*models.Entity{
		// AI Agent
		func() *models.Entity {
			e := testEntity("agent1")
			e.Classification.Primary = models.PrimaryClassificationAIAgent
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT2
			return e
		}(),
		// AI Tool
		func() *models.Entity {
			e := testEntity("tool1")
			e.Classification.Primary = models.PrimaryClassificationAITool
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT3
			return e
		}(),
		// AI Dataset
		func() *models.Entity {
			e := testEntity("dataset1")
			e.Classification.Primary = models.PrimaryClassificationAIDataset
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT1
			return e
		}(),
		// AI Skill
		func() *models.Entity {
			e := testEntity("skill1")
			e.Classification.Primary = models.PrimaryClassificationAISkill
			return e
		}(),
		// AI Infrastructure
		func() *models.Entity {
			e := testEntity("infra1")
			e.Classification.Primary = models.PrimaryClassificationAIInfrastructure
			return e
		}(),
		// AI Tutorial
		func() *models.Entity {
			e := testEntity("tutorial1")
			e.Classification.Primary = models.PrimaryClassificationAITutorial
			return e
		}(),
		// AI Collection
		func() *models.Entity {
			e := testEntity("collection1")
			e.Classification.Primary = models.PrimaryClassificationAICollection
			return e
		}(),
	}

	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())
	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	// Check ai-agents view
	agentMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-agents.md"))
	if !strings.Contains(string(agentMd), "Test Server agent1") {
		t.Errorf("expected agent1 in taiwan-ai-agents.md")
	}

	// Check ai-tools view
	toolMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-tools.md"))
	if !strings.Contains(string(toolMd), "Test Server tool1") {
		t.Errorf("expected tool1 in taiwan-ai-tools.md")
	}

	// Check ai-data view
	dataMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-data.md"))
	if !strings.Contains(string(dataMd), "Test Server dataset1") {
		t.Errorf("expected dataset1 in taiwan-ai-data.md")
	}

	// Check ai-skills view
	skillMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-skills.md"))
	if !strings.Contains(string(skillMd), "Test Server skill1") {
		t.Errorf("expected skill1 in taiwan-ai-skills.md")
	}

	// Check ai-infrastructure view
	infraMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-infrastructure.md"))
	if !strings.Contains(string(infraMd), "Test Server infra1") {
		t.Errorf("expected infra1 in taiwan-ai-infrastructure.md")
	}

	// Check ai-tutorials view
	tutorialMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-tutorials.md"))
	if !strings.Contains(string(tutorialMd), "Test Server tutorial1") {
		t.Errorf("expected tutorial1 in taiwan-ai-tutorials.md")
	}

	// Check ai-collections view
	collectionMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-collections.md"))
	if !strings.Contains(string(collectionMd), "Test Server collection1") {
		t.Errorf("expected collection1 in taiwan-ai-collections.md")
	}
}

func TestViewFilter_MCPCandidatesView(t *testing.T) {
	entities := []*models.Entity{
		// Candidate
		func() *models.Entity {
			e := testEntity("cand1")
			e.MCPIdentity.Status = models.MCPIdentityStatusCandidate
			return e
		}(),
		// Static verified
		func() *models.Entity {
			e := testEntity("static1")
			e.MCPIdentity.Status = models.MCPIdentityStatusStaticVerified
			return e
		}(),
	}

	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())
	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	candMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-mcp-candidates.md"))
	content := string(candMd)
	if !strings.Contains(content, "Test Server cand1") {
		t.Errorf("expected cand1 in taiwan-mcp-candidates.md")
	}
	if !strings.Contains(content, "Test Server static1") {
		t.Errorf("expected static1 in taiwan-mcp-candidates.md")
	}
}

func TestView_MarkdownSummary(t *testing.T) {
	entities := []*models.Entity{
		testEntity("a"),
		testEntity("b"),
		testEntity("c"),
	}

	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())
	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-ecosystem.md"))
	content := string(md)
	if !strings.Contains(content, "## Summary") {
		t.Errorf("markdown missing Summary section")
	}
}

func TestView_SortedByQuality(t *testing.T) {
	entities := []*models.Entity{
		func() *models.Entity {
			e := testEntity("low")
			e.Quality.Score = 30
			return e
		}(),
		func() *models.Entity {
			e := testEntity("high")
			e.Quality.Score = 95
			return e
		}(),
		func() *models.Entity {
			e := testEntity("mid")
			e.Quality.Score = 60
			return e
		}(),
	}

	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())
	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(dir, "taiwan-mcp.md"))
	content := string(md)

	// high (95) should come before mid (60) which should come before low (30)
	highIdx := strings.Index(content, "Test Server high")
	midIdx := strings.Index(content, "Test Server mid")
	lowIdx := strings.Index(content, "Test Server low")

	if highIdx < 0 || midIdx < 0 || lowIdx < 0 {
		t.Fatalf("couldn't find all entity names in markdown")
	}
	if !(highIdx < midIdx && midIdx < lowIdx) {
		t.Errorf("entities not sorted by quality descending: high=%d, mid=%d, low=%d", highIdx, midIdx, lowIdx)
	}
}

func TestGenerateViews_EmptyEntities(t *testing.T) {
	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())

	if err := vg.GenerateViews(nil, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	// All files should still be created
	for _, view := range allViews {
		path := filepath.Join(dir, view.Name+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist even with no entities", view.Name+".md")
		}
	}
}

func TestGenerateViews_JSONInvalidDir(t *testing.T) {
	vg := NewViewGenerator(testViewConfig())
	// Use a path under a file (not a dir) to cause failure
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	err := vg.GenerateViews(nil, filePath)
	if err == nil {
		t.Error("expected error when output dir is a file")
	}
}

func TestMarshalEntityJSON(t *testing.T) {
	e := testEntity("marshal-test")
	data, err := MarshalEntityJSON(e)
	if err != nil {
		t.Fatalf("MarshalEntityJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}

func TestRepoURL_Fallback(t *testing.T) {
	e := testEntity("url-test")
	e.Repository.URL = "" // No repo URL
	e.Endpoints = []models.EndpointWithType{
		{
			Endpoint: models.Endpoint{URL: "http://localhost:8080"},
			Type:     models.EndpointTypeMCPRuntime,
		},
	}
	// No repository URL and no REPOSITORY_URL endpoint type
	url := repoURL(e)
	if url != "#" {
		t.Errorf("expected # for no repo URL, got %s", url)
	}
}

func TestViewGenerator_Integration(t *testing.T) {
	dir := t.TempDir()
	vg := NewViewGenerator(testViewConfig())

	entities := []*models.Entity{
		testEntity("verified-mcp"),
		func() *models.Entity {
			e := testEntity("candidate-mcp")
			e.MCPIdentity.Status = models.MCPIdentityStatusCandidate
			return e
		}(),
		func() *models.Entity {
			e := testEntity("ai-agent")
			e.Classification.Primary = models.PrimaryClassificationAIAgent
			return e
		}(),
		func() *models.Entity {
			e := testEntity("ai-tool")
			e.Classification.Primary = models.PrimaryClassificationAITool
			return e
		}(),
		func() *models.Entity {
			e := testEntity("dataset")
			e.Classification.Primary = models.PrimaryClassificationAIDataset
			return e
		}(),
		// T0 — should not appear in taiwan-ai-ecosystem
		func() *models.Entity {
			e := testEntity("t0-entity")
			e.TaiwanRelevance.Level = models.TaiwanRelevanceLevelT0
			return e
		}(),
	}

	if err := vg.GenerateViews(entities, dir); err != nil {
		t.Fatalf("GenerateViews failed: %v", err)
	}

	// Verify files exist for all views
	for _, view := range allViews {
		mdPath := filepath.Join(dir, view.Name+".md")
		jsonPath := filepath.Join(dir, view.Name+".json")
		if _, err := os.Stat(mdPath); err != nil {
			t.Errorf("missing %s", view.Name+".md")
		}
		if _, err := os.Stat(jsonPath); err != nil {
			t.Errorf("missing %s", view.Name+".json")
		}
	}

	// Verify taiwan-mcp.md contains only the verified server
	mcpMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-mcp.md"))
	mcpContent := string(mcpMd)
	if !strings.Contains(mcpContent, "Test Server verified-mcp") {
		t.Errorf("verified-mcp should be in taiwan-mcp.md")
	}
	if strings.Contains(mcpContent, "Test Server candidate-mcp") {
		t.Errorf("candidate-mcp should NOT be in taiwan-mcp.md")
	}

	// Verify taiwan-mcp-candidates.md contains the candidate
	candMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-mcp-candidates.md"))
	if !strings.Contains(string(candMd), "Test Server candidate-mcp") {
		t.Errorf("candidate-mcp should be in taiwan-mcp-candidates.md")
	}

	// Verify t0-entity is NOT in taiwan-ai-ecosystem
	ecoMd, _ := os.ReadFile(filepath.Join(dir, "taiwan-ai-ecosystem.md"))
	if strings.Contains(string(ecoMd), "Test Server t0-entity") {
		t.Errorf("t0-entity should NOT be in taiwan-ai-ecosystem.md")
	}
}
