package models

import "time"

// EntityStatus represents the lifecycle status of an entity.
// Based on spec §8, §55, §64.
type EntityStatus string

const (
	// EntityStatusDiscovered - Just discovered, not yet classified (spec §8).
	EntityStatusDiscovered EntityStatus = "DISCOVERED"
	// EntityStatusCandidate - Classified, awaiting verification (spec §55).
	EntityStatusCandidate EntityStatus = "CANDIDATE"
	// EntityStatusVerified - Passed verification (runtime verification, security scan, etc.).
	EntityStatusVerified EntityStatus = "VERIFIED"
	// EntityStatusQuarantined - Suspected malicious, isolated for review (spec §12, §56 Test 12).
	EntityStatusQuarantined EntityStatus = "QUARANTINED"
	// EntityStatusRejected - Explicitly non-target type or malicious.
	EntityStatusRejected EntityStatus = "REJECTED"
)

// ValidEntityStatuses contains all valid entity status values.
var ValidEntityStatuses = []EntityStatus{
	EntityStatusDiscovered,
	EntityStatusCandidate,
	EntityStatusVerified,
	EntityStatusQuarantined,
	EntityStatusRejected,
}

var validEntityStatusSet = map[EntityStatus]bool{
	EntityStatusDiscovered:  true,
	EntityStatusCandidate:   true,
	EntityStatusVerified:    true,
	EntityStatusQuarantined: true,
	EntityStatusRejected:    true,
}

// IsValidEntityStatus returns true if the status is valid.
func IsValidEntityStatus(s EntityStatus) bool {
	return validEntityStatusSet[s]
}

// CanTransitionEntityStatus returns true if the transition from -> to is valid.
// Valid transitions (spec §8, §55):
// DISCOVERED → CANDIDATE (classification complete)
// CANDIDATE → VERIFIED (verification passed)
// CANDIDATE → QUARANTINED (security scan found suspicious)
// CANDIDATE → REJECTED (classified as NON_AI or explicitly non-target)
// QUARANTINED → REJECTED (confirmed malicious) or VERIFIED (false positive)
// VERIFIED → REJECTED (later found issues)
func CanTransitionEntityStatus(from, to EntityStatus) bool {
	switch from {
	case EntityStatusDiscovered:
		return to == EntityStatusCandidate
	case EntityStatusCandidate:
		return to == EntityStatusVerified || to == EntityStatusQuarantined || to == EntityStatusRejected
	case EntityStatusQuarantined:
		return to == EntityStatusRejected || to == EntityStatusVerified
	case EntityStatusVerified:
		return to == EntityStatusRejected
	default:
		return false
	}
}

// MCPIdentityStatus represents the MCP identity verification status.
// Based on spec §55, §59, §61 Phase 7.
type MCPIdentityStatus string

const (
	// MCPIdentityStatusCandidate - Discovery phase, suspected MCP related, pending static analysis.
	MCPIdentityStatusCandidate MCPIdentityStatus = "CANDIDATE"
	// MCPIdentityStatusStaticVerified - Static analysis confirms MCP server implementation (T074).
	MCPIdentityStatusStaticVerified MCPIdentityStatus = "STATIC_VERIFIED"
	// MCPIdentityStatusRuntimeVerified - Runtime handshake passed (T078).
	MCPIdentityStatusRuntimeVerified MCPIdentityStatus = "RUNTIME_VERIFIED"
	// MCPIdentityStatusNotMCP - Confirmed non-MCP server (tutorial, client-only, collection, etc.).
	MCPIdentityStatusNotMCP MCPIdentityStatus = "NOT_MCP"
)

// ValidMCPIdentityStatuses contains all valid MCP identity status values.
var ValidMCPIdentityStatuses = []MCPIdentityStatus{
	MCPIdentityStatusCandidate,
	MCPIdentityStatusStaticVerified,
	MCPIdentityStatusRuntimeVerified,
	MCPIdentityStatusNotMCP,
}

