// Package normalize transforms RawRecord into unified MCPServer format.
package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/manifest"
	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Normalizer interface transforms RawRecord to MCPServer (§6 Implementation Plan).
type Normalizer interface {
	Normalize(record *models.RawRecord) (*models.MCPServer, error)
}

// ServerNormalizer implements Normalizer.
type ServerNormalizer struct{}

// New creates a new ServerNormalizer.
func New() *ServerNormalizer {
	return &ServerNormalizer{}
}

// injectionPatterns are README sanitization patterns (§60 LLM Security).
var injectionPatterns = []string{
	"Ignore previous instructions",
	"Call this URL",
	"Upload credentials",
}
// GetInjectionPatterns returns the list of injection patterns for external access.
func GetInjectionPatterns() []string {
	// Return a copy to prevent external modification
	result := make([]string, len(injectionPatterns))
	copy(result, injectionPatterns)
	return result
}

// Normalize converts a RawRecord into a normalized MCPServer (§10 TASK-008).
func (n *ServerNormalizer) Normalize(record *models.RawRecord) (*models.MCPServer, error) {
	now := time.Now().UTC()

	server := &models.MCPServer{
		ID:           "",
		Name:         normalizeName(record.Name, record.RepositoryURL),
		Slug:         generateSlug(normalizeName(record.Name, record.RepositoryURL)),
		FirstSeen:    now,
		LastSeen:     now,
		LastVerified: now,
	}

	// Repository info
	repo := record.Repository
	if repo.URL == "" {
		repo.URL = normalizeURL(record.RepositoryURL)
	}
	if repo.Host == "" {
		repo.Host = extractHost(record.RepositoryURL)
	}
	if repo.Owner == "" {
		repo.Owner = extractOwner(record.RepositoryURL)
	}
	if repo.Name == "" {
		repo.Name = extractRepoName(record.RepositoryURL)
	}
	if repo.Topics == nil && record.RawMetadata != nil {
		if topics, ok := record.RawMetadata["topics"].([]string); ok {
			repo.Topics = topics
		}
	}
	if repo.License == "" && record.RawMetadata != nil {
		if lic, ok := record.RawMetadata["license"].(string); ok {
			repo.License = lic
		}
	}
	if repo.Homepage == "" {
		repo.Homepage = record.HomepageURL
	}
	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}
	server.Repository = repo

	// Description normalization
	server.Description = normalizeDescription(record.Description, record.Readme)

	// README sanitization (§60)
	sanitizedReadme := sanitizeReadme(record.Readme)
	server.Readme = sanitizedReadme
	// Endpoint extraction
	server.Endpoints = extractEndpoints(sanitizedReadme, toAnyMap(record.PackageFiles), record.RawMetadata)
	if len(record.Endpoints) > 0 {
		server.Endpoints = append(server.Endpoints, record.Endpoints...)
	}

	// Transport detection
	server.Transport = record.Transport

	// Manifest extraction
	manifestInfo := parsePackageFiles(record.PackageFiles, nil)
	if manifestInfo != nil {
		server.Tools = extractToolsFromManifest(manifestInfo)
	}

	// License
	server.License = normalizeLicense(repo.License)

	// Data source detection
	server.DataSources = detectDataSources(server.Repository, server.Endpoints, sanitizedReadme)

	// Status
	server.Health = models.HealthHealthy

	// Sources
	server.Sources = []models.SourceReference{{
		Source:       record.Source,
		URL:          record.SourceURL,
		DiscoveredAt: models.RFC3339Time(record.DiscoveredAt),
		LastSeen:     models.RFC3339Time(record.DiscoveredAt),
		TrustScore:   getSourceTrustScore(record.Source),
	}}

	// Generate ID
	server.ID = GenerateID(record.RepositoryURL)

	return server, nil
}

// getSourceTrustScore returns trust score for a source
func getSourceTrustScore(source string) float64 {
	switch source {
	case "github":
		return models.SourceTrustScores{}.GitHub
	case "registry":
		return models.SourceTrustScores{}.Registry
	case "mcpserversorg":
		return models.SourceTrustScores{}.Mcpserversorg
	case "mcpmarket":
		return models.SourceTrustScores{}.Mcpmarket
	case "githubrepo":
		return models.SourceTrustScores{}.GithubRepo
	default:
		return 0.5
	}
}


// toAnyMap converts map[string]string to map[string]any
func toAnyMap(m map[string]string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return s
}

