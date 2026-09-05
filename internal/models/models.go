// Package models defines the domain model for the Taiwan MCP Crawler.
// All structs marshal/unmarshal to/from JSON with snake_case field names
// and RFC3339 timestamps.
package models

import (
	"time"
)

// Status represents repository activity status.
type Status string

const (
	StatusActive     Status = "ACTIVE"
	StatusMaintenance Status = "MAINTENANCE"
	StatusStale      Status = "STALE"
	StatusDormant    Status = "DORMANT"
	StatusArchived   Status = "ARCHIVED"
	StatusDeleted    Status = "DELETED"
	StatusUnknown    Status = "UNKNOWN"
)

// HealthStatus represents MCP endpoint health.
type HealthStatus string

const (
	HealthHealthy     HealthStatus = "HEALTHY"
	HealthDegraded    HealthStatus = "DEGRADED"
	HealthUnavailable HealthStatus = "UNAVAILABLE"
	HealthInvalid     HealthStatus = "INVALID"
	HealthUnknown     HealthStatus = "UNKNOWN"
)

// Transport type for MCP endpoints.
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportSSE            Transport = "sse"
	TransportStreamableHTTP  Transport = "streamable-http"
	TransportHTTP           Transport = "http"
	TransportWebsocket      Transport = "websocket"
	TransportUnknown        Transport = "unknown"
)

// DataSourceType classifies data source types.
type DataSourceType string

const (
	DataSourceOfficialGovAPI  DataSourceType = "official-government-api"
	DataSourceOfficialCompany DataSourceType = "official-company-api"
	DataSourceGovOpenData    DataSourceType = "government-open-data"
	DataSourceThirdPartyAPI  DataSourceType = "third-party-api"
	DataSourceWebScraping    DataSourceType = "web-scraping"
	DataSourceDatabase       DataSourceType = "database"
	DataSourceStaticDataset  DataSourceType = "static-dataset"
	DataSourceUnknown        DataSourceType = "unknown"
)

// SecuritySeverity levels.
type SecuritySeverity string

const (
	SeverityLow      SecuritySeverity = "LOW"
	SeverityMedium   SecuritySeverity = "MEDIUM"
	SeverityHigh     SecuritySeverity = "HIGH"
	SeverityCritical SecuritySeverity = "CRITICAL"
	SeverityUnknown  SecuritySeverity = "UNKNOWN"
)

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

// IsValidCategory returns true if cat is in the controlled vocabulary.
func IsValidCategory(cat string) bool {
	for _, c := range ValidCategories {
		if c == cat {
			return true
		}
	}
	return false
}

// ValidLevels are the Taiwan relevance levels (§14, §17).
var ValidLevels = []string{"T0", "T1", "T2", "T3", "T4", "T5"}

