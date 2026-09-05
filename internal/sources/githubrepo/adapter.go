// Package githubrepo implements a SourceAdapter for discovering MCP servers
// from GitHub repository directory structures (e.g., modelcontextprotocol/servers).
package githubrepo

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

// GitHubRepoAdapter discovers MCP servers from GitHub repos that contain
// directory-based server implementations (e.g. src/everything, src/fetch).
type GitHubRepoAdapter struct {
	RepoPath string
	Token    string
	BaseURL  string
	HTTPClient *retry.RetryableClient
}

// New creates a new GitHubRepoAdapter.
func New(repoPath, token string) *GitHubRepoAdapter {
	return &GitHubRepoAdapter{
		RepoPath:  repoPath,
		Token:     token,
		BaseURL:   "https://api.github.com",
		HTTPClient: retry.NewClient(retry.DefaultConfig()),
	}
}

// Name returns the source identifier.
func (a *GitHubRepoAdapter) Name() string {
	return "github-repo:" + a.RepoPath
}

// Discover lists directories in the GitHub repo and creates candidates.
func (a *GitHubRepoAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	// Get repo root contents
	items, err := a.fetchDirectory(ctx, a.RepoPath, "src")
	if err != nil {
		// Try root if src/ doesn't exist
		items, err = a.fetchDirectory(ctx, a.RepoPath, "")
		if err != nil {
			return nil, fmt.Errorf("fetch directory: %w", err)
		}
	}

	var candidates []models.RawCandidate
	repoURL := fmt.Sprintf("https://github.com/%s", a.RepoPath)
	for _, item := range items {
		if item.Type != "dir" {
			continue
		}
		candidates = append(candidates, models.RawCandidate{
			Source:        a.Name(),
			SourceURL:     fmt.Sprintf("https://github.com/%s/tree/main/%s", a.RepoPath, item.Name),
			Name:          item.Name,
			RepositoryURL: repoURL,
			HomepageURL:   fmt.Sprintf("%s/tree/main/%s", repoURL, item.Name),
			Endpoint:      "",
			Description:   item.Description,
			RawMetadata: map[string]interface{}{
				"subdirectory": item.Name,
				"repo":         a.RepoPath,
			},
			DiscoveredAt: time.Now().UTC(),
		})
	}
	return candidates, nil
}

// repoDirItem represents a directory entry from GitHub API
type repoDirItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (a *GitHubRepoAdapter) fetchDirectory(ctx context.Context, repoPath, subdir string) ([]repoDirItem, error) {
	apiPath := repoPath
	if subdir != "" {
		apiPath = fmt.Sprintf("%s/contents/%s", repoPath, subdir)
	}
	apiURL := fmt.Sprintf("%s/repos/%s", a.BaseURL, apiPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := a.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned: %d", resp.StatusCode)
	}

	var items []repoDirItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode directory response: %w", err)
	}

	return items, nil
}

// Fetch retrieves server details (README, manifest, package files).
func (a *GitHubRepoAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*sources.RawRecord, error) {
	subdir, ok := candidate.RawMetadata["subdirectory"].(string)
	if !ok {
		subdir = ""
	}

	repoPath := a.RepoPath
	repo, err := a.fetchRepository(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("fetch repo: %w", err)
	}

	readmePath := "README.md"
	if subdir != "" {
		readmePath = subdir + "/README.md"
	}
	readme, _ := a.fetchFile(ctx, repoPath, readmePath, "main")

	pkgPath := "package.json"
	if subdir != "" {
		pkgPath = subdir + "/package.json"
	}
	packageFile, _ := a.fetchFile(ctx, repoPath, pkgPath, "main")

	var manifest map[string]interface{}
	if packageFile != "" {
		json.Unmarshal([]byte(packageFile), &manifest)
	}

	return &sources.RawRecord{
		Candidate: candidate,
		Repository: &models.RepositoryInfo{
			URL:          fmt.Sprintf("https://github.com/%s", repoPath),
			Owner:        repo.Owner.Login,
			Name:         repo.Name,
			Host:         "github.com",
			DefaultBranch: repo.DefaultBranch,
			License:      repo.License.SPDX,
			Stars:        repo.StargazersCount,
		},
		Readme:     readme,
		Manifest:   manifest,
		PackageFiles: map[string]string{
			"package.json": packageFile,
		},
		Endpoints: []models.Endpoint{
			{URL: candidate.HomepageURL, Transport: "stdio"},
		},
		Tools:     []models.Tool{},
		Transport: []string{"stdio"},
	}, nil
}

// GitHubRepoDetail represents full repository metadata
type githubRepoDetail struct {
	Owner          struct{ Login string } `json:"owner"`
	Name           string                 `json:"name"`
	DefaultBranch  string                 `json:"default_branch"`
	StargazersCount int                   `json:"stargazers_count"`
	License        struct{ SPDX string `json:"spdx_id"` } `json:"license"`
}

func (a *GitHubRepoAdapter) fetchRepository(ctx context.Context, repoPath string) (*githubRepoDetail, error) {
	apiURL := fmt.Sprintf("%s/repos/%s", a.BaseURL, repoPath)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := a.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub repo API returned: %d", resp.StatusCode)
	}

	var repo githubRepoDetail
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("decode repo: %w", err)
	}
	return &repo, nil
}

func (a *GitHubRepoAdapter) fetchFile(ctx context.Context, repoPath, filepath, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", a.BaseURL, repoPath, url.PathEscape(filepath), branch)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	resp, err := a.HTTPClient.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch file %s: HTTP %d", filepath, resp.StatusCode)
	}

	var result struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Content, nil
}

// Ensure GitHubRepoAdapter implements SourceAdapter
var _ sources.SourceAdapter = (*GitHubRepoAdapter)(nil)
