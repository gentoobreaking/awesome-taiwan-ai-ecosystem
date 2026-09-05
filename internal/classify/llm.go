// Package classify implements Taiwan relevance classification (§17).
package classify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/normalize"
)

// llmModels is the fallback chain for LLM classification.
// First model is primary; subsequent are fallbacks.
// NOTE: opencode.ai/zen/v1 expects bare model IDs without "opencode/" prefix.
// Do NOT add provider prefix; use "muse-spark-1.2-contributor-free" etc.
var llmModels = []string{
	"muse-spark-1.2-contributor-free",
	"nemotron-3-ultra-free",
}

// AuthError indicates authentication failure (invalid API key / AuthError).
// Classify must fail-fast on AuthError and NOT try fallback models.
// Caller can use IsAuthError / errors.As to detect it.
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

// LLMCallCount tracks total LLM invocations for observability (§TST-050).
// T0 (score < 20) and T4 (score >= 70) candidates must result in 0 calls.
// Can be reset via ResetLLMCallCount for testing.
var llmCallCount int64

// LLMClassifier provides LLM-based classification for ambiguous candidates.
// It only processes candidates with 20 <= taiwan_score <= 55 (§18).
type LLMClassifier struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	maxRetries int
}

// NewLLMClassifier creates a new LLMClassifier from environment variables.
// Returns nil if OPENAI_API_KEY is not set.
func NewLLMClassifier() *LLMClassifier {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	// Idempotent baseURL handling: if already contains /chat/completions, do not append again.
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		baseURL = baseURL + "/chat/completions"
	}

	// OPENAI_MODEL support: if set, override the fallback chain with the single custom model.
	// This allows operators to pin the deployment to a model known to exist on their endpoint
	// (e.g. OPENAI_MODEL=muse-spark-1.3-contributor-free) without code change.
	// TrimSpace to tolerate accidental whitespace.
	if m := strings.TrimSpace(os.Getenv("OPENAI_MODEL")); m != "" {
		llmModels = []string{m}
	}

	return &LLMClassifier{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
		maxRetries: 3,
	}
}

// ShouldClassifyLLM returns true if this server's score falls in the
// ambiguous range where LLM classification is needed (§18).
// Scores 0-19 (T0/T1) and 56+ (T4/T5) are deterministic — no LLM needed.
func ShouldClassifyLLM(score float64) bool {
	return score >= 20 && score <= 55
}

// Classify runs LLM classification on an ambiguous server.
// The LLM can ONLY provide: Taiwan relevance, category, evidence.
// It CANNOT modify: repository_url, stars, last_commit, license,
// tool_count, endpoint, health_status (§18, §2.3 LLM Isolation).
//
// On LLM failure (timeout, invalid JSON, hallucination):
// - Server metadata remains unchanged
// - Returns empty TaiwanRelevance with confidence=0
// - No panic, no crash (§TST-053)
func (lc *LLMClassifier) Classify(ctx context.Context, server *models.MCPServer) (*models.TaiwanRelevance, error) {
	LLMCallsIncrement()

	// Sanitize README text — strip injection patterns (§60)
	readmeText := normalize.SanitizeReadme(server.GetReadme())

	// Build LLM prompt with only classification-relevant data
	prompt := buildLLMPrompt(server, readmeText)

	// Try models in fallback order.
	// Circuit: AuthError is fail-fast — do not try second model (key is invalid for all models).
	// Retry policy: only 429 / 5xx / network errors are retryable (§22); 401/403 (except rate-limit) are not.
	// This classifier uses a bare http.Client (no RetryableClient); therefore 429/5xx are NOT retried here.
	// If retry is added in the future, it MUST be limited to 429 and 5xx with exponential backoff (1s→2s→4s→8s, max 30s),
	// and MUST NOT retry AuthError / ModelError (401/403 without rate-limit header).
	var lastErr error
	for _, model := range llmModels {
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

		// Validate: LLM must not hallucinate invalid levels
		if !isValidLevel(result.TaiwanRelevance) {
			lastErr = fmt.Errorf("invalid level from LLM: %s", result.TaiwanRelevance)
			continue
		}

		// Validate: confidence must be > 0
		if result.Confidence <= 0 {
			lastErr = fmt.Errorf("confidence <= 0 from LLM")
			continue
		}

		// Build evidence — always marked as classifier=llm (§24)
		evidence := []models.Evidence{{
			Type:       "llm_classification",
			Source:     "llm:" + model,
			Location:   "taiwan_relevance",
			Rule:       "llm_classifier",
			Score:      result.Score,
			Confidence: result.Confidence,
			Timestamp:  time.Now().UTC(),
		}}

		return &models.TaiwanRelevance{
			Level:      result.TaiwanRelevance,
			Score:      result.Score,
			Confidence: result.Confidence,
			Evidence:   evidence,
		}, nil
	}

	// All models failed — preserve T2 score from keyword matching instead of T0
	// Keyword scoring for 20-55 range already indicated Taiwan relevance
	evidence := []models.Evidence{{
		Type:     "llm_failure",
		Source:   "llm_classifier",
		Rule:     "fallback_preserve_t2",
		Location: fmt.Sprintf("LLM failed: %v", lastErr),
		Timestamp: time.Now().UTC(),
	}}
	return &models.TaiwanRelevance{
		Level:      "T2",
		Score:      35,
		Confidence: 0.5,
		Evidence:   evidence,
	}, lastErr
}

