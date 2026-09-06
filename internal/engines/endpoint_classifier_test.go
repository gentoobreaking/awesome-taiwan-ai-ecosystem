package engines

import (
	"strings"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)
func TestEndpointClassifier_RepositoryURL(t *testing.T) {
	ec := NewEndpointClassifier()

	testCases := []struct {
		name     string
		url      string
		expected models.EndpointType
		minConf  float64
	}{
		{
			name:     "GitHub repo root",
			url:      "https://github.com/owner/repo",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "GitHub repo with trailing slash",
			url:      "https://github.com/owner/repo/",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "GitHub repo tree",
			url:      "https://github.com/owner/repo/tree/main",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "GitHub repo blob",
			url:      "https://github.com/owner/repo/blob/main/file.go",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "GitLab repo",
			url:      "https://gitlab.com/owner/repo",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "GitLab repo tree",
			url:      "https://gitlab.com/owner/repo/-/tree/main",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "Bitbucket repo",
			url:      "https://bitbucket.org/owner/repo",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "Git SSH URL",
			url:      "git@github.com:owner/repo.git",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
		{
			name:     "SourceForge",
			url:      "https://sourceforge.net/projects/myproject",
			expected: models.EndpointTypeRepositoryURL,
			minConf:  0.9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entity := &models.Entity{
				Repository: models.RepositoryInfo{URL: tc.url},
			}
			results := ec.ClassifyEndpoints(entity)

			found := false
			for _, r := range results {
				if r.Endpoint.URL == tc.url {
					found = true
					if r.Type != tc.expected {
						t.Errorf("Expected %s, got %s", tc.expected, r.Type)
					}
					if r.Confidence < tc.minConf {
						t.Errorf("Expected confidence >= %.2f, got %.2f", tc.minConf, r.Confidence)
					}
					if len(r.Evidence) == 0 {
						t.Error("Expected evidence")
					}
					break
				}
			}
			if !found {
				t.Errorf("URL not found in results: %s", tc.url)
			}
		})
	}
}

func TestEndpointClassifier_DocumentationURL(t *testing.T) {
	ec := NewEndpointClassifier()

	testCases := []struct {
		name     string
		url      string
		expected models.EndpointType
		minConf  float64
	}{
		{
			name:     "docs subdomain",
			url:      "https://docs.example.com",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "readthedocs.io",
			url:      "https://myproject.readthedocs.io",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "readthedocs.org",
			url:      "https://myproject.readthedocs.org",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "gitbook.io",
			url:      "https://myproject.gitbook.io",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "gitbook.com",
			url:      "https://myproject.gitbook.com",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "notion.site",
			url:      "https://myproject.notion.site",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "notion.so",
			url:      "https://myproject.notion.so",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "/docs/ path",
			url:      "https://example.com/docs/getting-started",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "/documentation/ path",
			url:      "https://example.com/documentation/api",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "/wiki/ path",
			url:      "https://example.com/wiki",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     ".md file",
			url:      "https://example.com/README.md",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "GitHub README anchor",
			url:      "https://github.com/owner/repo#readme",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
		{
			name:     "GitHub wiki",
			url:      "https://github.com/owner/repo/wiki",
			expected: models.EndpointTypeDocumentation,
			minConf:  0.8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entity := &models.Entity{
				Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
				RawContent: tc.url,
			}
			results := ec.ClassifyEndpoints(entity)

			found := false
			for _, r := range results {
				if r.Endpoint.URL == tc.url {
					found = true
					if r.Type != tc.expected {
						t.Errorf("Expected %s, got %s", tc.expected, r.Type)
					}
					if r.Confidence < tc.minConf {
						t.Errorf("Expected confidence >= %.2f, got %.2f", tc.minConf, r.Confidence)
					}
					break
				}
			}
			if !found {
				t.Errorf("URL not found in results: %s", tc.url)
			}
		})
	}
}

