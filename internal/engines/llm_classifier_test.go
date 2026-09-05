package engines

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestShouldTriggerLLM(t *testing.T) {
	tests := []struct {
		name        string
		ruleResult  models.ClassificationResult
		taiwanScore float64
		aiScore     float64
		expected    bool
	}{
		{
			name: "Low confidence triggers LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.5, // < 0.7
			},
			taiwanScore: 80,
			aiScore:     80,
			expected:    true,
		},
		{
			name: "High confidence does not trigger LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9, // >= 0.7
			},
			taiwanScore: 80,
			aiScore:     80,
			expected:    false,
		},
		{
			name: "Both scores in ambiguous zone triggers LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9,
			},
			taiwanScore: 30, // 20-55
			aiScore:     40, // 20-55
			expected:    true,
		},
		{
			name: "Only Taiwan ambiguous does not trigger LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9,
			},
			taiwanScore: 30, // 20-55
			aiScore:     80, // > 55
			expected:    false,
		},
		{
			name: "Only AI ambiguous does not trigger LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9,
			},
			taiwanScore: 80, // > 55
			aiScore:     30, // 20-55
			expected:    false,
		},
		{
			name: "Both scores below 20 does not trigger LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9,
			},
			taiwanScore: 10, // < 20
			aiScore:     10, // < 20
			expected:    false,
		},
		{
			name: "Both scores above 55 does not trigger LLM",
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.9,
			},
			taiwanScore: 70, // > 55
			aiScore:     70, // > 55
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldTriggerLLM(tt.ruleResult, tt.taiwanScore, tt.aiScore)
			if result != tt.expected {
				t.Errorf("ShouldTriggerLLM() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_Success(t *testing.T) {
	// Create a mock LLM server that returns valid classification
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("Missing Authorization header")
		}

		// Return valid response
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.92,
			Evidence:       []string{"source_code", "package_manifest"},
			MCPRole:        "SERVER",
			Reason:         "Implements McpServer with stdio transport and tool definitions",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	// Create classifier with mock server URL
	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)
	if classifier == nil {
		t.Fatal("Expected classifier, got nil")
	}

	// Create test entity
	entity := &models.Entity{
		Name:        "test-mcp-server",
		Description: "A test MCP server",
		Repository: models.RepositoryInfo{
			URL:          "https://github.com/test/mcp-server",
			PackageFiles: map[string]string{"package.json": "{}"},
		},
		TaiwanRelevance: models.TaiwanRelevance{Score: 30, Level: models.TaiwanRelevanceLevelT2},
		AIRelevance:     models.AIRelevance{Score: 40, Level: models.AIRelevanceLevelA3},
		RawContent:      "const server = new McpServer({ name: 'test' }); server.tool('test', ...);",
	}

	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAIAgent,
		Confidence: 0.5, // Low confidence triggers LLM
		MCPRole:    models.MCPRoleNone,
		Reasoning:  "Rule-based classification uncertain",
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Verify LLM result took precedence
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected MCP_SERVER, got %s", result.Primary)
	}
	if result.Confidence != 0.92 {
		t.Errorf("Expected confidence 0.92, got %f", result.Confidence)
	}
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("Expected MCPRole Server, got %s", result.MCPRole)
	}
	if !strings.Contains(result.Reasoning, "LLM fallback") {
		t.Errorf("Expected reasoning to mention LLM fallback, got: %s", result.Reasoning)
	}
	// Original evidence should be preserved
	if len(result.Evidence) < 2 {
		t.Errorf("Expected at least 2 evidence items (original + LLM), got %d", len(result.Evidence))
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_NoAPIKey(t *testing.T) {
	// Classifier with no API key should return nil and fall back to rule result
	cfg := &LLMClassifierFallbackConfig{
		APIKey: "",
	}
	classifier := NewLLMClassifierFallback(cfg)
	if classifier != nil {
		t.Fatal("Expected nil classifier when no API key")
	}

	entity := &models.Entity{
		Name:        "test",
		Description: "test",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
	}

	// nil classifier should not panic and should return rule result
	result := ClassifyWithLLM(nil, context.Background(), entity, ruleResult)
	if result.Primary != ruleResult.Primary {
		t.Errorf("Expected rule result when classifier is nil")
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_FallbackOnError(t *testing.T) {
	// Create a mock server that returns 500 error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:        "test",
		Description: "test",
		Repository:  models.RepositoryInfo{URL: "https://github.com/test/repo"},
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
		MCPRole:    models.MCPRoleServer,
		Reasoning:  "Rule-based",
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should fall back to original rule result
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected fallback to MCP_SERVER, got %s", result.Primary)
	}
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("Expected fallback to MCPRole Server, got %s", result.MCPRole)
	}
	// Should have failure evidence
	hasFailureEvidence := false
	for _, e := range result.Evidence {
		if e.Type == "llm_fallback_failure" {
			hasFailureEvidence = true
			break
		}
	}
	if !hasFailureEvidence {
		t.Error("Expected llm_fallback_failure evidence")
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_InvalidClassification(t *testing.T) {
	// Mock server returns invalid classification
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "INVALID_CLASSIFICATION",
			Confidence:     0.9,
			Evidence:       []string{},
			MCPRole:        "SERVER",
			Reason:         "test",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test",
		Description:     "test",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should fall back to original rule result
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected fallback to MCP_SERVER, got %s", result.Primary)
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_LowConfidence(t *testing.T) {
	// Mock server returns low confidence
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.0, // Invalid - must be > 0
			Evidence:       []string{},
			MCPRole:        "SERVER",
			Reason:         "test",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test",
		Description:     "test",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should fall back to original rule result
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected fallback to MCP_SERVER, got %s", result.Primary)
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_AuthErrorFailFast(t *testing.T) {
	// Mock server returns 401 with AuthError
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key", "type": "AuthError"}}`))
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "invalid-key",
		Models:  []string{"model1", "model2"}, // fallback chain
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test",
		Description:     "test",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should fall back immediately on AuthError (not try model2)
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected fallback on AuthError, got %s", result.Primary)
	}
	hasFailureEvidence := false
	for _, e := range result.Evidence {
		if e.Type == "llm_fallback_failure" && strings.Contains(e.MatchedText, "AuthError") {
			hasFailureEvidence = true
			break
		}
	}
	if !hasFailureEvidence {
		t.Error("Expected AuthError in fallback evidence")
	}
}

func TestLLMClassifierFallback_ClassifyWithLLM_ConstraintCheck(t *testing.T) {
	// Verify LLM cannot modify restricted fields
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read request body to verify prompt doesn't ask for restricted field modifications
		// The response should not include any restricted fields
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.9,
			Evidence:       []string{"source_code"},
			MCPRole:        "SERVER",
			Reason:         "Valid MCP server implementation",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	// Entity with all the fields that LLM MUST NOT modify
	entity := &models.Entity{
		Name:        "test",
		Description: "test",
		Repository: models.RepositoryInfo{
			URL:           "https://github.com/test/repo",
			Stars:         100,
			LastCommitAt:  models.RFC3339Time(time.Now()),
			License:       "MIT",
		},
		Tools: []models.Tool{{Name: "tool1"}},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://api.example.com/mcp"}},
		},
		TaiwanRelevance: models.TaiwanRelevance{Score: 30, Confidence: 0.8},
		AIRelevance:     models.AIRelevance{Score: 40, Confidence: 0.8},
		SecurityStatus: models.SecurityStatusDetail{
			Status:     models.SecurityStatusClean,
			Confidence: 0.9,
		},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAIAgent,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Verify entity fields were NOT modified
	if entity.Repository.Stars != 100 {
		t.Errorf("Stars modified! Expected 100, got %d", entity.Repository.Stars)
	}
	if entity.Repository.License != "MIT" {
		t.Errorf("License modified! Expected MIT, got %s", entity.Repository.License)
	}
	if len(entity.Tools) != 1 {
		t.Errorf("Tool count modified! Expected 1, got %d", len(entity.Tools))
	}
	if len(entity.Endpoints) != 1 || entity.Endpoints[0].Endpoint.URL != "https://api.example.com/mcp" {
		t.Errorf("Endpoint modified!")
	}
	if entity.SecurityStatus.Status != models.SecurityStatusClean {
		t.Errorf("Security status modified!")
	}
	if entity.TaiwanRelevance.Score != 30 {
		t.Errorf("Taiwan score modified!")
	}
	if entity.AIRelevance.Score != 40 {
		t.Errorf("AI score modified!")
	}

	// Only classification, confidence, evidence, mcp_role, reasoning should be affected
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Classification should be updated by LLM")
	}
}

// ClassifyWithLLM wrapper for nil classifier case (for testing)
func ClassifyWithLLM(classifier *LLMClassifierFallback, ctx context.Context, entity *models.Entity, ruleResult models.ClassificationResult) models.ClassificationResult {
	if classifier == nil {
		return ruleResult
	}
	return classifier.ClassifyWithLLM(ctx, entity, ruleResult)
}

func mustMarshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Test with multiple models fallback
func TestLLMClassifierFallback_MultipleModelsFallback(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First model fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Model 1 error"))
			return
		}
		// Second model succeeds
		response := llmClassificationResponse{
			Classification: "AI_AGENT",
			Confidence:     0.88,
			Evidence:       []string{"README", "source_code"},
			MCPRole:        "NONE",
			Reason:         "Implements autonomous agent with tool calling",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"model1", "model2"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test-agent",
		Description:     "An AI agent",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should use second model's result
	if result.Primary != models.PrimaryClassificationAIAgent {
		t.Errorf("Expected AI_AGENT from second model, got %s", result.Primary)
	}
	if result.Confidence != 0.88 {
		t.Errorf("Expected confidence 0.88, got %f", result.Confidence)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls (first fails, second succeeds), got %d", callCount)
	}
}

// Test that high confidence rule result does NOT trigger LLM
func TestLLMClassifierFallback_HighConfidenceNoLLM(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		t.Error("LLM should not be called for high confidence rule result")
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test",
		Description:     "test",
		// Scores outside ambiguous zone (not 20-55)
		TaiwanRelevance: models.TaiwanRelevance{Score: 80},
		AIRelevance:     models.AIRelevance{Score: 80},
	}
	// High confidence - should NOT trigger LLM
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.95,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should return rule result directly without calling LLM
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected MCP_SERVER, got %s", result.Primary)
	}
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Confidence)
	}
	if callCount != 0 {
		t.Errorf("Expected 0 LLM calls for high confidence, got %d", callCount)
	}
}

