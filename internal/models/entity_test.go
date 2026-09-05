package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEntity_JSONRoundTrip(t *testing.T) {
	now := RFC3339Time(time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC))
	entity := &Entity{
		ID:          "abc123",
		Name:        "Test Server",
		Slug:        "test-server",
		Description: "A test MCP server",
		Classification: ClassificationResult{
			Primary:    PrimaryClassificationMCPServer,
			Confidence: 0.95,
			MCPRole:    MCPRoleServer,
			Reasoning:  "Implements McpServer with stdio transport",
		},
		TaiwanRelevance: TaiwanRelevance{
			Score:      75.0,
			Level:      TaiwanRelevanceLevelT5,
			Confidence: 0.9,
			Evidence: []Evidence{
				{Type: "official_domain", Source: "source_code", Rule: "gov_domain", Score: 40, Confidence: 1.0, Timestamp: now},
			},
		},
		AIRelevance: AIRelevance{
			Score:      80.0,
			Level:      AIRelevanceLevelA5,
			Confidence: 0.95,
			Evidence: []Evidence{
				{Type: "core_ai_impl", Source: "source_code", Rule: "mcp_server_impl", Score: 40, Confidence: 1.0, Timestamp: now},
			},
		},
		MCPIdentity: MCPIdentity{
			Status:     MCPIdentityStatusRuntimeVerified,
			Confidence: 0.98,
			Role:       MCPRoleServer,
			StaticCheckedAt: func() *RFC3339Time { t := now; return &t }(),
			RuntimeVerifiedAt: func() *RFC3339Time { t := now; return &t }(),
		},
		RuntimeVerification: &RuntimeVerification{
			Status: RuntimeVerificationStatusPassed,
			InitializeResult: &InitializeResult{
				Success:     true,
				LatencyMs:   150,
				ServerInfo:  "Test MCP Server",
				ProtocolVer: "2024-11-05",
			},
			ToolsListResult: &ToolsListResult{
				Success:    true,
				ToolCount:  5,
				LatencyMs:  50,
			},
			Timestamp: now,
		},
		SecurityStatus: SecurityStatusDetail{
			Status:         SecurityStatusClean,
			ScannerVersion: "1.0.0",
			ScannedAt:      now,
			Confidence:     1.0,
		},
		Quality: QualityScore{
			Score:  85,
			Grade:  QualityGradeB,
			Components: QualityComponents{
				DataSource: 20, Maintenance: 15, Documentation: 10,
				MCPCompliance: 15, ToolSchema: 10, Health: 10,
				Repository: 5, License: 5, Security: 5, Community: 5,
			},
		},
		Repository: RepositoryInfo{
			URL:      "https://github.com/test/test-server",
			Host:     "github.com",
			Owner:    "test",
			Name:     "test-server",
			Stars:    100,
			Forks:    10,
			Language: "Go",
			License:  "MIT",
			Topics:   []string{"mcp", "taiwan", "ai"},
			CreatedAt: now,
			UpdatedAt: now,
			PushedAt: now,
			LastCommitAt: now,
		},
		Endpoints: []EndpointWithType{
			{
				Endpoint: Endpoint{
					URL:       "stdio://test-server",
					Transport: string(TransportStdio),
					TLS:       false,
					Status:    "reachable",
				},
				Type:       EndpointTypeMCPRuntime,
				Confidence: 1.0,
				Evidence: []EndpointEvidence{
					{Rule: "runtime_verified", Source: "runtime_handshake", Confidence: 1.0, Timestamp: now},
				},
			},
		},
		Tools: []Tool{
			{Name: "get_data", Description: "Get Taiwan data", InputSchema: map[string]any{"type": "object"}},
		},
		EntityStatus: EntityStatusVerified,
		Sources: []SourceReference{
			{Source: "github", URL: "https://github.com/test/test-server", TrustScore: 0.95, DiscoveredAt: now, LastSeen: now},
		},
		FirstSeen: now,
		LastSeen:  now,
	}

	// Marshal to JSON
	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal back
	var decoded Entity
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify key fields
	if decoded.ID != entity.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, entity.ID)
	}
	if decoded.Name != entity.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, entity.Name)
	}
	if decoded.Classification.Primary != entity.Classification.Primary {
		t.Errorf("Classification.Primary mismatch: got %s, want %s", decoded.Classification.Primary, entity.Classification.Primary)
	}
	if decoded.TaiwanRelevance.Level != entity.TaiwanRelevance.Level {
		t.Errorf("TaiwanRelevance.Level mismatch: got %s, want %s", decoded.TaiwanRelevance.Level, entity.TaiwanRelevance.Level)
	}
	if decoded.AIRelevance.Level != entity.AIRelevance.Level {
		t.Errorf("AIRelevance.Level mismatch: got %s, want %s", decoded.AIRelevance.Level, entity.AIRelevance.Level)
	}
	if decoded.MCPIdentity.Status != entity.MCPIdentity.Status {
		t.Errorf("MCPIdentity.Status mismatch: got %s, want %s", decoded.MCPIdentity.Status, entity.MCPIdentity.Status)
	}
	if decoded.SecurityStatus.Status != entity.SecurityStatus.Status {
		t.Errorf("SecurityStatus.Status mismatch: got %s, want %s", decoded.SecurityStatus.Status, entity.SecurityStatus.Status)
	}
	if decoded.EntityStatus != entity.EntityStatus {
		t.Errorf("EntityStatus mismatch: got %s, want %s", decoded.EntityStatus, entity.EntityStatus)
	}
	if len(decoded.Endpoints) != len(entity.Endpoints) {
		t.Errorf("Endpoints count mismatch: got %d, want %d", len(decoded.Endpoints), len(entity.Endpoints))
	}
	if len(decoded.Tools) != len(entity.Tools) {
		t.Errorf("Tools count mismatch: got %d, want %d", len(decoded.Tools), len(entity.Tools))
	}
}