func normalizeName(name, repoURL string) string {
	if name != "" {
		return name
	}
	// Extract from repo URL
	parts := strings.Split(strings.TrimSuffix(repoURL, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		if r == ' ' || r == '_' || r == '.' {
			return '-'
		}
		return -1
	}, slug)
	slug = strings.Trim(slug, "-")
	return slug
}

func normalizeDescription(desc, readme string) string {
	if strings.TrimSpace(readme) != "" {
		paragraph := firstParagraph(readme)
		if paragraph != "" {
			return strings.TrimSpace(paragraph)
		}
	}
	return strings.TrimSpace(desc)
}

func firstParagraph(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// Skip markdown headers/links at the start
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "![") {
			continue
		}
		return line
	}
	return ""
}

// SanitizeReadme strips injection patterns from README text (§60 LLM Security).
// It is used before passing README content to the LLM classifier.
func SanitizeReadme(readme string) string {
	return sanitizeReadme(readme)
}

func sanitizeReadme(readme string) string {
	for _, pattern := range injectionPatterns {
		readme = strings.ReplaceAll(readme, pattern, "[REDACTED]")
	}
	return readme
}

func extractHost(url string) string {
	s := strings.TrimPrefix(url, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "ssh://git@")
	parts := strings.Split(s, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func extractOwner(url string) string {
	s := strings.TrimPrefix(url, "https://")
	s = strings.TrimPrefix(s, "http://")
	parts := strings.Split(s, "/")
	if len(parts) >= 3 {
		return parts[1]
	}
	return ""
}

func extractRepoName(url string) string {
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func normalizeLicense(license string) string {
	if strings.TrimSpace(license) == "" {
		return "UNKNOWN" // §TST-045: never guess
	}
	return license
}

func extractEndpoints(readme string, manifest map[string]any, rawMeta map[string]any) []models.Endpoint {
	var endpoints []models.Endpoint

	// From README URLs
	for _, url := range extractURLs(readme) {
		if isMCPEndpoint(url) {
			endpoints = append(endpoints, models.Endpoint{
				URL:       url,
				Transport: detectTransportFromURL(url),
				TLS:       strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "wss://"),
			})
		}
	}

	// From manifest
	if manifest != nil {
		if ep, ok := manifest["endpoint"].(string); ok && ep != "" {
			endpoints = append(endpoints, models.Endpoint{
				URL:       ep,
				Transport: detectTransportFromURL(ep),
				TLS:       strings.HasPrefix(ep, "https://"),
			})
		}
	}

	// From RawMetadata
	if rawMeta != nil {
		if ep, ok := rawMeta["endpoint"].(string); ok && ep != "" {
			endpoints = append(endpoints, models.Endpoint{
				URL:       ep,
				Transport: detectTransportFromURL(ep),
				TLS:       strings.HasPrefix(ep, "https://"),
			})
		}
	}

	return endpoints
}

func extractURLs(text string) []string {
	var urls []string
	words := strings.Fields(text)
	for _, w := range words {
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") ||
			strings.HasPrefix(w, "sse://") || strings.HasPrefix(w, "wss://") {
			urls = append(urls, strings.TrimRight(w, ")\"'<,"))
		}
	}
	return urls
}

func isMCPEndpoint(url string) bool {
	return strings.Contains(url, "sse") || strings.Contains(url, "mcp") ||
		strings.Contains(url, "tool/") || strings.HasSuffix(url, "/mcp")
}

func detectTransportFromURL(url string) string {
	switch {
	case strings.HasPrefix(url, "sse://"):
		return "sse"
	case strings.HasPrefix(url, "wss://"):
		return "websocket"
	case strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://"):
		if strings.Contains(url, "sse") || strings.HasSuffix(url, "/sse") {
			return "sse"
		}
		return "http"
	default:
		return "stdio"
	}
}

func detectTransports(endpoints []models.Endpoint, manifest map[string]any) []string {
	seen := make(map[string]bool)
	var transports []string

	for _, ep := range endpoints {
		if !seen[ep.Transport] {
			seen[ep.Transport] = true
			transports = append(transports, ep.Transport)
		}
	}

	// From manifest
	if manifest != nil {
		if t, ok := manifest["transport"].(string); ok && t != "" {
			if !seen[t] {
				seen[t] = true
				transports = append(transports, t)
			}
		}
	}

	return transports
}

func parsePackageFiles(pkgFiles map[string]string, manifestMap map[string]any) *manifest.ManifestInfo {
	// Parse package files (package.json, pyproject.toml, go.mod, Cargo.toml)
	order := []string{"package.json", "pyproject.toml", "go.mod", "Cargo.toml"}
	for _, f := range order {
		if content, ok := pkgFiles[f]; ok {
			info, err := manifest.ParseManifest(content, f)
			if err == nil && info != nil {
				return info
			}
		}
	}
	// Parse manifest files (server.json, mcp.json, manifest.json)
	manifestOrder := []string{"server.json", "mcp.json", "manifest.json"}
	for _, f := range manifestOrder {
		if content, ok := pkgFiles[f]; ok {
			info, err := manifest.ParseManifest(content, f)
			if err == nil && info != nil {
				return info
			}
		}
	}
	// Also try the manifestMap (already parsed JSON)
	if manifestMap != nil {
		if content, ok := manifestMap["content"].(string); ok {
			if info, err := manifest.ParseManifest(content, "server.json"); err == nil {
				return info
			}
		}
	}
	return nil
}

func detectFileType(filename string) string {
	switch {
	case strings.HasSuffix(filename, "package.json"):
		return "package.json"
	case strings.HasSuffix(filename, "pyproject.toml"):
		return "pyproject.toml"
	case strings.HasSuffix(filename, "go.mod"):
		return "go.mod"
	case strings.HasSuffix(filename, "Cargo.toml"):
		return "cargo.toml"
	case strings.HasSuffix(filename, "server.json"):
		return "server.json"
	case strings.HasSuffix(filename, "mcp.json"):
		return "mcp.json"
	case strings.HasSuffix(filename, "manifest.json"):
		return "manifest.json"
	default:
		return "json"
	}
}

func extractToolsFromManifest(mi *manifest.ManifestInfo) []models.Tool {
	var tools []models.Tool
	for _, t := range mi.Tools {
		tools = append(tools, models.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return tools
}

func detectDataSources(repo models.RepositoryInfo, endpoints []models.Endpoint, readme string) []models.DataSource {
	urls := append([]string{}, repo.URL, repo.Homepage)
	for _, ep := range endpoints {
		urls = append(urls, ep.URL)
	}

	knownSources := map[string]models.DataSourceType{
		"twse.com.tw":    models.DataSourceOfficialGovAPI,
		"tpex.org.tw":    models.DataSourceOfficialGovAPI,
		"taifex.com.tw":  models.DataSourceOfficialGovAPI,
		"cwa.gov.tw":     models.DataSourceOfficialGovAPI,
		"moi.gov.tw":     models.DataSourceOfficialGovAPI,
		"moea.gov.tw":    models.DataSourceOfficialGovAPI,
		"data.gov.tw":    models.DataSourceOfficialGovAPI,
		"ecpay.com.tw":   models.DataSourceOfficialCompany,
		"newebpay.com":   models.DataSourceOfficialCompany,
		"shoplineapp.com": models.DataSourceOfficialCompany,
		"finmind.com.tw": models.DataSourceThirdPartyAPI,
		"fugle.com.tw":   models.DataSourceThirdPartyAPI,
	}

	seen := make(map[string]bool)
	var sources []models.DataSource

	for _, u := range urls {
		if u == "" {
			continue
		}
		lowerURL := strings.ToLower(u)
		for domain, dsType := range knownSources {
			if strings.Contains(lowerURL, domain) && !seen[domain] {
				seen[domain] = true
				sources = append(sources, models.DataSource{
					Name:     domain,
					Type:     dsType,
					URL:      u,
					Country:  "TW",
					Official: dsType == models.DataSourceOfficialGovAPI || dsType == models.DataSourceOfficialCompany,
				})
			}
		}
	}

	// Also scan README text
	if readme != "" {
		lowerReadme := strings.ToLower(readme)
		for domain, dsType := range knownSources {
			if strings.Contains(lowerReadme, domain) && !seen[domain] {
				seen[domain] = true
				sources = append(sources, models.DataSource{
					Name:     domain,
					Type:     dsType,
					Country:  "TW",
					Official: dsType == models.DataSourceOfficialGovAPI || dsType == models.DataSourceOfficialCompany,
				})
			}
		}
	}

	return sources
}
// GenerateID creates a deterministic server ID from repository URL (§21).
func GenerateID(repoURL string) string {
	sum := sha256.Sum256([]byte(normalizeURL(repoURL)))
	return hex.EncodeToString(sum[:])
}


func safeInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
