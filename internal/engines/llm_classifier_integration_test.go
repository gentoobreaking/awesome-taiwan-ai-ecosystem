package engines

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// TestLLMClassifierFallback_Integration_AmbiguousCase tests full integration with ambiguous case
func TestLLMClassifierFallback_Integration_AmbiguousCase(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.89,
			Evidence:       []string{"source_code: McpServer class found", "package_manifest: @modelcontextprotocol/sdk", "README: MCP server for Taiwan stock data"},
			MCPRole:        "SERVER",
			Reason:         "Implements MCP server with stdio transport, tool definitions, and resource handlers",
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
		ID:          "test-entity-1",
		Name:        "twstock-mcp",
		Description: "MCP server for Taiwan stock market data",
		Repository: models.RepositoryInfo{
			URL:           "https://github.com/example/twstock-mcp",
			Stars:         150,
			License:       "MIT",
			PackageFiles:  map[string]string{"package.json": `{"dependencies": {"@modelcontextprotocol/sdk": "^1.0.0"}}`},
		},
		TaiwanRelevance: models.TaiwanRelevance{
			Score:      45,
			Level:      models.TaiwanRelevanceLevelT3,
			Confidence: 0.8,
		},
		AIRelevance: models.AIRelevance{
			Score:      35,
			Level:      models.AIRelevanceLevelA3,
			Confidence: 0.85,
		},
		RawContent: "import { McpServer, StdioServerTransport } from \"@modelcontextprotocol/sdk/server/stdio.js\";\nimport { z } from \"zod\";\n\nconst server = new McpServer({ name: \"twstock-mcp\", version: \"1.0.0\" });\n\nserver.tool(\"get_stock_price\", \"Get Taiwan stock price\", { symbol: z.string() }, async ({ symbol }) => {\n  const response = await fetch(`https://www.twse.com.tw/exchangeReport/STOCK_DAY?stockNo=${symbol}`);\n  return { content: [{ type: \"text\", text: await response.text() }] };\n});\n\nserver.tool(\"get_market_summary\", \"Get Taiwan market summary\", {}, async () => {\n  const response = await fetch(\"https://www.twse.com.tw/exchangeReport/FMTQIK\");\n  return { content: [{ type: \"text\", text: await response.text() }] };\n});\n\nconst transport = new StdioServerTransport();\nawait server.connect(transport);",
		EntityStatus: models.EntityStatusDiscovered,
	}

	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAIAgent,
		Confidence: 0.55,
		MCPRole:    models.MCPRoleNone,
		Reasoning:  "Rule-based classifier uncertain - could be agent or server",
		Evidence: []models.ClassificationEvidence{
			{
				Evidence: models.Evidence{
					Type:       "rule_based",
					Source:     "classifier",
					Rule:       "isAIAgent",
					MatchedText: "tool calling patterns",
					Confidence: 0.55,
				},
			},
		},
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Expected LLM to classify as MCP_SERVER, got %s", result.Primary)
	}

	if result.Confidence < 0.8 {
		t.Errorf("Expected high confidence from LLM, got %f", result.Confidence)
	}

	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("Expected MCPRole Server, got %s", result.MCPRole)
	}

	foundOriginalEvidence := false
	foundLLMEvidence := false
	for _, e := range result.Evidence {
		if e.Type == "rule_based" && e.Rule == "isAIAgent" {
			foundOriginalEvidence = true
		}
		if e.Type == "llm_classification" {
			foundLLMEvidence = true
			if e.MatchedText == "" {
				t.Error("LLM evidence should have matched text")
			}
		}
	}
	if !foundOriginalEvidence {
		t.Error("Original rule-based evidence should be preserved")
	}
	if !foundLLMEvidence {
		t.Error("LLM evidence should be added")
	}

	if result.Reasoning == "" || len(result.Reasoning) < 10 {
		t.Error("Reasoning should include LLM fallback explanation")
	}

	t.Logf("Integration test passed: %s -> %s (confidence: %.2f)", ruleResult.Primary, result.Primary, result.Confidence)
}