func TestEndpointClassifier_InstallerURL(t *testing.T) {
	ec := NewEndpointClassifier()

	testCases := []struct {
		name     string
		url      string
		expected models.EndpointType
		minConf  float64
	}{
		{
			name:     "install.sh",
			url:      "https://example.com/install.sh",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
		{
			name:     "install.ps1",
			url:      "https://example.com/install.ps1",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
		{
			name:     "setup.sh",
			url:      "https://example.com/setup.sh",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
		{
			name:     "raw.githubusercontent install.sh",
			url:      "https://raw.githubusercontent.com/owner/repo/main/install.sh",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
		{
			name:     "get.sh",
			url:      "https://get.example.com/install.sh",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
		{
			name:     "bootstrap.sh",
			url:      "https://example.com/bootstrap.sh",
			expected: models.EndpointTypeInstaller,
			minConf:  0.8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entity := &models.Entity{
				Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
				RawContent: tc.url,
			}
			results := ec.ClassifyEndpoints(entity)

			found := false
			for _, r := range results {
				if r.Endpoint.URL == tc.url {
					found = true
					if r.Type != tc.expected {
						t.Errorf("Expected %s, got %s", tc.expected, r.Type)
					}
					if r.Confidence < tc.minConf {
						t.Errorf("Expected confidence >= %.2f, got %.2f", tc.minConf, r.Confidence)
					}
					break
				}
			}
			if !found {
				t.Errorf("URL not found in results: %s", tc.url)
			}
		})
	}
}

func TestEndpointClassifier_HomepageURL(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/owner/repo",
			Homepage: "https://myproject.io",
		},
	}
	results := ec.ClassifyEndpoints(entity)

	found := false
	for _, r := range results {
		if r.Endpoint.URL == "https://myproject.io" {
			found = true
			if r.Type != models.EndpointTypeHomepage {
				t.Errorf("Expected HOMEPAGE_URL, got %s", r.Type)
			}
			if r.Confidence < 0.8 {
				t.Errorf("Expected confidence >= 0.8, got %.2f", r.Confidence)
			}
			break
		}
	}
	if !found {
		t.Error("Homepage URL not found in results")
	}
}

func TestEndpointClassifier_MCPRuntimeEndpoint(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{
					URL:       "https://api.example.com/mcp",
					Transport: "streamable-http",
				},
				Type:      models.EndpointTypeUnknown,
				Confidence: 0.1,
			},
		},
		RuntimeVerification: &models.RuntimeVerification{
			Status: models.RuntimeVerificationStatusPassed,
		},
		RawContent: "const server = new McpServer(); server.connect(new StdioServerTransport());",
	}
	results := ec.ClassifyEndpoints(entity)

	found := false
	for _, r := range results {
		if r.Endpoint.URL == "https://api.example.com/mcp" {
			found = true
			if r.Type != models.EndpointTypeMCPRuntime {
				t.Errorf("Expected MCP_RUNTIME_ENDPOINT, got %s", r.Type)
			}
			if r.Confidence != 1.0 {
				t.Errorf("Expected confidence 1.0 after runtime verification, got %.2f", r.Confidence)
			}
			hasRuntimeEvidence := false
			for _, e := range r.Evidence {
				if e.Rule == "runtime_verified" {
					hasRuntimeEvidence = true
					break
				}
			}
			if !hasRuntimeEvidence {
				t.Error("Expected runtime_verified evidence")
			}
			break
		}
	}
	if !found {
		t.Error("MCP runtime endpoint not found")
	}
}