var validMCPIdentityStatusSet = map[MCPIdentityStatus]bool{
	MCPIdentityStatusCandidate:      true,
	MCPIdentityStatusStaticVerified: true,
	MCPIdentityStatusRuntimeVerified: true,
	MCPIdentityStatusNotMCP:         true,
}

func IsValidMCPIdentityStatus(s MCPIdentityStatus) bool {
	return validMCPIdentityStatusSet[s]
}

// EndpointEvidence holds evidence for endpoint type classification.
type EndpointEvidence struct {
	Rule        string      `json:"rule"`
	Source      string      `json:"source"`
	Location    string      `json:"location"`
	MatchedText string      `json:"matched_text"`
	Pattern     string      `json:"pattern"`
	Confidence  float64     `json:"confidence"`
	Timestamp   RFC3339Time `json:"timestamp"`
}

// CanTransitionMCPIdentityStatus returns true if the transition is valid.
// Valid transitions:
// CANDIDATE → STATIC_VERIFIED (static analysis passed)
// CANDIDATE → NOT_MCP (static analysis negative)
// STATIC_VERIFIED → RUNTIME_VERIFIED (runtime verification passed)
// STATIC_VERIFIED → NOT_MCP (runtime verification failed and confirmed non-server)
// RUNTIME_VERIFIED → NOT_MCP (later found issues, rare)
func CanTransitionMCPIdentityStatus(from, to MCPIdentityStatus) bool {
	switch from {
	case MCPIdentityStatusCandidate:
		return to == MCPIdentityStatusStaticVerified || to == MCPIdentityStatusNotMCP
	case MCPIdentityStatusStaticVerified:
		return to == MCPIdentityStatusRuntimeVerified || to == MCPIdentityStatusNotMCP
	case MCPIdentityStatusRuntimeVerified:
		return to == MCPIdentityStatusNotMCP
	default:
		return false
	}
}

// SecurityStatus represents the security assessment status.
// Based on spec §12, §54, §56 Test 12, §61 Phase 8.
type SecurityStatus string

const (
	// SecurityStatusClean - No security issues found.
	SecurityStatusClean SecurityStatus = "CLEAN"
	// SecurityStatusSuspicious - Low/medium risk patterns found, needs attention but not blocking.
	SecurityStatusSuspicious SecurityStatus = "SUSPICIOUS"
	// SecurityStatusQuarantined - High risk/suspicious malicious patterns, isolated for manual review (spec §12, §56 Test 12).
	SecurityStatusQuarantined SecurityStatus = "QUARANTINED"
	// SecurityStatusBlocked - Confirmed malicious, permanently blocked.
	SecurityStatusBlocked SecurityStatus = "BLOCKED"
)

// ValidSecurityStatuses contains all valid security status values.
var ValidSecurityStatuses = []SecurityStatus{
	SecurityStatusClean,
	SecurityStatusSuspicious,
	SecurityStatusQuarantined,
	SecurityStatusBlocked,
}

var validSecurityStatusSet = map[SecurityStatus]bool{
	SecurityStatusClean:       true,
	SecurityStatusSuspicious:  true,
	SecurityStatusQuarantined: true,
	SecurityStatusBlocked:     true,
}

// IsValidSecurityStatus returns true if the status is valid.
func IsValidSecurityStatus(s SecurityStatus) bool {
	return validSecurityStatusSet[s]
}

// CanTransitionSecurityStatus returns true if the transition is valid.
// Valid transitions:
// CLEAN → SUSPICIOUS (new scan finds risk)
// SUSPICIOUS → QUARANTINED (risk escalated or manual judgment)
// QUARANTINED → BLOCKED (confirmed malicious)
// QUARANTINED → CLEAN (false positive, manually confirmed)
// Any → BLOCKED (emergency block)
func CanTransitionSecurityStatus(from, to SecurityStatus) bool {
	switch from {
	case SecurityStatusClean:
		return to == SecurityStatusSuspicious || to == SecurityStatusBlocked
	case SecurityStatusSuspicious:
		return to == SecurityStatusQuarantined || to == SecurityStatusBlocked || to == SecurityStatusClean
	case SecurityStatusQuarantined:
		return to == SecurityStatusBlocked || to == SecurityStatusClean
	case SecurityStatusBlocked:
		return false // Once blocked, stays blocked
	default:
		return false
	}
}

