package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// MCPServer is the normalized domain model used for export.
// Mirrors internal/models.MCPServer but kept local to avoid import cycles in tests.
type MCPServer struct {
	ID              string
	Name            string
	Slug            string
	Description     string
	Category        []string
	Region          []string
	TaiwanRelevance TaiwanRelevance
	Repository      RepositoryInfo
	Endpoints       []Endpoint
	Transport       []string
	Tools           []Tool
	Resources       []Resource
	DataSources     []DataSource
	License         string
	Status          string
	Quality         QualityScore
	Sources         []SourceReference
	FirstSeen       time.Time
	LastSeen        time.Time
	LastVerified    time.Time
}

type TaiwanRelevance struct {
	Score      float64
	Level      string
	Evidence   []Evidence
	Confidence float64
}

type Evidence struct {
	Type        string
	Source      string
	Location    string
	ContentHash string
	MatchedText string
	Rule        string
	Score       float64
	Confidence  float64
	Timestamp   time.Time
}

type RepositoryInfo struct {
	URL           string
	Host          string
	Owner         string
	Name          string
	Stars         int
	Forks         int
	Watchers      int
	OpenIssues    int
	Language      string
	License       string
	Topics        []string
	DefaultBranch string
	Archived      bool
	Fork          bool
	Homepage      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PushedAt      time.Time
}

