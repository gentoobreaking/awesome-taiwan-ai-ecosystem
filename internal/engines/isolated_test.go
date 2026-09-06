package engines

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestEndpointClassifier_IsolatedDocumentationURL(t *testing.T) {
	ec := NewEndpointClassifier()

	testCases := []struct {
		name     string
		url      string
		expected models.EndpointType
		minConf  float64
	}{
		{
			name:     ".md file",
			url:      "https://example.com/README.md",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "GitHub wiki",
			url:      "https://github.com/owner/repo/wiki",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entity := &models.Entity{
				Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
				RawContent: tc.url,
			}
			results := ec.ClassifyEndpoints(entity)

			found := false
			for _, r := range results {
				if r.Endpoint.URL == tc.url {
					found = true
					if r.Type != tc.expected {
						t.Errorf("Expected %s, got %s", tc.expected, r.Type)
					}
					if r.Confidence < tc.minConf {
						t.Errorf("Expected confidence >= %.2f, got %.2f", tc.minConf, r.Confidence)
					}
					break
				}
			}
			if !found {
				t.Errorf("URL not found in results: %s", tc.url)
			}
		})
	}
}