// llmResponse represents the expected structured JSON output from LLM (§18).
type llmResponse struct {
	TaiwanRelevance string   `json:"taiwan_relevance"`
	Score           float64  `json:"score"`
	Confidence      float64  `json:"confidence"`
	Categories      []string `json:"categories"`
	Reason          string   `json:"reason"`
}

// callLLM sends a request to the OpenAI-compatible API.
func (lc *LLMClassifier) callLLM(ctx context.Context, model string, prompt string) (*llmResponse, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name": "taiwan_classification",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"taiwan_relevance": map[string]interface{}{"type": "string", "enum": []string{"T0", "T1", "T2", "T3", "T4", "T5"}},
						"score":            map[string]interface{}{"type": "number", "minimum": 0, "maximum": 100},
						"confidence":       map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
						"categories":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"reason":           map[string]interface{}{"type": "string"},
					},
					"required": []string{"taiwan_relevance", "score", "confidence", "categories", "reason"},
				},
			},
		},
		"temperature": 0.3,
		"max_tokens":  512,
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
		// 401/403 body sniffing: distinguish AuthError (invalid key) vs ModelError (unknown model).
		// AuthError is fail-fast (do not try fallback); ModelError falls through to next model.
		// Only 429 / 5xx are considered retryable (with exponential backoff via RetryableClient).
		// This bare client does NOT retry; retry annotation kept for future migration to RetryableClient.
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			if strings.Contains(msg, "AuthError") || strings.Contains(msg, "invalid_api_key") || strings.Contains(msg, "Invalid API key") {
				return nil, &AuthError{Status: resp.StatusCode, Body: msg}
			}
			// ModelError or other 401/403 — return plain error; Classify will try next model.
			// Do NOT retry same model; fallback is at most one extra request.
		}
		// For 429 / 5xx, caller would retry with backoff if RetryableClient were used;
		// bare client returns error immediately (no retry). See Classify comment for policy.
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

	var result llmResponse
	if err := json.Unmarshal([]byte(raw.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("unmarshal LLM JSON: %w", err)
	}

	return &result, nil
}

// isValidLevel checks the LLM returned a valid Taiwan relevance level.
func isValidLevel(level string) bool {
	switch level {
	case "T0", "T1", "T2", "T3", "T4", "T5":
		return true
	default:
		return false
	}
}

// buildLLMPrompt constructs the LLM prompt with only classification-relevant
// data. Factual metadata is included for context but explicitly marked as
// read-only (§2.3 LLM Isolation).
func buildLLMPrompt(server *models.MCPServer, readmeText string) string {
	topics := strings.Join(server.TopicList(), ", ")

	// Truncate readme to avoid excessive length
	if len(readmeText) > 3000 {
		readmeText = readmeText[:3000]
	}

	return fmt.Sprintf(`You are a classifier for Taiwan-relevance of MCP servers.

CRITICAL: You can ONLY output a JSON object with these fields:
- taiwan_relevance: one of T0, T1, T2, T3, T4, T5
- score: 0-100
- confidence: 0-1
- categories: array of category strings
- reason: brief explanation

You CANNOT modify any factual metadata. The following fields are READ-ONLY
and must NOT be changed: repository_url, stars, last_commit, license,
tool_count, endpoint, health_status.

Classification rules:
- T0 (0-19): No Taiwan relevance
- T1 (20-35): Weak Taiwan relevance
- T2 (36-55): Moderate Taiwan relevance
- T3 (56-70): Strong Taiwan relevance
- T4 (71-85): Very strong Taiwan relevance
- T5 (86-100): Definitively Taiwan-focused

Server data:
Name: %s
Description: %s
Repository URL: %s
Topics: %s
Readme (sanitized): %s

Output ONLY valid JSON:
{"taiwan_relevance": "T3", "score": 55, "confidence": 0.9, "categories": ["finance"], "reason": "brief explanation"}`,
		server.Name, server.Description, server.Repository.URL, topics, readmeText)
}

// LLMCallsIncrement atomically increments the LLM call counter.
func LLMCallsIncrement() {
	atomic.AddInt64(&llmCallCount, 1)
}

// LLMCalls returns the total number of LLM invocations.
func LLMCalls() int64 {
	return atomic.LoadInt64(&llmCallCount)
}

// ResetLLMCallCount resets the call counter (for testing).

// GetLLMModels returns the current fallback chain (for testing).
func GetLLMModels() []string {
    c := make([]string, len(llmModels))
    copy(c, llmModels)
    return c
}

// BaseURL returns the classifier's base URL (for testing).
func (lc *LLMClassifier) BaseURL() string {
    return lc.baseURL
}

// ResetLLMModels resets the fallback chain to defaults (for testing).
func ResetLLMModels() {
    llmModels = []string{
        "muse-spark-1.2-contributor-free",
        "nemotron-3-ultra-free",
    }
}

func ResetLLMCallCount() {
	atomic.StoreInt64(&llmCallCount, 0)
}
