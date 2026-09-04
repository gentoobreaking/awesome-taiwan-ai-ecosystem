// Package registry provides the Official MCP Registry adapter.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/retry"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// OfficialRegistryAdapter implements SourceAdapter for the MCP Registry.
type OfficialRegistryAdapter struct {
	HTTPClient *retry.RetryableClient
	BaseURL    string
	Token      string
}

// New creates a new OfficialRegistryAdapter.
func New() *OfficialRegistryAdapter {
	return &OfficialRegistryAdapter{
		HTTPClient: retry.NewClient(retry.DefaultConfig()),
		BaseURL:    "https://api.mcp-servers.dev",
	}
}

// Name returns the source identifier.
func (r *OfficialRegistryAdapter) Name() string {
	return "official-registry"
}

// Discover lists all known MCP servers from the official registry.
func (r *OfficialRegistryAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	apiURL := fmt.Sprintf("%s/servers", r.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry API returned: %d", resp.StatusCode)
	}

	var registryResp struct {
		Servers []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			RepoURL     string `json:"repository_url"`
			Homepage    string `json:"homepage"`
			Registry    string `json:"registry_url"`
		} `json:"servers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return nil, fmt.Errorf("decode registry response: %w", err)
	}

	var candidates []models.RawCandidate
	for _, s := range registryResp.Servers {
		candidates = append(candidates, models.RawCandidate{
			Source:        "official-registry",
			SourceURL:     s.Registry,
			Name:          s.Name,
			Description:   s.Description,
			RepositoryURL: s.RepoURL,
			HomepageURL:   s.Homepage,
			Endpoint:      s.Homepage,
			Author:        extractOwnerFromURL(s.RepoURL),
			RawMetadata: map[string]any{
				"official_registry": true,
			},
			DiscoveredAt: time.Now().UTC(),
		})
	}

	return candidates, nil
}

// Fetch retrieves detailed metadata for a single candidate.
func (r *OfficialRegistryAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*sources.RawRecord, error) {
	u, err := url.Parse(candidate.SourceURL)
	if err != nil {
		return nil, err
	}
	pathParts := splitPath(u.Path)
	registryName := ""
	if len(pathParts) > 0 {
		registryName = pathParts[len(pathParts)-1]
	}

	apiURL := fmt.Sprintf("%s/servers/%s", r.BaseURL, registryName)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := r.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("server not found in registry: %s", registryName)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry API returned: %d", resp.StatusCode)
	}

	var detail struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RepoURL     string `json:"repository_url"`
		Homepage    string `json:"homepage"`
		Version     string `json:"version"`
		Runtime     string `json:"runtime"`
		Transport   string `json:"transport"`
		License     string `json:"license"`
		Readme      string `json:"readme"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decode registry detail: %w", err)
	}

	record := &sources.RawRecord{
		Readme:       detail.Readme,
		Transport:    []string{detail.Transport},
		PackageFiles: map[string]string{},
	}
	record.Candidate = models.RawCandidate{
		Source:        "official-registry",
		Name:          detail.Name,
		Description:   detail.Description,
		RepositoryURL: detail.RepoURL,
		HomepageURL:   detail.Homepage,
		Endpoint:      detail.Homepage,
		Author:        extractOwnerFromURL(detail.RepoURL),
		RawMetadata: map[string]any{
			"official_registry": true,
			"version":           detail.Version,
			"runtime":           detail.Runtime,
			"license":           detail.License,
		},
		DiscoveredAt: time.Now().UTC(),
	}

	return record, nil
}

func extractOwnerFromURL(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	parts := splitPath(u.Path)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func splitPath(p string) []string {
	var result []string
	current := ""
	for _, c := range p {
		if c == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
			continue
		}
		current += string(c)
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// Compile-time interface check.
var _ sources.SourceAdapter = (*OfficialRegistryAdapter)(nil)
