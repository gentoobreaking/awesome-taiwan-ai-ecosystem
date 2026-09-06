package engines

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// EndpointClassifier classifies endpoint URLs into their types.
type EndpointClassifier struct {
	docPatterns       []*regexp.Regexp
	installerPatterns []*regexp.Regexp
	repoPatterns      []*regexp.Regexp
	homepagePatterns  []*regexp.Regexp
}

// NewEndpointClassifier creates a new endpoint classifier with pre-compiled patterns.
func NewEndpointClassifier() *EndpointClassifier {
	return &EndpointClassifier{
		docPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^https?://docs\.[^/]+\.`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.readthedocs\.(io|org)`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.gitbook\.(io|com)`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.notion\.site`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.notion\.so`),
			regexp.MustCompile(`(?i)^https?://[^/]+/wiki(?:/|$)`),
			regexp.MustCompile(`(?i)^https?://[^/]+/docs/`),
			regexp.MustCompile(`(?i)^https?://[^/]+/documentation/`),
			regexp.MustCompile(`(?i)^https?://[^/]+/README`),
			regexp.MustCompile(`(?i)^https?://[^/]+/.*\.md([?#]|$)`),
			regexp.MustCompile(`(?i)^https?://github\.com/[^/]+/[^/]+#readme`),
			regexp.MustCompile(`(?i)^https?://github\.com/[^/]+/[^/]+/wiki`),
		},
		repoPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^https?://github\.com/[^/#?]+/[^/#?]+/?$`),
			regexp.MustCompile(`(?i)^https?://github\.com/[^/#?]+/[^/#?]+/tree/`),
			regexp.MustCompile(`(?i)^https?://github\.com/[^/#?]+/[^/#?]+/blob/`),
			regexp.MustCompile(`(?i)^https?://gitlab\.com/[^/]+/[^/]+/?$`),
			regexp.MustCompile(`(?i)^https?://gitlab\.com/[^/]+/[^/]+/-/tree/`),
			regexp.MustCompile(`(?i)^https?://bitbucket\.org/[^/]+/[^/]+/?$`),
			regexp.MustCompile(`(?i)^https?://sourceforge\.net/projects/[^/]+.*`),
			regexp.MustCompile(`(?i)^git@github\.com:[^/]+/[^/]+\.git$`),
			regexp.MustCompile(`(?i)^git@gitlab\.com:[^/]+/[^/]+\.git$`),
		},
		installerPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)install\.sh$`),
			regexp.MustCompile(`(?i)install\.ps1$`),
			regexp.MustCompile(`(?i)setup\.sh$`),
			regexp.MustCompile(`(?i)setup\.ps1$`),
			regexp.MustCompile(`(?i)get\.sh$`),
			regexp.MustCompile(`(?i)bootstrap\.sh$`),
			regexp.MustCompile(`(?i)raw\.githubusercontent\.com/.*/(install|setup|get|bootstrap)\.(sh|ps1)`),
			regexp.MustCompile(`(?i)^https?://get\.[^/]+\.sh`),
			regexp.MustCompile(`(?i)^https?://install\.[^/]+\.sh`),
			regexp.MustCompile(`(?i)^https?://[^/]+/install\.sh`),
			regexp.MustCompile(`(?i)^https?://[^/]+/scripts/install`),
			regexp.MustCompile(`(?i)curl.*\|\s*(sh|bash)`),
			regexp.MustCompile(`(?i)wget.*\|\s*(bash|sh)`),
		},
		homepagePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^https?://[^/]+\.io/?$`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.dev/?$`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.app/?$`),
			regexp.MustCompile(`(?i)^https?://[^/]+\.com/?$`),
		},
	}
}

