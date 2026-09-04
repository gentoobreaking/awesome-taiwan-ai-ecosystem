// Package verify implements repository and MCP protocol verification for the crawler pipeline.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// HTTPClient is the minimal HTTP interface used by verification.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// RepositoryVerifier verifies repository metadata.
type RepositoryVerifier struct {
	httpClient HTTPClient
}

// RepositoryVerificationResult holds verification output (§25).
type RepositoryVerificationResult struct {
	Reachable   bool
	HTTPStatus  int
	Status      models.Status
	PushedAt    time.Time
	HasManifest bool
	HasReadme   bool
	Archived    bool
	Error       string
	LastChecked time.Time
}

// NewRepository creates a RepositoryVerifier.
func NewRepository(client HTTPClient) *RepositoryVerifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RepositoryVerifier{httpClient: client}
}

// VerifyRepository checks repository existence and metadata (§25, §26).
func (rv *RepositoryVerifier) VerifyRepository(ctx context.Context, server *models.MCPServer) RepositoryVerificationResult {
	result := RepositoryVerificationResult{
		LastChecked: time.Now().UTC(),
		Status:      models.StatusUnknown,
	}

	repoURL := server.Repository.URL
	if repoURL == "" {
		result.Status = models.StatusDeleted
		result.Error = "no repository URL"
		return result
	}

	// Check archived flag first (§TST-046: archived takes priority)
	result.Archived = server.Repository.Archived
	if server.Repository.Archived {
		result.Status = models.StatusArchived
		result.Reachable = true // GitHub API confirmed existence
		return result
	}

	// Compute status from PushedAt if available
	if !server.Repository.PushedAt.IsZero() {
		result.PushedAt = server.Repository.PushedAt
		result.Status = statusFromPushedAt(server.Repository.PushedAt, server.Repository.Archived)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", repoURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("request error: %v", err)
		return result
	}

	resp, err := rv.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("network error: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		result.Status = models.StatusDeleted
		return result
	}

	result.Reachable = resp.StatusCode == http.StatusOK

	// If PushedAt is available, compute status from it
	if !server.Repository.PushedAt.IsZero() {
		result.PushedAt = server.Repository.PushedAt
		result.Status = statusFromPushedAt(server.Repository.PushedAt, server.Repository.Archived)
	}
	// Check manifest presence from tools (extracted from manifest)
	if len(server.Tools) > 0 {
		result.HasManifest = true
	}

	// Check README presence from description
	if server.Description != "" {
		result.HasReadme = true
	}

	if result.Reachable && result.Status == models.StatusUnknown {
		result.Status = models.StatusActive
	}

	return result
}

// statusFromPushedAt computes status based on last push date (§26).
func statusFromPushedAt(pushed time.Time, archived bool) models.Status {
	if archived {
		return models.StatusArchived
	}
	if pushed.IsZero() {
		return models.StatusUnknown
	}
	days := time.Since(pushed).Hours() / 24
	switch {
	case days < 90:
		return models.StatusActive
	case days < 180:
		return models.StatusMaintenance
	case days < 365:
		return models.StatusStale
	default:
		return models.StatusDormant
	}
}

// VerifyMCPProtocolResult holds protocol verification output (§21, §30).
type VerifyMCPProtocolResult struct {
	ToolsListable     bool
	ResourcesListable bool
	PromptsListable   bool
	ProtocolVersion   string
	Tools             []models.Tool
	Error             string
	LastChecked       time.Time
}

// ProtocolVerifier verifies MCP protocol compliance.
type ProtocolVerifier struct {
	httpClient HTTPClient
}

// NewProtocol creates a ProtocolVerifier.
func NewProtocol(client HTTPClient) *ProtocolVerifier {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ProtocolVerifier{httpClient: client}
}

// VerifyMCPProtocol attempts to call MCP protocol methods via HTTP (§21, §30).
func (pv *ProtocolVerifier) VerifyMCPProtocol(ctx context.Context, server *models.MCPServer) VerifyMCPProtocolResult {
	result := VerifyMCPProtocolResult{
		LastChecked: time.Now().UTC(),
	}

	// Find HTTP endpoint
	var endpoint string
	for _, ep := range server.Endpoints {
		if strings.HasPrefix(ep.Transport, "http") {
			endpoint = ep.URL
			break
		}
	}

	if endpoint == "" {
		result.Error = "stdio transport, cannot verify over HTTP"
		return result
	}

	// Try tools/list (§30)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	))
	if err != nil {
		result.Error = fmt.Sprintf("request error: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := pv.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("network error: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		result.ToolsListable = len(body) > 0
		result.ProtocolVersion = "2.0"
		// Extract tools from response (§28, T024)
		result.Tools = extractToolsFromToolsList(body)
		result.ResourcesListable = len(result.Tools) > 0

	} else {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// extractToolsFromToolsList parses an MCP tools/list JSON-RPC response (§9.1, T024).
// Expected format: {"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"...","description":"...","inputSchema":{...},"annotations":{...}}]}}
func extractToolsFromToolsList(body []byte) []models.Tool {
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
				Annotations struct {
					ReadOnly     bool  `json:"readOnly"`
					Destructive  bool  `json:"destructive"`
					Idempotent   *bool `json:"idempotent,omitempty"`
					Open         *bool `json:"open,omitempty"`
				} `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var tools []models.Tool
	for _, t := range resp.Result.Tools {
		// Validate: reject tools with empty name, description, and no input_schema
		if t.Name == "" && t.Description == "" && t.InputSchema == nil {
			continue
		}
		tool := models.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
			Annotations: models.ToolAnnotations{
				ReadOnly:    t.Annotations.ReadOnly,
				Destructive: t.Annotations.Destructive,
			},
		}
		if t.Annotations.Idempotent != nil {
			tool.Annotations.Idempotent = *t.Annotations.Idempotent
		}
		if t.Annotations.Open != nil {
			tool.Annotations.Open = *t.Annotations.Open
		}
		tools = append(tools, tool)
	}
	return tools
}