// TaiwanRelevanceLevel represents Taiwan relevance level T0-T5.
type TaiwanRelevanceLevel string

const (
	TaiwanRelevanceLevelT0 TaiwanRelevanceLevel = "T0"
	TaiwanRelevanceLevelT1 TaiwanRelevanceLevel = "T1"
	TaiwanRelevanceLevelT2 TaiwanRelevanceLevel = "T2"
	TaiwanRelevanceLevelT3 TaiwanRelevanceLevel = "T3"
	TaiwanRelevanceLevelT4 TaiwanRelevanceLevel = "T4"
	TaiwanRelevanceLevelT5 TaiwanRelevanceLevel = "T5"
)

var validTaiwanRelevanceLevelSet = map[TaiwanRelevanceLevel]bool{
	TaiwanRelevanceLevelT0: true,
	TaiwanRelevanceLevelT1: true,
	TaiwanRelevanceLevelT2: true,
	TaiwanRelevanceLevelT3: true,
	TaiwanRelevanceLevelT4: true,
	TaiwanRelevanceLevelT5: true,
}

func IsValidTaiwanRelevanceLevel(l TaiwanRelevanceLevel) bool {
	return validTaiwanRelevanceLevelSet[l]
}

// AIRelevanceLevel represents AI relevance level A0-A5.
type AIRelevanceLevel string

const (
	AIRelevanceLevelA0 AIRelevanceLevel = "A0"
	AIRelevanceLevelA1 AIRelevanceLevel = "A1"
	AIRelevanceLevelA2 AIRelevanceLevel = "A2"
	AIRelevanceLevelA3 AIRelevanceLevel = "A3"
	AIRelevanceLevelA4 AIRelevanceLevel = "A4"
	AIRelevanceLevelA5 AIRelevanceLevel = "A5"
)

var validAIRelevanceLevelSet = map[AIRelevanceLevel]bool{
	AIRelevanceLevelA0: true,
	AIRelevanceLevelA1: true,
	AIRelevanceLevelA2: true,
	AIRelevanceLevelA3: true,
	AIRelevanceLevelA4: true,
	AIRelevanceLevelA5: true,
}

func IsValidAIRelevanceLevel(l AIRelevanceLevel) bool {
	return validAIRelevanceLevelSet[l]
}

// QualityGrade represents quality grade A-F.
type QualityGrade string

const (
	QualityGradeA QualityGrade = "A"
	QualityGradeB QualityGrade = "B"
	QualityGradeC QualityGrade = "C"
	QualityGradeD QualityGrade = "D"
	QualityGradeF QualityGrade = "F"
)

var validQualityGradeSet = map[QualityGrade]bool{
	QualityGradeA: true,
	QualityGradeB: true,
	QualityGradeC: true,
	QualityGradeD: true,
	QualityGradeF: true,
}

func IsValidQualityGrade(g QualityGrade) bool {
	return validQualityGradeSet[g]
}

// GradeForScore converts quality score (0-100) to grade (spec §15).
func GradeForScore(score int) QualityGrade {
	switch {
	case score >= 90:
		return QualityGradeA
	case score >= 80:
		return QualityGradeB
	case score >= 70:
		return QualityGradeC
	case score >= 60:
		return QualityGradeD
	default:
		return QualityGradeF
	}
}

// ScoreToTaiwanLevel maps a score to its Taiwan relevance level (spec §17).
func ScoreToTaiwanLevel(score float64) TaiwanRelevanceLevel {
	switch {
	case score >= 70:
		return TaiwanRelevanceLevelT5
	case score >= 55:
		return TaiwanRelevanceLevelT4
	case score >= 40:
		return TaiwanRelevanceLevelT3
	case score >= 20:
		return TaiwanRelevanceLevelT2
	case score >= 5:
		return TaiwanRelevanceLevelT1
	default:
		return TaiwanRelevanceLevelT0
	}
}

