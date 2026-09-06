package engines

import (
	"fmt"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestEndpointClassifier_DebugReadmeAnchor3(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
		RawContent: "https://github.com/owner/repo#readme",
	}
	results := ec.ClassifyEndpoints(entity)

	fmt.Printf("DEBUG: Results for readme anchor URL:\n")
	for _, r := range results {
		fmt.Printf("  URL: %s, Type: %s, Confidence: %.2f\n", r.Endpoint.URL, r.Type, r.Confidence)
		for _, e := range r.Evidence {
			fmt.Printf("    Evidence: Rule=%s, Conf=%.2f, Pattern=%s\n", e.Rule, e.Confidence, e.Pattern)
		}
	}
}
