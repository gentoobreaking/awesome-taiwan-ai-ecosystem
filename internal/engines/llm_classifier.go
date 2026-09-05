package engines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// AuthError indicates authentication failure (invalid API key).
// ClassifyWithLLM must fail-fast on AuthError and NOT try fallback models.
type AuthError struct {
	Status int
	Body   string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("auth error %d: %s", e.Status, e.Body)
}

// IsAuthError reports whether err is an *AuthError (authentication failure).
func IsAuthError(err error) bool {
	var ae *AuthError
	return errors.As(err, &ae)
}

// LLMClassifierFallback provides LLM-based classification fallback for ambiguous cases.
// It is invoked only when rule-based classifier confidence is low (< 0.7) OR
// when both Taiwan score (20-55) AND AI score (20-55) are in ambiguous ranges.
type LLMClassifierFallback struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	maxRetries int
	models     []string // fallback chain
}

// LLMClassifierFallbackConfig holds configuration for LLM classifier fallback.
type LLMClassifierFallbackConfig struct {
	BaseURL    string
	APIKey     string
	MaxRetries int
	Models     []string
}

// DefaultLLMClassifierFallbackConfig returns default configuration.
func DefaultLLMClassifierFallbackConfig() *LLMClassifierFallbackConfig {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	// Ensure baseURL ends with /chat/completions
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = baseURL + "/chat/completions"
	}

	models := []string{"gpt-4o-mini", "gpt-4o"} // default fallback chain
	if m := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); m != "" {
		models = []string{m}
	}

	return &LLMClassifierFallbackConfig{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		MaxRetries: 3,
		Models:     models,
	}
}

// NewLLMClassifierFallback creates a new LLM classifier fallback.
// Returns nil if no API key is configured.
func NewLLMClassifierFallback(cfg *LLMClassifierFallbackConfig) *LLMClassifierFallback {
	if cfg == nil {
		cfg = DefaultLLMClassifierFallbackConfig()
	}
	if cfg.APIKey == "" {
		return nil
	}
	return &LLMClassifierFallback{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		maxRetries: cfg.MaxRetries,
		models:     cfg.Models,
	}
}

// ShouldTriggerLLM checks if LLM fallback should be triggered.
// Trigger conditions (spec §4.4, algs/taiwan-classification.md §176-187):
// - Rule-based classifier confidence < 0.7
// - OR (TaiwanScore in 20-55 range AND AIScore in 20-55 range)
func ShouldTriggerLLM(ruleResult models.ClassificationResult, taiwanScore, aiScore float64) bool {
	// Low confidence from rule-based classifier
	if ruleResult.Confidence < 0.7 {
		return true
	}

	// Both Taiwan and AI scores in ambiguous zone (20-55)
	taiwanAmbiguous := taiwanScore >= 20 && taiwanScore <= 55
	aiAmbiguous := aiScore >= 20 && aiScore <= 55
	if taiwanAmbiguous && aiAmbiguous {
		return true
	}

	return false
}

// llmClassificationResponse represents the expected structured JSON output from LLM.
// Based on spec §4.4, algs/taiwan-classification.md §179-187.
type llmClassificationResponse struct {
	Classification string   `json:"classification"`
	Confidence     float64  `json:"confidence"`
	Evidence       []string `json:"evidence"`
	MCPRole        string   `json:"mcp_role"`
	Reason         string   `json:"reason"`
}

