package engines

import (
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/config"
	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// AIRelevanceEngine scores AI relevance independently from Taiwan relevance and MCP identity.
// Implements spec §7, §4.3, §45, §61 Phase 3.
type AIRelevanceEngine struct {
	signals *config.AISignals
}

// NewAIRelevanceEngine creates a new AI relevance engine.
func NewAIRelevanceEngine(signals *config.AISignals) *AIRelevanceEngine {
	if signals == nil {
		signals = config.DefaultAISignals()
	}
	return &AIRelevanceEngine{signals: signals}
}

// Score computes AI relevance for an entity.
// Does NOT depend on entity.Classification, entity.MCPIdentity, or entity.TaiwanRelevance (spec §4.3).
func (e *AIRelevanceEngine) Score(entity *models.Entity) models.AIRelevance {
	var evidence []models.Evidence
	score := 0.0

	// Check repository topics for AI keywords
	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)

		// Core AI keywords
		for _, kw := range e.signals.CoreAIKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 10
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "core_ai_keyword",
					Score:       10,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		// Agent keywords
		for _, kw := range e.signals.AgentKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "agent_keyword",
					Score:       15,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		// RAG keywords
		for _, kw := range e.signals.RAGKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "rag_keyword",
					Score:       15,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		// LLM Provider keywords
		for _, kw := range e.signals.LLMProviderKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 10
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "llm_provider_keyword",
					Score:       10,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		// MCP keywords
		for _, kw := range e.signals.MCPKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 20
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "mcp_keyword",
					Score:       20,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		// Framework keywords
		for _, kw := range e.signals.FrameworkKeywords {
			if strings.Contains(topicLower, strings.ToLower(kw)) {
				score += 25
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "repository_topics",
					Location:    entity.Repository.URL,
					Rule:        "framework_keyword",
					Score:       25,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}
	}

	// Check description for AI keywords
	if entity.Description != "" {
		descLower := strings.ToLower(entity.Description)
		for _, kw := range e.signals.CoreAIKeywords {
			if strings.Contains(descLower, strings.ToLower(kw)) {
				score += 5
				evidence = append(evidence, models.Evidence{
					Type:        "keyword",
					Source:      "description",
					Location:    entity.Repository.URL,
					Rule:        "ai_description_mention",
					Score:       5,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}
	}

	// Check package patterns in repository (topics, description)
	// This would ideally scan package.json, go.mod, pyproject.toml, Cargo.toml
	// For now, check topics and description for package patterns
	allText := entity.Description + " " + strings.Join(entity.Repository.Topics, " ")
	allTextLower := strings.ToLower(allText)
	for _, pattern := range e.signals.PackagePatterns {
		if strings.Contains(allTextLower, strings.ToLower(pattern)) {
			score += 10
			evidence = append(evidence, models.Evidence{
				Type:        "package_dependency",
				Source:      "repository_metadata",
				Location:    entity.Repository.URL,
				Rule:        "ai_package_pattern",
				Score:       10,
				Confidence:  1.0,
				MatchedText: pattern,
			})
		}
	}

	// Check data sources for AI-related types
	for _, ds := range entity.DataSources {
		dsName := strings.ToLower(ds.Name)
		dsURL := strings.ToLower(ds.URL)

		// Check if data source is AI-related
		for _, kw := range e.signals.CoreAIKeywords {
			if strings.Contains(dsName, strings.ToLower(kw)) || strings.Contains(dsURL, strings.ToLower(kw)) {
				score += 20
				evidence = append(evidence, models.Evidence{
					Type:        "ai_data_source",
					Source:      "data_source",
					Location:    ds.URL,
					Rule:        "ai_data_source",
					Score:       20,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}
	}

	// Check tools for AI functionality
	for _, tool := range entity.Tools {
		toolName := strings.ToLower(tool.Name)
		toolDesc := strings.ToLower(tool.Description)

		for _, kw := range e.signals.CoreAIKeywords {
			if strings.Contains(toolName, strings.ToLower(kw)) || strings.Contains(toolDesc, strings.ToLower(kw)) {
				score += 15
				evidence = append(evidence, models.Evidence{
					Type:        "ai_tool",
					Source:      "tool_definition",
					Location:    entity.Repository.URL,
					Rule:        "ai_tool",
					Score:       15,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}

		for _, kw := range e.signals.AgentKeywords {
			if strings.Contains(toolName, strings.ToLower(kw)) || strings.Contains(toolDesc, strings.ToLower(kw)) {
				score += 25
				evidence = append(evidence, models.Evidence{
					Type:        "ai_tool",
					Source:      "tool_definition",
					Location:    entity.Repository.URL,
					Rule:        "agent_tool",
					Score:       25,
					Confidence:  1.0,
					MatchedText: kw,
				})
			}
		}
	}

	level := models.ScoreToAILevel(score)

	// Confidence is 1.0 for deterministic rules
	confidence := 1.0
	if len(evidence) == 0 {
		confidence = 0.0
	}

	return models.AIRelevance{
		Score:      score,
		Level:      level,
		Evidence:   evidence,
		Confidence: confidence,
	}
}