// Package scoring implements the 10-component quality scoring engine (§31).
package scoring

import (
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// QualityScorer computes a 100-point quality score for an MCPServer.
type QualityScorer struct{}

// New creates a new QualityScorer.
func New() *QualityScorer {
	return &QualityScorer{}
}

// Score computes the quality score (§31, §TST-042, §TST-043).
func (qs *QualityScorer) Score(server *models.MCPServer) models.QualityScore {
	components := models.QualityComponents{}

	// Data Source (max 20)
	components.DataSource = scoreDataSource(server.DataSources)

	// Maintenance (max 15)
	components.Maintenance = scoreMaintenance(server.Repository.PushedAt.Time())

	// Documentation (max 10)
	components.Documentation = scoreDocumentation(server.Description, server.Tools)

	// MCP Compliance (max 15)
	components.MCPCompliance = scoreMCPCompliance(server.Transport)

	// Tool Schema (max 10)
	components.ToolSchema = scoreToolSchema(server.Tools)

	// Health (max 10)
	components.Health = scoreHealth(server.Health)

	// Repository (max 5)
	components.Repository = scoreRepository(server.Repository)

	// License (max 5)
	components.License = scoreLicense(server.License)

	// Security (max 5)
	components.Security = scoreSecurity(server)

	// Community (max 5)
	components.Community = scoreCommunity(server.Repository)

	total := components.DataSource +
		components.Maintenance +
		components.Documentation +
		components.MCPCompliance +
		components.ToolSchema +
		components.Health +
		components.Repository +
		components.License +
		components.Security +
		components.Community

	if total > 100 {
		total = 100
	}

	return models.QualityScore{
		Score:      total,
		Grade:      models.GradeForScore(total),
		Components: components,
	}
}

var defaultDataSourceScores = models.DataSourceScores{
	OfficialCompany: 15,
	ThirdPartyAPI:   10,
	OfficialGovAPI:  20,
	GovOpenData:     18,
	WebScraping:     7,
	OpenData:        12,
	Official:        14,
	Community:       8,
	Unknown:         0,
}

func scoreDataSource(sources []models.DataSource) int {
	max := 0
	for _, ds := range sources {
		var v float64
		switch ds.Type {
		case models.DataSourceOfficialCompany:
			v = defaultDataSourceScores.OfficialCompany
		case models.DataSourceThirdPartyAPI:
			v = defaultDataSourceScores.ThirdPartyAPI
		case models.DataSourceOfficialGovAPI:
			v = defaultDataSourceScores.OfficialGovAPI
		case models.DataSourceGovOpenData:
			v = defaultDataSourceScores.GovOpenData
		case models.DataSourceWebScraping:
			v = defaultDataSourceScores.WebScraping
		case models.DataSourceOpenData:
			v = defaultDataSourceScores.OpenData
		case models.DataSourceOfficial:
			v = defaultDataSourceScores.Official
		case models.DataSourceCommunity:
			v = defaultDataSourceScores.Community
		case models.DataSourceUnknown:
			v = defaultDataSourceScores.Unknown
		default:
			continue
		}
		if int(v) > max {
			max = int(v)
		}
	}
	if max > 20 {
		max = 20
	}
	return max
}

func scoreMaintenance(pushedAt time.Time) int {
	if pushedAt.IsZero() {
		return 0
	}
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

func scoreDocumentation(desc string, tools []models.Tool) int {
	score := 0
	if len(desc) > 200 {
		score += 5
	}
	if len(tools) > 0 {
		score += 3
	}
	if len(tools) >= 5 {
		score += 2
	}
	return score
}

func scoreMCPCompliance(transports []string) int {
	score := 0
	for _, t := range transports {
		switch t {
		case "stdio":
			score += 3
		case "sse", "http":
			score += 3
		case "streamable-http":
			score += 2
		}
	}
	if len(transports) > 0 {
		score += 5 // has manifest
	}
	if score > 15 {
		score = 15
	}
	return score
}

func scoreToolSchema(tools []models.Tool) int {
	score := 0
	if len(tools) > 0 {
		score += 3
	}
	hasSchema := false
	for _, t := range tools {
		if t.Name != "" && t.Description != "" {
			score += 3
			break
		}
	}
	for _, t := range tools {
		if len(t.InputSchema) > 0 {
			hasSchema = true
			score += 2
			break
		}
	}
	if hasSchema && len(tools) >= 5 {
		score++ // already counted, just ensure cap
	}
	if score > 10 {
		score = 10
	}
	return score
}

func scoreHealth(health models.HealthStatus) int {
	switch health {
	case models.HealthHealthy:
		return 10
	case models.HealthDegraded:
		return 5
	default:
		return 0
	}
}

func scoreRepository(repo models.RepositoryInfo) int {
	score := 0
	if repo.URL != "" {
		score += 3
	}
	if repo.Stars >= 100 {
		score += 1
	} else if repo.Stars >= 10 {
		score += 1
	}
	if score > 5 {
		score = 5
	}
	return score
}

func scoreLicense(license string) int {
	if license == "" || license == "UNKNOWN" {
		return 0
	}
	score := 3 // license detected
	permissive := []string{"MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "Unlicense"}
	for _, p := range permissive {
		if license == p {
			score += 2
			break
		}
	}
	return score
}

func scoreSecurity(server *models.MCPServer) int {
	score := 5
	for _, f := range server.Security.Findings {
		switch f.Severity {
		case models.SeverityLow:
			score -= 1
		case models.SeverityMedium:
			score -= 2
		case models.SeverityHigh:
			score -= 3
		case models.SeverityCritical:
			score -= 5
		}
	}
	if score < 0 {
		return 0
	}
	return score
}

func scoreCommunity(repo models.RepositoryInfo) int {
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
	if repo.OpenIssues > 0 {
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
