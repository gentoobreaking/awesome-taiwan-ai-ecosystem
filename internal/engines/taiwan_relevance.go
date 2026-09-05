package engines

import (
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/config"
	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// TaiwanRelevanceEngine scores Taiwan relevance independently from MCP identity and AI relevance.
// Implements spec §17, §4.3, §45, algs/taiwan-classification.md.
type TaiwanRelevanceEngine struct {
	signals *config.TaiwanSignals
}

// NewTaiwanRelevanceEngine creates a new Taiwan relevance engine with default signals.
func NewTaiwanRelevanceEngine() *TaiwanRelevanceEngine {
	signals := config.DefaultTaiwanSignals()
	return &TaiwanRelevanceEngine{signals: signals}
}

// NewTaiwanRelevanceEngineWithSignals creates a new engine with custom signals (for testing).
func NewTaiwanRelevanceEngineWithSignals(signals *config.TaiwanSignals) *TaiwanRelevanceEngine {
	return &TaiwanRelevanceEngine{signals: signals}
}

// Score computes Taiwan relevance for an entity.
// Does NOT depend on entity.Classification, entity.MCPIdentity, or entity.AIRelevance (spec §4.3).
func (e *TaiwanRelevanceEngine) Score(entity *models.Entity) models.TaiwanRelevance {
	var evidence []models.Evidence
	score := 0.0
	now := models.RFC3339Time{}

	// Check repository URL and homepage for official Taiwan domains
	if entity.Repository.URL != "" {
		for _, domain := range e.signals.OfficialDomains {
			if strings.Contains(strings.ToLower(entity.Repository.URL), strings.ToLower(domain)) {
				score += 40
				evidence = append(evidence, models.Evidence{
					Type:        "official_domain",
					Source:      "repository_metadata",
					Location:    entity.Repository.URL,
					Rule:        "official_taiwan_domain",
					Score:       40,
					Confidence:  1.0,
					MatchedText: domain,
					Timestamp:   now,
				})
			}
		}
	}

	// Check homepage
	if entity.Repository.Homepage != "" {
		for _, domain := range e.signals.OfficialDomains {
			if strings.Contains(strings.ToLower(entity.Repository.Homepage), strings.ToLower(domain)) {
				score += 40
				evidence = append(evidence, models.Evidence{
					Type:        "official_domain",
					Source:      "repository_metadata",
					Location:    entity.Repository.Homepage,
					Rule:        "official_taiwan_domain",
					Score:       40,
					Confidence:  1.0,
					MatchedText: domain,
					Timestamp:   now,
				})
			}
		}
	}

	// Check endpoints for official domains (only MCP_RUNTIME_ENDPOINT)
	for _, ep := range entity.Endpoints {
		if ep.Type == models.EndpointTypeMCPRuntime {
			for _, domain := range e.signals.OfficialDomains {
				if strings.Contains(strings.ToLower(ep.Endpoint.URL), strings.ToLower(domain)) {
					score += 40
					evidence = append(evidence, models.Evidence{
						Type:        "official_domain",
						Source:      "runtime_endpoint",
						Location:    ep.Endpoint.URL,
						Rule:        "official_taiwan_domain",
						Score:       40,
						Confidence:  1.0,
						MatchedText: domain,
						Timestamp:   now,
					})
				}
			}
		}
	}

	// Check data sources for Taiwan government APIs
	for _, ds := range entity.DataSources {
		dsName := strings.ToLower(ds.Name)
		dsURL := strings.ToLower(ds.URL)

		// Government API
		for _, agency := range e.signals.GovernmentAgencies {
			if strings.Contains(dsName, strings.ToLower(agency)) || strings.Contains(dsURL, strings.ToLower(agency)) {
				score += 40
				evidence = append(evidence, models.Evidence{
					Type:        "government_api",
					Source:      "data_source",
					Location:    ds.URL,
					Rule:        "taiwan_government_api",
					Score:       40,
					Confidence:  1.0,
					MatchedText: agency,
					Timestamp:   now,
				})
			}
		}

		// Financial API
		for _, kw := range e.signals.FinancialKeywords {
			if strings.Contains(dsName, strings.ToLower(kw)) || strings.Contains(dsURL, strings.ToLower(kw)) {
				score += 35
				evidence = append(evidence, models.Evidence{
					Type:        "financial_api",
					Source:      "data_source",
					Location:    ds.URL,
					Rule:        "taiwan_financial_api",
					Score:       35,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}

		// Taiwan-specific dataset
		for _, kw := range e.signals.DatasetKeywords {
			if strings.Contains(dsName, strings.ToLower(kw)) || strings.Contains(dsURL, strings.ToLower(kw)) {
				score += 30
				evidence = append(evidence, models.Evidence{
					Type:        "dataset",
					Source:      "data_source",
					Location:    ds.URL,
					Rule:        "taiwan_dataset",
					Score:       30,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}
	}

	// Check repository topics for Taiwan keywords
	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, kw := range e.signals.TaiwanKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 20
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "taiwan_keyword",
					Score:       20,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}
	}

	// Check for Taiwan language indicators in repository
	if entity.Repository.Language != "" {
		langLower := strings.ToLower(entity.Repository.Language)
		for _, kw := range e.signals.LanguageKeywords {
			if strings.Contains(langLower, strings.ToLower(kw)) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "language",
					Source:      "repository_language",
					Location:    entity.Repository.URL,
					Rule:        "taiwan_language",
					Score:       15,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}
	}

	// Check for Taiwan company/service keywords
	for _, kw := range e.signals.CompanyKeywords {
		kwLower := strings.ToLower(kw)
		// Check repository name
		if strings.Contains(strings.ToLower(entity.Repository.Name), kwLower) {
			score += 15
			evidence = append(evidence, models.Evidence{
				Type:        "company",
				Source:      "repository_name",
				Location:    entity.Repository.URL,
				Rule:        "taiwan_company",
				Score:       15,
				Confidence:  1.0,
				MatchedText: kw,
				Timestamp:   now,
			})
		}
		// Check topics
		for _, topic := range entity.Repository.Topics {
			if strings.Contains(strings.ToLower(topic), kwLower) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "company",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "taiwan_company",
					Score:       15,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}
	}

	// Check description for Taiwan mention
	if entity.Description != "" {
		descLower := strings.ToLower(entity.Description)
		for _, kw := range e.signals.TaiwanKeywords {
			if strings.Contains(descLower, strings.ToLower(kw)) {
				score += 5
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "description",
					Location:    entity.Repository.URL,
					Rule:        "taiwan_readme_mention",
					Score:       5,
					Confidence:  1.0,
					MatchedText: kw,
					Timestamp:   now,
				})
			}
		}
	}

	level := models.ScoreToTaiwanLevel(score)

	// Confidence is 1.0 for deterministic rules
	confidence := 1.0
	if len(evidence) == 0 {
		confidence = 0.0
	}

	return models.TaiwanRelevance{
		Score:      score,
		Level:      level,
		Evidence:   evidence,
		Confidence: confidence,
	}
}