// ScoreToAILevel maps a score to its AI relevance level.
func ScoreToAILevel(score float64) AIRelevanceLevel {
	switch {
	case score >= 70:
		return AIRelevanceLevelA5
	case score >= 50:
		return AIRelevanceLevelA4
	case score >= 30:
		return AIRelevanceLevelA3
	case score >= 15:
		return AIRelevanceLevelA2
	case score >= 5:
		return AIRelevanceLevelA1
	default:
		return AIRelevanceLevelA0
	}
}

// RFC3339Time is a time.Time that marshals to RFC3339 format.
type RFC3339Time time.Time

func (t RFC3339Time) MarshalJSON() ([]byte, error) {
	return time.Time(t).MarshalJSON()
}

func (t *RFC3339Time) UnmarshalJSON(data []byte) error {
	return (*time.Time)(t).UnmarshalJSON(data)
}

func (t RFC3339Time) String() string {
	return time.Time(t).Format(time.RFC3339)
}

func (t RFC3339Time) Time() time.Time {
	return time.Time(t)
}

func (t RFC3339Time) IsZero() bool {
	return time.Time(t).IsZero()
}

// Evidence represents scoring rule evidence (spec §16, §4.4, §66).
type Evidence struct {
	Type         string      `json:"type"`          // official_domain, repository_keyword, data_source, etc.
	Source       string      `json:"source"`        // README, package.json, manifest, mcp_protocol, source_code, etc.
	Location     string      `json:"location"`      // file path or URL
	ContentHash  string      `json:"content_hash"`  // sha256 of matched text
	MatchedText  string      `json:"matched_text"`
	Rule         string      `json:"rule"`          // scoring rule name that produced this evidence
	Score        float64     `json:"score"`         // weight contributed
	Confidence   float64     `json:"confidence"`    // confidence in this evidence
	Timestamp    RFC3339Time `json:"timestamp"`
}

// ClassificationEvidence extends Evidence with classification-specific fields.
type ClassificationEvidence struct {
	Evidence
	Classification PrimaryClassification `json:"classification,omitempty"`
	MCPRole        MCPRole               `json:"mcp_role,omitempty"`
}

// ClassificationResult holds the result of entity classification.
type ClassificationResult struct {
	Primary      PrimaryClassification  `json:"primary"`
	Confidence   float64                `json:"confidence"`
	Evidence     []ClassificationEvidence `json:"evidence"`
	MCPRole      MCPRole                `json:"mcp_role"`
	Reasoning    string                 `json:"reasoning"`
}

// TaiwanRelevance holds Taiwan classification result (spec §14, §17).
type TaiwanRelevance struct {
	Score       float64            `json:"score"`
	Level       TaiwanRelevanceLevel `json:"level"`
	Evidence    []Evidence         `json:"evidence"`
	Confidence  float64            `json:"confidence"`
}

// AIRelevance holds AI classification result.
type AIRelevance struct {
	Score       float64          `json:"score"`
	Level       AIRelevanceLevel `json:"level"`
	Evidence    []Evidence       `json:"evidence"`
	Confidence  float64          `json:"confidence"`
}

// MCPIdentity holds MCP identity verification result (spec §4.3, §45, §59).
type MCPIdentity struct {
	Status              MCPIdentityStatus      `json:"status"`
	Evidence            []Evidence             `json:"evidence"`
	Confidence          float64                `json:"confidence"`
	Role                MCPRole                `json:"role"`
	SecondaryRoles      []MCPRole              `json:"secondary_roles,omitempty"`
	StaticCheckedAt     *RFC3339Time           `json:"static_checked_at,omitempty"`
	RuntimeVerifiedAt   *RFC3339Time           `json:"runtime_verified_at,omitempty"`
}