type Endpoint struct {
	URL             string
	Transport       string
	ProtocolVersion string
	TLS             bool
	Status          string
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Resource struct {
	URI         string
	Name        string
	Description string
	MIMEType    string
}

type DataSource struct {
	Name         string
	Type         string
	URL          string
	Country      string
	Official     bool
	AccessMethod string
}

type QualityScore struct {
	Score      int
	Grade      string
	Components QualityComponents
}

type QualityComponents struct {
	DataSource    int
	Maintenance   int
	Documentation int
	MCPCompliance int
	ToolSchema    int
	Health        int
	Repository    int
	License       int
	Security      int
	Community     int
}

type SourceReference struct {
	Source       string
	URL          string
	DiscoveredAt time.Time
	LastSeen     time.Time
	TrustScore   float64
}

// RegistryExporter handles JSON and Markdown registry export.
type RegistryExporter struct {
	Servers        []MCPServer
	CrawlerVersion string
	GeneratedAt    time.Time
}

// Export generates all JSON registry files into dir/registry.
func (e *RegistryExporter) Export(dir string) error {
	registryDir := filepath.Join(dir, "registry")
	if err := os.MkdirAll(registryDir, 0755); err != nil {
		return err
	}
	if err := e.ExportJSON(registryDir); err != nil {
		return err
	}
	if err := e.ExportMarkdown(registryDir); err != nil {
		return err
	}
	return nil
}

// ExportJSON generates the 6 JSON files.
func (e *RegistryExporter) ExportJSON(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// registry.json
	registry := map[string]any{
		"schema_version":  "0.1",
		"generated_at":    e.GeneratedAt.UTC().Format(time.RFC3339),
		"crawler_version": e.CrawlerVersion,
		"servers":         e.serversForJSON(),
	}
	if err := writeJSON(filepath.Join(dir, "registry.json"), registry); err != nil {
		return err
	}
	// registry.min.json
	minServers := e.serversForMinJSON()
	minRegistry := map[string]any{
		"schema_version": "0.1",
		"generated_at":   e.GeneratedAt.UTC().Format(time.RFC3339),
		"servers":        minServers,
	}
	if err := writeJSON(filepath.Join(dir, "registry.min.json"), minRegistry); err != nil {
		return err
	}
	// categories.json (normalized, uses parent mapping so stock->finance etc.)
	categories := make(map[string]int)
	for _, s := range e.Servers {
		if len(s.Category) == 0 {
			categories["uncategorized"]++
			continue
		}
		normed := normalizedCategories(s.Category)
		if len(normed) == 0 {
			categories["uncategorized"]++
			continue
		}
		// Count each normalized category (deduped per server already)
		for _, c := range normed {
			categories[c]++
		}
		// Also keep raw counts for audit? No, normalized is desired to align with markdown
	}
	if err := writeJSON(filepath.Join(dir, "categories.json"), categories); err != nil {
		return err
	}
	// sources.json
	sources := make(map[string]int)
	for _, s := range e.Servers {
		for _, src := range s.Sources {
			sources[src.Source]++
		}
	}
	if len(sources) == 0 {
		sources["github"] = len(e.Servers)
	}
	if err := writeJSON(filepath.Join(dir, "sources.json"), sources); err != nil {
		return err
	}
	// statistics.json
	stats := e.computeStatistics()
	if err := writeJSON(filepath.Join(dir, "statistics.json"), stats); err != nil {
		return err
	}
	// health.json
	health := e.computeHealth()
	if err := writeJSON(filepath.Join(dir, "health.json"), health); err != nil {
		return err
	}
	return nil
}

func (e *RegistryExporter) serversForJSON() []map[string]any {
	out := make([]map[string]any, 0, len(e.Servers))
	for _, s := range e.Servers {
		desc := sanitizeUTF8([]byte(s.Description))
		// ensure description is valid UTF-8 for JSON
		desc = strings.ToValidUTF8(desc, "�")
		out = append(out, map[string]any{
			"id":          s.ID,
			"name":        s.Name,
			"description": desc,
			"category":    s.Category,
			"region":      s.Region,
			"taiwan_relevance": map[string]any{
				"level":      s.TaiwanRelevance.Level,
				"score":      s.TaiwanRelevance.Score,
				"confidence": s.TaiwanRelevance.Confidence,
			},
			"official_data_source": len(s.DataSources) > 0,
			"repository": map[string]any{
				"url":     s.Repository.URL,
				"stars":   s.Repository.Stars,
				"license": s.Repository.License,
			},
			"transport": s.Transport,
			"tools":     s.Tools,
			"quality": map[string]any{
				"score": s.Quality.Score,
			},
			"status": s.Status,
		})
	}
	return out
}

func (e *RegistryExporter) serversForMinJSON() []map[string]any {
	out := make([]map[string]any, 0, len(e.Servers))
	for _, s := range e.Servers {
		desc := sanitizeUTF8([]byte(s.Description))
		out = append(out, map[string]any{
			"id":          s.ID,
			"name":        s.Name,
			"description": desc,
			"category":    s.Category,
			"transport":   s.Transport,
		})
	}
	return out
}

func (e *RegistryExporter) computeStatistics() map[string]any {
	byLevel := map[string]int{"T5": 0, "T4": 0, "T3": 0, "T2": 0, "T1": 0, "T0": 0}
	byHealth := map[string]int{"healthy": 0, "degraded": 0, "unavailable": 0, "unknown": 0}
	qualityDist := map[string]int{"A": 0, "B": 0, "C": 0, "D": 0, "F": 0}
	taiwanRelevant := 0
	for _, s := range e.Servers {
		level := s.TaiwanRelevance.Level
		if level == "" {
			level = "T0"
		}
		byLevel[level]++
		if level != "T0" && level != "" {
			taiwanRelevant++
		}
		grade := s.Quality.Grade
		if grade == "" {
			grade = "F"
		}
		qualityDist[grade]++
		byHealth["unknown"]++
	}
	return map[string]any{
		"schema_version":       "0.1",
		"generated_at":         e.GeneratedAt.UTC().Format(time.RFC3339),
		"total_servers":        len(e.Servers),
		"taiwan_relevant":      taiwanRelevant,
		"by_level":             byLevel,
		"by_health":            byHealth,
		"quality_distribution": qualityDist,
	}
}

func (e *RegistryExporter) computeHealth() map[string]any {
	servers := make([]map[string]any, 0, len(e.Servers))
	for _, s := range e.Servers {
		servers = append(servers, map[string]any{
			"id":         s.ID,
			"name":       s.Name,
			"health":     "UNKNOWN",
			"latency_ms": 0,
			"checks": map[string]bool{
				"repository": true,
				"endpoint":   true,
				"tls":        true,
				"initialize": true,
				"tools_list": true,
			},
		})
	}
	return map[string]any{
		"schema_version": "0.1",
		"generated_at":   e.GeneratedAt.UTC().Format(time.RFC3339),
		"servers":        servers,
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	// Ensure the JSON itself is valid UTF-8
	dataStr := sanitizeUTF8(data)
	return os.WriteFile(path, []byte(dataStr), 0644)
}

// sanitizeUTF8 ensures data is valid UTF-8, attempting Big5/GBK transcoding if needed.
// It uses "�" (Unicode replacement character) as the fallback replacement.
func sanitizeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	// Try Big5 (Traditional Chinese) decoding — with round-trip validation to avoid shadowing GBK
	if decoded, err := traditionalchinese.Big5.NewDecoder().Bytes(data); err == nil && utf8.Valid(decoded) {
		// Validate round-trip: re-encoding should reproduce original bytes (or at least not fail)
		if reenc, err2 := traditionalchinese.Big5.NewEncoder().Bytes(decoded); err2 == nil && bytes.Equal(reenc, data) {
			return strings.ToValidUTF8(string(decoded), "�")
		} else if err2 != nil {
			// If re-encoding fails, still accept if decoded contains no replacement and looks plausible
			// Fall through to GBK attempt
		} else {
			// Round-trip mismatch: likely GBK bytes mis-decoded as Big5, try GBK next
		}
		// If we want to keep simple Big5 acceptance when GBK would also match, we can still return Big5
		// but to handle GBK shadowing, we only return Big5 if GBK fails or round-trip succeeded.
		// Check GBK first for ambiguous case: try GBK and see if it round-trips better
		if decodedGBK, errGBK := simplifiedchinese.GBK.NewDecoder().Bytes(data); errGBK == nil && utf8.Valid(decodedGBK) {
			if reencGBK, errGBK2 := simplifiedchinese.GBK.NewEncoder().Bytes(decodedGBK); errGBK2 == nil && bytes.Equal(reencGBK, data) {
				return strings.ToValidUTF8(string(decodedGBK), "�")
			}
		}
		// If neither round-trips, fall back to Big5 decoded (still valid UTF-8)
		return strings.ToValidUTF8(string(decoded), "�")
	}
	// Try GBK (Simplified Chinese) decoding
	if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data); err == nil && utf8.Valid(decoded) {
		return strings.ToValidUTF8(string(decoded), "�")
	}
	return strings.ToValidUTF8(string(data), "�")
}

