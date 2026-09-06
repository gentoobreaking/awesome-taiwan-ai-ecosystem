package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// ViewConfig controls how views are generated.
type ViewConfig struct {
	SchemaVersion  string
	CrawlerVersion string
	GeneratedAt    time.Time
}

// viewFilter is a predicate that selects which entities belong in a view.
type viewFilter func(e *models.Entity) bool

// RegistryView represents a single generated registry view.
type RegistryView struct {
	Name        string
	Description string
	Filter      viewFilter
	Categories  []viewCategory // for markdown grouping
}

// viewCategory represents a markdown grouping within a view.
type viewCategory struct {
	Key       string
	Title     string
	Filter    viewFilter
	SortBy    string // "quality_score" or "name"
}

// isTaiwanRelevantT1Plus checks if a TaiwanRelevance level is >= T1.
// T0 is excluded; T1, T2, T3, T4, T5 are included.
func isTaiwanRelevantT1Plus(level models.TaiwanRelevanceLevel) bool {
	return string(level) >= "T1" && string(level) <= "T5"
}

// isNotBlocked checks if the entity's security status allows registry inclusion.
func isNotBlocked(e *models.Entity) bool {
	return e.SecurityStatus.Status != models.SecurityStatusBlocked
}

// isVerifiedMCPServer checks the verified MCP server criteria per spec §44, §54.
func isVerifiedMCPServer(e *models.Entity) bool {
	return e.Classification.Primary == models.PrimaryClassificationMCPServer &&
		e.MCPIdentity.Status == models.MCPIdentityStatusRuntimeVerified &&
		isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level) &&
		isNotBlocked(e)
}

// isMCPCandidate checks the MCP candidate criteria per spec §44.
func isMCPCandidate(e *models.Entity) bool {
	if e.Classification.Primary != models.PrimaryClassificationMCPServer {
		return false
	}
	return e.MCPIdentity.Status == models.MCPIdentityStatusCandidate ||
		e.MCPIdentity.Status == models.MCPIdentityStatusStaticVerified
}

// isAIAgent checks if the entity is an AI agent.
func isAIAgent(e *models.Entity) bool {
	return e.Classification.Primary == models.PrimaryClassificationAIAgent &&
		isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level)
}

// isAITool checks if the entity is an AI tool/sdk/framework/plugin.
func isAITool(e *models.Entity) bool {
	p := e.Classification.Primary
	return (p == models.PrimaryClassificationAITool ||
		p == models.PrimaryClassificationAISDK ||
		p == models.PrimaryClassificationAIFramework ||
		p == models.PrimaryClassificationAIPlugin) &&
		isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level)
}

// isAIData checks if the entity is a data source.
func isAIData(e *models.Entity) bool {
	p := e.Classification.Primary
	return (p == models.PrimaryClassificationAIDataset ||
		p == models.PrimaryClassificationAIKnowledgeBase ||
		p == models.PrimaryClassificationAIAPI) &&
		isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level)
}

// isAISkill checks if the entity is an AI/MCP skill.
func isAISkill(e *models.Entity) bool {
	p := e.Classification.Primary
	return p == models.PrimaryClassificationAISkill || p == models.PrimaryClassificationMCPSkill
}

// isAIInfrastructure checks if the entity is AI infrastructure.
func isAIInfrastructure(e *models.Entity) bool {
	return e.Classification.Primary == models.PrimaryClassificationAIInfrastructure
}

// isAITutorial checks if the entity is a tutorial/example.
func isAITutorial(e *models.Entity) bool {
	p := e.Classification.Primary
	return p == models.PrimaryClassificationAITutorial || p == models.PrimaryClassificationAIExample
}

// isAICollection checks if the entity is a collection/registry.
func isAICollection(e *models.Entity) bool {
	p := e.Classification.Primary
	return p == models.PrimaryClassificationAICollection ||
		p == models.PrimaryClassificationAIRegistry ||
		p == models.PrimaryClassificationMCPCollection
}

// isTaiwanAIEntity checks if the entity is a Taiwan AI ecosystem entity (T1+).
func isTaiwanAIEntity(e *models.Entity) bool {
	return isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level)
}

// isMCPRelated checks if the entity is MCP-related.
func isMCPRelated(e *models.Entity) bool {
	return models.IsMCPRelated(e.Classification.Primary)
}

// isAIRelated checks if the entity is AI-related.
func isAIRelated(e *models.Entity) bool {
	return models.IsAIRelated(e.Classification.Primary)
}