// RuntimeVerificationStatus represents runtime verification status.
type RuntimeVerificationStatus string

const (
	RuntimeVerificationStatusPassed  RuntimeVerificationStatus = "PASSED"
	RuntimeVerificationStatusFailed  RuntimeVerificationStatus = "FAILED"
	RuntimeVerificationStatusTimeout RuntimeVerificationStatus = "TIMEOUT"
	RuntimeVerificationStatusError   RuntimeVerificationStatus = "ERROR"
)

// InitializeResult holds initialize handshake result.
type InitializeResult struct {
	Success       bool   `json:"success"`
	Response      string `json:"response,omitempty"`
	LatencyMs     int    `json:"latency_ms"`
	Error         string `json:"error,omitempty"`
	ServerInfo    string `json:"server_info,omitempty"`
	ProtocolVer   string `json:"protocol_version,omitempty"`
	Capabilities  string `json:"capabilities,omitempty"`
}

// ToolsListResult holds tools/list result.
type ToolsListResult struct {
	Success     bool   `json:"success"`
	ToolCount   int    `json:"tool_count"`
	ToolsSummary string `json:"tools_summary,omitempty"`
	LatencyMs   int    `json:"latency_ms"`
	Error       string `json:"error,omitempty"`
}

// RuntimeVerification holds runtime verification result (spec §59, T078).
type RuntimeVerification struct {
	Status            RuntimeVerificationStatus `json:"status"`
	InitializeResult  *InitializeResult         `json:"initialize_result,omitempty"`
	ToolsListResult   *ToolsListResult          `json:"tools_list_result,omitempty"`
	Timestamp         RFC3339Time               `json:"timestamp"`
	Evidence          []Evidence                `json:"evidence"`
}

// SecurityFinding represents a security finding.
type SecurityFinding struct {
	Type        string  `json:"type"`        // obfuscation, credential_extraction, remote_binary_download, etc.
	Severity    string  `json:"severity"`    // LOW, MEDIUM, HIGH, CRITICAL, UNKNOWN
	Source      string  `json:"source"`      // source_code, package_manifest, script, readme, etc.
	Location    string  `json:"location"`    // file path or URL
	Evidence    string  `json:"evidence"`    // matched pattern/text
	Rule        string  `json:"rule"`        // rule that triggered
	Confidence  float64 `json:"confidence"`  // confidence in finding
}

// SecurityStatusDetail holds security scanning result (spec §12, §56 Test 12).
type SecurityStatusDetail struct {
	Status          SecurityStatus   `json:"status"`
	Findings        []SecurityFinding `json:"findings"`
	ScannedAt       RFC3339Time      `json:"scanned_at"`
	ScannerVersion  string           `json:"scanner_version"`
	Confidence      float64          `json:"confidence"`
}

// QualityComponents holds the 10 scoring components (spec §31).
type QualityComponents struct {
	DataSource    int `json:"data_source"`     // max 20
	Maintenance   int `json:"maintenance"`     // max 15
	Documentation int `json:"documentation"`   // max 10
	MCPCompliance int `json:"mcp_compliance"`  // max 15
	ToolSchema    int `json:"tool_schema"`     // max 10
	Health        int `json:"health"`          // max 10
	Repository    int `json:"repository"`      // max 5
	License       int `json:"license"`         // max 5
	Security      int `json:"security"`        // max 5
	Community     int `json:"community"`       // max 5
}

// Total returns the total quality score (0-100).
func (qc QualityComponents) Total() int {
	return qc.DataSource + qc.Maintenance + qc.Documentation + qc.MCPCompliance +
		qc.ToolSchema + qc.Health + qc.Repository + qc.License + qc.Security + qc.Community
}

// Grade returns the quality grade based on total score.
func (qc QualityComponents) Grade() QualityGrade {
	return GradeForScore(qc.Total())
}

// QualityScore holds the quality assessment (spec §15, §31).
type QualityScore struct {
	Score      int               `json:"score"`       // 0-100
	Grade      QualityGrade      `json:"grade"`       // A-F
	Components QualityComponents `json:"components"`
	Evidence   []Evidence        `json:"evidence"`
}