func TestPrimaryClassification_JSONRoundTrip(t *testing.T) {
	classifications := []PrimaryClassification{
		PrimaryClassificationMCPServer,
		PrimaryClassificationMCPClient,
		PrimaryClassificationMCPHost,
		PrimaryClassificationMCPSDK,
		PrimaryClassificationMCPLibrary,
		PrimaryClassificationMCPExtension,
		PrimaryClassificationMCPSkill,
		PrimaryClassificationMCPCollection,
		PrimaryClassificationAIAgent,
		PrimaryClassificationAITool,
		PrimaryClassificationAISDK,
		PrimaryClassificationAIFramework,
		PrimaryClassificationAISkill,
		PrimaryClassificationAIKnowledgeBase,
		PrimaryClassificationAIDataset,
		PrimaryClassificationAIAPI,
		PrimaryClassificationAIApplication,
		PrimaryClassificationAIInfrastructure,
		PrimaryClassificationAIPlugin,
		PrimaryClassificationAITutorial,
		PrimaryClassificationAIExample,
		PrimaryClassificationAICollection,
		PrimaryClassificationAIRegistry,
		PrimaryClassificationAIRelatedProject,
		PrimaryClassificationNonAIProject,
		PrimaryClassificationUnknown,
	}

	for _, c := range classifications {
		data, err := json.Marshal(c)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", c, err)
			continue
		}
		var decoded PrimaryClassification
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", c, err)
			continue
		}
		if decoded != c {
			t.Errorf("Round-trip mismatch for %s: got %s", c, decoded)
		}
	}
}

func TestMCPRole_JSONRoundTrip(t *testing.T) {
	roles := []MCPRole{
		MCPRoleServer,
		MCPRoleClient,
		MCPRoleHost,
		MCPRoleSDK,
		MCPRoleLibrary,
		MCPRoleExtension,
		MCPRoleSkill,
		MCPRoleNone,
	}

	for _, r := range roles {
		data, err := json.Marshal(r)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", r, err)
			continue
		}
		var decoded MCPRole
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", r, err)
			continue
		}
		if decoded != r {
			t.Errorf("Round-trip mismatch for %s: got %s", r, decoded)
		}
	}
}

