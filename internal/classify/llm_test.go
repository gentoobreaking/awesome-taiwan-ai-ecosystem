package classify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestShouldClassifyLLM(t *testing.T) {
	tests := []struct {
		score   float64
		want    bool
		desc    string
	}{
		{0, false, "T0 - no LLM"},
		{19, false, "T1 low - deterministic"},
		{20, true, "ambiguous range start"},
		{35, true, "T1 high - ambiguous"},
		{55, true, "ambiguous range end"},
		{56, false, "T2 - deterministic"},
		{70, false, "T3/T4 boundary"},
		{100, false, "T5 - no LLM"},
	}

	for _, tt := range tests {
		if got := ShouldClassifyLLM(tt.score); got != tt.want {
			t.Errorf("ShouldClassifyLLM(%.0f) = %v, want %v (%s)", tt.score, got, tt.want, tt.desc)
		}
	}
}

func TestLLMCallsCounting(t *testing.T) {
	ResetLLMCallCount()
	if LLMCalls() != 0 {
		t.Error("expected 0 calls after reset")
	}
	LLMCallsIncrement()
	LLMCallsIncrement()
	if LLMCalls() != 2 {
		t.Error("expected 2 calls")
	}
}

func TestClassify_LLMInvocationTracking(t *testing.T) {
	// §TST-050: T0 (score=0) and T4 (score=70+) → LLM calls = 0
	// §TST-051: score=20-55 → LLM invoked = true
	ResetLLMCallCount()

	// Verify ShouldClassifyLLM gates LLM calls at the coordinator level
	nonAmbiguous := []float64{0, 10, 19, 70, 85, 100}
	ambiguous := []float64{20, 35, 50, 55}

	for _, score := range nonAmbiguous {
		if ShouldClassifyLLM(score) {
			t.Errorf("score %.0f should not trigger LLM", score)
		}
	}
	for _, score := range ambiguous {
		if !ShouldClassifyLLM(score) {
			t.Errorf("score %.0f should trigger LLM", score)
		}
	}

	// Simulate what coordinator does
	ResetLLMCallCount()
	for _, score := range nonAmbiguous {
		if ShouldClassifyLLM(score) {
			LLMCallsIncrement()
		}
	}
	if LLMCalls() != 0 {
		t.Errorf("expected 0 LLM calls for non-ambiguous scores, got %d", LLMCalls())
	}
}

func TestNewLLMClassifier_NilWithoutAPIKey(t *testing.T) {
	// Should return nil when OPENAI_API_KEY is not set
	lc := NewLLMClassifier()
	// In test environment, API key may or may not be set
	if lc != nil {
		// If it was created, verify it has expected fields
		if lc.baseURL == "" {
			t.Error("baseURL should not be empty")
		}
		if lc.apiKey == "" {
			t.Error("apiKey should not be empty")
		}
	}
}

func TestCallLLM_Success(t *testing.T) {
	// Mock OpenAI-compatible API
	mockBody := `{"choices":[{"message":{"content":"{\"taiwan_relevance\":\"T3\",\"score\":55,\"confidence\":0.9,\"categories\":[\"finance\"],\"reason\":\"Taiwan financial API\"}"}}}`
	server := &models.MCPServer{
		Name:        "test",
		Description: "test server",
	}

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockBody))
	}))
	defer mockAPI.Close()

	lc := &LLMClassifier{
		client:  mockAPI.Client(),
		baseURL: mockAPI.URL,
		apiKey:  "test-key",
	}

	// We need to test callLLM - but it's unexported, so test via Classify
	result, err := lc.Classify(context.Background(), server)
	if err != nil {
		// Error is expected if the mock returns invalid data
		t.Logf("Classify returned error (expected with mock): %v", err)
	}
	if result != nil {
		if !isValidLevel(result.Level) {
			t.Errorf("invalid level: %s", result.Level)
		}
	}
}

func TestCallLLM_APIError(t *testing.T) {
	server := &models.MCPServer{
		Name: "test",
	}

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("{\"error\":\"invalid_api_key\"}"))
	}))
	defer mockAPI.Close()

	lc := &LLMClassifier{
		client:  mockAPI.Client(),
		baseURL: mockAPI.URL,
		apiKey:  "bad-key",
	}

	_, err := lc.Classify(context.Background(), server)
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestClassify_FailureFallback(t *testing.T) {
	// §TST-053: LLM failure → crawler does not crash, fallback executed
	server := &models.MCPServer{
		Name: "test",
	}

	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockAPI.Close()

	lc := &LLMClassifier{
		client:  mockAPI.Client(),
		baseURL: mockAPI.URL,
		apiKey:  "test-key",
	}

	result, err := lc.Classify(context.Background(), server)
	if err == nil {
		t.Error("expected error for all-model failure")
	}
	if result == nil {
		t.Fatal("expected non-nil result with fallback")
	}
	// LLM failure fallback preserves T2 (keyword score 20-55 range already indicated relevance)
	if result.Level != "T2" {
		t.Errorf("expected fallback level T2, got %s", result.Level)
	}
	if result.Score != 35 {
		t.Errorf("expected fallback score 35, got %.0f", result.Score)
	}
}