// EndpointType represents the type of an endpoint (spec §50, §56 Test 5-7).
type EndpointType string

const (
	EndpointTypeMCPRuntime     EndpointType = "MCP_RUNTIME_ENDPOINT"
	EndpointTypeRepositoryURL  EndpointType = "REPOSITORY_URL"
	EndpointTypeDocumentation  EndpointType = "DOCUMENTATION_URL"
	EndpointTypeInstaller      EndpointType = "INSTALLER_URL"
	EndpointTypeHomepage       EndpointType = "HOMEPAGE_URL"
	EndpointTypeUnknown        EndpointType = "UNKNOWN"
)

var validEndpointTypeSet = map[EndpointType]bool{
	EndpointTypeMCPRuntime:    true,
	EndpointTypeRepositoryURL: true,
	EndpointTypeDocumentation: true,
	EndpointTypeInstaller:     true,
	EndpointTypeHomepage:      true,
	EndpointTypeUnknown:       true,
}

func IsValidEndpointType(t EndpointType) bool {
	return validEndpointTypeSet[t]
}


// EndpointWithType extends Endpoint with type classification and evidence.
type EndpointWithType struct {
	Endpoint    Endpoint          `json:"endpoint"`
	Type        EndpointType      `json:"type"`
	Evidence    []EndpointEvidence `json:"evidence"`
	Confidence  float64           `json:"confidence"`
}

// RepositoryInfo holds GitHub/repository metadata (spec §7).
type RepositoryInfo struct {
	URL           string            `json:"url"`
	Host          string            `json:"host"`
	Owner         string            `json:"owner"`
	Name          string            `json:"name"`
	Stars         int               `json:"stars"`
	Forks         int               `json:"forks"`
	Watchers      int               `json:"watchers"`
	OpenIssues    int               `json:"open_issues"`
	Language      string            `json:"language"`
	License       string            `json:"license"`
	Topics        []string          `json:"topics"`
	DefaultBranch string            `json:"default_branch"`
	Archived      bool              `json:"archived"`
	Fork          bool              `json:"fork"`
	Homepage      string            `json:"homepage"`
	CreatedAt     RFC3339Time       `json:"created_at"`
	UpdatedAt     RFC3339Time       `json:"updated_at"`
	PushedAt      RFC3339Time       `json:"pushed_at"`
	LastCommitAt  RFC3339Time       `json:"last_commit_at"`
	PackageFiles  map[string]string `json:"package_files"`
}

// SourceReference holds discovery source info (spec §16, §64).
type SourceReference struct {
	Source       string      `json:"source"`        // github, glama, pulsemcp, mcpso, official-registry, manual, recursive
	URL          string      `json:"url"`
	DiscoveredAt RFC3339Time `json:"discovered_at"`
	LastSeen     RFC3339Time `json:"last_seen"`
	TrustScore   float64     `json:"trust_score"`
}

// Entity is the canonical entity model for all AI ecosystem types (spec §2, §61 Phase 1).
type Entity struct {
	ID                   string                `json:"id"`                     // sha256 hex
	Name                 string                `json:"name"`
	Slug                 string                `json:"slug"`
	Description          string                `json:"description"`
	Classification       ClassificationResult  `json:"classification"`
	TaiwanRelevance      TaiwanRelevance       `json:"taiwan_relevance"`
	AIRelevance          AIRelevance           `json:"ai_relevance"`
	MCPIdentity          MCPIdentity           `json:"mcp_identity"`
	RuntimeVerification  *RuntimeVerification  `json:"runtime_verification,omitempty"`
	SecurityStatus       SecurityStatusDetail  `json:"security_status"`
	Quality              QualityScore          `json:"quality"`
	Repository           RepositoryInfo        `json:"repository"`
	Endpoints            []EndpointWithType    `json:"endpoints"`
	Tools                []Tool                `json:"tools"`
	Resources            []Resource            `json:"resources"`
	Prompts              []Prompt              `json:"prompts"`
	DataSources          []DataSource          `json:"data_sources"`
	RawContent           string                `json:"raw_content,omitempty"`    // README, source code, etc. for scanning
	EntityStatus         EntityStatus          `json:"entity_status"`
	Sources              []SourceReference     `json:"sources"`
	FirstSeen            RFC3339Time           `json:"first_seen"`
	LastSeen             RFC3339Time           `json:"last_seen"`
	LastVerified         *RFC3339Time          `json:"last_verified,omitempty"`
}