// ClassifyEndpoints classifies all endpoints for an entity.
// Returns a slice of EndpointWithType with type, evidence, and confidence.
func (ec *EndpointClassifier) ClassifyEndpoints(entity *models.Entity) []models.EndpointWithType {
	var results []models.EndpointWithType
	now := models.RFC3339Time(time.Now().UTC())
	seen := make(map[string]bool)

	// 1. Classify existing endpoints from entity.Endpoints
	for _, ep := range entity.Endpoints {
		if seen[ep.Endpoint.URL] {
			continue
		}
		seen[ep.Endpoint.URL] = true

		epType, evidence, confidence := ec.classifySingleURL(ep.Endpoint.URL, entity, "endpoint")
		results = append(results, models.EndpointWithType{
			Endpoint:   ep.Endpoint,
			Type:       epType,
			Evidence:   evidence,
			Confidence: confidence,
		})
	}

	// 2. Classify repository URL
	if entity.Repository.URL != "" && !seen[entity.Repository.URL] {
		seen[entity.Repository.URL] = true
		epType, evidence, confidence := ec.classifySingleURL(entity.Repository.URL, entity, "repository_info")
		results = append(results, models.EndpointWithType{
			Endpoint: models.Endpoint{
				URL:       entity.Repository.URL,
				Transport: "",
			},
			Type:       epType,
			Evidence:   evidence,
			Confidence: confidence,
		})
	}

	// 3. Classify homepage URL
	if entity.Repository.Homepage != "" && !seen[entity.Repository.Homepage] {
		seen[entity.Repository.Homepage] = true
		epType, evidence, confidence := ec.classifySingleURL(entity.Repository.Homepage, entity, "repository_info")
		results = append(results, models.EndpointWithType{
			Endpoint: models.Endpoint{
				URL:       entity.Repository.Homepage,
				Transport: "",
			},
			Type:       epType,
			Evidence:   evidence,
			Confidence: confidence,
		})
	}

	// 4. Extract and classify URLs from RawContent (README, source code, etc.)
	if entity.RawContent != "" {
		urls := ec.extractURLs(entity.RawContent)
		for _, u := range urls {
			if !seen[u] {
				seen[u] = true
				epType, evidence, confidence := ec.classifySingleURL(u, entity, "raw_content")
				results = append(results, models.EndpointWithType{
					Endpoint: models.Endpoint{
						URL:       u,
						Transport: "",
					},
					Type:       epType,
					Evidence:   evidence,
					Confidence: confidence,
				})
			}
		}
	}

	// 5. Check for MCP_RUNTIME_ENDPOINT from runtime verification
	if entity.RuntimeVerification != nil && entity.RuntimeVerification.Status == models.RuntimeVerificationStatusPassed {
		// Find or create the runtime endpoint
		hasRuntime := false
		for i := range results {
			if results[i].Type == models.EndpointTypeMCPRuntime {
				// Update confidence if we have runtime verification
				results[i].Confidence = 1.0
				results[i].Evidence = append(results[i].Evidence, models.EndpointEvidence{
					Rule:        "runtime_verified",
					Source:      "runtime_handshake",
					Location:    "mcp_protocol",
					MatchedText: "MCP initialize + tools/list handshake passed",
					Pattern:     "mcp_handshake",
					Confidence:  1.0,
					Timestamp:   now,
				})
				hasRuntime = true
				break
			}
		}
		if !hasRuntime && len(entity.Endpoints) > 0 {
			// Use the first endpoint as the runtime endpoint
			ep := entity.Endpoints[0]
			results = append(results, models.EndpointWithType{
				Endpoint:   ep.Endpoint,
				Type:       models.EndpointTypeMCPRuntime,
				Evidence: []models.EndpointEvidence{
					{
						Rule:        "runtime_verified",
						Source:      "runtime_handshake",
						Location:    "mcp_protocol",
						MatchedText: "MCP initialize + tools/list handshake passed",
						Pattern:     "mcp_handshake",
						Confidence:  1.0,
						Timestamp:   now,
					},
				},
				Confidence: 1.0,
			})
		}
	}

	return results
}

