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
	PushedAt    models.RFC3339Time
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
		result.Status = statusFromPushedAt(server.Repository.PushedAt.Time(), server.Repository.Archived)
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
		result.Status = statusFromPushedAt(server.Repository.PushedAt.Time(), server.Repository.Archived)
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
	Resources         []models.Resource
	Prompts           []models.Prompt
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

	// Step 1: Send initialize request (§TASK-022)
	if err := pv.sendRequest(ctx, endpoint, "initialize", func(body []byte) {
		var resp struct {
			Result struct {
				ProtocolVersion string         `json:"protocolVersion"`
				Capabilities    map[string]any `json:"capabilities"`
				ServerInfo      map[string]any `json:"serverInfo"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &resp); err == nil {
			result.ProtocolVersion = resp.Result.ProtocolVersion
			if caps := resp.Result.Capabilities; caps != nil {
				if _, ok := caps["tools"]; ok {
					result.ToolsListable = true
				}
				if _, ok := caps["resources"]; ok {
					result.ResourcesListable = true
				}
				if _, ok := caps["prompts"]; ok {
					result.PromptsListable = true
				}
			}
		}
	}); err != nil {
		result.Error = fmt.Sprintf("initialize: %v", err)
		return result
	}
	// Step 2: Send tools/list request (§30)
	pv.sendRequest(ctx, endpoint, "tools/list", func(body []byte) {
		result.ToolsListable = len(body) > 0
		result.Tools = extractToolsFromToolsList(body)
	})

	// Step 3: Send resources/list request (§22)
	pv.sendRequest(ctx, endpoint, "resources/list", func(body []byte) {
		result.ResourcesListable = len(body) > 0
		result.Resources = extractResourcesFromList(body)
	})

	// Step 4: Send prompts/list request (§22)
	pv.sendRequest(ctx, endpoint, "prompts/list", func(body []byte) {
		result.PromptsListable = len(body) > 0
		result.Prompts = extractPromptsFromList(body)
	})

	return result
}

// sendRequest sends a JSON-RPC request to the MCP endpoint and calls the handler with the body.
func (pv *ProtocolVerifier) sendRequest(ctx context.Context, endpoint, method string, handler func(body []byte)) error {
	reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":{}}`, method)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := pv.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if handler != nil {
		handler(body)
	}
	return nil
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

// extractResourcesFromList parses an MCP resources/list JSON-RPC response (§22, T022).
func extractResourcesFromList(body []byte) []models.Resource {
	var resp struct {
		Result struct {
			Resources []struct {
				URI         string `json:"uri"`
				Name        string `json:"name"`
				Description string `json:"description"`
				MIMEType    string `json:"mimeType"`
				Text        string `json:"text"`
				Blob        string `json:"blob"`
			} `json:"resources"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var resources []models.Resource
	for _, r := range resp.Result.Resources {
		if r.URI == "" {
			continue
		}
		resources = append(resources, models.Resource{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MIMEType:    r.MIMEType,
		})
	}
	return resources
}

// extractPromptsFromList parses an MCP prompts/list JSON-RPC response (§22, T022).
func extractPromptsFromList(body []byte) []models.Prompt {
	var resp struct {
		Result struct {
			Prompts []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"prompts"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil
	}

	var prompts []models.Prompt
	for _, p := range resp.Result.Prompts {
		if p.Name == "" {
			continue
		}
		prompts = append(prompts, models.Prompt{
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return prompts
}