func TestEntityStatus_JSONRoundTrip(t *testing.T) {
	statuses := []EntityStatus{
		EntityStatusDiscovered,
		EntityStatusCandidate,
		EntityStatusVerified,
		EntityStatusQuarantined,
		EntityStatusRejected,
	}

	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", s, err)
			continue
		}
		var decoded EntityStatus
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", s, err)
			continue
		}
		if decoded != s {
			t.Errorf("Round-trip mismatch for %s: got %s", s, decoded)
		}
	}
}

func TestMCPIdentityStatus_JSONRoundTrip(t *testing.T) {
	statuses := []MCPIdentityStatus{
		MCPIdentityStatusCandidate,
		MCPIdentityStatusStaticVerified,
		MCPIdentityStatusRuntimeVerified,
		MCPIdentityStatusNotMCP,
	}

	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", s, err)
			continue
		}
		var decoded MCPIdentityStatus
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", s, err)
			continue
		}
		if decoded != s {
			t.Errorf("Round-trip mismatch for %s: got %s", s, decoded)
		}
	}
}

func TestSecurityStatus_JSONRoundTrip(t *testing.T) {
	statuses := []SecurityStatus{
		SecurityStatusClean,
		SecurityStatusSuspicious,
		SecurityStatusQuarantined,
		SecurityStatusBlocked,
	}

	for _, s := range statuses {
		data, err := json.Marshal(s)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", s, err)
			continue
		}
		var decoded SecurityStatus
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", s, err)
			continue
		}
		if decoded != s {
			t.Errorf("Round-trip mismatch for %s: got %s", s, decoded)
		}
	}
}

func TestEndpointType_JSONRoundTrip(t *testing.T) {
	types := []EndpointType{
		EndpointTypeMCPRuntime,
		EndpointTypeRepositoryURL,
		EndpointTypeDocumentation,
		EndpointTypeInstaller,
		EndpointTypeHomepage,
		EndpointTypeUnknown,
	}

	for _, et := range types {
		data, err := json.Marshal(et)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", et, err)
			continue
		}
		var decoded EndpointType
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", et, err)
			continue
		}
		if decoded != et {
			t.Errorf("Round-trip mismatch for %s: got %s", et, decoded)
		}
	}
}

func TestTaiwanRelevanceLevel_JSONRoundTrip(t *testing.T) {
	levels := []TaiwanRelevanceLevel{
		TaiwanRelevanceLevelT0,
		TaiwanRelevanceLevelT1,
		TaiwanRelevanceLevelT2,
		TaiwanRelevanceLevelT3,
		TaiwanRelevanceLevelT4,
		TaiwanRelevanceLevelT5,
	}

	for _, l := range levels {
		data, err := json.Marshal(l)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", l, err)
			continue
		}
		var decoded TaiwanRelevanceLevel
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", l, err)
			continue
		}
		if decoded != l {
			t.Errorf("Round-trip mismatch for %s: got %s", l, decoded)
		}
	}
}

func TestAIRelevanceLevel_JSONRoundTrip(t *testing.T) {
	levels := []AIRelevanceLevel{
		AIRelevanceLevelA0,
		AIRelevanceLevelA1,
		AIRelevanceLevelA2,
		AIRelevanceLevelA3,
		AIRelevanceLevelA4,
		AIRelevanceLevelA5,
	}

	for _, l := range levels {
		data, err := json.Marshal(l)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", l, err)
			continue
		}
		var decoded AIRelevanceLevel
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", l, err)
			continue
		}
		if decoded != l {
			t.Errorf("Round-trip mismatch for %s: got %s", l, decoded)
		}
	}
}

func TestQualityGrade_JSONRoundTrip(t *testing.T) {
	grades := []QualityGrade{
		QualityGradeA,
		QualityGradeB,
		QualityGradeC,
		QualityGradeD,
		QualityGradeF,
	}

	for _, g := range grades {
		data, err := json.Marshal(g)
		if err != nil {
			t.Errorf("Marshal %s failed: %v", g, err)
			continue
		}
		var decoded QualityGrade
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Errorf("Unmarshal %s failed: %v", g, err)
			continue
		}
		if decoded != g {
			t.Errorf("Round-trip mismatch for %s: got %s", g, decoded)
		}
	}
}

