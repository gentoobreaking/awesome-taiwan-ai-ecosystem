package githubrepo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// GitHubRepoAdapter discovers MCP servers from GitHub repo directory structures
// (e.g., modelcontextprotocol/servers src/ subdirectories).
type GitHubRepoAdapter struct {
	RepoPath   string
	Token      string
	HTTPClient HTTPClient
	BaseURL    string
}

type HTTPClient interface {
	Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error)
}

// New creates a new GitHubRepoAdapter.
func New(repoPath, token string) *GitHubRepoAdapter {
	return &GitHubRepoAdapter{
		RepoPath: repoPath,
		Token:    token,
		BaseURL:  "https://api.github.com",
	}
}

func (a *GitHubRepoAdapter) Name() string { return "github-repo:" + a.RepoPath }
// TrustScore returns the trust score for GitHub repo source.
func (a *GitHubRepoAdapter) TrustScore() float64 { return 0.9 }

// Discover lists directories in the GitHub repo and creates candidates.
// Placeholder: returns empty; real implementation would call GitHub Contents API.
func (a *GitHubRepoAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	// Minimal placeholder to satisfy interface. Real implementation fetches
	// src/ directory via GitHub API and emits one candidate per subdirectory.
	_ = time.Now()
	return nil, nil
}

// Fetch retrieves server details (placeholder).
func (a *GitHubRepoAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	return &models.RawRecord{
		RawCandidate: candidate,
	}, nil
}

var _ sources.SourceAdapter = (*GitHubRepoAdapter)(nil)

// fetchFile fetches a file from a GitHub repository.
func (a *GitHubRepoAdapter) fetchFile(ctx context.Context, owner, repo, path, ref string) (string, error) {
	if a.BaseURL == "" {
		a.BaseURL = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", a.BaseURL, owner, repo, path, ref)
	_ = url
	_ = ctx
	return "", nil
}

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
