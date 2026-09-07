package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// TrustScore for official registry source — discovery source, not authoritative proof (spec §5, §207-211).
const TrustScore = 0.9

// DefaultBaseURL is the placeholder URL. The actual registry has
// migrated; pass an empty string (or set MCP_REGISTRY_URL) to disable
// the source instead of hitting a dead host. (T103)
const DefaultBaseURL = ""

// ErrSourceDisabled is returned from Discover/Fetch when the adapter was
// constructed with an empty BaseURL. The coordinator logs this as
// source_skipped (info) rather than source_error. (T103)
var ErrSourceDisabled = errors.New("registry source disabled (no MCP_REGISTRY_URL configured)")

// HTTPClient is the interface for making HTTP requests.
type HTTPClient interface {
	Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error)
}

// StdHTTPClient wraps *http.Client to implement HTTPClient.
type StdHTTPClient struct {
	Client *http.Client
}

// Get performs an HTTP GET request and returns the body bytes, status code, and error.
func (c *StdHTTPClient) Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// Adapter implements SourceAdapter for the official MCP registry.
type Adapter struct {
	BaseURL    string
	Token      string
	HTTPClient HTTPClient
}

// New creates a new official registry adapter. Pass the registry URL
// explicitly (e.g. via MCP_REGISTRY_URL). Empty baseURL means the
// source is disabled and Discover/Fetch return ErrSourceDisabled.
func New(baseURL string) *Adapter {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Adapter{BaseURL: baseURL}
}

// Name returns the source name.
func (a *Adapter) Name() string { return "registry" }

// TrustScore returns the trust score for OfficialRegistry source (spec §5, §207-211).
// Registry sources are discovery sources, not authoritative proof.
func (a *Adapter) TrustScore() float64 { return TrustScore }

// Discover fetches the list of servers from the official registry API.
func (a *Adapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	if a.BaseURL == "" {
		return nil, ErrSourceDisabled
	}
	if a.HTTPClient == nil {
		a.HTTPClient = &StdHTTPClient{Client: &http.Client{Timeout: 30 * time.Second}}
	}

	headers := map[string]string{}
	if a.Token != "" {
		headers["Authorization"] = "Bearer " + a.Token
	}

	url := fmt.Sprintf("%s/servers", a.BaseURL)
	resp, statusCode, err := a.HTTPClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("registry Discover: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("registry Discover: HTTP %d", statusCode)
	}

	var list struct {
		Servers []struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			RepositoryURL string `json:"repository_url"`
			Homepage      string `json:"homepage"`
			RegistryURL   string `json:"registry_url"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(resp, &list); err != nil {
		return nil, fmt.Errorf("registry Discover: parse error: %w", err)
	}

	var candidates []models.RawCandidate
	for _, s := range list.Servers {
		candidate := models.RawCandidate{
			Source:        "registry",
			Name:          s.Name,
			Description:   s.Description,
			RepositoryURL: s.RepositoryURL,
			HomepageURL:   s.Homepage,
			SourceURL:     s.RegistryURL,
			DiscoveredAt:  time.Now(),
		}
		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// Fetch retrieves full metadata for a candidate from the official registry.
func (a *Adapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	if a.BaseURL == "" {
		return nil, ErrSourceDisabled
	}
	if a.HTTPClient == nil {
		a.HTTPClient = &StdHTTPClient{Client: &http.Client{Timeout: 30 * time.Second}}
	}

	headers := map[string]string{}
	if a.Token != "" {
		headers["Authorization"] = "Bearer " + a.Token
	}

	url := fmt.Sprintf("%s/servers/%s", a.BaseURL, candidate.Name)
	resp, statusCode, err := a.HTTPClient.Get(ctx, url, headers)
	if err != nil {
		return nil, fmt.Errorf("registry Fetch: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("registry Fetch: HTTP %d", statusCode)
	}

	var detail struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		RepositoryURL string   `json:"repository_url"`
		Homepage      string   `json:"homepage"`
		Version       string   `json:"version"`
		Runtime       string   `json:"runtime"`
		Transport     []string `json:"transport"`
		License       string   `json:"license"`
		Readme        string   `json:"readme"`
	}
	if err := json.Unmarshal(resp, &detail); err != nil {
		return nil, fmt.Errorf("registry Fetch: parse error: %w", err)
	}

	record := &models.RawRecord{
		RawCandidate: candidate,
		Repository: models.RepositoryInfo{
			URL:        detail.RepositoryURL,
			Owner:      extractOwner(detail.RepositoryURL),
			Name:       detail.Name,
			License:    detail.License,
		},
		Readme:       detail.Readme,
		PackageFiles: map[string]string{},
	}

	if detail.Transport != nil {
		record.Transport = detail.Transport
	} else {
		record.Transport = []string{"stdio"}
	}

	return record, nil
}

var _ sources.SourceAdapter = (*Adapter)(nil)

// extractOwner extracts the owner from a GitHub URL like "https://github.com/owner/repo".
func extractOwner(repoURL string) string {
	idx := strings.LastIndex(repoURL, "github.com/")
	if idx < 0 {
		return ""
	}
	rest := repoURL[idx+len("github.com/"):]
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