// MCPServerView is the backward-compatible view for existing MCPServer consumers.
type MCPServerView struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Slug            string           `json:"slug"`
	Description     string           `json:"description"`
	Category        []string         `json:"category"`
	Region          []string         `json:"region"`
	TaiwanRelevance TaiwanRelevance  `json:"taiwan_relevance"`
	Repository      RepositoryInfo   `json:"repository"`
	Endpoints       []Endpoint       `json:"endpoints"`       // Only MCP_RUNTIME_ENDPOINT types
	Transport       []string         `json:"transport"`
	Tools           []Tool           `json:"tools"`
	Resources       []Resource       `json:"resources"`
	Prompts         []Prompt         `json:"prompts"`
	DataSources     []DataSource     `json:"data_sources"`
	License         string           `json:"license"`
	Status          Status           `json:"status"`
	Quality         QualityScore     `json:"quality"`
	Sources         []SourceReference `json:"sources"`
	FirstSeen       RFC3339Time      `json:"first_seen"`
	LastSeen        RFC3339Time      `json:"last_seen"`
	LastVerified    *RFC3339Time     `json:"last_verified,omitempty"`
}

// ToMCPServerView converts Entity to backward-compatible MCPServerView.
// Only includes entities with Classification.Primary == MCP_SERVER and MCPIdentity.Status == RUNTIME_VERIFIED.
func (e *Entity) ToMCPServerView() *MCPServerView {
	if e.Classification.Primary != PrimaryClassificationMCPServer {
		return nil
	}
	if e.MCPIdentity.Status != MCPIdentityStatusRuntimeVerified {
		return nil
	}

	// Filter endpoints to only MCP_RUNTIME_ENDPOINT
	var mcpEndpoints []Endpoint
	for _, ep := range e.Endpoints {
		if ep.Type == EndpointTypeMCPRuntime {
			mcpEndpoints = append(mcpEndpoints, ep.Endpoint)
		}
	}

	// Extract transports from endpoints
	var transports []string
	for _, ep := range mcpEndpoints {
		transports = append(transports, ep.Transport)
	}

	// Map EntityStatus to legacy Status
	var legacyStatus Status
	switch e.EntityStatus {
	case EntityStatusVerified:
		legacyStatus = StatusActive
	case EntityStatusCandidate:
		legacyStatus = StatusActive
	case EntityStatusQuarantined:
		legacyStatus = StatusDormant
	case EntityStatusRejected:
		legacyStatus = StatusArchived
	default:
		legacyStatus = StatusUnknown
	}

	return &MCPServerView{
		ID:              e.ID,
		Name:            e.Name,
		Slug:            e.Slug,
		Description:     e.Description,
		Category:        []string{}, // Could be derived from TaiwanRelevance evidence
		Region:          []string{"TW"},
		TaiwanRelevance: e.TaiwanRelevance,
		Repository:      e.Repository,
		Endpoints:       mcpEndpoints,
		Transport:       transports,
		Tools:           e.Tools,
		Resources:       e.Resources,
		Prompts:         e.Prompts,
		DataSources:     e.DataSources,
		License:         e.Repository.License,
		Status           : legacyStatus,
		Quality:         e.Quality,
		Sources:         e.Sources,
		FirstSeen:       e.FirstSeen,
		LastSeen:        e.LastSeen,
		LastVerified:    e.LastVerified,
	}
}