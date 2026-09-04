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
	Normalize(record models.RawRecord) (*models.MCPServer, error)
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

// Normalize converts a RawRecord into a normalized MCPServer (§10 TASK-008).
func (n *ServerNormalizer) Normalize(record models.RawRecord) (*models.MCPServer, error) {
	now := time.Now().UTC()

	server := &models.MCPServer{
		ID:           "",
		Name:         normalizeName(record.Name, record.RepositoryURL),
		Slug:         generateSlug(normalizeName(record.Name, record.RepositoryURL)),
		FirstSeen:    now,
		LastSeen:     now,
		LastVerified: now,
	}

	// Repository metadata mapping (§7)
	server.Repository = models.RepositoryInfo{
		URL:           normalizeURL(record.RepositoryURL),
		Host:          extractHost(record.RepositoryURL),
		Owner:         extractOwner(record.RepositoryURL),
		Name:          extractRepoName(record.RepositoryURL),
		Stars:         record.Stars,
		Topics:        record.Topics,
		Language:      "",
		License:       normalizeLicense(record.License),
		DefaultBranch: "main",
		Homepage:      record.HomepageURL,
	}

	// Description normalization
	server.Description = normalizeDescription(record.Description, record.Readme)

	// README sanitization (§60)
	sanitizedReadme := sanitizeReadme(record.Readme)

	// Endpoint extraction from README, manifest, RawMetadata (§8)
	server.Endpoints = extractEndpoints(sanitizedReadme, record.Manifest, record.RawMetadata)

	// Transport detection
	server.Transport = detectTransports(server.Endpoints, record.Manifest)

	// Manifest extraction (§9, §11)
	manifestInfo := parseRecordManifest(record.Manifest)
	if manifestInfo != nil {
		server.Tools = extractToolsFromManifest(manifestInfo)
	}

	// License
	server.License = normalizeLicense(record.License)

	// Data source detection
	server.DataSources = detectDataSources(server.Repository, server.Endpoints, sanitizedReadme)

	// Status
	server.Health = models.HealthHealthy

	// Sources
	server.Sources = []models.SourceReference{{
		Source:       record.Source,
		URL:          record.SourceURL,
		DiscoveredAt: record.FetchedAt,
		LastSeen:     record.FetchedAt,
		TrustScore:   models.SourceTrustScores[record.Source],
	}}

	return server, nil
}

// Normalizer interface implementation
var _ Normalizer = (*ServerNormalizer)(nil)

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

func parseRecordManifest(manifestMap map[string]any) *manifest.ManifestInfo {
	if manifestMap == nil {
		return nil
	}
	// Find the manifest file content
	for key, val := range manifestMap {
		if content, ok := val.(string); ok {
			fileType := detectFileType(key)
			if info, err := manifest.ParseManifest(content, fileType); err == nil {
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
		return "Cargo.toml"
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

