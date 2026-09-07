// TestAcceptance_FPRateOnGroundTruth (T107) — integration test that
// runs the full classifier pipeline against the 150 ground-truth
// fixtures and asserts the false positive rate is below the spec
// §58 KPI of 5%.
//
// The ground truth lives in tests/fixtures/ground_truth/{positive,negative}/
// and the existing TestFPRate in fp_rate_test.go is the same machinery;
// this test simply wraps it with explicit acceptance-test semantics
// and a clearer pass/fail message for CI logs.
package engines

import (
	"testing"
)

func TestAcceptance_FPRateOnGroundTruth(t *testing.T) {
	// Run the existing FP rate test. If it fails, this acceptance
	// test fails too with the same evidence.
	t.Run("delegates_to_TestFPRate", func(t *testing.T) {
		// Invoking TestFPRate directly keeps the test runner's
		// failure reporting intact.
		ok := t.Run("TestFPRate_inner", func(t *testing.T) {
			// Reuse the body of TestFPRate by calling the package-level
			// helper. We duplicate the call rather than reach into a
			// private symbol because go test treats subtests of an
			// inner func as opaque.
			TestFPRate(t)
		})
		if !ok {
			t.Fatal("FPR >= 5% threshold (spec §58 KPI). See TestFPRate output above for the misclassified list.")
		}
	})
}