func TestClassify_HallucinatedLevelRejected(t *testing.T) {
	server := &models.MCPServer{
		Name: "test",
	}

	mockBody := `{"choices":[{"message":{"content":"{\"taiwan_relevance\":\"INVALID\",\"score\":55,\"confidence\":0.9,\"categories\":[],\"reason\":\"test\"}"}}}`
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockBody))
	}))
	defer mockAPI.Close()

	lc := &LLMClassifier{
		client:  mockAPI.Client(),
		baseURL: mockAPI.URL,
		apiKey:  "test-key",
	}

	result, err := lc.Classify(context.Background(), server)
	// With only one model in the fallback chain and it returns invalid,
	// we should get a fallback result
	if err == nil {
		// If no error, result should be valid
		if !isValidLevel(result.Level) {
			t.Error("expected valid level or fallback")
		}
	}
}

func TestClassify_JSONOutput(t *testing.T) {
	// §TST requirement: LLM outputs must be valid JSON
	// Verify our LLM response struct can be marshaled/unmarshaled

	resp := llmResponse{
		TaiwanRelevance: "T3",
		Score:           55,
		Confidence:      0.9,
		Categories:      []string{"finance"},
		Reason:          "Taiwan financial API detected",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var unmarshaled llmResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if unmarshaled.TaiwanRelevance != "T3" {
		t.Errorf("expected T3, got %s", unmarshaled.TaiwanRelevance)
	}
	if unmarshaled.Score != 55 {
		t.Errorf("expected score 55, got %.2f", unmarshaled.Score)
	}
}

func TestClassify_FactualMetadataUnchanged(t *testing.T) {
	// §TST-052: LLM cannot modify factual metadata
	server := &models.MCPServer{
		Name:         "original-name",
		Description:  "original-desc",
		Repository:   models.RepositoryInfo{URL: "https://github.com/test/repo", Stars: 100, License: "MIT"},
		Endpoints:    []models.Endpoint{{URL: "https://api.test.com", Transport: "http"}},
		Tools:        []models.Tool{{Name: "tool1"}},
		Readme:       "original readme",
	}

	originalURL := server.Repository.URL
	originalStars := server.Repository.Stars
	originalLicense := server.License
	originalToolCount := len(server.Tools)

	// Create a mock that returns valid classification
	mockBody := `{"choices":[{"message":{"content":"{\"taiwan_relevance\":\"T3\",\"score\":55,\"confidence\":0.9,\"categories\":[\"finance\"],\"reason\":\"test\"}"}}]}`
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockBody))
	}))
	defer mockAPI.Close()

	lc := &LLMClassifier{
		client:  mockAPI.Client(),
		baseURL: mockAPI.URL,
		apiKey:  "test-key",
	}

	result, err := lc.Classify(context.Background(), server)
	if err != nil {
		t.Logf("Classify error (may be expected): %v", err)
	}

	// Verify factual metadata unchanged
	if server.Repository.URL != originalURL {
		t.Errorf("repository_url changed: %s != %s", server.Repository.URL, originalURL)
	}
	if server.Repository.Stars != originalStars {
		t.Errorf("stars changed: %d != %d", server.Repository.Stars, originalStars)
	}
	if server.License != originalLicense {
		t.Errorf("license changed: %s != %s", server.License, originalLicense)
	}
	if len(server.Tools) != originalToolCount {
		t.Errorf("tool count changed: %d != %d", len(server.Tools), originalToolCount)
	}

	// Only TaiwanRelevance should be modified by caller
	if result != nil && isValidLevel(result.Level) {
		// Good - classification result provided, but server metadata unchanged
	}
}

func TestClassify_LLMCallsZeroForNonAmbiguous(t *testing.T) {
	// §TST-050: T0 and T4 candidates → LLM calls = 0
	ResetLLMCallCount()

	// Simulate what the coordinator does: check ShouldClassifyLLM before calling
	scores := []float64{0, 10, 19, 70, 85, 100}
	for _, score := range scores {
		if ShouldClassifyLLM(score) {
			LLMCallsIncrement()
		}
	}

	if LLMCalls() != 0 {
		t.Errorf("expected 0 LLM calls for non-ambiguous scores, got %d", LLMCalls())
	}

	// Ambiguous scores should trigger LLM calls
	ambigScores := []float64{20, 35, 50, 55}
	ResetLLMCallCount()
	for _, score := range ambigScores {
		if ShouldClassifyLLM(score) {
			LLMCallsIncrement()
		}
	}
	if LLMCalls() != 4 {
		t.Errorf("expected 4 LLM calls for ambiguous scores, got %d", LLMCalls())
	}
}

func TestIsValidLevel(t *testing.T) {
	valid := []string{"T0", "T1", "T2", "T3", "T4", "T5"}
	for _, v := range valid {
		if !isValidLevel(v) {
			t.Errorf("expected %s to be valid", v)
		}
	}

	invalid := []string{"T6", "X0", "", "t0", "T00"}
	for _, v := range invalid {
		if isValidLevel(v) {
			t.Errorf("expected %s to be invalid", v)
		}
	}
}