func TestEndpointClassifier_NoFalsePositiveRepoAsRuntime(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{URL: "https://github.com/owner/mcp-server"},
		RawContent: `This is a great MCP server for Taiwan stock data.
Repository: https://github.com/owner/mcp-server
Documentation: https://docs.example.com
Install: curl https://raw.githubusercontent.com/owner/mcp-server/main/install.sh | sh`,
	}
	results := ec.ClassifyEndpoints(entity)

	for _, r := range results {
		if r.Endpoint.URL == "https://github.com/owner/mcp-server" {
			if r.Type == models.EndpointTypeMCPRuntime {
				t.Errorf("CRITICAL: GitHub repo URL incorrectly classified as MCP_RUNTIME_ENDPOINT")
			}
			if r.Type != models.EndpointTypeRepositoryURL {
				t.Errorf("Expected REPOSITORY_URL, got %s", r.Type)
			}
		}
		if r.Endpoint.URL == "https://docs.example.com" {
			if r.Type != models.EndpointTypeDocumentation {
				t.Errorf("Expected DOCUMENTATION_URL for docs, got %s", r.Type)
			}
		}
		if strings.Contains(r.Endpoint.URL, "install.sh") {
			if r.Type != models.EndpointTypeInstaller {
				t.Errorf("Expected INSTALLER_URL for install script, got %s", r.Type)
			}
		}
	}
}

func TestEndpointClassifier_GetMCPRuntimeEndpoints(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://api.example.com/mcp", Transport: "streamable-http"},
				Type:     models.EndpointTypeMCPRuntime,
				Confidence: 1.0,
			},
			{
				Endpoint: models.Endpoint{URL: "https://github.com/owner/repo", Transport: ""},
				Type:     models.EndpointTypeRepositoryURL,
				Confidence: 0.95,
			},
		},
		RuntimeVerification: &models.RuntimeVerification{
			Status: models.RuntimeVerificationStatusPassed,
		},
	}

	runtimeEps := ec.GetMCPRuntimeEndpoints(entity)
	if len(runtimeEps) != 1 {
		t.Errorf("Expected 1 MCP runtime endpoint, got %d", len(runtimeEps))
	}
	if runtimeEps[0].URL != "https://api.example.com/mcp" {
		t.Errorf("Expected api.example.com/mcp, got %s", runtimeEps[0].URL)
	}
}

func TestEndpointClassifier_ExtractURLs(t *testing.T) {
	ec := NewEndpointClassifier()

	text := `Check out our repo at https://github.com/owner/repo and docs at https://docs.example.com.
Also install via https://raw.githubusercontent.com/owner/repo/main/install.sh`
	urls := ec.extractURLs(text)

	expected := []string{
		"https://github.com/owner/repo",
		"https://docs.example.com",
		"https://raw.githubusercontent.com/owner/repo/main/install.sh",
	}

	for _, exp := range expected {
		found := false
		for _, u := range urls {
			if u == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected URL not found: %s", exp)
		}
	}
}

func TestEndpointClassifier_DuplicateHandling(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/owner/repo",
			Homepage: "https://github.com/owner/repo",
		},
		Endpoints: []models.EndpointWithType{
			{Endpoint: models.Endpoint{URL: "https://github.com/owner/repo"}},
		},
	}
	results := ec.ClassifyEndpoints(entity)

	count := 0
	for _, r := range results {
		if r.Endpoint.URL == "https://github.com/owner/repo" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 1 result for duplicate URL, got %d", count)
	}
}

func TestEndpointClassifier_EmptyEntity(t *testing.T) {
	ec := NewEndpointClassifier()
	entity := &models.Entity{}
	results := ec.ClassifyEndpoints(entity)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty entity, got %d", len(results))
	}
}

func TestEndpointClassifier_UnknownURL(t *testing.T) {
	ec := NewEndpointClassifier()

	entity := &models.Entity{
		Repository: models.RepositoryInfo{URL: "https://github.com/owner/repo"},
		RawContent: "https://random-api.example.com/v1/data",
	}
	results := ec.ClassifyEndpoints(entity)

	found := false
	for _, r := range results {
		if r.Endpoint.URL == "https://random-api.example.com/v1/data" {
			found = true
			if r.Type != models.EndpointTypeUnknown {
				t.Errorf("Expected UNKNOWN for random API, got %s", r.Type)
			}
			if r.Confidence > 0.5 {
				t.Errorf("Expected low confidence for unknown, got %.2f", r.Confidence)
			}
			break
		}
	}
	if !found {
		t.Error("Random API URL not found in results")
	}
}