// SanitizeUTF8 is the exported wrapper for sanitizeUTF8, used by adapters and tests.
func SanitizeUTF8(data []byte) string {
	return sanitizeUTF8(data)
}

// SanitizeString ensures a string is valid UTF-8, with Big5/GBK fallback.
func SanitizeString(s string) string {
	return sanitizeUTF8([]byte(s))
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagRegex.ReplaceAllString(s, "")
}

// functionalCategories defines markdown grouping categories.
var functionalCategories = []struct {
	key         string
	displayName string
	emoji       string
}{
	{"finance", "Finance & Fintech", "💰"},
	{"real-estate", "Real Estate", "🏠"},
	{"government", "Government & Open Data", "🏛️"},
	{"weather", "Weather & Environment", "🌤️"},
	{"transport", "Transport & Logistics", "🚌"},
	{"healthcare", "Healthcare", "🏥"},
	{"education", "Education", "📚"},
	{"geography", "Geography & GIS", "🗺️"},
	{"language", "Language & Culture", "🈶"},
	{"ecommerce", "E-Commerce", "🛒"},
	{"payment", "Payment & Invoicing", "💳"},
	{"news", "News & Media", "📰"},
	{"search", "Search & Data Extraction", "🔎"},
	{"coding-agents", "Coding Agents", "🤖"},
	{"communication", "Communication", "💬"},
	{"databases", "Databases", "🗄️"},
	{"knowledge", "Knowledge & Memory", "🧠"},
	{"legal", "Legal & Compliance", "⚖️"},
	{"security", "Security", "🔒"},
	{"other", "Other Tools", "📦"},
}

