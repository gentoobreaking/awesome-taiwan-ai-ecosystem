package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"unicode/utf8"
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
	Slug        string // base filename without extension, e.g. "taiwan-ai-ecosystem"
	Group       string // spec §60 tree group: "MCP" / "AI" / "Data" / "Other"
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

// AllViews returns a copy of the static view catalogue (used by
// the /api/v1/registry/index endpoint so the API can describe
// every available view without running the export first).
func AllViews() []RegistryView {
	out := make([]RegistryView, len(allViews))
	copy(out, allViews)
	return out
}

// ViewSummary is the per-view row returned by the registry index
// endpoint. Mirrors the columns in the README "Output: Registry
// Views" table.
type ViewSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Count       int    `json:"count"`
	Filename    string `json:"filename"`
}

// ViewSummaryFromEntities computes, for every registered view, how
// many entities in the slice match its filter.
func ViewSummaryFromEntities(entities []*models.Entity) []ViewSummary {
	out := make([]ViewSummary, 0, len(allViews))
	for _, v := range allViews {
		n := 0
		for _, e := range entities {
			if v.Filter == nil || v.Filter(e) {
				n++
			}
		}
		out = append(out, ViewSummary{
			Slug:        v.Slug,
			Name:        v.Name,
			Description: v.Description,
			Group:       v.Group,
			Count:       n,
			Filename:    v.Slug + ".md",
		})
	}
	return out
}

// isMCPRelated checks if the entity is MCP-related.
func isMCPRelated(e *models.Entity) bool {
	return models.IsMCPRelated(e.Classification.Primary)
}

// isAIRelated checks if the entity is AI-related.
func isAIRelated(e *models.Entity) bool {
	return models.IsAIRelated(e.Classification.Primary)
}