func TestRFC3339Time_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	rfcTime := RFC3339Time(now)

	data, err := json.Marshal(rfcTime)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded RFC3339Time
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !decoded.Time().Equal(now) {
		t.Errorf("Time mismatch: got %v, want %v", decoded.Time(), now)
	}
}

func TestIsValidPrimaryClassification(t *testing.T) {
	valid := []PrimaryClassification{
		PrimaryClassificationMCPServer, PrimaryClassificationAITool,
	}
	for _, c := range valid {
		if !IsValidPrimaryClassification(c) {
			t.Errorf("IsValidPrimaryClassification(%s) = false, want true", c)
		}
	}
	if IsValidPrimaryClassification("INVALID") {
		t.Error("IsValidPrimaryClassification(INVALID) = true, want false")
	}
}

func TestIsValidEntityStatus(t *testing.T) {
	valid := []EntityStatus{
		EntityStatusDiscovered, EntityStatusCandidate,
	}
	for _, s := range valid {
		if !IsValidEntityStatus(s) {
			t.Errorf("IsValidEntityStatus(%s) = false, want true", s)
		}
	}
	if IsValidEntityStatus("INVALID") {
		t.Error("IsValidEntityStatus(INVALID) = true, want false")
	}
}

func TestCanTransitionEntityStatus(t *testing.T) {
	tests := []struct {
		from, to EntityStatus
		want     bool
	}{
		{EntityStatusDiscovered, EntityStatusCandidate, true},
		{EntityStatusCandidate, EntityStatusVerified, true},
		{EntityStatusCandidate, EntityStatusQuarantined, true},
		{EntityStatusCandidate, EntityStatusRejected, true},
		{EntityStatusQuarantined, EntityStatusRejected, true},
		{EntityStatusQuarantined, EntityStatusVerified, true},
		{EntityStatusVerified, EntityStatusRejected, true},
		{EntityStatusDiscovered, EntityStatusVerified, false},
		{EntityStatusVerified, EntityStatusCandidate, false},
		{EntityStatusRejected, EntityStatusVerified, false},
	}
	for _, tt := range tests {
		got := CanTransitionEntityStatus(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransitionEntityStatus(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransitionMCPIdentityStatus(t *testing.T) {
	tests := []struct {
		from, to MCPIdentityStatus
		want     bool
	}{
		{MCPIdentityStatusCandidate, MCPIdentityStatusStaticVerified, true},
		{MCPIdentityStatusCandidate, MCPIdentityStatusNotMCP, true},
		{MCPIdentityStatusStaticVerified, MCPIdentityStatusRuntimeVerified, true},
		{MCPIdentityStatusStaticVerified, MCPIdentityStatusNotMCP, true},
		{MCPIdentityStatusRuntimeVerified, MCPIdentityStatusNotMCP, true},
		{MCPIdentityStatusCandidate, MCPIdentityStatusRuntimeVerified, false},
		{MCPIdentityStatusNotMCP, MCPIdentityStatusCandidate, false},
	}
	for _, tt := range tests {
		got := CanTransitionMCPIdentityStatus(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransitionMCPIdentityStatus(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransitionSecurityStatus(t *testing.T) {
	tests := []struct {
		from, to SecurityStatus
		want     bool
	}{
		{SecurityStatusClean, SecurityStatusSuspicious, true},
		{SecurityStatusClean, SecurityStatusBlocked, true},
		{SecurityStatusSuspicious, SecurityStatusQuarantined, true},
		{SecurityStatusSuspicious, SecurityStatusBlocked, true},
		{SecurityStatusSuspicious, SecurityStatusClean, true},
		{SecurityStatusQuarantined, SecurityStatusBlocked, true},
		{SecurityStatusQuarantined, SecurityStatusClean, true},
		{SecurityStatusBlocked, SecurityStatusClean, false},
		{SecurityStatusBlocked, SecurityStatusSuspicious, false},
	}
	for _, tt := range tests {
		got := CanTransitionSecurityStatus(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransitionSecurityStatus(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestScoreToTaiwanLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  TaiwanRelevanceLevel
	}{
		{80, TaiwanRelevanceLevelT5},
		{70, TaiwanRelevanceLevelT5},
		{60, TaiwanRelevanceLevelT4},
		{55, TaiwanRelevanceLevelT4},
		{50, TaiwanRelevanceLevelT3},
		{40, TaiwanRelevanceLevelT3},
		{30, TaiwanRelevanceLevelT2},
		{20, TaiwanRelevanceLevelT2},
		{10, TaiwanRelevanceLevelT1},
		{5, TaiwanRelevanceLevelT1},
		{4, TaiwanRelevanceLevelT0},
		{0, TaiwanRelevanceLevelT0},
	}
	for _, tt := range tests {
		got := ScoreToTaiwanLevel(tt.score)
		if got != tt.want {
			t.Errorf("ScoreToTaiwanLevel(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreToAILevel(t *testing.T) {
	tests := []struct {
		score float64
		want  AIRelevanceLevel
	}{
		{80, AIRelevanceLevelA5},
		{70, AIRelevanceLevelA5},
		{60, AIRelevanceLevelA4},
		{50, AIRelevanceLevelA4},
		{40, AIRelevanceLevelA3},
		{30, AIRelevanceLevelA3},
		{20, AIRelevanceLevelA2},
		{15, AIRelevanceLevelA2},
		{10, AIRelevanceLevelA1},
		{5, AIRelevanceLevelA1},
		{4, AIRelevanceLevelA0},
		{0, AIRelevanceLevelA0},
	}
	for _, tt := range tests {
		got := ScoreToAILevel(tt.score)
		if got != tt.want {
			t.Errorf("ScoreToAILevel(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestGradeForScore(t *testing.T) {
	tests := []struct {
		score int
		want  QualityGrade
	}{
		{95, QualityGradeA},
		{90, QualityGradeA},
		{85, QualityGradeB},
		{80, QualityGradeB},
		{75, QualityGradeC},
		{70, QualityGradeC},
		{65, QualityGradeD},
		{60, QualityGradeD},
		{59, QualityGradeF},
		{0, QualityGradeF},
	}
	for _, tt := range tests {
		got := GradeForScore(tt.score)
		if got != tt.want {
			t.Errorf("GradeForScore(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestQualityComponents_Total(t *testing.T) {
	qc := QualityComponents{
		DataSource: 20, Maintenance: 15, Documentation: 10,
		MCPCompliance: 15, ToolSchema: 10, Health: 10,
		Repository: 5, License: 5, Security: 5, Community: 5,
	}
	if total := qc.Total(); total != 100 {
		t.Errorf("QualityComponents.Total() = %d, want 100", total)
	}
	if grade := qc.Grade(); grade != QualityGradeA {
		t.Errorf("QualityComponents.Grade() = %s, want A", grade)
	}
}

func TestIsMCPRelated(t *testing.T) {
	mcpTypes := []PrimaryClassification{
		PrimaryClassificationMCPServer, PrimaryClassificationMCPClient,
		PrimaryClassificationMCPHost, PrimaryClassificationMCPSDK,
		PrimaryClassificationMCPLibrary, PrimaryClassificationMCPExtension,
		PrimaryClassificationMCPSkill, PrimaryClassificationMCPCollection,
	}
	for _, c := range mcpTypes {
		if !IsMCPRelated(c) {
			t.Errorf("IsMCPRelated(%s) = false, want true", c)
		}
	}
	nonMCP := []PrimaryClassification{
		PrimaryClassificationAIAgent, PrimaryClassificationAITool,
		PrimaryClassificationNonAIProject, PrimaryClassificationUnknown,
	}
	for _, c := range nonMCP {
		if IsMCPRelated(c) {
			t.Errorf("IsMCPRelated(%s) = true, want false", c)
		}
	}
}

func TestIsAIRelated(t *testing.T) {
	aiTypes := []PrimaryClassification{
		PrimaryClassificationAIAgent, PrimaryClassificationAITool,
		PrimaryClassificationAISDK, PrimaryClassificationAIFramework,
		PrimaryClassificationAISkill, PrimaryClassificationAIKnowledgeBase,
		PrimaryClassificationAIDataset, PrimaryClassificationAIAPI,
		PrimaryClassificationAIApplication, PrimaryClassificationAIInfrastructure,
		PrimaryClassificationAIPlugin, PrimaryClassificationAITutorial,
		PrimaryClassificationAIExample, PrimaryClassificationAICollection,
		PrimaryClassificationAIRegistry, PrimaryClassificationAIRelatedProject,
	}
	for _, c := range aiTypes {
		if !IsAIRelated(c) {
			t.Errorf("IsAIRelated(%s) = false, want true", c)
		}
	}
	if IsAIRelated(PrimaryClassificationNonAIProject) {
		t.Error("IsAIRelated(NON_AI_PROJECT) = true, want false")
	}
}

func TestIsValidMCPRole(t *testing.T) {
	for _, r := range ValidMCPRoles {
		if !IsValidMCPRole(r) {
			t.Errorf("IsValidMCPRole(%s) = false, want true", r)
		}
	}
	if IsValidMCPRole("INVALID") {
		t.Error("IsValidMCPRole(INVALID) = true, want false")
	}
}

func TestToMCPServerView(t *testing.T) {
	now := RFC3339Time(time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC))
	entity := &Entity{
		ID:          "abc123",
		Name:        "Test Server",
		Slug:        "test-server",
		Description: "A test MCP server",
		Classification: ClassificationResult{
			Primary:    PrimaryClassificationMCPServer,
			Confidence: 0.95,
			MCPRole:    MCPRoleServer,
		},
		TaiwanRelevance: TaiwanRelevance{
			Score: 75.0, Level: TaiwanRelevanceLevelT5, Confidence: 0.9,
		},
		MCPIdentity: MCPIdentity{
			Status:     MCPIdentityStatusRuntimeVerified,
			Confidence: 0.98,
			Role:       MCPRoleServer,
		},
		SecurityStatus: SecurityStatusDetail{
			Status: SecurityStatusClean,
		},
		EntityStatus: EntityStatusVerified,
		Repository: RepositoryInfo{
			URL: "https://github.com/test/test-server", License: "MIT",
		},
		Endpoints: []EndpointWithType{
			{
				Endpoint: Endpoint{URL: "stdio://test", Transport: string(TransportStdio), TLS: false},
				Type:     EndpointTypeMCPRuntime,
			},
			{
				Endpoint: Endpoint{URL: "https://github.com/test/test-server", Transport: string(TransportHTTP)},
				Type:     EndpointTypeRepositoryURL,
			},
		},
		FirstSeen: now, LastSeen: now,
	}

	view := entity.ToMCPServerView()
	if view == nil {
		t.Fatal("ToMCPServerView() returned nil for valid MCP server")
	}
	if view.ID != entity.ID {
		t.Errorf("View ID mismatch: %s != %s", view.ID, entity.ID)
	}
	if len(view.Endpoints) != 1 {
		t.Errorf("View endpoints count = %d, want 1 (only MCP_RUNTIME_ENDPOINT)", len(view.Endpoints))
	}
	// Verify it's the MCP runtime endpoint (stdio transport)
	if view.Endpoints[0].Transport != string(TransportStdio) {
		t.Errorf("View endpoint transport = %s, want stdio", view.Endpoints[0].Transport)
	}

	// Test non-MCP server returns nil
	entity.Classification.Primary = PrimaryClassificationAITool
	if view := entity.ToMCPServerView(); view != nil {
		t.Error("ToMCPServerView() should return nil for non-MCP_SERVER")
	}

	// Test non-runtime-verified returns nil
	entity.Classification.Primary = PrimaryClassificationMCPServer
	entity.MCPIdentity.Status = MCPIdentityStatusStaticVerified
	if view := entity.ToMCPServerView(); view != nil {
		t.Error("ToMCPServerView() should return nil for non-RUNTIME_VERIFIED")
	}
}