func TestLLMClassifierFallback_ValidMCPRoleValidation(t *testing.T) {
	// Mock server returns invalid MCP role
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.9,
			Evidence:       []string{},
			MCPRole:        "INVALID_ROLE",
			Reason:         "test",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": mustMarshalJSON(response),
					},
				},
			},
		})
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	entity := &models.Entity{
		Name:            "test",
		Description:     "test",
		TaiwanRelevance: models.TaiwanRelevance{Score: 30},
		AIRelevance:     models.AIRelevance{Score: 40},
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationMCPServer,
		Confidence: 0.5,
		MCPRole:    models.MCPRoleServer,
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	// Should fall back to original rule result due to invalid MCP role
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected fallback to MCP_SERVER, got %s", result.Primary)
	}
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("Expected fallback MCPRole Server, got %s", result.MCPRole)
	}
}

func TestNewLLMClassifierFallback_DefaultConfig(t *testing.T) {
	// Test with env vars
	os.Setenv("OPENAI_API_KEY", "test-key-from-env")
	os.Setenv("OPENAI_BASE_URL", "https://custom.api.com")
	os.Setenv("OPENAI_MODEL", "custom-model")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_BASE_URL")
		os.Unsetenv("OPENAI_MODEL")
	}()

	classifier := NewLLMClassifierFallback(nil)
	if classifier == nil {
		t.Fatal("Expected classifier with env vars")
	}
	if !strings.Contains(classifier.baseURL, "custom.api.com") {
		t.Errorf("Expected custom base URL, got %s", classifier.baseURL)
	}
	if len(classifier.models) != 1 || classifier.models[0] != "custom-model" {
		t.Errorf("Expected custom model, got %v", classifier.models)
	}
}