// IsValidLevel returns true if level is T0–T5.
func IsValidLevel(level string) bool {
	for _, l := range ValidLevels {
		if l == level {
			return true
		}
	}
	return false
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

// RawRecord is a fully fetched candidate with all metadata (§12, §16).
type RawRecord struct {
	Source        string         `json:"source"`
	SourceURL     string         `json:"source_url"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	RepositoryURL string         `json:"repository_url"`
	HomepageURL   string         `json:"homepage_url"`
	Readme        string         `json:"readme"`
	Manifest      map[string]any `json:"manifest"`
	Endpoint      string         `json:"endpoint"`
	Transport     string         `json:"transport"`
	Author        string         `json:"author"`
	License       string         `json:"license"`
	Stars         int            `json:"stars"`
	Topics        []string       `json:"topics"`
	RawMetadata   map[string]any `json:"raw_metadata"`
	FetchedAt     time.Time      `json:"fetched_at"`
}

// MCPServer is the normalized, classified MCP server (§13).
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
	Resources       []Resource       `json:"resources"`
	Prompts         []Prompt         `json:"prompts"`
	DataSources     []DataSource     `json:"data_sources"`
	License         string           `json:"license"`
	Status          Status           `json:"status" yaml:"status"`
	Health          HealthStatus     `json:"health" yaml:"health"`
	Quality         QualityScore     `json:"quality"`
	Security        []SecurityFinding `json:"security_findings"`
	Sources         []SourceReference `json:"sources"`
	Readme          string            `json:"readme,omitempty"`
	FirstSeen       time.Time         `json:"first_seen_at"`
	LastSeen        time.Time         `json:"last_seen_at"`
	LastVerified    time.Time         `json:"last_verified_at"`
}

// GetReadme returns the sanitized README text for LLM classification.
func (s *MCPServer) GetReadme() string {
	return s.Readme
}

// TopicList returns the server's topics.
func (s *MCPServer) TopicList() []string {
	return s.Repository.Topics
}

// TaiwanRelevance holds the Taiwan classification (§14, §17).
type TaiwanRelevance struct {
	Level      string      `json:"level"`
	Score      float64     `json:"score"`
	Confidence float64     `json:"confidence"`
	Evidence   []Evidence  `json:"evidence"`
}

// RepositoryInfo holds GitHub/repository metadata (§7).
type RepositoryInfo struct {
	URL           string    `json:"url"`
	Host          string    `json:"host"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	Stars         int       `json:"stars"`
	Forks         int       `json:"forks"`
	Watchers      int       `json:"watchers"`
	OpenIssues    int       `json:"open_issues"`
	Language      string    `json:"language"`
	License       string    `json:"license"`
	Topics         []string  `json:"topics"`
	DefaultBranch  string    `json:"default_branch"`
	Archived       bool      `json:"archived"`
	Fork            bool      `json:"fork"`
	Homepage        string    `json:"homepage"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	PushedAt        time.Time `json:"pushed_at"`
	LastCommitAt    time.Time `json:"last_commit_at"`
}

// Endpoint holds MCP endpoint connection info (§8).
type Endpoint struct {
	URL            string    `json:"url"`
	Transport      string    `json:"transport"`
	ProtocolVersion string   `json:"protocol_version"`
	Authentication AuthenticationInfo `json:"authentication"`
	TLS           bool      `json:"tls"`
	Status         string    `json:"status"`
}

// AuthenticationInfo holds endpoint auth info.
type AuthenticationInfo struct {
	Required bool   `json:"required"`
	Type     string `json:"type"`
}

// Tool represents an MCP tool (§9.1).
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Annotations ToolAnnotations `json:"annotations"`
}

// ToolAnnotations holds tool capability flags (§9.1).
type ToolAnnotations struct {
	ReadOnly     bool `json:"read_only"`
	Destructive  bool `json:"destructive"`
	Idempotent   bool `json:"idempotent"`
	Open         bool `json:"open"`
}

// Resource represents an MCP resource (§9.2).
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MIMEType     string `json:"mime_type"`
}

// Prompt represents an MCP prompt (§9.3).
type Prompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DataSource represents a data source used by the MCP (§10).
type DataSource struct {
	Name         string         `json:"name"`
	Type         DataSourceType `json:"type"`
	URL          string         `json:"url"`
	Country      string         `json:"country"`
	Official     bool           `json:"official"`
	AccessMethod string         `json:"access_method"`
}

// Evidence holds scoring rule evidence (§16, §66).
type Evidence struct {
	Type          string    `json:"type"`
	Source        string    `json:"source"`
	Location      string    `json:"location"`
	ContentHash   string    `json:"content_hash"`
	MatchedText   string    `json:"matched_text"`
	Rule          string    `json:"rule"`
	Score         float64   `json:"score"`
	Confidence     float64   `json:"confidence"`
	Timestamp     time.Time `json:"timestamp"`
}

// QualityScore holds the 100-point quality assessment (§15, §31).
type QualityScore struct {
	Score      int                `json:"score"`
	Grade      string             `json:"grade"`
	Components QualityComponents  `json:"components"`
}

// QualityComponents holds the 10 scoring components (§31).
type QualityComponents struct {
	DataSource     int `json:"data_source"`
	Maintenance    int `json:"maintenance"`
	Documentation   int `json:"documentation"`
	MCPCompliance   int `json:"mcp_compliance"`
	ToolSchema      int `json:"tool_schema"`
	Health          int `json:"health"`
	Repository      int `json:"repository"`
	License         int `json:"license"`
	Security        int `json:"security"`
	Community       int `json:"community"`
}

// SourceReference holds discovery source info (§16, §64).
type SourceReference struct {
	Source       string    `json:"source"`
	URL          string    `json:"url"`
	DiscoveredAt time.Time `json:"discovered_at"`
	LastSeen     time.Time `json:"last_seen"`
	TrustScore   float64   `json:"trust_score"`
}

// SecurityFinding holds a security scan result (§33).
type SecurityFinding struct {
	Type     string           `json:"type"`
	Severity SecuritySeverity `json:"severity"`
	Source   string         `json:"source"`
	Location string         `json:"location"`
	Evidence string         `json:"evidence"`
}

// CrawlRun holds metadata for a crawl execution (§37).
type CrawlRun struct {
	CrawlID            string    `json:"crawl_id"`
	StartedAt          time.Time `json:"started_at"`
	FinishedAt         time.Time `json:"finished_at"`
	SourcesScanned     int       `json:"sources_scanned"`
	CandidatesFound    int       `json:"candidates_found"`
	CandidatesNorm     int       `json:"candidates_normalized"`
	DuplicatesRemoved  int       `json:"duplicates_removed"`
	TaiwanCandidates   int       `json:"taiwan_candidates"`
	Verified           int       `json:"verified"`
	Failed             int       `json:"failed"`
	Errors             []string  `json:"errors"`
}

// Taiwan Relevance Scoring Constants (§17)
const (
	ScoreOfficialDomain      = 40
	ScoreGovAPI              = 40
	ScoreFinancialAPI        = 35
	ScoreTaiwanDataset       = 30
	ScoreTaiwanKeyword       = 20
	ScoreTaiwanLanguage      = 15
	ScoreTaiwanCompany       = 15
	ScoreReadmeMention       = 5
)

// Level thresholds (§17)
var LevelThresholds = []struct {
	MinScore int
	Level    string
}{
	{70, "T5"},
	{55, "T4"},
	{40, "T3"},
	{20, "T2"},
	{5, "T1"},
	{0, "T0"},
}

// ScoreToLevel maps a score to its Taiwan relevance level.
func ScoreToLevel(score float64) string {
	for _, t := range LevelThresholds {
		if int(score) >= t.MinScore {
			return t.Level
		}
	}
	return "T0"
}

// Quality component max values (§31)
const (
	QualityMaxDataSource    = 20
	QualityMaxMaintenance   = 15
	QualityMaxDocumentation = 10
	QualityMaxMCPCompliance = 15
	QualityMaxToolSchema     = 10
	QualityMaxHealth        = 10
	QualityMaxRepository    = 5
	QualityMaxLicense       = 5
	QualityMaxSecurity      = 5
	QualityMaxCommunity     = 5
)

// Source trust scores (§64)
var SourceTrustScores = map[string]float64{
	"official-registry": 1.00,
	"github":            0.95,
	"glama":             0.85,
	"pulsemcp":          0.80,
	"mcpso":             0.75,
	"manual":            0.50,
}

// Data source scores (§32)
var DataSourceScores = map[DataSourceType]int{
	DataSourceOfficialGovAPI:  20,
	DataSourceGovOpenData:     18,
	DataSourceOfficialCompany: 15,
	DataSourceThirdPartyAPI:   10,
	DataSourceWebScraping:     7,
	DataSourceDatabase:        7,
	DataSourceStaticDataset:   7,
	DataSourceUnknown:         0,
}

// GradeMap converts quality score to grade (§31).
func GradeForScore(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