// ClassifyWithLLM runs LLM classification fallback for ambiguous entities.
// Returns the original ruleResult if LLM fails/times out/parsing fails (fallback mechanism).
// The LLM can ONLY provide: classification, confidence, evidence, mcp_role, reasoning, description normalization.
// It CANNOT modify: stars, last_commit, license, tool_count, repository_url, endpoint, health_status.
func (lc *LLMClassifierFallback) ClassifyWithLLM(ctx context.Context, entity *models.Entity, ruleResult models.ClassificationResult) models.ClassificationResult {
	if lc == nil {
		// No LLM configured, return rule-based result
		return ruleResult
	}

	// Check if LLM fallback should be triggered
	if !ShouldTriggerLLM(ruleResult, entity.TaiwanRelevance.Score, entity.AIRelevance.Score) {
		// High confidence or scores not in ambiguous zone - return rule-based result directly
		return ruleResult
	}

	// Build prompt with entity info and existing classification result
	prompt := lc.buildPrompt(entity, ruleResult)

	// Try models in fallback order
	var lastErr error
	for _, model := range lc.models {
		result, err := lc.callLLM(ctx, model, prompt)
		if err != nil {
			if IsAuthError(err) {
				// Authentication failure — no point trying fallback model with same key.
				lastErr = err
				break
			}
			lastErr = err
			continue
		}
		if result == nil {
			lastErr = fmt.Errorf("nil result from LLM")
			continue
		}

		// Validate: confidence must be > 0
		if result.Confidence <= 0 {
			lastErr = fmt.Errorf("confidence <= 0 from LLM")
			continue
		}

		// Validate classification
		if !models.IsValidPrimaryClassification(models.PrimaryClassification(result.Classification)) {
			lastErr = fmt.Errorf("invalid classification from LLM: %s", result.Classification)
			continue
		}

		// Validate MCP role if provided
		if result.MCPRole != "" && !models.IsValidMCPRole(models.MCPRole(result.MCPRole)) {
			lastErr = fmt.Errorf("invalid MCP role from LLM: %s", result.MCPRole)
			continue
		}

		// Build evidence from LLM response
		evidence := make([]models.ClassificationEvidence, 0, len(result.Evidence)+len(ruleResult.Evidence))

		// Keep original evidence
		evidence = append(evidence, ruleResult.Evidence...)

		// Add LLM evidence
		for _, e := range result.Evidence {
			evidence = append(evidence, models.ClassificationEvidence{
				Evidence: models.Evidence{
					Type:         "llm_classification",
					Source:       "llm:" + model,
					Location:     "classification",
					Rule:         "llm_fallback",
					MatchedText:  e,
					Confidence:   result.Confidence,
					Score:        result.Confidence * 100,
					Timestamp:    models.RFC3339Time(time.Now().UTC()),
					ContentHash:  "", // computed from matched text if needed
				},
				Classification: models.PrimaryClassification(result.Classification),
				MCPRole:        models.MCPRole(result.MCPRole),
			})
		}

		// Build final result - LLM result takes precedence for classification
		// but we preserve high-confidence rule-based evidence
		mcpRole := ruleResult.MCPRole
		if result.MCPRole != "" {
			mcpRole = models.MCPRole(result.MCPRole)
		}

		return models.ClassificationResult{
			Primary:    models.PrimaryClassification(result.Classification),
			Confidence: result.Confidence,
			Evidence:   evidence,
			MCPRole:    mcpRole,
			Reasoning:  result.Reason + " (LLM fallback)",
		}
	}

	// All models failed - return original rule-based result (fallback mechanism)
	// Add evidence of LLM failure
	failureEvidence := models.ClassificationEvidence{
		Evidence: models.Evidence{
			Type:         "llm_fallback_failure",
			Source:       "llm_classifier",
			Rule:         "fallback_error",
			Location:     "classification",
			MatchedText:  fmt.Sprintf("LLM fallback failed: %v", lastErr),
			Confidence:   0,
			Score:        0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		},
	}
	ruleResult.Evidence = append(ruleResult.Evidence, failureEvidence)
	return ruleResult
}

