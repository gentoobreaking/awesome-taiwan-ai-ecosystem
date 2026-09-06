package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// TrustScore for GitHub source — highest trust among all sources.
const TrustScore = 0.95

// KeywordMatrix defines the discovery query strategy (spec §6, §42).
// Combines Taiwan signals + AI signals for broad discovery, not just MCP keywords.
var KeywordMatrix = []string{
	// Taiwan + MCP (high precision)
	"mcp Taiwan",
	"mcp Taiwanese",
	"mcp 台灣",
	`mcp "data.gov.tw"`,
	"mcp TWSE",
	"mcp TAIEX",
	"mcp FinTech Taiwan",

	// Taiwan + AI (broad discovery)
	"Taiwan AI",
	"Taiwan artificial intelligence",
	"台灣 AI",
	"台灣 人工智慧",
	"台灣 生成式 AI",
	"Taipei AI",
	"TAIPEI AI Lab",
	"Taiwan LLM",
	"Taiwan LLM MCP",
	"Taiwan LLMOps",
	"Taiwan RAG",
	"Taiwan embedding",
	"Taiwan vector database",

	// AI + Taiwan financial keywords
	"AI stock Taiwan",
	"AI trading TWSE",
	"AI FinTech Taiwan",
	"AI data.gov.tw",

	// Taiwan government + AI
	"AI 政府",
	"AI 政务",
	"AI 政务 台湾",

	// AI frameworks/toolkits + Taiwan
	"LangChain Taiwan",
	"LlamaIndex Taiwan",
	"AutoGen Taiwan",
	"CrewAI Taiwan",
	"LangGraph Taiwan",
	"Semantic Kernel Taiwan",
}

// GitHubAdapter implements SourceAdapter for GitHub.
type GitHubAdapter struct {
	Token      string
	HTTPClient HTTPClient
	BaseURL    string
}

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

// NewStdHTTPClient creates a new StdHTTPClient wrapping the given http.Client.
func NewStdHTTPClient(client *http.Client) *StdHTTPClient {
	return &StdHTTPClient{Client: client}
}

// Name returns the source name.
func (a *GitHubAdapter) Name() string { return "github" }

// TrustScore returns the trust score for this source.
func (a *GitHubAdapter) TrustScore() float64 { return TrustScore }