// TestLLMClassifierFallback_Integration_TaiwanAIAmbiguous tests both scores in ambiguous zone
func TestLLMClassifierFallback_Integration_TaiwanAIAmbiguous(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "AI_AGENT",
			Confidence:     0.91,
			Evidence:       []string{"README: autonomous agent with planning", "source_code: ReAct loop implementation", "package_manifest: langchain dependency"},
			MCPRole:        "NONE",
			Reason:         "Implements autonomous agent with ReAct pattern, tool calling, and memory",
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
		ID:          "test-entity-2",
		Name:        "taiwan-travel-agent",
		Description: "AI agent for Taiwan travel planning",
		Repository: models.RepositoryInfo{
			URL:          "https://github.com/example/taiwan-travel-agent",
			PackageFiles: map[string]string{"pyproject.toml": `[project] dependencies = ["langchain", "openai"]`},
		},
		TaiwanRelevance: models.TaiwanRelevance{
			Score:      30,
			Level:      models.TaiwanRelevanceLevelT2,
			Confidence: 0.9,
		},
		AIRelevance: models.AIRelevance{
			Score:      40,
			Level:      models.AIRelevanceLevelA3,
			Confidence: 0.9,
		},
		RawContent: "from langchain.agents import create_react_agent\nfrom langchain_openai import ChatOpenAI\n\nllm = ChatOpenAI(model=\"gpt-4\")\ntools = [search_taiwan_attractions, book_hotel, get_weather_taiwan]\n\nagent = create_react_agent(llm, tools, prompt)\nagent_executor = AgentExecutor(agent=agent, tools=tools)\n\n# Autonomous planning and execution\nresult = agent_executor.invoke({\"input\": \"Plan a 3-day trip to Taipei\"})",
		EntityStatus: models.EntityStatusDiscovered,
	}

	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAITool,
		Confidence: 0.75,
		MCPRole:    models.MCPRoleNone,
		Reasoning:  "Has tool calling but also agent patterns",
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	if result.Primary != models.PrimaryClassificationAIAgent {
		t.Errorf("Expected LLM to classify as AI_AGENT, got %s", result.Primary)
	}

	if result.Confidence < 0.9 {
		t.Errorf("Expected high confidence from LLM, got %f", result.Confidence)
	}

	if result.MCPRole != models.MCPRoleNone {
		t.Errorf("Expected MCPRole None for AI_AGENT, got %s", result.MCPRole)
	}
}

// TestLLMClassifierFallback_Integration_NonTriggering cases that should NOT trigger LLM
func TestLLMClassifierFallback_Integration_NonTriggering(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		t.Errorf("LLM should not be called, but got request #%d", callCount)
	}))
	defer mockServer.Close()

	cfg := &LLMClassifierFallbackConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-key",
		Models:  []string{"test-model"},
	}
	classifier := NewLLMClassifierFallback(cfg)

	testCases := []struct {
		name        string
		entity      *models.Entity
		ruleResult  models.ClassificationResult
		shouldCall  bool
	}{
		{
			name: "High confidence rule result, high Taiwan score",
			entity: &models.Entity{
				Name: "test",
				TaiwanRelevance: models.TaiwanRelevance{Score: 80},
				AIRelevance:     models.AIRelevance{Score: 80},
			},
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.95,
			},
			shouldCall: false,
		},
		{
			name: "High confidence rule result, low Taiwan score",
			entity: &models.Entity{
				Name: "test",
				TaiwanRelevance: models.TaiwanRelevance{Score: 10},
				AIRelevance:     models.AIRelevance{Score: 10},
			},
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationNonAIProject,
				Confidence: 0.9,
			},
			shouldCall: false,
		},
		{
			name: "High confidence, only Taiwan ambiguous",
			entity: &models.Entity{
				Name: "test",
				TaiwanRelevance: models.TaiwanRelevance{Score: 30},
				AIRelevance:     models.AIRelevance{Score: 80},
			},
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationMCPServer,
				Confidence: 0.85,
			},
			shouldCall: false,
		},
		{
			name: "High confidence, only AI ambiguous",
			entity: &models.Entity{
				Name: "test",
				TaiwanRelevance: models.TaiwanRelevance{Score: 80},
				AIRelevance:     models.AIRelevance{Score: 30},
			},
			ruleResult: models.ClassificationResult{
				Primary:    models.PrimaryClassificationAIAgent,
				Confidence: 0.8,
			},
			shouldCall: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callCount = 0
			ctx := context.Background()
			result := classifier.ClassifyWithLLM(ctx, tc.entity, tc.ruleResult)

			if tc.shouldCall && callCount == 0 {
				t.Error("Expected LLM to be called")
			}
			if !tc.shouldCall && callCount > 0 {
				t.Errorf("Expected NO LLM calls, got %d", callCount)
			}

			if result.Primary != tc.ruleResult.Primary {
				t.Errorf("Expected rule result unchanged, got %s", result.Primary)
			}
			if result.Confidence != tc.ruleResult.Confidence {
				t.Errorf("Expected rule confidence unchanged, got %f", result.Confidence)
			}
		})
	}
}

