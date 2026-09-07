// Package engines implements quality scoring for discovered entities.
// QualityEngine scores entities on a 100-point scale across 10 components,
// fully independent from Classification, MCP Identity, Taiwan/AI Relevance,
// and SecurityStatus (spec §45).
package engines

import (
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// QualityEngine computes a 100-point quality score for an Entity (§31).
// It does NOT read entity.Classification, entity.MCPIdentity,
// entity.TaiwanRelevance, entity.AIRelevance, or entity.SecurityStatus.
type QualityEngine struct {
	// lightweight security patterns for the Security quality component
	// (independent of the SecurityStatus field from T080/T081)
	criticalPatterns []CompiledPattern
}

// CompiledPattern holds a regex pattern with its metadata.
type CompiledPattern struct {
	Rule     string
	Regex    *regexp.Regexp
	Severity string
}

// NewQualityEngine creates a new quality engine.
func NewQualityEngine() *QualityEngine {
	return &QualityEngine{
		criticalPatterns: []CompiledPattern{
			{
				Rule:     "curl_pipe_bash",
				Regex:    regexp.MustCompile(`(?i)curl\s+[^|]*\|\s*(bash|sh)`),
				Severity: "CRITICAL",
			},
			{
				Rule:     "wget_pipe_sh",
				Regex:    regexp.MustCompile(`(?i)wget\s+[^|]*\|\s*(bash|sh)`),
				Severity: "CRITICAL",
			},
			{
				Rule:     "hardcoded_aws_key",
				Regex:    regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
				Severity: "CRITICAL",
			},
			{
				Rule:     "private_key_block",
				Regex:    regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
				Severity: "CRITICAL",
			},
		},
	}
}

// Score computes the quality score for an entity (spec §31, §TST-042, §TST-043).
// Does NOT depend on entity.Classification, entity.MCPIdentity,
// entity.TaiwanRelevance, entity.AIRelevance, or entity.SecurityStatus.
func (qe *QualityEngine) Score(entity *models.Entity) models.QualityScore {
	components := models.QualityComponents{}
	var evidence []models.Evidence

	// 1. Data Source (max 20)
	dsScore := qe.scoreDataSource(entity.DataSources)
	components.DataSource = dsScore
	if dsScore > 0 {
		evidence = append(evidence, models.Evidence{
			Type:       "data_source",
			Source:     "data_sources",
			Rule:       "data_source_score",
			Score:      float64(dsScore),
			Confidence: 1.0,
		})
	}

	// 2. Maintenance (max 15)
	maintScore := qe.scoreMaintenance(entity.Repository.PushedAt.Time())
	components.Maintenance = maintScore
	if maintScore > 0 {
		evidence = append(evidence, models.Evidence{
			Type:    "maintenance",
			Source:  "repository",
			Rule:    "maintenance_score",
			Score:   float64(maintScore),
		})
	}

	// 3. Documentation (max 10)
	docScore := qe.scoreDocumentation(entity.Description, entity.Tools)
	components.Documentation = docScore

	// 4. MCP Compliance (max 15)
	transports := qe.extractTransports(entity.Endpoints)
	mcpScore := qe.scoreMCPCompliance(transports)
	components.MCPCompliance = mcpScore

	// 5. Tool Schema (max 10)
	toolScore := qe.scoreToolSchema(entity.Tools)
	components.ToolSchema = toolScore

	// 6. Health (max 10)
	healthScore := qe.scoreHealth(entity)
	components.Health = healthScore

	// 7. Repository (max 5)
	repoScore := qe.scoreRepository(entity.Repository)
	components.Repository = repoScore

	// 8. License (max 5)
	licenseScore := qe.scoreLicense(entity.Repository.License)
	components.License = licenseScore

	// 9. Security (max 5) — independent of SecurityStatus, scans RawContent
	secScore := qe.scoreSecurity(entity.RawContent)
	components.Security = secScore

	// 10. Community (max 5)
	communityScore := qe.scoreCommunity(entity.Repository)
	components.Community = communityScore

	total := components.Total()
	if total > 100 {
		total = 100
	}

	return models.QualityScore{
		Score:      total,
		Grade:      models.GradeForScore(total),
		Components: components,
		Evidence:   evidence,
	}
}

// extractTransports extracts transport types from endpoints.
func (qe *QualityEngine) extractTransports(endpoints []models.EndpointWithType) []string {
	seen := make(map[string]bool)
	var transports []string
	for _, ep := range endpoints {
		t := ep.Endpoint.Transport
		if t == "" {
			// Infer from URL scheme
			url := ep.Endpoint.URL
			switch {
			case strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"):
				if ep.Endpoint.TLS {
					t = "sse"
				} else {
					t = "http"
				}
			default:
				t = "stdio"
			}
		}
		if !seen[t] {
			seen[t] = true
			transports = append(transports, t)
		}
	}
	return transports
}

// scoreDataSource scores based on data source types (max 20).
func (qe *QualityEngine) scoreDataSource(sources []models.DataSource) int {
	max := 0
	for _, ds := range sources {
		var v int
		switch ds.Type {
		case models.DataSourceOfficialGovAPI:
			v = 20
		case models.DataSourceOpenData:
			v = 18
		case models.DataSourceOfficialCompany:
			v = 15
		case models.DataSourceOfficial:
			v = 15
		case models.DataSourceThirdPartyAPI:
			v = 10
		case models.DataSourceCommunity:
			// Web scraping from community source
			v = 7
		default:
			continue
		}
		if v > max {
			max = v
		}
	}
	return max
}

// scoreMaintenance scores based on last commit date (max 15).
// Uses a reference time for determinism — if pushedAt is zero, returns 0.
func (qe *QualityEngine) scoreMaintenance(pushedAt time.Time) int {
	if pushedAt.IsZero() {
		return 0
	}
	// Use a fixed reference time for deterministic scoring.
	// The entity's LastSeen is a better anchor since it represents the
	// last time the crawler observed this entity.
	// We use time.Now() here as the reference for "recency from crawl time".
	// For truly deterministic scoring, call tests should use fixed dates.
	days := time.Since(pushedAt).Hours() / 24
	switch {
	case days < 90:
		return 15
	case days < 180:
		return 12
	case days < 365:
		return 8
	default:
		return 3
	}
}

// scoreDocumentation scores based on description and tool count (max 10).
func (qe *QualityEngine) scoreDocumentation(description string, tools []models.Tool) int {
	score := 0
	if len(description) > 200 {
		score += 5
	} else if len(description) > 50 {
		score += 3
	}
	if len(tools) > 0 {
		score += 3
	}
	if len(tools) >= 5 {
		score += 2
	}
	return score
}

// scoreMCPCompliance scores based on transport support (max 15).
func (qe *QualityEngine) scoreMCPCompliance(transports []string) int {
	score := 0
	hasAny := false
	for _, t := range transports {
		hasAny = true
		switch t {
		case "stdio":
			score += 3
		case "sse", "http":
			score += 3
		case "streamable-http":
			score += 2
		case "websocket":
			score += 2
		}
	}
	// Has manifest/config (any transport means there's config)
	if hasAny {
		score += 5
	}
	// Bonus for multiple transports
	if len(transports) >= 2 {
		score += 2
	}
	if score > 15 {
		score = 15
	}
	return score
}

// scoreToolSchema scores based on tool schema completeness (max 10).
func (qe *QualityEngine) scoreToolSchema(tools []models.Tool) int {
	score := 0
	if len(tools) > 0 {
		score += 3 // tools/list successful
	}
	for _, t := range tools {
		if t.Name != "" && t.Description != "" {
			score += 3 // at least 1 tool with name + description
			break
		}
	}
	for _, t := range tools {
		if len(t.InputSchema) > 0 {
			score += 2 // at least 1 tool with input schema
			break
		}
	}
	if len(tools) >= 5 {
		score += 2 // >= 5 tools total
	}
	if score > 10 {
		score = 10
	}
	return score
}

// scoreHealth scores based on endpoint health (max 10).
// Derives health from endpoints and repository info — does NOT read SecurityStatus.
func (qe *QualityEngine) scoreHealth(entity *models.Entity) int {
	if len(entity.Endpoints) > 0 {
		// Check for healthy-looking endpoints
		hasHTTPS := false
		for _, ep := range entity.Endpoints {
			if strings.HasPrefix(ep.Endpoint.URL, "https://") {
				hasHTTPS = true
			}
		}
		if hasHTTPS {
			return 10
		}
		// Has endpoints but no HTTPS — degraded
		return 5
	}
	// No endpoints — check if repository is accessible
	if entity.Repository.URL != "" {
		return 5
	}
	return 0
}

// scoreRepository scores based on repository info (max 5).
func (qe *QualityEngine) scoreRepository(repo models.RepositoryInfo) int {
	score := 0
	if repo.URL != "" {
		score += 3
	}
	if repo.Stars >= 100 {
		score += 1
	} else if repo.Stars >= 10 {
		score += 1
	}
	if repo.Forks >= 5 {
		score += 1
	}
	if score > 5 {
		score = 5
	}
	return score
}

// scoreLicense scores based on license presence and type (max 5).
func (qe *QualityEngine) scoreLicense(license string) int {
	if license == "" || license == "UNKNOWN" || license == "unknown" {
		return 0
	}
	score := 3 // license detected
	permissive := []string{"MIT", "Apache-2.0", "Apache 2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "Unlicense", "MIT-0"}
	for _, p := range permissive {
		if license == p {
			score += 2
			break
		}
	}
	return score
}

// scoreSecurity scores based on code quality — scans RawContent for
// critical patterns. This is independent of the SecurityStatus field
// (spec §45: do not combine quality with security status).
// Returns 5 for clean code, reduced for critical issues (max 5).
func (qe *QualityEngine) scoreSecurity(rawContent string) int {
	score := 5
	for _, p := range qe.criticalPatterns {
		if p.Regex.MatchString(rawContent) {
			switch p.Severity {
			case "CRITICAL":
				score = 0
				return score // critical always zeros security component
			case "HIGH":
				score -= 2
			}
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// scoreCommunity scores based on stars, forks, topics (max 5).
func (qe *QualityEngine) scoreCommunity(repo models.RepositoryInfo) int {
	score := 0
	if repo.Stars >= 10 {
		score++
	}
	if repo.Stars >= 50 {
		score++
	}
	if repo.Stars >= 100 {
		score++
	}
	if repo.Forks >= 5 {
		score++
	}
	if len(repo.Topics) > 0 {
		score++
	}
	if score > 5 {
		score = 5
	}
	return score
}

// computeConfidence aggregates per-evidence confidence into a single
// 0..1 score using a type-based weight (spec §4.4). Runtime and
// source-code rows carry more weight than rule-only rows. (T109)
func computeConfidence(evidences []models.ClassificationEvidence) float64 {
	if len(evidences) == 0 {
		return 0.0
	}
	var sum, weight float64
	for _, e := range evidences {
		w := 0.5
		switch e.Type {
		case "SOURCE_CODE":
			w = 3.0
		case "RUNTIME":
			w = 5.0
		case "PACKAGE_MANIFEST":
			w = 2.0
		case "README":
			w = 1.0
		}
		sum += e.Confidence * w
		weight += w
	}
	if weight == 0 {
		return 0
	}
	c := sum / weight
	if c > 1.0 {
		c = 1.0
	}
	return c
}