// buildPrompt constructs the LLM prompt with entity info and existing classification.
func (lc *LLMClassifierFallback) buildPrompt(entity *models.Entity, ruleResult models.ClassificationResult) string {
	// Get source code summary (truncated)
	sourceCode := entity.RawContent
	if len(sourceCode) > 3000 {
		sourceCode = sourceCode[:3000] + "..."
	}

	// Get README summary (truncated)
	readme := ""
	if entity.Repository.URL != "" {
		readme = "Repository: " + entity.Repository.URL
	}

	// Package manifest info
	manifestInfo := ""
	if len(entity.Repository.PackageFiles) > 0 {
		files := make([]string, 0, len(entity.Repository.PackageFiles))
		for name := range entity.Repository.PackageFiles {
			files = append(files, name)
		}
		manifestInfo = "Package files: " + strings.Join(files, ", ")
	}

	// Current classification result
	ruleClassification := string(ruleResult.Primary)
	ruleConfidence := ruleResult.Confidence
	ruleReasoning := ruleResult.Reasoning
	ruleMCPRole := string(ruleResult.MCPRole)

	// Taiwan and AI scores
	taiwanScore := entity.TaiwanRelevance.Score
	aiScore := entity.AIRelevance.Score

	return fmt.Sprintf(`You are an expert classifier for Taiwan AI ecosystem projects.

CRITICAL: You can ONLY output a JSON object with these fields:
- classification: one of the valid primary classifications (MCP_SERVER, MCP_CLIENT, MCP_HOST, MCP_SDK, MCP_LIBRARY, MCP_EXTENSION, MCP_SKILL, MCP_COLLECTION, AI_AGENT, AI_TOOL, AI_SDK, AI_FRAMEWORK, AI_SKILL, AI_KNOWLEDGE_BASE, AI_DATASET, AI_API, AI_APPLICATION, AI_INFRASTRUCTURE, AI_PLUGIN, AI_TUTORIAL, AI_EXAMPLE, AI_COLLECTION, AI_REGISTRY, AI_RELATED_PROJECT, NON_AI_PROJECT, UNKNOWN)
- confidence: 0.0-1.0 (your confidence in this classification)
- evidence: array of strings describing what evidence supports this classification
- mcp_role: if MCP-related, one of SERVER, CLIENT, HOST, SDK, LIBRARY, EXTENSION, SKILL, NONE
- reason: brief explanation for the classification

You CANNOT modify any factual metadata. The following fields are READ-ONLY and must NOT be changed:
- stars, last_commit, license, tool_count
- repository_url, endpoint, health_status

Current Rule-Based Classification:
- Primary: %s
- Confidence: %.2f
- MCP Role: %s
- Reasoning: %s

Entity Information:
- Name: %s
- Description: %s
- Repository URL: %s
- Taiwan Relevance Score: %.1f (Level: %s)
- AI Relevance Score: %.1f (Level: %s)
- %s

Source Code Summary:
%s

README:
%s

Valid Classifications (must match exactly):
MCP_SERVER, MCP_CLIENT, MCP_HOST, MCP_SDK, MCP_LIBRARY, MCP_EXTENSION, MCP_SKILL, MCP_COLLECTION,
AI_AGENT, AI_TOOL, AI_SDK, AI_FRAMEWORK, AI_SKILL, AI_KNOWLEDGE_BASE, AI_DATASET, AI_API,
AI_APPLICATION, AI_INFRASTRUCTURE, AI_PLUGIN, AI_TUTORIAL, AI_EXAMPLE, AI_COLLECTION, AI_REGISTRY,
AI_RELATED_PROJECT, NON_AI_PROJECT, UNKNOWN

Valid MCP Roles: SERVER, CLIENT, HOST, SDK, LIBRARY, EXTENSION, SKILL, NONE

Output ONLY valid JSON:
{"classification": "MCP_SERVER", "confidence": 0.91, "evidence": ["README", "source_code", "package_manifest"], "mcp_role": "SERVER", "reason": "Implements McpServer with stdio transport..."}`,
		ruleClassification, ruleConfidence, ruleMCPRole, ruleReasoning,
		entity.Name, entity.Description, entity.Repository.URL,
		taiwanScore, entity.TaiwanRelevance.Level,
		aiScore, entity.AIRelevance.Level,
		manifestInfo,
		sourceCode,
		readme,
	)
}

// callLLM sends a request to the OpenAI-compatible API.
func (lc *LLMClassifierFallback) callLLM(ctx context.Context, model string, prompt string) (*llmClassificationResponse, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name": "entity_classification",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"classification": map[string]interface{}{
							"type": "string",
							"enum": []string{
								"MCP_SERVER", "MCP_CLIENT", "MCP_HOST", "MCP_SDK", "MCP_LIBRARY",
								"MCP_EXTENSION", "MCP_SKILL", "MCP_COLLECTION",
								"AI_AGENT", "AI_TOOL", "AI_SDK", "AI_FRAMEWORK", "AI_SKILL",
								"AI_KNOWLEDGE_BASE", "AI_DATASET", "AI_API", "AI_APPLICATION",
								"AI_INFRASTRUCTURE", "AI_PLUGIN", "AI_TUTORIAL", "AI_EXAMPLE",
								"AI_COLLECTION", "AI_REGISTRY", "AI_RELATED_PROJECT",
								"NON_AI_PROJECT", "UNKNOWN",
							},
						},
						"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
						"evidence": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"mcp_role": map[string]interface{}{
							"type": "string",
							"enum": []string{"SERVER", "CLIENT", "HOST", "SDK", "LIBRARY", "EXTENSION", "SKILL", "NONE"},
						},
						"reason": map[string]interface{}{"type": "string"},
					},
					"required": []string{"classification", "confidence", "evidence", "mcp_role", "reason"},
					"additionalProperties": false,
				},
			},
		},
		"temperature": 0.0, // deterministic
		"max_tokens":  1024,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", lc.baseURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+lc.apiKey)

	resp, err := lc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		msg := string(body)
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			if strings.Contains(msg, "AuthError") || strings.Contains(msg, "invalid_api_key") || strings.Contains(msg, "Invalid API key") {
				return nil, &AuthError{Status: resp.StatusCode, Body: msg}
			}
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, msg)
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(raw.Choices) == 0 || raw.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	var result llmClassificationResponse
	if err := json.Unmarshal([]byte(raw.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal LLM JSON: %w", err)
	}

	return &result, nil
}