// --- Category normalization (D02/D05/D08) ---
// ValidCategories is the controlled vocabulary (§19). Kept local to avoid import cycle.
var validCategoriesSet = map[string]bool{
	"finance": true, "stock": true, "etf": true, "banking": true, "insurance": true,
	"real-estate": true, "land": true, "housing": true,
	"government": true, "open-data": true, "legislative": true, "judicial": true, "procurement": true,
	"weather": true, "earthquake": true,
	"transport": true, "traffic": true, "railway": true, "metro": true, "bus": true,
	"logistics": true, "payment": true, "invoice": true, "tax": true,
	"company": true, "business": true,
	"healthcare": true, "education": true,
	"agriculture": true, "food": true,
	"tourism": true, "geography": true, "gis": true,
	"language": true, "traditional-chinese": true, "culture": true,
	"ecommerce": true, "devops": true, "news": true,
}

var functionalSet = map[string]bool{
	"finance": true, "real-estate": true, "government": true, "weather": true,
	"transport": true, "healthcare": true, "education": true, "geography": true,
	"language": true, "ecommerce": true, "payment": true, "news": true,
	"search": true, "coding-agents": true, "communication": true, "databases": true,
	"knowledge": true, "legal": true, "security": true, "other": true,
}

// categoryParentMap maps Valid sub-categories to functional parents (D02).
var categoryParentMap = map[string]string{
	"stock": "finance", "etf": "finance", "banking": "finance", "insurance": "finance",
	"land": "real-estate", "housing": "real-estate",
	"open-data": "government", "legislative": "government", "judicial": "government", "procurement": "government",
	"earthquake": "weather",
	"traffic": "transport", "railway": "transport", "metro": "transport", "bus": "transport", "logistics": "transport",
	"invoice": "payment", "tax": "payment",
	"company": "ecommerce", "business": "ecommerce",
	"tourism": "geography", "gis": "geography",
	"traditional-chinese": "language", "culture": "language",
	// valid but no functional parent -> map to other (prevents Other inflation via explicitness)
	"agriculture": "other", "food": "other", "devops": "other",
}

// categoryAliases handles hallucinations and case variants (D05/D11).
var categoryAliases = map[string]string{
	"finance & fintech": "finance",
	"finance-fintech":   "finance",
	"fintech":           "finance",
	"taiwan-stock":      "stock",
	"taiwan_stock":      "stock",
	"taiwan-stock-etf":  "stock",
	"open data":         "open-data",
	"traditional chinese": "traditional-chinese",
	"real estate":       "real-estate",
	"open_data":         "open-data",
}

func isFunctionalKey(key string) bool {
	return functionalSet[key]
}

// NormalizeCategory maps any raw category to its functional parent or "other".
// Normalization: trim, lower, replace spaces/underscores with hyphen, alias, parent mapping.
// Invalid or empty categories map to "other" so that other only receives truly unmatched.
func NormalizeCategory(cat string) string {
	c0 := strings.TrimSpace(strings.ToLower(cat))
	if c0 == "" {
		return "other"
	}
	// alias on raw lower form (before hyphen normalisation)
	if a, ok := categoryAliases[c0]; ok {
		c0 = a
	}
	c := strings.ReplaceAll(c0, " ", "-")
	c = strings.ReplaceAll(c, "_", "-")
	// collapse consecutive hyphens
	for strings.Contains(c, "--") {
		c = strings.ReplaceAll(c, "--", "-")
	}
	c = strings.Trim(c, "-")
	if a, ok := categoryAliases[c]; ok {
		c = a
	}
	if parent, ok := categoryParentMap[c]; ok {
		return parent
	}
	if isFunctionalKey(c) {
		return c
	}
	if validCategoriesSet[c] {
		// valid but not functional and not in parent map (should have been mapped)
		// treat as other
		return "other"
	}
	return "other"
}

