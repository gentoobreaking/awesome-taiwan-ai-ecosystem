// Package github implements the GitHub SourceAdapter for MCP server discovery.
package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/retry"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// KeywordMatrix contains Taiwan-related search keywords (§5.1, §6).
var KeywordMatrix = []string{
	// Taiwan keywords
	"mcp Taiwan", "mcp Taiwanese", "mcp 台灣", "mcp 臺灣", "mcp TW", "mcp zh-TW",
	"mcp 繁體中文", "mcp 繁體",
	// Government domains
	`mcp "data.gov.tw"`, `mcp "gov.tw"`, `mcp "moi.gov.tw"`, `mcp "moea.gov.tw"`,
	`mcp "mof.gov.tw"`, `mcp "mohw.gov.tw"`, `mcp "cwa.gov.tw"`, `mcp "ly.gov.tw"`,
	`mcp "judicial.gov.tw"`, `mcp "law.moj.gov.tw"`,
	// Finance
	"mcp TWSE", "mcp TPEx", "mcp TAIFEX", "mcp TDCC", "mcp FinMind", "mcp Fugle",
	"mcp 台股", "mcp 上市", "mcp 上櫃",
	// Real estate
	`mcp "實價登錄"`, "mcp LVR", `mcp "land.moi.gov.tw"`, "mcp 房價", "mcp 房地產",
	"mcp 土地", "mcp 預售屋",
	// Payment
	"mcp ECPay", "mcp NewebPay", "mcp 綠界", "mcp 藍新",
	// Language
	"mcp \"Traditional Chinese\"", "mcp zh-TW", "mcp 繁體中文", "mcp 台灣語言",
	// Topic-based
	"topic:mcp Taiwan", "topic:model-context-protocol Taiwan",
}

// GitHubAdapter implements SourceAdapter for GitHub repository discovery.
type GitHubAdapter struct {
	HTTPClient *retry.RetryableClient
	Token      string
	BaseURL    string
}

// New creates a new GitHubAdapter with retry and rate limiting.
func New(token string) *GitHubAdapter {
	return &GitHubAdapter{
		HTTPClient: retry.NewClient(retry.DefaultConfig()),
		Token:      token,
		BaseURL:    "https://api.github.com",
	}
}

// Name returns the source identifier.
func (g *GitHubAdapter) Name() string {
	return "github"
}

// searchResponse represents the GitHub Search API response.
type searchResponse struct {
	TotalCount int              `json:"total_count"`
	Items      []githubRepoItem `json:"items"`
}

type githubRepoItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Stargazers  int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	Watchers    int    `json:"subscribers_count"`
	Language    string `json:"language"`
	License     struct {
		SPDXIdentifier string `json:"spdx_id"`
	} `json:"license"`
	DefaultBranch string   `json:"default_branch"`
	OpenIssues    int      `json:"open_issues_count"`
	Archived      bool     `json:"archived"`
	IsFork        bool     `json:"fork"`
	Homepage      string   `json:"homepage"`
	Topics        []string `json:"topics"`
	PushedAt      string   `json:"pushed_at"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// Discover searches GitHub for MCP-related repositories using the keyword matrix.
func (g *GitHubAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	var candidates []models.RawCandidate
	seen := make(map[string]bool)

	for _, keyword := range KeywordMatrix {
		select {
		case <-ctx.Done():
			return candidates, ctx.Err()
		default:
		}

		items, err := g.searchRepositories(ctx, keyword+" in:name,description,readme")
		if err != nil {
			// Log error but continue with other keywords (failure isolation, §41)
			continue
		}

		for _, item := range items {
			if seen[item.FullName] {
				continue
			}
			seen[item.FullName] = true

			candidate := g.toRawCandidate(item)
			candidates = append(candidates, candidate)
		}
	}

	return candidates, nil
}

func (g *GitHubAdapter) searchRepositories(ctx context.Context, query string) ([]githubRepoItem, error) {
	apiURL := fmt.Sprintf("%s/search/repositories?q=%s&per_page=30&sort=updated",
		g.BaseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// RetryableClient handles 429/5xx retries automatically (§TASK-007)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned: %d", resp.StatusCode)
	}

	var searchResp searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	// Also fetch topics for each item (separate API call)
	for i := range searchResp.Items {
		topics := g.fetchTopics(ctx, searchResp.Items[i].Owner.Login, searchResp.Items[i].Name)
		searchResp.Items[i].Topics = topics
	}

	return searchResp.Items, nil
}

func (g *GitHubAdapter) fetchTopics(ctx context.Context, owner, repo string) []string {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/topics", g.BaseURL, owner, repo)

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.mercy-preview+json")

	resp, err := g.HTTPClient.Do(ctx, req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var topicsResp struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&topicsResp); err != nil {
		return nil
	}
	return topicsResp.Names
}

func (g *GitHubAdapter) toRawCandidate(item githubRepoItem) models.RawCandidate {
	return models.RawCandidate{
		Source:        "github",
		SourceURL:     item.HTMLURL,
		Name:          item.Name,
		Description:   item.Description,
		RepositoryURL: item.HTMLURL,
		HomepageURL:   item.Homepage,
		Author:        item.Owner.Login,
		RawMetadata: map[string]any{
			"stars":          item.Stargazers,
			"forks":          item.Forks,
			"watchers":       item.Watchers,
			"language":       item.Language,
			"license":        item.License.SPDXIdentifier,
			"default_branch": item.DefaultBranch,
			"open_issues":    item.OpenIssues,
			"archived":       item.Archived,
			"fork":           item.IsFork,
			"topics":         item.Topics,
			"pushed_at":      item.PushedAt,
			"created_at":     item.CreatedAt,
			"updated_at":     item.UpdatedAt,
		},
		DiscoveredAt: time.Now(),
	}
}

// Fetch retrieves full repository metadata: README, package files, manifest.
func (g *GitHubAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*sources.RawRecord, error) {
	repoPath := extractRepoPath(candidate.RepositoryURL)
	if repoPath == "" {
		return nil, fmt.Errorf("invalid repository URL: %s", candidate.RepositoryURL)
	}

	repo, err := g.fetchRepository(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	record := &sources.RawRecord{
		Candidate:   candidate,
		Repository:  g.toRepositoryInfo(repo, candidate),
		PackageFiles: make(map[string]string),
		Manifest:    make(map[string]any),
	}

	// Fetch README
	readme, err := g.fetchFile(ctx, repoPath, "README.md", repo.DefaultBranch)
	if err == nil && readme != "" {
		record.Readme = readme
	}

	// Fetch package files
	for _, pkgFile := range []string{"package.json", "pyproject.toml", "go.mod", "Cargo.toml"} {
		content, err := g.fetchFile(ctx, repoPath, pkgFile, repo.DefaultBranch)
		if err == nil && content != "" {
			record.PackageFiles[pkgFile] = content
		}
	}

	// Fetch manifest files
	for _, manifestFile := range []string{"server.json", "mcp.json", "manifest.json"} {
		content, err := g.fetchFile(ctx, repoPath, manifestFile, repo.DefaultBranch)
		if err == nil && content != "" {
			var manifest map[string]any
			if json.Unmarshal([]byte(content), &manifest) == nil {
				record.Manifest = manifest
				break
			}
		}
	}

	// Extract endpoint and transport from README
	record.Candidate.Endpoint = extractEndpointFromReadme(record.Readme)
	record.Transport = extractTransportsFromReadme(record.Readme)

	return record, nil
}

type githubRepoDetail struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	HTMLURL         string `json:"html_url"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	SubscribersCount int   `json:"subscribers_count"`
	Language        string `json:"language"`
	License         struct {
		SPDXIdentifier string `json:"spdx_id"`
	} `json:"license"`
	DefaultBranch   string `json:"default_branch"`
	OpenIssuesCount int    `json:"open_issues_count"`
	Archived        bool   `json:"archived"`
	Fork            bool   `json:"fork"`
	Homepage        string `json:"homepage"`
	PushedAt        string `json:"pushed_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
	Topics          []string `json:"topics"`
	Owner           struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (g *GitHubAdapter) fetchRepository(ctx context.Context, repoPath string) (*githubRepoDetail, error) {
	apiURL := fmt.Sprintf("%s/repos/%s", g.BaseURL, repoPath)

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.HTTPClient.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch repository %s: status %d", repoPath, resp.StatusCode)
	}

	var repo githubRepoDetail
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("decode repository: %w", err)
	}
	return &repo, nil
}

func (g *GitHubAdapter) fetchFile(ctx context.Context, repoPath, filepath, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", g.BaseURL, repoPath, filepath, branch)

	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := g.HTTPClient.Do(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch file %s: status %d", filepath, resp.StatusCode)
	}

	var fileResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return "", err
	}

	if fileResp.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(fileResp.Content)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	return fileResp.Content, nil
}

func (g *GitHubAdapter) toRepositoryInfo(repo *githubRepoDetail, candidate models.RawCandidate) *models.RepositoryInfo {
	pushedAt := parseTime(repo.PushedAt)
	updatedAt := parseTime(repo.UpdatedAt)

	return &models.RepositoryInfo{
		URL:         repo.HTMLURL,
		Host:        "github.com",
		Owner:       repo.Owner.Login,
		Name:        repo.Name,
		Stars:       repo.StargazersCount,
		Forks:       repo.ForksCount,
		Watchers:    repo.SubscribersCount,
		OpenIssues:  repo.OpenIssuesCount,
		Language:    repo.Language,
		License:     repo.License.SPDXIdentifier,
		Topics:      repo.Topics,
		DefaultBranch: repo.DefaultBranch,
		Archived:     repo.Archived,
		Fork:          repo.Fork,
		Homepage:      repo.Homepage,
		CreatedAt:     parseTime(repo.CreatedAt),
		UpdatedAt:     updatedAt,
		PushedAt:      pushedAt,
		LastCommitAt:  pushedAt,
	}
}

func extractRepoPath(repoURL string) string {
	// Extracts "owner/repo" from a GitHub URL
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, ".git")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return ""
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func extractEndpointFromReadme(readme string) string {
	// Simple endpoint extraction from README
	lines := strings.Split(readme, "\n")
	for _, line := range lines {
		if strings.Contains(line, "mcp") || strings.Contains(line, "http") {
			if strings.Contains(line, "http") {
				fields := strings.Fields(line)
				for _, f := range fields {
					if strings.HasPrefix(f, "http") {
						return strings.TrimRight(f, ")\"'")
					}
				}
			}
		}
	}
	return ""
}

func extractTransportsFromReadme(readme string) []string {
	transports := []string{}
	if strings.Contains(strings.ToLower(readme), "stdio") {
		transports = append(transports, "stdio")
	}
	if strings.Contains(strings.ToLower(readme), "sse") {
		transports = append(transports, "sse")
	}
	if strings.Contains(strings.ToLower(readme), "http") {
		transports = append(transports, "http")
	}
	if len(transports) == 0 {
		transports = append(transports, "stdio") // default
	}
	return transports
}

// Ensure GitHubAdapter implements SourceAdapter
var _ sources.SourceAdapter = (*GitHubAdapter)(nil)