// TestLLMClassifierFallback_Integration_FallbackBehavior tests various failure scenarios
func TestLLMClassifierFallback_Integration_FallbackBehavior(t *testing.T) {
	t.Run("Network error fallback", func(t *testing.T) {
		cfg := &LLMClassifierFallbackConfig{
			BaseURL: "http://localhost:9999/chat/completions",
			APIKey:  "test-key",
			Models:  []string{"test-model"},
		}
		classifier := NewLLMClassifierFallback(cfg)

		entity := &models.Entity{
			Name:            "test",
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

		if result.Primary != models.PrimaryClassificationMCPServer {
			t.Errorf("Expected fallback to MCP_SERVER, got %s", result.Primary)
		}
		if result.MCPRole != models.MCPRoleServer {
			t.Errorf("Expected fallback MCPRole, got %s", result.MCPRole)
		}
		hasFailure := false
		for _, e := range result.Evidence {
			if e.Type == "llm_fallback_failure" {
				hasFailure = true
				break
			}
		}
		if !hasFailure {
			t.Error("Expected failure evidence")
		}
	})

	t.Run("Invalid JSON response fallback", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices": [{"message": {"content": "not valid json"}}]}`))
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
			TaiwanRelevance: models.TaiwanRelevance{Score: 30},
			AIRelevance:     models.AIRelevance{Score: 40},
		}
		ruleResult := models.ClassificationResult{
			Primary:    models.PrimaryClassificationMCPServer,
			Confidence: 0.5,
		}

		ctx := context.Background()
		result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

		if result.Primary != models.PrimaryClassificationMCPServer {
			t.Errorf("Expected fallback, got %s", result.Primary)
		}
	})

	t.Run("Empty response fallback", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices": [{"message": {"content": ""}}]}`))
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
			TaiwanRelevance: models.TaiwanRelevance{Score: 30},
			AIRelevance:     models.AIRelevance{Score: 40},
		}
		ruleResult := models.ClassificationResult{
			Primary:    models.PrimaryClassificationMCPServer,
			Confidence: 0.5,
		}

		ctx := context.Background()
		result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

		if result.Primary != models.PrimaryClassificationMCPServer {
			t.Errorf("Expected fallback, got %s", result.Primary)
		}
	})
}

