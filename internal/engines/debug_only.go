package engines

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func (ec *EndpointClassifier) DebugClassifySingleURL(rawURL string, entity *models.Entity, source string) (models.EndpointType, []models.EndpointEvidence, float64) {
	var normalized string
	u, err := url.Parse(rawURL)
	if err != nil {
		normalized = strings.ToLower(rawURL)
	} else {
		normalized = strings.ToLower(u.String())
	}

	var evidence []models.EndpointEvidence
	now := models.RFC3339Time(time.Now().UTC())

	fmt.Printf("DEBUG classifySingleURL: rawURL=%s, normalized=%s, source=%s\n", rawURL, normalized, source)

	// Check DOCUMENTATION_URL patterns FIRST (more specific)
	for _, pattern := range ec.docPatterns {
		if pattern.MatchString(normalized) {
			fmt.Printf("DEBUG: DOC MATCH: %s\n", pattern.String())
			evidence = append(evidence, models.EndpointEvidence{
				Rule:        "documentation_url_pattern",
				Source:      source,
				Location:    normalized,
				MatchedText: pattern.String(),
				Pattern:     pattern.String(),
				Confidence:  0.85,
				Timestamp:   now,
			})
			return models.EndpointTypeDocumentation, evidence, 0.85
		}
	}

	// Check REPOSITORY_URL patterns
	for _, pattern := range ec.repoPatterns {
		if pattern.MatchString(normalized) {
			fmt.Printf("DEBUG: REPO MATCH: %s\n", pattern.String())
			evidence = append(evidence, models.EndpointEvidence{
				Rule:        "repository_url_pattern",
				Source:      source,
				Location:    normalized,
				MatchedText: pattern.String(),
				Pattern:     pattern.String(),
				Confidence:  0.95,
				Timestamp:   now,
			})
			return models.EndpointTypeRepositoryURL, evidence, 0.95
		}
	}

	fmt.Printf("DEBUG: NO MATCH - UNKNOWN\n")
	evidence = append(evidence, models.EndpointEvidence{
		Rule:        "unknown",
		Source:      source,
		Location:    normalized,
		MatchedText: rawURL,
		Pattern:     "no_match",
		Confidence:  0.1,
		Timestamp:   now,
	})
	return models.EndpointTypeUnknown, evidence, 0.1
}