// classifySingleURL classifies a single URL and returns type, evidence, and confidence.
func (ec *EndpointClassifier) classifySingleURL(rawURL string, entity *models.Entity, source string) (models.EndpointType, []models.EndpointEvidence, float64) {
	var normalized string
	u, err := url.Parse(rawURL)
	if err != nil {
		// For SSH URLs like git@github.com:owner/repo.git, use raw URL
		normalized = strings.ToLower(rawURL)
	} else {
		normalized = strings.ToLower(u.String())
	}

	var evidence []models.EndpointEvidence
	now := models.RFC3339Time(time.Now().UTC())

	// Track all matching patterns for evidence aggregation
	type match struct {
		endpointType models.EndpointType
		rule         string
		pattern      *regexp.Regexp
		confidence   float64
		source       string
		matchedText  string
	}
	var matches []match

	// Check DOCUMENTATION_URL patterns FIRST (more specific)
	for _, pattern := range ec.docPatterns {
		if pattern.MatchString(normalized) {
			rule := ec.getDocPatternRule(pattern)
			matches = append(matches, match{
				endpointType: models.EndpointTypeDocumentation,
				rule:         rule,
				pattern:      pattern,
				confidence:   ec.getConfidenceForRule(rule),
				source:       source,
				matchedText:  pattern.String(),
			})
		}
	}

	// Check INSTALLER_URL patterns
	for _, pattern := range ec.installerPatterns {
		if pattern.MatchString(normalized) {
			rule := ec.getInstallerPatternRule(pattern)
			matches = append(matches, match{
				endpointType: models.EndpointTypeInstaller,
				rule:         rule,
				pattern:      pattern,
				confidence:   ec.getConfidenceForRule(rule),
				source:       source,
				matchedText:  pattern.String(),
			})
		}
	}

	// Check REPOSITORY_URL patterns
	for _, pattern := range ec.repoPatterns {
		if pattern.MatchString(normalized) {
			rule := ec.getRepoPatternRule(pattern)
			matches = append(matches, match{
				endpointType: models.EndpointTypeRepositoryURL,
				rule:         rule,
				pattern:      pattern,
				confidence:   ec.getConfidenceForRule(rule),
				source:       source,
				matchedText:  pattern.String(),
			})
		}
	}

	// Check HOMEPAGE_URL - if it matches repository homepage
	if entity.Repository.Homepage != "" && strings.EqualFold(normalized, strings.ToLower(entity.Repository.Homepage)) {
		matches = append(matches, match{
			endpointType: models.EndpointTypeHomepage,
			rule:         "homepage_match",
			pattern:      nil,
			confidence:   0.9,
			source:       source,
			matchedText:  entity.Repository.Homepage,
		})
	}

	// Check for potential MCP runtime endpoint patterns (static analysis)
	if ec.isPotentialMCPRuntimeEndpoint(normalized, entity) {
		matches = append(matches, match{
			endpointType: models.EndpointTypeMCPRuntime,
			rule:         "potential_mcp_runtime_static",
			pattern:      nil,
			confidence:   0.4,
			source:       source,
			matchedText:  "Potential MCP runtime endpoint from static analysis",
		})
	}

	// If no matches, return UNKNOWN
	if len(matches) == 0 {
		evidence = append(evidence, models.EndpointEvidence{
			Rule:        "unknown",
			Source:      source,
			Location:    normalized,
			MatchedText: rawURL,
			Pattern:     "no_match",
			Confidence:  0.1,
			Timestamp:   now,
		})
		return models.EndpointTypeUnknown, evidence, 0.1
	}

	// Group matches by endpoint type
	typeGroups := make(map[models.EndpointType][]match)
	for _, m := range matches {
		typeGroups[m.endpointType] = append(typeGroups[m.endpointType], m)
	}

	// Select the endpoint type with highest total confidence
	var bestType models.EndpointType
	var bestConfidence float64
	for epType, typeMatches := range typeGroups {
		totalConf := 0.0
		for _, m := range typeMatches {
			totalConf += m.confidence
		}
		if totalConf > bestConfidence {
			bestConfidence = totalConf
			bestType = epType
		}
	}

	// Build evidence for the best type
	bestMatches := typeGroups[bestType]
	for _, m := range bestMatches {
		patternStr := ""
		if m.pattern != nil {
			patternStr = m.pattern.String()
		} else {
			patternStr = m.matchedText
		}
		evidence = append(evidence, models.EndpointEvidence{
			Rule:        m.rule,
			Source:      m.source,
			Location:    normalized,
			MatchedText: m.matchedText,
			Pattern:     patternStr,
			Confidence:  m.confidence,
			Timestamp:   now,
		})
	}

	// Calculate aggregated confidence (capped at 1.0)
	if bestConfidence > 1.0 {
		bestConfidence = 1.0
	}

	return bestType, evidence, bestConfidence
}

type match struct {
	endpointType models.EndpointType
	rule         string
	pattern      *regexp.Regexp
	confidence   float64
	source       string
	matchedText  string
}

// getDocPatternRule returns a specific rule name for documentation patterns.
func (ec *EndpointClassifier) getDocPatternRule(pattern *regexp.Regexp) string {
	patternStr := pattern.String()
	switch {
	case strings.Contains(patternStr, "github") && strings.Contains(patternStr, "wiki"):
		return "github_wiki"
	case strings.Contains(patternStr, "docs\\."):
		return "docs_subdomain"
	case strings.Contains(patternStr, "readthedocs"):
		return "readthedocs_domain"
	case strings.Contains(patternStr, "gitbook"):
		return "gitbook_domain"
	case strings.Contains(patternStr, "notion"):
		return "notion_domain"
	case strings.Contains(patternStr, "/docs/"):
		return "docs_path"
	case strings.Contains(patternStr, "/documentation/"):
		return "documentation_path"
	case strings.Contains(patternStr, "/wiki"):
		return "wiki_path"
	case strings.Contains(patternStr, "/README") || strings.Contains(patternStr, "#readme"):
		return "readme_anchor"
	case strings.Contains(patternStr, "\\.md"):
		return "markdown_file"
	default:
		return "documentation_url_pattern"
	}
}

// getInstallerPatternRule returns a specific rule name for installer patterns.
func (ec *EndpointClassifier) getInstallerPatternRule(pattern *regexp.Regexp) string {
	patternStr := pattern.String()
	switch {
	case strings.Contains(patternStr, "install\\.sh") || strings.Contains(patternStr, "setup\\.sh") || strings.Contains(patternStr, "bootstrap\\.sh"):
		return "install_script_name"
	case strings.Contains(patternStr, "raw.githubusercontent"):
		return "raw_github_install"
	case strings.Contains(patternStr, "get\\.") || strings.Contains(patternStr, "install\\."):
		return "install_domain"
	case strings.Contains(patternStr, "curl.*\\|") || strings.Contains(patternStr, "wget.*\\|"):
		return "pipe_install"
	default:
		return "installer_url_pattern"
	}
}

