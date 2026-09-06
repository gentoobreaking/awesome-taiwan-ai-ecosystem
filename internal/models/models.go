package models

import (
	"strings"
	"time"
)

// Status represents repository activity status.
type Status string

const (
	StatusActive      Status = "ACTIVE"
	StatusMaintenance Status = "MAINTENANCE"
	StatusStale       Status = "STALE"
	StatusDormant     Status = "DORMANT"
	StatusArchived    Status = "ARCHIVED"
	StatusDeleted     Status = "DELETED"
	StatusUnknown     Status = "UNKNOWN"
)

// HealthStatus represents MCP endpoint health.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)


// Transport type for MCP endpoints.
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportSSE            Transport = "sse"
	TransportStreamableHTTP Transport = "streamable-http"
	TransportHTTP           Transport = "http"
	TransportWebsocket      Transport = "websocket"
	TransportUnknown        Transport = "unknown"
)

// DataSourceType classifies data source types.
type DataSourceType string

const (
	DataSourceOfficialGovAPI DataSourceType = "official_gov_api"
	DataSourceOfficialCompany DataSourceType = "official_company"
	DataSourceThirdPartyAPI  DataSourceType = "third_party_api"
	DataSourceOpenData       DataSourceType = "open_data"
	DataSourceOfficial       DataSourceType = "official"
	DataSourceCommunity      DataSourceType = "community"
)

// Severity levels for security findings
const (
	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

// HealthStatus missing values
const (
	HealthInvalid     HealthStatus = "INVALID"
	HealthUnavailable HealthStatus = "UNAVAILABLE"
)

// DataSourceScores holds scores for different data source types
type DataSourceScores struct {
	OfficialCompany float64
	ThirdPartyAPI   float64
	OfficialGovAPI  float64
	OpenData        float64
	Official        float64
	Community       float64
}

// SourceTrustScores holds trust scores for different sources
type SourceTrustScores struct {
	GitHub           float64
	Registry         float64
	Mcpserversorg    float64
	Mcpmarket        float64
	GithubRepo       float64
}

// ValidCategories is the controlled vocabulary for server categories (§19).
var ValidCategories = []string{
	"finance", "stock", "etf", "banking", "insurance",
	"real-estate", "land", "housing",
	"government", "open-data", "legislative", "judicial", "procurement",
	"weather", "earthquake",
	"transport", "traffic", "railway", "metro", "bus",
	"logistics", "payment", "invoice", "tax",
	"company", "business",
	"healthcare", "education",
	"agriculture", "food",
	"tourism", "geography", "gis",
	"language", "traditional-chinese", "culture",
	"ecommerce", "devops", "news",
}

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
	"agriculture": "other", "food": "other", "devops": "other",
}

var categoryAliases = map[string]string{
	"finance & fintech": "finance",
	"finance-fintech":   "finance",
	"fintech":           "finance",
	"taiwan-stock":      "stock",
	"taiwan_stock":      "stock",
	"stock-market":      "stock",
	"etf-fund":          "etf",
	"bank":              "banking",
	"insurance-fin":     "insurance",
	"real estate":       "real-estate",
	"land-registry":     "land",
	"housing-price":     "housing",
	"gov":               "government",
	"open data":         "open-data",
	"legislative-yuan":  "legislative",
	"judicial-yuan":     "judicial",
	"gov-procurement":   "procurement",
	"weather-cwa":       "weather",
	"earthquake-tw":     "earthquake",
	"transport-tw":      "transport",
	"traffic-tw":        "traffic",
	"railway-tw":        "railway",
	"metro-tw":          "metro",
	"bus-tw":            "bus",
	"logistics-tw":      "logistics",
	"payment-tw":        "payment",
	"invoice-tw":        "invoice",
	"tax-tw":            "tax",
	"company-tw":        "company",
	"business-tw":       "business",
	"healthcare-tw":     "healthcare",
	"education-tw":      "education",
	"agriculture-tw":    "agriculture",
	"food-tw":           "food",
	"tourism-tw":        "tourism",
	"geography-tw":      "geography",
	"gis-tw":            "gis",
	"chinese-traditional": "traditional-chinese",
	"culture-tw":        "culture",
	"ecommerce-tw":      "ecommerce",
	"devops-tw":         "devops",
	"news-tw":           "news",
}

// IsValidCategory returns true if cat is in the controlled vocabulary.
func IsValidCategory(cat string) bool {
	return validCategoriesSet[cat]
}

// ValidLevels are the Taiwan relevance levels (§14, §17).
var ValidLevels = []string{"T0", "T1", "T2", "T3", "T4", "T5"}

// IsValidLevel returns true if level is T0–T5.
func IsValidLevel(level string) bool {
	switch level {
	case "T0", "T1", "T2", "T3", "T4", "T5":
		return true
	default:
		return false
	}
}

// NormalizeCategory maps any raw category to its functional parent or "other".
func NormalizeCategory(cat string) string {
	cat = strings.ToLower(strings.TrimSpace(cat))
	if alias, ok := categoryAliases[cat]; ok {
		cat = alias
	}
	if parent, ok := categoryParentMap[cat]; ok {
		return parent
	}
	if validCategoriesSet[cat] {
		return cat
	}
	return "other"
}

// NormalizeCategories deduplicates and normalizes a slice.
func NormalizeCategories(cats []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, c := range cats {
		norm := NormalizeCategory(c)
		if !seen[norm] {
			seen[norm] = true
			result = append(result, norm)
		}
	}
	return result
}

