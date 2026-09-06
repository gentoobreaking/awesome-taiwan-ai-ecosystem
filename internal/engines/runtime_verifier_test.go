package engines

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestRuntimeVerifier_New(t *testing.T) {
	rv := NewRuntimeVerifier()
	if rv == nil {
		t.Fatal("Expected non-nil RuntimeVerifier")
	}
	if rv.InitializeTimeout != 10*time.Second {
		t.Errorf("Expected InitializeTimeout 10s, got %v", rv.InitializeTimeout)
	}
	if rv.ToolsListTimeout != 10*time.Second {
		t.Errorf("Expected ToolsListTimeout 10s, got %v", rv.ToolsListTimeout)
	}
	if rv.TotalTimeout != 30*time.Second {
		t.Errorf("Expected TotalTimeout 30s, got %v", rv.TotalTimeout)
	}
}

func TestRuntimeVerifier_isEligibleForVerification(t *testing.T) {
	rv := NewRuntimeVerifier()

	tests := []struct {
		name       string
		entity     *models.Entity
		expected   bool
	}{
		{
			name: "Eligible - static verified with runtime endpoint",
			entity: &models.Entity{
				MCPIdentity: models.MCPIdentity{
					Status: models.MCPIdentityStatusStaticVerified,
				},
				Endpoints: []models.EndpointWithType{
					{
						Endpoint: models.Endpoint{URL: "stdio:./server"},
						Type:     models.EndpointTypeMCPRuntime,
					},
				},
			},
			expected: true,
		},
		{
			name: "Not eligible - candidate status",
			entity: &models.Entity{
				MCPIdentity: models.MCPIdentity{
					Status: models.MCPIdentityStatusCandidate,
				},
				Endpoints: []models.EndpointWithType{
					{
						Endpoint: models.Endpoint{URL: "stdio:./server"},
						Type:     models.EndpointTypeMCPRuntime,
					},
				},
			},
			expected: false,
		},
		{
			name: "Not eligible - no runtime endpoint",
			entity: &models.Entity{
				MCPIdentity: models.MCPIdentity{
					Status: models.MCPIdentityStatusStaticVerified,
				},
				Endpoints: []models.EndpointWithType{
					{
						Endpoint: models.Endpoint{URL: "https://github.com/owner/repo"},
						Type:     models.EndpointTypeRepositoryURL,
					},
				},
			},
			expected: false,
		},
		{
			name: "Not eligible - nil entity",
			entity: nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rv.isEligibleForVerification(tt.entity)
			if result != tt.expected {
				t.Errorf("isEligibleForVerification() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRuntimeVerifier_findRuntimeEndpoint(t *testing.T) {
	rv := NewRuntimeVerifier()

	entity := &models.Entity{
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://github.com/owner/repo"},
				Type:     models.EndpointTypeRepositoryURL,
			},
			{
				Endpoint: models.Endpoint{URL: "stdio:./server"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := rv.findRuntimeEndpoint(entity)
	if result == nil {
		t.Fatal("Expected to find runtime endpoint")
	}
	if result.Endpoint.URL != "stdio:./server" {
		t.Errorf("Expected stdio:./server, got %s", result.Endpoint.URL)
	}
	if result.Type != models.EndpointTypeMCPRuntime {
		t.Errorf("Expected MCPRuntime, got %s", result.Type)
	}

	// Test with no runtime endpoint
	entity2 := &models.Entity{
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://github.com/owner/repo"},
				Type:     models.EndpointTypeRepositoryURL,
			},
		},
	}
	result2 := rv.findRuntimeEndpoint(entity2)
	if result2 != nil {
		t.Errorf("Expected nil, got %v", result2)
	}
}

func TestRuntimeVerifier_detectTransport(t *testing.T) {
	rv := NewRuntimeVerifier()

	tests := []struct {
		name     string
		url      string
		expected MCPTransport
	}{
		{
			name:     "stdio prefix",
			url:      "stdio:./server",
			expected: TransportStdio,
		},
		{
			name:     "stdio in url",
			url:      "stdio:node server.js",
			expected: TransportStdio,
		},
		{
			name:     "sse in url",
			url:      "https://example.com/sse",
			expected: TransportSSE,
		},
		{
			name:     "http in url",
			url:      "http://localhost:3000/mcp",
			expected: TransportStreamableHTTP,
		},
		{
			name:     "https in url",
			url:      "https://api.example.com/mcp",
			expected: TransportStreamableHTTP,
		},
		{
			name:     "default to stdio",
			url:      "unknown://transport",
			expected: TransportStdio,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := &models.EndpointWithType{
				Endpoint: models.Endpoint{URL: tt.url},
			}
			result := rv.detectTransport(ep)
			if result != tt.expected {
				t.Errorf("detectTransport() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRuntimeVerifier_Verify_NotEligible(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	// Entity with candidate status
	entity := &models.Entity{
		MCPIdentity: models.MCPIdentity{
			Status: models.MCPIdentityStatusCandidate,
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "stdio:./server"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := rv.Verify(ctx, entity)

	if result.Status != RuntimeVerificationStatusFailed {
		t.Errorf("Expected FAILED, got %s", result.Status)
	}

	found := false
	for _, e := range result.Evidence {
		if e.Rule == "static_verified_required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected static_verified_required evidence")
	}
}

func TestRuntimeVerifier_Verify_NoRuntimeEndpoint(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	entity := &models.Entity{
		MCPIdentity: models.MCPIdentity{
			Status: models.MCPIdentityStatusStaticVerified,
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://github.com/owner/repo"},
				Type:     models.EndpointTypeRepositoryURL,
			},
		},
	}

	result := rv.Verify(ctx, entity)

	if result.Status != RuntimeVerificationStatusFailed {
		t.Errorf("Expected FAILED, got %s", result.Status)
	}

	found := false
	for _, e := range result.Evidence {
		if e.Rule == "runtime_endpoint_required" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected runtime_endpoint_required evidence")
	}
}

func TestRuntimeVerifier_Verify_NilEntity(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	result := rv.Verify(ctx, nil)

	if result.Status != RuntimeVerificationStatusFailed {
		t.Errorf("Expected FAILED, got %s", result.Status)
	}
}

func TestRuntimeVerifier_Verify_Stdio_NoCommand(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	entity := &models.Entity{
		MCPIdentity: models.MCPIdentity{
			Status: models.MCPIdentityStatusStaticVerified,
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "stdio:./server"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
		RawContent: "", // No command in raw content
	}

	result := rv.Verify(ctx, entity)

	if result.Status != RuntimeVerificationStatusFailed {
		t.Errorf("Expected FAILED, got %s", result.Status)
	}

	found := false
	for _, e := range result.Evidence {
		if e.Rule == "missing_command" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected missing_command evidence")
	}
}

func TestRuntimeVerifier_getServerCommandFromEntity(t *testing.T) {
	rv := NewRuntimeVerifier()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "package.json with mcp script",
			content:  `{"scripts": {"mcp": "node server.js", "test": "jest"}}`,
			expected: "node server.js",
		},
		{
			name:     "package.json with server script",
			content:  `{"scripts": {"server": "python server.py", "dev": "vite"}}`,
			expected: "python server.py",
		},
		{
			name:     "package.json with start script",
			content:  `{"scripts": {"start": "go run .", "build": "go build"}}`,
			expected: "go run .",
		},
		{
			name:     "go.mod content",
			content:  "module github.com/owner/repo\ngo 1.21",
			expected: "go run .",
		},
		{
			name:     "Cargo.toml content",
			content:  "[package]\nname = \"server\"\nversion = \"0.1.0\"",
			expected: "cargo run",
		},
		{
			name:     "pyproject.toml content",
			content:  "[project]\nname = \"server\"\nversion = \"0.1.0\"",
			expected: "python -m mcp_server",
		},
		{
			name:     "No matching content",
			content:  "random text",
			expected: "",
		},
		{
			name:     "Empty content",
			content:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &models.Entity{
				RawContent: tt.content,
			}
			result := getServerCommandFromEntityWrapper(rv, entity)
			if result != tt.expected {
				t.Errorf("getServerCommandFromEntity() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestRuntimeVerifier_Verify_SSE_NotImplemented(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	entity := &models.Entity{
		MCPIdentity: models.MCPIdentity{
			Status: models.MCPIdentityStatusStaticVerified,
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://example.com/sse"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := rv.Verify(ctx, entity)

	if result.Status != RuntimeVerificationStatusError {
		t.Errorf("Expected ERROR, got %s", result.Status)
	}

	found := false
	for _, e := range result.Evidence {
		if e.Rule == "sse_not_implemented" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected sse_not_implemented evidence")
	}
}

func TestRuntimeVerifier_Verify_StreamableHTTP_NotImplemented(t *testing.T) {
	rv := NewRuntimeVerifier()

	ctx := context.Background()

	entity := &models.Entity{
		MCPIdentity: models.MCPIdentity{
			Status: models.MCPIdentityStatusStaticVerified,
		},
		Endpoints: []models.EndpointWithType{
			{
				Endpoint: models.Endpoint{URL: "https://api.example.com/mcp"},
				Type:     models.EndpointTypeMCPRuntime,
			},
		},
	}

	result := rv.Verify(ctx, entity)

	if result.Status != RuntimeVerificationStatusError {
		t.Errorf("Expected ERROR, got %s", result.Status)
	}

	found := false
	for _, e := range result.Evidence {
		if e.Rule == "streamable_http_not_implemented" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected streamable_http_not_implemented evidence")
	}
}

func TestInitializeParams_JSONMarshal(t *testing.T) {
	params := initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]bool{},
		ClientInfo:      clientInfo{Name: "test", Version: "1.0.0"},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["protocolVersion"] != "2024-11-05" {
		t.Errorf("Expected protocolVersion 2024-11-05, got %v", parsed["protocolVersion"])
	}
}

func TestInitializeResult_JSONMarshal(t *testing.T) {
	result := InitializeResult{
		Success:      true,
		LatencyMs:    100,
		ServerInfo:   `{"name":"test","version":"1.0.0"}`,
		ProtocolVer:  "2024-11-05",
		Capabilities: `{"tools":true}`,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["success"] != true {
		t.Errorf("Expected success true, got %v", parsed["success"])
	}
}

func TestToolsListResult_JSONMarshal(t *testing.T) {
	result := ToolsListResult{
		Success:      true,
		ToolCount:    3,
		ToolsSummary: "tool1, tool2, tool3",
		LatencyMs:    50,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed["tool_count"] != float64(3) {
		t.Errorf("Expected toolCount 3, got %v", parsed["tool_count"])
	}
}

// Wrapper for testing private method
func getServerCommandFromEntityWrapper(rv *RuntimeVerifier, entity *models.Entity) string {
	return rv.getServerCommandFromEntity(entity)
}