// New creates a new GitHubAdapter with a default HTTP client.
func New(token string) *GitHubAdapter {
	return &GitHubAdapter{
		Token:      token,
		BaseURL:    "https://api.github.com",
		HTTPClient:  NewStdHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	}
}
func (a *GitHubAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	if a.BaseURL == "" {
		a.BaseURL = "https://api.github.com"
	}
	if a.HTTPClient == nil {
		a.HTTPClient = NewStdHTTPClient(&http.Client{Timeout: 30 * time.Second})
	}

	headers := map[string]string{}
	if a.Token != "" {
		headers["Authorization"] = "Bearer " + a.Token
	}

	var allCandidates []models.RawCandidate

	for _, keyword := range KeywordMatrix {
		select {
		case <-ctx.Done():
			return allCandidates, ctx.Err()
		default:
		}

		url := fmt.Sprintf("%s/search/repositories?q=%s+language:go+language:python+language:typescript+language:javascript&sort=updated&per_page=30",
			a.BaseURL, encodeSearchQuery(keyword))

		resp, _, err := a.HTTPClient.Get(ctx, url, headers)
		if err != nil {
			continue
		}

		var searchResp struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				ID            int64  `json:"id"`
				Name          string `json:"name"`
				FullName      string `json:"full_name"`
				Description   string `json:"description"`
				HTMLURL       string `json:"html_url"`
				Stargazers    int    `json:"stargazers_count"`
				Forks         int    `json:"forks_count"`
				Language      string `json:"language"`
				PushedAt      string `json:"pushed_at"`
				License       *struct {
					SPDXID string `json:"spdx_id"`
				} `json:"license"`
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"items"`
		}
		if err := json.Unmarshal(resp, &searchResp); err != nil {
			continue
		}

		for _, item := range searchResp.Items {
			candidate := models.RawCandidate{
				Source:        "github",
				SourceURL:     item.HTMLURL,
				Name:          item.Name,
				Description:   item.Description,
				RepositoryURL: item.HTMLURL,
				Author:        item.Owner.Login,
				RawMetadata: map[string]any{
					"stars":      item.Stargazers,
					"forks":      item.Forks,
					"language":   item.Language,
					"pushed_at":  item.PushedAt,
					"license":    item.License,
					"owner":      item.Owner.Login,
					"github_id":  item.ID,
					"full_name":  item.FullName,
				},
			}
			allCandidates = append(allCandidates, candidate)
		}
	}

	return allCandidates, nil
}

// Fetch retrieves full metadata for a candidate from GitHub.
func (a *GitHubAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	if a.HTTPClient == nil {
		a.HTTPClient = NewStdHTTPClient(&http.Client{Timeout: 30 * time.Second})
	}

	headers := map[string]string{}
	if a.Token != "" {
		headers["Authorization"] = "Bearer " + a.Token
	}

	repoPath := extractRepoPath(candidate.RepositoryURL)
	if repoPath == "" {
		return &models.RawRecord{RawCandidate: candidate}, nil
	}

	// Fetch repository metadata
	repoURL := fmt.Sprintf("%s/repos/%s", a.BaseURL, repoPath)
	resp, _, err := a.HTTPClient.Get(ctx, repoURL, headers)
	if err != nil {
		return &models.RawRecord{RawCandidate: candidate}, nil
	}

	var repo struct {
		HTMLURL    string `json:"html_url"`
		Description string `json:"description"`
		Stargazers  int    `json:"stargazers_count"`
		Forks      int    `json:"forks_count"`
		Language   string `json:"language"`
		License    *struct {
			SPDXID string `json:"spdx_id"`
		} `json:"license"`
		Topics       []string `json:"topics"`
		DefaultBranch  string  `json:"default_branch"`
		CreatedAt    string   `json:"created_at"`
		UpdatedAt    string   `json:"updated_at"`
		PushedAt     string   `json:"pushed_at"`
		Archived     bool     `json:"archived"`
	}
	if err := json.Unmarshal(resp, &repo); err != nil {
		return &models.RawRecord{RawCandidate: candidate}, nil
	}

	repoInfo := models.RepositoryInfo{
		URL:           repo.HTMLURL,
		Owner:         extractOwner(repoPath),
		Name:          extractRepoName(repoPath),
		Stars:         repo.Stargazers,
		Forks:         repo.Forks,
		Language:      repo.Language,
		License:       "",
		Topics:        repo.Topics,
		DefaultBranch: repo.DefaultBranch,
		Archived:      repo.Archived,
		CreatedAt:     parseRFC3339Time(repo.CreatedAt),
		UpdatedAt:     parseRFC3339Time(repo.UpdatedAt),
		PushedAt:      parseRFC3339Time(repo.PushedAt),
	}
	if repo.License != nil {
		repoInfo.License = repo.License.SPDXID
	}

	// Fetch topics (categories)
	topics, _ := a.fetchTopics(ctx, repoPath, headers)
	if len(topics) > 0 {
		repoInfo.Topics = topics
	}

	// Fetch README
	readme, _ := a.fetchFile(ctx, repoPath, "README.md", "main", headers)

	// Extract transport from README
	transports := extractTransportsFromReadme(readme)

	record := &models.RawRecord{
		RawCandidate: candidate,
		Repository:   repoInfo,
		Readme:       readme,
		Transport:    transports,
		PackageFiles: make(map[string]string),
	}

	detectPackageFiles(ctx, a, repoPath, headers, record)
	return record, nil
}

var _ sources.SourceAdapter = (*GitHubAdapter)(nil)

// encodeSearchQuery encodes a search query for GitHub's API.
func encodeSearchQuery(q string) string {
	return strings.ReplaceAll(q, " ", "+")
}

// extractRepoPath extracts "owner/repo" from a GitHub URL.
func extractRepoPath(url string) string {
	cleanURL := strings.TrimSuffix(url, ".git")
	cleanURL = strings.TrimSuffix(cleanURL, "/")

	idx := strings.LastIndex(cleanURL, "github.com/")
	if idx < 0 {
		return ""
	}
	parts := strings.SplitN(cleanURL[idx+len("github.com/"):], "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// extractOwner extracts the owner from a "owner/repo" path.
func extractOwner(repoPath string) string {
	parts := strings.Split(repoPath, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// extractRepoName extracts the repo name from a "owner/repo" path.
func extractRepoName(repoPath string) string {
	parts := strings.Split(repoPath, "/")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// parseRFC3339Time parses an RFC3339 time string into models.RFC3339Time.
func parseRFC3339Time(s string) models.RFC3339Time {
	return models.RFC3339Time(parseTime(s))
}

// fetchTopics fetches the topics for a repository.
func (a *GitHubAdapter) fetchTopics(ctx context.Context, repoPath string, headers map[string]string) ([]string, error) {
	url := fmt.Sprintf("%s/repos/%s/topics", a.BaseURL, repoPath)
	resp, statusCode, err := a.HTTPClient.Get(ctx, url, headers)
	if err != nil || statusCode != 200 {
		return nil, err
	}
	var topicResp struct {
		Names []string `json:"names"`
	}
	if err := json.Unmarshal(resp, &topicResp); err != nil {
		return nil, err
	}
	return topicResp.Names, nil
}

// fetchFile fetches a file from GitHub repository via Contents API.
func (a *GitHubAdapter) fetchFile(ctx context.Context, repoPath, path, ref string, headers map[string]string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", a.BaseURL, repoPath, path, ref)
	resp, statusCode, err := a.HTTPClient.Get(ctx, url, headers)
	if err != nil || statusCode != 200 {
		return "", err
	}

	var contentResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(resp, &contentResp); err != nil {
		return "", err
	}
	if contentResp.Encoding == "base64" {
		return fetchFileWithContent(contentResp.Content)
	}
	return sanitizeContent(resp), nil
}

// fetchFileWithContent is a testable helper that decodes base64 and handles charset.
func fetchFileWithContent(b64Content string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Content))
	if err != nil {
		decoded = []byte(b64Content)
	}
	return sanitizeContent(decoded), nil
}

func sanitizeContent(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	if decoded, err := traditionalchinese.Big5.NewDecoder().Bytes(data); err == nil && utf8.Valid(decoded) {
		return strings.ToValidUTF8(string(decoded), "�")
	}
	if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data); err == nil && utf8.Valid(decoded) {
		return strings.ToValidUTF8(string(decoded), "�")
	}
	return strings.ToValidUTF8(string(data), "�")
}

func sanitizeUTF8(data []byte) string {
	return sanitizeContent(data)
}

// detectPackageFiles checks for package.json, pyproject.toml, go.mod in the repo.
func detectPackageFiles(ctx context.Context, a *GitHubAdapter, repoPath string, headers map[string]string, record *models.RawRecord) {
	for _, file := range []string{"package.json", "pyproject.toml", "go.mod"} {
		content, err := a.fetchFile(ctx, repoPath, file, "main", headers)
		if err == nil && content != "" {
			record.PackageFiles[file] = content
		}
	}
}

// extractEndpointFromReadme extracts MCP endpoints from README text.
func extractEndpointFromReadme(readme string) string {
	patterns := []string{
		"Endpoint:", "endpoint:", "MCP Endpoint:", "mcp endpoint:",
	}
	for _, line := range strings.Split(readme, "\n") {
		for _, p := range patterns {
			if idx := strings.Index(line, p); idx >= 0 {
				rest := strings.TrimSpace(line[idx+len(p):])
				if strings.HasPrefix(rest, "http") {
					return extractURL(rest)
				}
			}
		}
	}
	return ""
}

func extractURL(text string) string {
	end := len(text)
	for i, c := range text {
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			end = i
			break
		}
	}
	return text[:end]
}

// extractTransportsFromReadme detects transport types from README.
func extractTransportsFromReadme(readme string) []string {
	var transports []string
	lower := strings.ToLower(readme)
	if strings.Contains(lower, "stdio") {
		transports = append(transports, "stdio")
	}
	if strings.Contains(lower, "http") || strings.Contains(lower, "https") {
		transports = append(transports, "http")
	}
	if strings.Contains(lower, "websocket") || strings.Contains(lower, "ws://") {
		transports = append(transports, "websocket")
	}
	if len(transports) == 0 {
		transports = append(transports, "stdio")
	}
	return transports
}

// parseTime parses an RFC3339 time string.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
