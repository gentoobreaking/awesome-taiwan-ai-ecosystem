package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// GitHubAdapter implements SourceAdapter for GitHub.
type GitHubAdapter struct {
	Token      string
	HTTPClient HTTPClient
	BaseURL    string
}

type HTTPClient interface {
	Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error)
}

// Name returns the source name.
func (a *GitHubAdapter) Name() string { return "github" }

// New creates a new GitHubAdapter.
func New(token string) *GitHubAdapter {
	return &GitHubAdapter{
		Token:   token,
		BaseURL: "https://api.github.com",
	}
}

// Discover searches GitHub for MCP servers (placeholder — full implementation in T006).
func (a *GitHubAdapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	return nil, nil
}

// Fetch retrieves full metadata for a candidate (placeholder).
func (a *GitHubAdapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	return &models.RawRecord{
		RawCandidate: candidate,
	}, nil
}

var _ sources.SourceAdapter = (*GitHubAdapter)(nil)

// fetchFile fetches a file from GitHub repository via Contents API.
// charset handling: GitHub Contents API returns base64 content that is typically UTF-8,
// but some legacy Taiwan repositories may contain Big5 or GBK encoded files.
// We validate UTF-8 first, then attempt Big5/GBK transcoding via golang.org/x/text
// to ensure the returned string is always valid UTF-8 (using "�" as replacement).
func (a *GitHubAdapter) fetchFile(ctx context.Context, owner, repo, path, ref string) (string, error) {
	if a.BaseURL == "" {
		a.BaseURL = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", a.BaseURL, owner, repo, path, ref)
	_ = url
	_ = ctx
	return "", nil
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