// Define all views per T083 spec.
var allViews = []RegistryView{
	{
		Name:        "taiwan-ai-ecosystem",
		Description: "All Taiwan AI ecosystem entities (T1+)",
		Filter:      isTaiwanAIEntity,
		Categories: []viewCategory{
			{Key: "mcp", Title: "MCP Servers & Tools", Filter: isMCPRelated, SortBy: "quality_score"},
			{Key: "ai", Title: "AI Agents & Models", Filter: isAIRelated, SortBy: "quality_score"},
			{Key: "data", Title: "Data & Datasets", Filter: isAIData, SortBy: "quality_score"},
			{Key: "other", Title: "Other", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-mcp",
		Description: "Verified MCP Servers (Runtime Verified, T1+, not blocked)",
		Filter:      isVerifiedMCPServer,
		Categories: []viewCategory{
			{Key: "verified", Title: "Verified MCP Servers", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-mcp-candidates",
		Description: "MCP Server Candidates (Candidate, Static Verified)",
		Filter:      isMCPCandidate,
		Categories: []viewCategory{
			{Key: "candidates", Title: "MCP Server Candidates", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-agents",
		Description: "Taiwan AI Agents (T1+)",
		Filter:      isAIAgent,
		Categories: []viewCategory{
			{Key: "agents", Title: "AI Agents", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-tools",
		Description: "Taiwan AI Tools, SDKs, Frameworks, Plugins (T1+)",
		Filter:      isAITool,
		Categories: []viewCategory{
			{Key: "tools", Title: "AI Tools", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-data",
		Description: "Taiwan AI Datasets, Data Libraries, APIs (T1+)",
		Filter:      isAIData,
		Categories: []viewCategory{
			{Key: "data", Title: "AI Data Sources", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-skills",
		Description: "AI Skills and MCP Skills",
		Filter:      isAISkill,
		Categories: []viewCategory{
			{Key: "skills", Title: "AI Skills", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-infrastructure",
		Description: "AI Infrastructure entities",
		Filter:      isAIInfrastructure,
		Categories: []viewCategory{
			{Key: "infra", Title: "Infrastructure", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-tutorials",
		Description: "AI Tutorials and Examples",
		Filter:      isAITutorial,
		Categories: []viewCategory{
			{Key: "tutorials", Title: "Tutorials", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-collections",
		Description: "AI Collections, Registries, MCP Collections",
		Filter:      isAICollection,
		Categories: []viewCategory{
			{Key: "collections", Title: "Collections", Filter: nil, SortBy: "quality_score"},
		},
	},
}

// ViewGenerator generates registry views from Entity data.
type ViewGenerator struct {
	config ViewConfig
}

// NewViewGenerator creates a ViewGenerator with the given config.
func NewViewGenerator(config ViewConfig) *ViewGenerator {
	return &ViewGenerator{config: config}
}

// GenerateViews generates all registry views (Markdown + JSON) into outputDir.
// Each view produces two files: <name>.md and <name>.json.
func (vg *ViewGenerator) GenerateViews(entities []*models.Entity, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for _, view := range allViews {
		entitiesInView := vg.filterView(entities, &view)

		// Generate JSON
		jsonPath := filepath.Join(outputDir, view.Name+".json")
		if err := vg.generateJSON(jsonPath, view, entitiesInView); err != nil {
			return fmt.Errorf("generate JSON for %s: %w", view.Name, err)
		}

		// Generate Markdown
		mdPath := filepath.Join(outputDir, view.Name+".md")
		if err := vg.generateMarkdown(mdPath, view, entitiesInView); err != nil {
			return fmt.Errorf("generate Markdown for %s: %w", view.Name, err)
		}
	}

	return nil
}

// generateJSON produces a JSON registry view file.
func (vg *ViewGenerator) generateJSON(path string, view RegistryView, entities []*models.Entity) error {
	registry := map[string]any{
		"schema_version":  vg.config.SchemaVersion,
		"view_name":       view.Name,
		"description":     view.Description,
		"generated_at":    vg.config.GeneratedAt.UTC().Format(time.RFC3339),
		"crawler_version": vg.config.CrawlerVersion,
		"total_entities":  len(entities),
		"entities":        entities,
	}
	return writeJSON(path, registry)
}

// generateMarkdown produces a Markdown registry view file.
func (vg *ViewGenerator) generateMarkdown(path string, view RegistryView, entities []*models.Entity) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", view.Name))
	sb.WriteString(fmt.Sprintf("> %s\n\n", view.Description))
	sb.WriteString(fmt.Sprintf("- Generated: %s\n", vg.config.GeneratedAt.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- Total entities: %d\n\n", len(entities)))

	// Group entities by category
	categories := vg.groupByCategory(entities, view)

	for _, cat := range view.Categories {
		if len(cat.Title) == 0 {
			continue
		}
		catEntities := categories[cat.Key]
		if len(catEntities) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", cat.Title, len(catEntities)))

		for i, e := range catEntities {
			sb.WriteString(fmt.Sprintf("%d. [%s](%s) — %s\n", i+1, e.Name, repoURL(e), e.Description))
			if e.Quality.Score > 0 {
				grade := models.GradeForScore(e.Quality.Score)
				sb.WriteString(fmt.Sprintf("   - Quality: %d (%s) | Taiwan: %s | MCP: %s\n",
					e.Quality.Score, grade, e.TaiwanRelevance.Level, e.MCPIdentity.Status))
			}
			var transports []string
			for _, ep := range e.Endpoints {
				if models.EndpointType(ep.Type) == models.EndpointTypeMCPRuntime {
					transports = append(transports, ep.Endpoint.Transport)
				}
			}
			if len(transports) > 0 {
				sb.WriteString(fmt.Sprintf("   - Transports: %s\n", strings.Join(transports, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// Category summary
	sb.WriteString("## Summary\n\n")
	for _, cat := range view.Categories {
		if len(cat.Title) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", cat.Title, len(categories[cat.Key])))
	}

	return writeFile(path, sb.String())
}

// filterView applies a view's filter (and optional category filters) to entities.
func (vg *ViewGenerator) filterView(entities []*models.Entity, view *RegistryView) []*models.Entity {
	var result []*models.Entity
	for _, e := range entities {
		if view.Filter(e) {
			result = append(result, e)
		}
	}
	vg.sortEntities(result)
	return result
}

// groupByCategory groups entities within a view by category.
func (vg *ViewGenerator) groupByCategory(entities []*models.Entity, view RegistryView) map[string][]*models.Entity {
	result := make(map[string][]*models.Entity)
	for _, cat := range view.Categories {
		result[cat.Key] = nil
	}

	for _, e := range entities {
		matched := false
		for _, cat := range view.Categories {
			if cat.Filter == nil {
				continue
			}
			if cat.Filter(e) {
				result[cat.Key] = append(result[cat.Key], e)
				matched = true
				break
			}
		}
		if !matched && len(view.Categories) > 0 {
			// Put in the first category with nil filter (the "other" bucket)
			for _, cat := range view.Categories {
				if cat.Filter == nil {
					result[cat.Key] = append(result[cat.Key], e)
					break
				}
			}
		}
	}

	for _, cat := range view.Categories {
		vg.sortEntities(result[cat.Key])
	}
	return result
}

// sortEntities sorts entities by quality_score DESC, then name ASC.
func (vg *ViewGenerator) sortEntities(entities []*models.Entity) {
	sort.Slice(entities, func(i, j int) bool {
		if entities[i].Quality.Score != entities[j].Quality.Score {
			return entities[i].Quality.Score > entities[j].Quality.Score
		}
		return entities[i].Name < entities[j].Name
	})
}

// repoURL extracts the repository URL from an entity.
func repoURL(e *models.Entity) string {
	if e.Repository.URL != "" {
		return e.Repository.URL
	}
	if len(e.Endpoints) > 0 {
		for _, ep := range e.Endpoints {
			if models.EndpointType(ep.Type) == models.EndpointTypeRepositoryURL {
				return ep.Endpoint.URL
			}
		}
	}
	return "#"
}

// writeFile writes content to a file.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// FilterVerifiedMCPServers returns entities that are verified MCP servers.
// This is exported for use by callers that need just the filtered list.
func FilterVerifiedMCPServers(entities []*models.Entity) []*models.Entity {
	var result []*models.Entity
	for _, e := range entities {
		if e.IsVerifiedMCPServer() && isTaiwanRelevantT1Plus(e.TaiwanRelevance.Level) && isNotBlocked(e) {
			result = append(result, e)
		}
	}
	return result
}

// FilterMCPCandidates returns entities that are MCP server candidates.
func FilterMCPCandidates(entities []*models.Entity) []*models.Entity {
	var result []*models.Entity
	for _, e := range entities {
		if e.IsMCPServerCandidate() {
			result = append(result, e)
		}
	}
	return result
}

// IsTaiwanRelevantT1Plus is exported for external callers.
func IsTaiwanRelevantT1Plus(level models.TaiwanRelevanceLevel) bool {
	return isTaiwanRelevantT1Plus(level)
}

// MarshalEntityJSON marshals an entity to JSON, returning an error instead of panicking.
func MarshalEntityJSON(e *models.Entity) ([]byte, error) {
	return json.Marshal(e)
}