// TestLLMClassifierFallback_Integration_ConstraintVerification verifies LLM cannot modify restricted fields
func TestLLMClassifierFallback_Integration_ConstraintVerification(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.95,
			Evidence:       []string{"source_code"},
			MCPRole:        "SERVER",
			Reason:         "Valid MCP server",
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

	originalStars := 500
	originalLicense := "Apache-2.0"
	originalToolCount := 10
	originalEndpoint := "https://api.example.com/mcp"
	originalSecurityStatus := models.SecurityStatusClean

	entity := &models.Entity{
		Name:        "test-mcp",
		Description: "Test MCP server",
		Repository: models.RepositoryInfo{
			URL:       "https://github.com/test/mcp",
			Stars:     originalStars,
			License:   originalLicense,
			Topics:    []string{"mcp", "taiwan"},
		},
		Tools: []models.Tool{
			{Name: "tool1"}, {Name: "tool2"}, {Name: "tool3"},
			{Name: "tool4"}, {Name: "tool5"}, {Name: "tool6"},
			{Name: "tool7"}, {Name: "tool8"}, {Name: "tool9"}, {Name: "tool10"},
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: originalEndpoint}},
		},
		SecurityStatus: models.SecurityStatusDetail{
			Status:     originalSecurityStatus,
			Confidence: 0.9,
		},
		TaiwanRelevance: models.TaiwanRelevance{Score: 30, Confidence: 0.8},
		AIRelevance:     models.AIRelevance{Score: 40, Confidence: 0.8},
		RawContent:      "McpServer implementation...",
	}

	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAIAgent,
		Confidence: 0.5,
		MCPRole:    models.MCPRoleNone,
		Reasoning:  "Uncertain classification",
	}

	ctx := context.Background()
	result := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	if entity.Repository.Stars != originalStars {
		t.Errorf("RESTRICTED FIELD MODIFIED: Stars changed from %d to %d", originalStars, entity.Repository.Stars)
	}
	if entity.Repository.License != originalLicense {
		t.Errorf("RESTRICTED FIELD MODIFIED: License changed from %s to %s", originalLicense, entity.Repository.License)
	}
	if len(entity.Tools) != originalToolCount {
		t.Errorf("RESTRICTED FIELD MODIFIED: Tool count changed from %d to %d", originalToolCount, len(entity.Tools))
	}
	if len(entity.Endpoints) != 1 || entity.Endpoints[0].Endpoint.URL != originalEndpoint {
		t.Errorf("RESTRICTED FIELD MODIFIED: Endpoint changed")
	}
	if entity.SecurityStatus.Status != originalSecurityStatus {
		t.Errorf("RESTRICTED FIELD MODIFIED: Security status changed from %s to %s", originalSecurityStatus, entity.SecurityStatus.Status)
	}
	if entity.TaiwanRelevance.Score != 30 {
		t.Errorf("RESTRICTED FIELD MODIFIED: Taiwan score changed from 30 to %.1f", entity.TaiwanRelevance.Score)
	}
	if entity.AIRelevance.Score != 40 {
		t.Errorf("RESTRICTED FIELD MODIFIED: AI score changed from 40 to %.1f", entity.AIRelevance.Score)
	}
	if entity.Repository.URL != "https://github.com/test/mcp" {
		t.Errorf("RESTRICTED FIELD MODIFIED: Repository URL changed")
	}

	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Classification should be updated by LLM")
	}
	if result.Confidence != 0.95 {
		t.Errorf("Confidence should be updated by LLM")
	}
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("MCPRole should be updated by LLM")
	}
	if result.Reasoning == "" {
		t.Errorf("Reasoning should be updated by LLM")
	}
	if len(result.Evidence) == 0 {
		t.Errorf("Evidence should be updated by LLM")
	}

	t.Log("Constraint verification passed: All restricted fields preserved, only classification fields updated")
}

// TestLLMClassifierFallback_Integration_TemperatureZero tests deterministic output
func TestLLMClassifierFallback_Integration_TemperatureZero(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := llmClassificationResponse{
			Classification: "MCP_SERVER",
			Confidence:     0.9,
			Evidence:       []string{"source_code"},
			MCPRole:        "SERVER",
			Reason:         "Deterministic result",
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
		RawContent:      "McpServer code...",
	}
	ruleResult := models.ClassificationResult{
		Primary:    models.PrimaryClassificationAIAgent,
		Confidence: 0.5,
	}

	ctx := context.Background()
	result1 := classifier.ClassifyWithLLM(ctx, entity, ruleResult)
	result2 := classifier.ClassifyWithLLM(ctx, entity, ruleResult)

	if result1.Primary != result2.Primary {
		t.Errorf("Non-deterministic: first=%s, second=%s", result1.Primary, result2.Primary)
	}
	if result1.Confidence != result2.Confidence {
		t.Errorf("Non-deterministic confidence: first=%.2f, second=%.2f", result1.Confidence, result2.Confidence)
	}
	if result1.Reasoning != result2.Reasoning {
		t.Errorf("Non-deterministic reasoning")
	}

	if callCount != 2 {
		t.Logf("Made %d calls (expected 2 for two invocations)", callCount)
	}
}