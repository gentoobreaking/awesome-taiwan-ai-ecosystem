package engines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Ground truth fixtures for FP rate testing (spec §57, §58, §64).
// Positive samples: verified MCP servers (source code present).
// Negative samples: tutorials, clients, collections, SDKs, data libraries, AI agents.
//
// Each fixture is a struct that gets converted to a models.Entity for classification.

type GroundTruthFixture struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	RawContent  string                 `json:"raw_content"`
	Topics      []string               `json:"topics"`
	PackageFiles map[string]string      `json:"package_files,omitempty"`
	Tools       []string                `json:"tools,omitempty"`
	IsMCPServer bool                   `json:"is_mcp_server"`
	Category    string                 `json:"category"`
}

// loadGroundTruthFixtures reads all JSON fixtures from tests/fixtures/ground_truth/.
func loadGroundTruthFixtures(t *testing.T) []GroundTruthFixture {
	t.Helper()
	var fixtures []GroundTruthFixture
	for _, dir := range []string{"positive", "negative"} {
		fixtureDir := filepath.Join("../../tests/fixtures/ground_truth", dir)
		entries, err := os.ReadDir(fixtureDir)
		if err != nil {
			t.Fatalf("Failed to read ground truth directory %s: %v", fixtureDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(fixtureDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read fixture %s: %v", path, err)
			}
			var f GroundTruthFixture
			if err := json.Unmarshal(data, &f); err != nil {
				t.Fatalf("Failed to parse fixture %s: %v", path, err)
			}
			fixtures = append(fixtures, f)
		}
	}
	if len(fixtures) < 50 {
		t.Fatalf("Expected at least 50 ground truth fixtures, got %d", len(fixtures))
	}
	return fixtures
}

// toEntity converts a GroundTruthFixture to a models.Entity for classification.
func (f GroundTruthFixture) toEntity() *models.Entity {
	e := &models.Entity{
		ID:          f.ID,
		Name:        f.Name,
		Slug:        f.ID,
		Description: f.Description,
		RawContent:  f.RawContent,
		Repository: models.RepositoryInfo{
			URL:    fmt.Sprintf("https://github.com/test/%s", f.ID),
			Host:   "github.com",
			Owner:  "test",
			Name:   f.ID,
			Topics: f.Topics,
		},
	}
	if f.PackageFiles != nil {
		e.Repository.PackageFiles = f.PackageFiles
	}
	if len(f.Tools) > 0 {
		e.Tools = make([]models.Tool, 0, len(f.Tools))
		for _, name := range f.Tools {
			e.Tools = append(e.Tools, models.Tool{
				Name:        name,
				Description: name,
			})
		}
	}
	e.MCPIdentity = models.MCPIdentity{Status: models.MCPIdentityStatusCandidate}
	e.AIRelevance = models.AIRelevance{Level: "A3", Score: 60}
	e.TaiwanRelevance = models.TaiwanRelevance{Level: "T2", Score: 30}
	return e
}

// FPRReport holds false positive rate test results.
type FPRReport struct {
	TotalSamples int             `json:"total_samples"`
	TP           int             `json:"true_positives"`
	FP           int             `json:"false_positives"`
	TN           int             `json:"true_negatives"`
	FN           int             `json:"false_negatives"`
	FPR          float64         `json:"false_positive_rate"`
	Precision    float64         `json:"precision"`
	Recall       float64         `json:"recall"`
	F1           float64         `json:"f1_score"`
	Status       string          `json:"status"`
	Thresholds   FPRThresholds   `json:"thresholds"`
	Timestamp    string          `json:"timestamp"`
	Misclassified []Misclassified `json:"misclassified,omitempty"`
}

// FPRThresholds defines FPR threshold values.
type FPRThresholds struct {
	PassThreshold float64 `json:"pass_threshold"`
	ExcellentThreshold float64 `json:"excellent_threshold"`
}

// Misclassified records a sample that was misclassified.
type Misclassified struct {
	ID             string `json:"id"`
	ExpectedMCPServer bool   `json:"expected_mcp_server"`
	PredictedMCPServer bool   `json:"predicted_mcp_server"`
	Category       string `json:"category"`
}

// TestFPRate tests the MCP false positive rate against ground truth fixtures.
// Spec §58: FPR < 5% is PASS, < 2% is EXCELLENT.
func TestFPRate(t *testing.T) {
	fixtures := loadGroundTruthFixtures(t)

	classifier := NewClassifier()
	var report FPRReport
	report.Thresholds = FPRThresholds{
		PassThreshold:      0.05,
		ExcellentThreshold: 0.02,
	}
	report.TotalSamples = len(fixtures)
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)

	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].ID < fixtures[j].ID
	})

	for _, f := range fixtures {
		entity := f.toEntity()
		result := classifier.Classify(entity)
		predicted := result.Primary == models.PrimaryClassificationMCPServer

		if f.IsMCPServer && predicted {
			report.TP++
		} else if !f.IsMCPServer && predicted {
			report.FP++
			report.Misclassified = append(report.Misclassified, Misclassified{
				ID:               f.ID,
				ExpectedMCPServer: false,
				PredictedMCPServer: true,
				Category:           f.Category,
			})
		} else if f.IsMCPServer && !predicted {
			report.FN++
			report.Misclassified = append(report.Misclassified, Misclassified{
				ID:               f.ID,
				ExpectedMCPServer: true,
				PredictedMCPServer: false,
				Category:           f.Category,
			})
		} else {
			report.TN++
		}
	}

	// Calculate metrics
	if report.TP+report.FP > 0 {
		report.Precision = float64(report.TP) / float64(report.TP+report.FP)
	}
	if report.TP+report.FN > 0 {
		report.Recall = float64(report.TP) / float64(report.TP+report.FN)
	}
	if report.FP > 0 {
		report.FPR = float64(report.FP) / float64(report.TP+report.FP)
	}
	if report.Precision+report.Recall > 0 {
		report.F1 = 2 * report.Precision * report.Recall / (report.Precision + report.Recall)
	}

	if report.FPR < report.Thresholds.ExcellentThreshold {
		report.Status = "EXCELLENT"
	} else if report.FPR < report.Thresholds.PassThreshold {
		report.Status = "PASS"
	} else {
		report.Status = "FAIL"
	}

	// Output report
	t.Logf("FPR Report: Total=%d, TP=%d, FP=%d, TN=%d, FN=%d",
		report.TotalSamples, report.TP, report.FP, report.TN, report.FN)
	t.Logf("FPR=%.4f, Precision=%.4f, Recall=%.4f, F1=%.4f, Status=%s",
		report.FPR, report.Precision, report.Recall, report.F1, report.Status)
	if len(report.Misclassified) > 0 {
		t.Logf("Misclassified samples:")
		for _, m := range report.Misclassified {
			t.Logf("  - %s (category=%s, expected_mcp=%v, predicted_mcp=%v)",
				m.ID, m.Category, m.ExpectedMCPServer, m.PredictedMCPServer)
		}
	}

	// Write JSON report
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal report: %v", err)
	}
	reportPath := filepath.Join("../../tests/fixtures/ground_truth", "fp_rate_report.json")
	if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
		t.Logf("Warning: failed to write report to %s: %v", reportPath, err)
	} else {
		t.Logf("Report written to %s", reportPath)
	}

	// Historical tracking: append FPR to history file
	historyPath := filepath.Join("../../tests/fixtures/ground_truth", "fp_rate_history.json")
	historyEntry := struct {
		Timestamp string  `json:"timestamp"`
		FPR       float64 `json:"fpr"`
		Precision float64 `json:"precision"`
		Recall    float64 `json:"recall"`
		F1        float64 `json:"f1"`
		Status    string  `json:"status"`
	}{
		Timestamp: report.Timestamp,
		FPR:       report.FPR,
		Precision: report.Precision,
		Recall:    report.Recall,
		F1:        report.F1,
		Status:    report.Status,
	}
	var history []interface{}
	if histData, err := os.ReadFile(historyPath); err == nil {
		json.Unmarshal(histData, &history)
	}
	history = append(history, historyEntry)
	historyJSON, _ := json.MarshalIndent(history, "", "  ")
	if err := os.WriteFile(historyPath, historyJSON, 0644); err != nil {
		t.Logf("Warning: failed to write history to %s: %v", historyPath, err)
	}

	// Threshold check
	if report.FPR >= report.Thresholds.PassThreshold {
		t.Errorf("FPR %.4f >= %.4f threshold (PASS). %d false positives detected. Status: %s",
			report.FPR, report.Thresholds.PassThreshold, report.FP, report.Status)
	}
}