// getRepoPatternRule returns a specific rule name for repository patterns.
func (ec *EndpointClassifier) getRepoPatternRule(pattern *regexp.Regexp) string {
	patternStr := pattern.String()
	switch {
	case strings.Contains(patternStr, "github\\.com"):
		return "github_repo_pattern"
	case strings.Contains(patternStr, "gitlab\\.com"):
		return "gitlab_repo_pattern"
	case strings.Contains(patternStr, "bitbucket\\.org"):
		return "bitbucket_repo_pattern"
	case strings.Contains(patternStr, "sourceforge"):
		return "sourceforge_repo_pattern"
	case strings.Contains(patternStr, "git@"):
		return "git_ssh_pattern"
	default:
		return "repository_url_pattern"
	}
}

// getConfidenceForRule returns confidence based on rule name.
func (ec *EndpointClassifier) getConfidenceForRule(rule string) float64 {
	confidenceMap := map[string]float64{
		"runtime_verified":         1.0,
		"github_repo_pattern":      0.95,
		"gitlab_repo_pattern":      0.95,
		"bitbucket_repo_pattern":   0.95,
		"sourceforge_repo_pattern": 0.9,
		"git_ssh_pattern":          0.95,
		"docs_subdomain":           0.9,
		"readthedocs_domain":       0.9,
		"gitbook_domain":           0.9,
		"notion_domain":            0.9,
		"docs_path":                0.85,
		"documentation_path":       0.85,
		"wiki_path":                0.85,
		"readme_anchor":            0.85,
		"markdown_file":            0.8,
		"github_wiki":              0.85,
		"install_script_name":      0.9,
		"raw_github_install":       0.9,
		"install_domain":           0.85,
		"pipe_install":             0.85,
		"homepage_match":           0.9,
		"potential_mcp_runtime_static": 0.4,
	}
	if c, ok := confidenceMap[rule]; ok {
		return c
	}
	return 0.5
}

// isPotentialMCPRuntimeEndpoint checks if a URL looks like an MCP runtime endpoint
// based on static analysis patterns (not from registry/docs).
func (ec *EndpointClassifier) isPotentialMCPRuntimeEndpoint(normalized string, entity *models.Entity) bool {
	// Check entity raw content for hardcoded server addresses
	if entity.RawContent == "" {
		return false
	}
	content := strings.ToLower(entity.RawContent)

	// Look for localhost/server address patterns in source code
	// e.g., "localhost:3000", "127.0.0.1:8080", "unix:///tmp/mcp.sock", "stdio"
	runtimePatterns := []string{
		"localhost:",
		"127.0.0.1:",
		"0.0.0.0:",
		"unix://",
		"stdio",
		"streamable-http",
		"sse",
		"mcp server",
		"mcpserver",
		"newmcp",
		"mcp.NewServer",
		"server.Listen",
		"transport",
	}

	for _, pattern := range runtimePatterns {
		if strings.Contains(content, pattern) {
			// Additional check: URL should look like an API endpoint, not a repo/doc
			if strings.HasPrefix(normalized, "http://localhost") ||
				strings.HasPrefix(normalized, "http://127.0.0.1") ||
				strings.HasPrefix(normalized, "http://0.0.0.0") ||
				strings.HasPrefix(normalized, "stdio:") ||
				strings.HasPrefix(normalized, "https://") && (strings.Contains(normalized, "/mcp") || strings.Contains(normalized, "/api") || strings.Contains(normalized, "/rpc")) {
				return true
			}
		}
	}
	return false
}

// extractURLs extracts all URLs from text content.
func (ec *EndpointClassifier) extractURLs(text string) []string {
	urlRegex := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
	matches := urlRegex.FindAllString(text, -1)

	// Clean up trailing punctuation
	var urls []string
	for _, m := range matches {
		m = strings.TrimRight(m, ".,;:)]}>")
		urls = append(urls, m)
	}
	return urls
}

// GetMCPRuntimeEndpoints returns only endpoints classified as MCP_RUNTIME_ENDPOINT.
func (ec *EndpointClassifier) GetMCPRuntimeEndpoints(entity *models.Entity) []models.Endpoint {
	allEndpoints := ec.ClassifyEndpoints(entity)
	var runtimeEndpoints []models.Endpoint
	for _, ep := range allEndpoints {
		if ep.Type == models.EndpointTypeMCPRuntime && ep.Confidence > 0.5 {
			runtimeEndpoints = append(runtimeEndpoints, ep.Endpoint)
		}
	}
	return runtimeEndpoints
}