// IsFunctionalKey reports whether key is a functional category key.
func IsFunctionalKey(key string) bool {
	return functionalSet[key]
}

// RawCandidate represents a raw MCP discovered from a source (§12).
type RawCandidate struct {
	Source        string         `json:"source"`
	SourceURL     string         `json:"source_url"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	RepositoryURL string         `json:"repository_url"`
	HomepageURL   string         `json:"homepage_url"`
	Endpoint      string         `json:"endpoint"`
	Author        string         `json:"author"`
	RawMetadata   map[string]any `json:"raw_metadata"`
	DiscoveredAt  time.Time      `json:"discovered_at"`
}

// Prompt represents an MCP prompt (§9.3).
type Prompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RawRecord is a fully fetched candidate with all metadata (§12, §16).
type RawRecord struct {
	RawCandidate
	Repository        RepositoryInfo `json:"repository"`
	Endpoints         []Endpoint     `json:"endpoints"`
	Transport         []string       `json:"transport"`
	Readme            string         `json:"readme"`
	PackageFiles      map[string]string `json:"package_files"`
	SourceTrustScore  float64        `json:"source_trust_score"`
}
// MCPServer is the normalized, classified MCP server (§13).
// Deprecated: Use Entity with ToMCPServerView() instead.
type MCPServer struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description"`
	Region          []string         `json:"region"`
	Category        []string         `json:"category"`
	TaiwanRelevance TaiwanRelevance  `json:"taiwan_relevance"`
	Repository      RepositoryInfo   `json:"repository"`
	Endpoints       []Endpoint       `json:"endpoints"`
	Transport       []string         `json:"transport"`
	Tools           []Tool           `json:"tools"`
	Resources       []Resource       `json:"resources,omitempty"`
	Prompts         []Prompt         `json:"prompts,omitempty"`
	DataSources     []DataSource     `json:"data_sources"`
	License         string           `json:"license"`
	Status          Status           `json:"status"`
	Health          HealthStatus     `json:"health"`
	Quality         QualityScore     `json:"quality"`
	Security        SecurityStatusDetail `json:"security,omitempty"`
	Sources         []SourceReference `json:"sources"`
	FirstSeen       time.Time        `json:"first_seen"`
	LastSeen        time.Time        `json:"last_seen"`
	LastVerified    time.Time        `json:"last_verified"`
	Readme          string           `json:"readme,omitempty"`
}

// Endpoint holds MCP endpoint connection info (§8).
type Endpoint struct {
	URL            string            `json:"url"`
	Transport      string            `json:"transport"`
	ProtocolVersion string           `json:"protocol_version"`
	Authentication AuthenticationInfo `json:"authentication"`
	TLS            bool              `json:"tls"`
	Status         string            `json:"status"`
}

// AuthenticationInfo holds authentication details.
type AuthenticationInfo struct {
	Required bool   `json:"required"`
	Type     string `json:"type"`
}

// Tool represents an MCP tool (§9.1).
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]any         `json:"input_schema"`
	Annotations ToolAnnotations        `json:"annotations"`
}

type ToolAnnotations struct {
	ReadOnly    bool `json:"read_only"`
	Destructive bool `json:"destructive"`
	Idempotent  bool `json:"idempotent"`
	Open        bool `json:"open"`
}

// Resource represents an MCP resource (§9.2).
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MIMEType    string `json:"mime_type"`
}

// DataSource represents a data source used by the MCP (§10).
type DataSource struct {
	Name         string        `json:"name"`
	Type         DataSourceType `json:"type"`
	URL          string        `json:"url"`
	Country      string        `json:"country"`
	Official     bool          `json:"official"`
	AccessMethod string        `json:"access_method"`
}

// Level thresholds (§17)
var LevelThresholds = []struct {
	Level  string
	Min    float64
	Max    float64
}{
	{"T5", 70, 100},
	{"T4", 55, 69.99},
	{"T3", 40, 54.99},
	{"T2", 20, 39.99},
	{"T1", 5, 19.99},
	{"T0", 0, 4.99},
}

// ScoreToLevel maps a score to its Taiwan relevance level.
// Deprecated: Use ScoreToTaiwanLevel instead.

// CrawlRun metadata for tracking crawl executions
type CrawlRun struct {
	ID               string          `json:"id"`
	StartedAt        RFC3339Time     `json:"started_at"`
	CompletedAt      *RFC3339Time    `json:"completed_at,omitempty"`
	Status           string          `json:"status"`
	SourcesRun       []string        `json:"sources_run"`
	TotalFound       int             `json:"total_found"`
	TotalNew         int             `json:"total_new"`
	TotalUpdated     int             `json:"total_updated"`
	Errors           []string        `json:"errors,omitempty"`
	CrawlID          string          `json:"crawl_id"`
	FinishedAt       *RFC3339Time    `json:"finished_at,omitempty"`
	SourcesScanned   int             `json:"sources_scanned"`
	CandidatesFound  int             `json:"candidates_found"`
	CandidatesNorm   int             `json:"candidates_norm"`
	DuplicatesRemoved int            `json:"duplicates_removed"`
	TaiwanCandidates int             `json:"taiwan_candidates"`
	Verified         int             `json:"verified"`
	Failed           int             `json:"failed"`
}
// ServerSnapshot represents a historical snapshot of a server from a crawl run.
type ServerSnapshot struct {
	ID         int64           `json:"id"`
	ServerID   string          `json:"server_id"`
	CrawlID    string          `json:"crawl_id"`
	Snapshot   *MCPServer      `json:"snapshot"`
	CreatedAt  RFC3339Time     `json:"created_at"`
}