func normalizedCategories(cats []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, c := range cats {
		n := NormalizeCategory(c)
		if n == "other" {
			// keep single other, but don't add if already has functional?
			// we allow other to be collected; deduped
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
			continue
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	// If the only entry is "other" and original had at least one functional-mappable,
	// normalizedCategories already reflects parent mapping, so other will be alone only when truly unmatched.
	// Caller handles empty case separately.
	if len(out) == 1 && out[0] == "other" {
		return out
	}
	// Remove "other" if we also have functional categories (other is fallback only)
	filtered := out[:0]
	for _, v := range out {
		if v == "other" {
			continue
		}
		filtered = append(filtered, v)
	}
	if len(filtered) > 0 {
		return filtered
	}
	// Only other remains
	return out
}

func hasFunctionalMapping(cats []string) bool {
	normed := normalizedCategories(cats)
	for _, c := range normed {
		if isFunctionalKey(c) && c != "other" {
			return true
		}
	}
	return false
}

func filterByCategory(servers []MCPServer, catKey string) []MCPServer {
	var result []MCPServer
	for _, s := range servers {
		if s.TaiwanRelevance.Level == "T0" {
			continue
		}
		if catKey == "other" {
			if len(s.Category) == 0 {
				result = append(result, s)
				continue
			}
			normed := normalizedCategories(s.Category)
			if len(normed) == 0 {
				result = append(result, s)
				continue
			}
			hasFunc := false
			for _, c := range normed {
				if isFunctionalKey(c) && c != "other" {
					hasFunc = true
					break
				}
			}
			if !hasFunc {
				result = append(result, s)
			}
			continue
		}
		// normalize catKey (functional keys are already normalized but be safe)
		nk := strings.TrimSpace(strings.ToLower(catKey))
		normed := normalizedCategories(s.Category)
		for _, c := range normed {
			if c == nk {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

func filterIntlServers(servers []MCPServer) []MCPServer {
	var result []MCPServer
	for _, s := range servers {
		if s.TaiwanRelevance.Level == "T0" || s.TaiwanRelevance.Level == "" {
			// Include T0 and empty level as international
			// But for Taiwan-relevant grouping we only exclude T0 with categories
			if s.TaiwanRelevance.Level == "T0" {
				result = append(result, s)
			} else if s.TaiwanRelevance.Level == "" && len(servers) > 0 {
				// empty level treated as T0 for intl display
				result = append(result, s)
			}
		}
	}
	// Actually simpler: return all T0
	result = nil
	for _, s := range servers {
		if s.TaiwanRelevance.Level == "T0" {
			result = append(result, s)
		}
	}
	return result
}

func serverMarkdown(s MCPServer) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", sanitizeUTF8([]byte(s.Name))))
	// Description: strip HTML, ensure UTF-8 valid, then rune-safe truncation
	desc := stripHTMLTags(s.Description)
	// Ensure utf-8 valid after stripping
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
		sb.WriteString(fmt.Sprintf("- **Repository**: [%s](%s)\n", sanitizeUTF8([]byte(s.Repository.URL)), sanitizeUTF8([]byte(s.Repository.URL))))
	}
	if s.TaiwanRelevance.Level != "" {
		desc := levelDescription(s.TaiwanRelevance.Level)
		if desc != "" && desc != "Unknown" {
			sb.WriteString(fmt.Sprintf("- **Taiwan**: %s (score: %.0f) - %s\n", sanitizeUTF8([]byte(s.TaiwanRelevance.Level)), s.TaiwanRelevance.Score, desc))
		} else {
			sb.WriteString(fmt.Sprintf("- **Taiwan**: %s (score: %.0f)\n", sanitizeUTF8([]byte(s.TaiwanRelevance.Level)), s.TaiwanRelevance.Score))
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
			// Show all evidence types (legacy keyword/domain plus production types)
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

func serverMarkdownIntl(s MCPServer) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s\n\n", sanitizeUTF8([]byte(s.Name))))
	desc := stripHTMLTags(s.Description)
	// Ensure utf-8 valid after stripping
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
		sb.WriteString(fmt.Sprintf("- **Repository**: [%s](%s)\n", sanitizeUTF8([]byte(s.Repository.URL)), sanitizeUTF8([]byte(s.Repository.URL))))
	}
	// Intl intentionally omits Taiwan field for T0, but we ensure license is shown
	if s.License != "" {
		sb.WriteString(fmt.Sprintf("- **License**: %s\n", sanitizeUTF8([]byte(s.License))))
	}
	if len(s.Transport) > 0 {
		sb.WriteString(fmt.Sprintf("- **Transport**: %s\n", strings.Join(s.Transport, ", ")))
	}
	if len(s.Tools) > 0 {
		// Fixed: previously toolNames was declared but not written to sb (dead code bug)
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

func levelDescription(level string) string {
	switch level {
	case "T5":
		return "Highly Taiwan-specific, official source"
	case "T4":
		return "Strong Taiwan relevance"
	case "T3":
		return "Moderate Taiwan relevance"
	case "T2":
		return "Weak Taiwan relevance"
	case "T1":
		return "Minimal Taiwan relevance"
	case "T0":
		return "No Taiwan relevance (International)"
	default:
		return "Unknown"
	}
}

// ExportMarkdown generates REGISTRY.md with Taiwan-relevant and International sections.
func (e *RegistryExporter) ExportMarkdown(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("# Taiwan MCP Registry\n\n")
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", e.GeneratedAt.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Total servers: %d\n\n", len(e.Servers)))
	// Statistics with levelDescription (P3)
	stats := e.computeStatistics()
	if tl, ok := stats["taiwan_relevant"]; ok {
		sb.WriteString(fmt.Sprintf("Taiwan-relevant: %v\n\n", tl))
	}
	sb.WriteString("### By Level\n\n")
	if bl, ok := stats["by_level"].(map[string]int); ok {
		for _, lvl := range []string{"T5", "T4", "T3", "T2", "T1", "T0"} {
			cnt := bl[lvl]
			desc := levelDescription(lvl)
			sb.WriteString(fmt.Sprintf("- **%s**: %d - %s\n", lvl, cnt, desc))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Taiwan-relevant MCP Servers\n\n")
	// Group by functional categories with dedup (D03)
	taiwanServers := make([]MCPServer, 0)
	for _, s := range e.Servers {
		if s.TaiwanRelevance.Level != "T0" {
			// include empty level? treat empty as T0? skip for now -> but we include empty as Taiwan-relevant to avoid disappearing (D07)
			// Actually we exclude T0 only; empty will go to Other via filter
			if s.TaiwanRelevance.Level == "" {
				// keep empty level servers in taiwan list so they can fall to Other, not vanish
			}
			taiwanServers = append(taiwanServers, s)
		}
	}
	// Filter out T0 explicitly: keep only non-T0 for Taiwan section
	filteredTaiwan := taiwanServers[:0]
	for _, s := range taiwanServers {
		if s.TaiwanRelevance.Level != "T0" {
			filteredTaiwan = append(filteredTaiwan, s)
		}
	}
	taiwanServers = filteredTaiwan
	// Sort by score descending for deterministic output
	sort.Slice(taiwanServers, func(i, j int) bool {
		return taiwanServers[i].TaiwanRelevance.Score > taiwanServers[j].TaiwanRelevance.Score
	})
	rendered := make(map[string]bool)
	for _, fc := range functionalCategories {
		if fc.key == "other" {
			continue
		}
		matched := filterByCategory(taiwanServers, fc.key)
		var unique []MCPServer
		for _, s := range matched {
			if !rendered[s.ID] {
				unique = append(unique, s)
				rendered[s.ID] = true
			}
		}
		if len(unique) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s %s\n\n", fc.emoji, fc.displayName))
		for _, s := range unique {
			sb.WriteString(serverMarkdown(s))
		}
	}
	// Other category (deduped, only truly unmatched)
	otherServers := filterByCategory(taiwanServers, "other")
	var uniqueOther []MCPServer
	for _, s := range otherServers {
		if !rendered[s.ID] {
			uniqueOther = append(uniqueOther, s)
			rendered[s.ID] = true
		}
	}
	if len(uniqueOther) > 0 {
		sb.WriteString("### 📦 Other Tools\n\n")
		for _, s := range uniqueOther {
			sb.WriteString(serverMarkdown(s))
		}
	}
	// International section
	sb.WriteString("## International MCP Servers\n\n")
	intlServers := filterIntlServers(e.Servers)
	sort.Slice(intlServers, func(i, j int) bool {
		return intlServers[i].Name < intlServers[j].Name
	})
	for _, s := range intlServers {
		sb.WriteString(serverMarkdownIntl(s))
	}
	// Sanitize entire markdown to ensure valid UTF-8
	markdown := sanitizeUTF8([]byte(sb.String()))
	markdown = strings.ToValidUTF8(markdown, "�")
	path := filepath.Join(dir, "REGISTRY.md")
	return os.WriteFile(path, []byte(markdown), 0644)
}
