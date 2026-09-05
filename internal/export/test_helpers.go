package export

import (
	"testing"
)

// TestSeverityRank_Shared tests severity ranking
func TestSeverityRank_Shared(t *testing.T) {
	tests := []struct {
		sev      string
		expected int
	}{
		{"CRITICAL", 4},
		{"HIGH", 3},
		{"MEDIUM", 2},
		{"LOW", 1},
		{"UNKNOWN", 0},
	}

	for _, tt := range tests {
		t.Run(tt.sev, func(t *testing.T) {
			rank := severityRank(tt.sev)
			if rank != tt.expected {
				t.Errorf("severityRank(%s) = %d, want %d", tt.sev, rank, tt.expected)
			}
		})
	}
}

// TestTruncateForMarkdown_Shared tests markdown truncation
func TestTruncateForMarkdown_Shared(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exact length", 12, "exact length"},
		{"very long string that should be truncated", 10, "very long ..."},
		{"with|pipe", 20, "with\\|pipe"},
		{"with\nnewline", 20, "with newline"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := truncateForMarkdown(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateForMarkdown(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

// contains is a shared helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

// findSubstring is a shared helper
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}