// Define all views per T083 spec. Slug and Group are used by
// the /api/v1/registry/index summary endpoint (T-summary).
var allViews = []RegistryView{
	{
		Name:        "taiwan-ai-ecosystem",
		Slug:        "taiwan-ai-ecosystem",
		Group:       "Other",
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
		Slug:        "taiwan-mcp",
		Group:       "MCP",
		Description: "Verified MCP Servers (Runtime Verified, T1+, not blocked)",
		Filter:      isVerifiedMCPServer,
		Categories: []viewCategory{
			{Key: "verified", Title: "Verified MCP Servers", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-mcp-candidates",
		Slug:        "taiwan-mcp-candidates",
		Group:       "MCP",
		Description: "MCP Server Candidates (Candidate, Static Verified)",
		Filter:      isMCPCandidate,
		Categories: []viewCategory{
			{Key: "candidates", Title: "MCP Server Candidates", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-agents",
		Slug:        "taiwan-ai-agents",
		Group:       "AI",
		Description: "Taiwan AI Agents (T1+)",
		Filter:      isAIAgent,
		Categories: []viewCategory{
			{Key: "agents", Title: "AI Agents", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-tools",
		Slug:        "taiwan-ai-tools",
		Group:       "AI",
		Description: "Taiwan AI Tools, SDKs, Frameworks, Plugins (T1+)",
		Filter:      isAITool,
		Categories: []viewCategory{
			{Key: "tools", Title: "AI Tools", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-data",
		Slug:        "taiwan-ai-data",
		Group:       "Data",
		Description: "Taiwan AI Datasets, Data Libraries, APIs (T1+)",
		Filter:      isAIData,
		Categories: []viewCategory{
			{Key: "data", Title: "AI Data Sources", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-skills",
		Slug:        "taiwan-ai-skills",
		Group:       "AI",
		Description: "AI Skills and MCP Skills",
		Filter:      isAISkill,
		Categories: []viewCategory{
			{Key: "skills", Title: "AI Skills", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-infrastructure",
		Slug:        "taiwan-ai-infrastructure",
		Group:       "AI",
		Description: "AI Infrastructure entities",
		Filter:      isAIInfrastructure,
		Categories: []viewCategory{
			{Key: "infra", Title: "Infrastructure", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-tutorials",
		Slug:        "taiwan-ai-tutorials",
		Group:       "Other",
		Description: "AI Tutorials and Examples",
		Filter:      isAITutorial,
		Categories: []viewCategory{
			{Key: "tutorials", Title: "Tutorials", Filter: nil, SortBy: "quality_score"},
		},
	},
	{
		Name:        "taiwan-ai-collections",
		Slug:        "taiwan-ai-collections",
		Group:       "Other",
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

	// Generate backward-compatible awesome-taiwan-mcp.md (spec §53)
	// Uses old serverMarkdown format for backward compatibility.
	// Only includes RUNTIME_VERIFIED MCP servers — stricter than the old version.
	if err := vg.generateLegacyMarkdown(outputDir, entities); err != nil {
		return fmt.Errorf("generate legacy markdown: %w", err)
	}

	// Generate INDEX.md — navigable index of every view file
	// produced by this run. (T-INDEX)
	if err := vg.generateIndex(outputDir, entities); err != nil {
		return fmt.Errorf("generate index: %w", err)
	}

	return nil
}

// generateLegacyMarkdown generates the backward-compatible awesome-taiwan-mcp.md
// using the old serverMarkdown format. Only includes verified MCP servers (RUNTIME_VERIFIED).
// Spec §53: backward-compatible view for downstream consumers.
func (vg *ViewGenerator) generateLegacyMarkdown(outputDir string, entities []*models.Entity) error {
	var views []*models.MCPServerView
	for _, e := range entities {
		if view := e.ToMCPServerView(); view != nil {
			views = append(views, view)
		}
	}

	// Sort by quality score descending (same as old format)
	sort.Slice(views, func(i, j int) bool {
		return views[i].Quality.Score > views[j].Quality.Score
	})

	var sb strings.Builder
	sb.WriteString("# Awesome Taiwan MCP\n\n")
	sb.WriteString(fmt.Sprintf("> Generated at: %s\n\n", vg.config.GeneratedAt.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total servers: %d\n\n", len(views)))
	sb.WriteString("## Taiwan-relevant MCP Servers\n\n")

	for _, v := range views {
		sb.WriteString(legacyServerMarkdown(v))
	}

	markdown := sanitizeUTF8([]byte(sb.String()))
	markdown = strings.ToValidUTF8(markdown, "�")
	path := filepath.Join(outputDir, "awesome-taiwan-mcp.md")
	return os.WriteFile(path, []byte(markdown), 0644)
}

// generateIndex writes INDEX.md to outputDir. INDEX.md is a
// navigable table of every view, legacy, and security report
// produced by this run. Each row links to the actual markdown
// file so the dashboard / GitHub viewer can hop directly into
// any view. (T-INDEX)
func (vg *ViewGenerator) generateIndex(outputDir string, entities []*models.Entity) error {
	type row struct {
		Group     string
		File      string
		Title     string
		Subtitle  string
		Count     int
	}

	// Per-view counts: apply the same filter that GenerateViews
	// uses so the counts match what each view file actually contains.
	countFor := func(v RegistryView) int {
		n := 0
		for _, e := range entities {
			if v.Filter == nil || v.Filter(e) {
				n++
			}
		}
		return n
	}

	rows := make([]row, 0, len(allViews)+4)
	for _, v := range allViews {
		rows = append(rows, row{
			Group:    v.Group,
			File:     v.Slug + ".md",
			Title:    v.Name,
			Subtitle: v.Description,
			Count:    countFor(v),
		})
	}
	// Legacy + security reports. Their counts are file-level
	// metadata, not per-entity, so we leave the count blank.
	rows = append(rows, row{
		Group:    "Other",
		File:     "awesome-taiwan-mcp.md",
		Title:    "awesome-taiwan-mcp",
		Subtitle: "Legacy back-compat view (RUNTIME_VERIFIED MCP servers only, old markdown format)",
	})
	rows = append(rows, row{
		Group:    "Security",
		File:     "malicious/MALICIOUS_REPORT.md",
		Title:    "MALICIOUS_REPORT",
		Subtitle: "Source-code-level threat scan: obfuscation, credential extraction, remote binary downloads, shell injection, persistence, network beaconing, filesystem abuse",
	})
	rows = append(rows, row{
		Group:    "Security",
		File:     "security/injection/INJECTION_REPORT.md",
		Title:    "INJECTION_REPORT",
		Subtitle: "Prompt-injection / English-jailbreak / obfuscated-payload detection in README and description",
	})

	var sb strings.Builder
	sb.WriteString("# Registry Index\n\n")
	sb.WriteString("> Auto-generated by `cmd/export`. Each row links to a view file produced by the same export run.\n\n")
	sb.WriteString("Total entities scanned: **")
	sb.WriteString(strconv.Itoa(len(entities)))
	sb.WriteString("**\n\n")

	groups := map[string]bool{}
	for _, r := range rows {
		groups[r.Group] = true
	}
	groupOrder := []string{"MCP", "AI", "Data", "Other", "Security"}

	for _, g := range groupOrder {
		if !groups[g] {
			continue
		}
		sb.WriteString("## ")
		sb.WriteString(g)
		sb.WriteString("\n\n")
		sb.WriteString("| File | What it is | Count |\n")
		sb.WriteString("|---|---|---:|\n")
		for _, r := range rows {
			if r.Group != g {
				continue
			}
			sb.WriteString("| [")
			sb.WriteString(r.File)
			sb.WriteString("](")
			sb.WriteString(r.File)
			sb.WriteString(") — ")
			sb.WriteString(r.Title)
			if r.Subtitle != "" {
				sb.WriteString("<br/><sub>");
				sb.WriteString(r.Subtitle)
				sb.WriteString("</sub>")
			}
			sb.WriteString(" | ")
			if r.Count > 0 {
				sb.WriteString(strconv.Itoa(r.Count))
			} else {
				sb.WriteString("—")
			}
			sb.WriteString(" |\n")
		}
		sb.WriteString("\n")
	}

	path := filepath.Join(outputDir, "INDEX.md")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// legacyServerMarkdown renders a single server in the old markdown format (backward compatible).
func legacyServerMarkdown(s *models.MCPServerView) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", sanitizeUTF8([]byte(s.Name))))
	// Description: strip HTML, ensure UTF-8 valid, then rune-safe truncation
	desc := stripHTMLTags(s.Description)
	if !utf8.ValidString(desc) {
		desc = sanitizeUTF8([]byte(desc))
	}
	desc = strings.ToValidUTF8(desc, "�")
	runes := []rune(desc)
	if len(runes) > 150 {
		desc = string(runes[:150]) + "..."
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", desc))
	if s.Repository.URL != "" {
		sb.WriteString(fmt.Sprintf("- **Repository**: [%s](%s)\n",
			sanitizeUTF8([]byte(s.Repository.URL)), sanitizeUTF8([]byte(s.Repository.URL))))
	}
	if s.TaiwanRelevance.Level != "" {
		desc := levelDescription(string(s.TaiwanRelevance.Level))
		if desc != "" && desc != "Unknown" {
			sb.WriteString(fmt.Sprintf("- **Taiwan**: %s (score: %.0f) - %s\n",
				sanitizeUTF8([]byte(string(s.TaiwanRelevance.Level))), s.TaiwanRelevance.Score, desc))
		} else {
			sb.WriteString(fmt.Sprintf("- **Taiwan**: %s (score: %.0f)\n",
				sanitizeUTF8([]byte(string(s.TaiwanRelevance.Level))), s.TaiwanRelevance.Score))
		}
	}
	if len(s.TaiwanRelevance.Evidence) > 0 {
		for _, e := range s.TaiwanRelevance.Evidence {
			if e.Type == "" && e.MatchedText == "" && e.Rule == "" {
				continue
			}
			evText := sanitizeUTF8([]byte(e.MatchedText))
			evRunes := []rune(evText)
			if len(evRunes) > 80 {
				evText = string(evRunes[:80]) + "..."
			}
			typeLabel := e.Type
			if typeLabel == "" {
				typeLabel = e.Rule
			}
			if evText != "" {
				sb.WriteString(fmt.Sprintf("  - Evidence: %s (%s)\n", evText, typeLabel))
			} else {
				sb.WriteString(fmt.Sprintf("  - Evidence: %s\n", typeLabel))
			}
		}
	}
	if s.License != "" {
		sb.WriteString(fmt.Sprintf("- **License**: %s\n", sanitizeUTF8([]byte(s.License))))
	}
	if len(s.Transport) > 0 {
		sb.WriteString(fmt.Sprintf("- **Transport**: %s\n", strings.Join(s.Transport, ", ")))
	}
	if len(s.Tools) > 0 {
		sb.WriteString("- **Tools**:\n")
		for _, t := range s.Tools {
			toolDesc := sanitizeUTF8([]byte(t.Description))
			toolRunes := []rune(toolDesc)
			if len(toolRunes) > 100 {
				toolDesc = string(toolRunes[:100]) + "..."
			}
			sb.WriteString(fmt.Sprintf("  - `%s`: %s\n", sanitizeUTF8([]byte(t.Name)), toolDesc))
		}
	}
	sb.WriteString("\n")
	return sb.String()
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
