package schema_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// validateMCPServer performs basic validation matching the schema constraints.
// This is a lightweight validator that checks the critical rules from schema/mcp-server.json
// without requiring an external jsonschema library.
type validationResult struct {
	valid  bool
	errors []string
}

func (r *validationResult) addError(msg string) {
	r.valid = false
	r.errors = append(r.errors, msg)
}

func validateMCPServer(data map[string]any) validationResult {
	var result validationResult
	result.valid = true

	// Required fields
	for _, field := range []string{"id", "name", "slug", "description"} {
		if val, ok := data[field]; !ok || val == nil || val == "" {
			result.addError(fmt.Sprintf("required field '%s' missing or empty", field))
		}
	}

	// Validate taiwan_relevance.level enum
	if tr, ok := data["taiwan_relevance"].(map[string]any); ok {
		level, _ := tr["level"].(string)
		validLevels := map[string]bool{"T0": true, "T1": true, "T2": true, "T3": true, "T4": true, "T5": true}
		if !validLevels[level] {
			result.addError(fmt.Sprintf("taiwan_relevance.level '%s' is not in enum T0-T5", level))
		}
	}

	// Validate health.status enum
	if health, ok := data["health"].(map[string]any); ok {
		status, _ := health["status"].(string)
		validHealth := map[string]bool{"HEALTHY": true, "DEGRADED": true, "UNAVAILABLE": true, "INVALID": true, "UNKNOWN": true}
		if !validHealth[status] {
			result.addError(fmt.Sprintf("health.status '%s' is not a valid enum", status))
		}
	}

	// Validate quality.score range (0-100)
	if quality, ok := data["quality"].(map[string]any); ok {
		if score, ok := quality["score"].(float64); ok {
			if score < 0 || score > 100 {
				result.addError(fmt.Sprintf("quality.score %v is out of range [0, 100]", score))
			}
		}
		grade, _ := quality["grade"].(string)
		validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}
		if !validGrades[grade] {
			result.addError(fmt.Sprintf("quality.grade '%s' is not a valid enum", grade))
		}
	}

	return result
}

func loadAndValidateFixture(t *testing.T, name string) validationResult {
	t.Helper()
	path, _ := filepath.Abs("../../tests/fixtures/schema/" + name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", name, err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse fixture %s: %v", name, err)
	}
	return validateMCPServer(parsed)
}

func TestValidRecordPasses(t *testing.T) {
	result := loadAndValidateFixture(t, "valid.json")
	if !result.valid {
		t.Errorf("Valid record should pass, got errors: %v", result.errors)
	}
}

func TestInvalidEnumFails(t *testing.T) {
	result := loadAndValidateFixture(t, "invalid.json")
	if result.valid {
		t.Error("Invalid enum record should fail validation")
	}
}

func TestMissingRequiredFails(t *testing.T) {
	result := loadAndValidateFixture(t, "missing-required.json")
	if result.valid {
		t.Error("Missing required fields should fail validation")
	}
}

func TestInvalidScoreFails(t *testing.T) {
	result := loadAndValidateFixture(t, "invalid-score.json")
	if result.valid {
		t.Error("Invalid quality score should fail validation")
	}
}
