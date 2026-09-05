package engines

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// createTestEntity creates a minimal Entity for testing.
func createTestEntity(opts ...func(*models.Entity)) *models.Entity {
	now := models.RFC3339Time{}
	e := &models.Entity{
		ID:                   "test-entity",
		Name:                 "Test Entity",
		Slug:                 "test-entity",
		Description:          "Test entity for MCP identity detection",
		Classification:       models.ClassificationResult{Primary: "mcp_server", Confidence: 0.9},
		TaiwanRelevance:      models.TaiwanRelevance{Level: "T2", Score: 30, Confidence: 0.8},
		AIRelevance:          models.AIRelevance{Level: "A1", Score: 60, Confidence: 0.7},
		MCPIdentity:          models.MCPIdentity{Status: models.MCPIdentityStatusCandidate},
		SecurityStatus:       models.SecurityStatusDetail{Status: models.SecurityStatusClean},
		Quality:              models.QualityScore{Score: 80, Grade: "A"},
		Repository:           models.RepositoryInfo{URL: "https://github.com/test/repo", Host: "github.com", Owner: "test", Name: "repo"},
		Endpoints:            []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "streamable-http"}, Type: models.EndpointTypeMCPRuntime}},
		Tools:                []models.Tool{{Name: "test_tool", Description: "Test tool", InputSchema: map[string]any{"type": "object"}}},
		Resources:            []models.Resource{},
		Prompts:              []models.Prompt{},
		DataSources:          []models.DataSource{},
		EntityStatus:         models.EntityStatusDiscovered,
		Sources:              []models.SourceReference{{Source: "github", URL: "https://github.com/test/repo", TrustScore: 0.9}},
		FirstSeen:            now,
		LastSeen:             now,
		LastVerified:         &now,
		RawContent:           "",
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func TestMCPIdentityEngine_New(t *testing.T) {
	engine := NewMCPIdentityEngine()
	if engine == nil {
		t.Fatal("NewMCPIdentityEngine returned nil")
	}
}

func TestDetectMCPIdentity_RealMCPServer(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)
func main() {
	server := mcp.NewServer(&mcp.ServerOptions{})
	server.AddTool(mcp.NewTool("test_tool", mcp.ToolOptions{}))
	server.Run(context.Background(), mcp.NewStdioTransport())
}
`
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://example.com/mcp", Transport: "streamable-http"}, Type: models.EndpointTypeMCPRuntime},
			}
			e.Tools = []models.Tool{{Name: "test_tool", Description: "Test tool", InputSchema: map[string]any{"type": "object"}}}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-server",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-server",
				Topics:       []string{"mcp", "server"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusStaticVerified && result.Status != models.MCPIdentityStatusRuntimeVerified {
		t.Errorf("Expected STATIC_VERIFIED or RUNTIME_VERIFIED for real MCP server, got %s", result.Status)
	}
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("Expected MCPRole SERVER, got %s", result.MCPRole)
	}
	if len(result.Evidence) == 0 {
		t.Error("Expected evidence for real MCP server")
	}
	if result.Confidence < 0.5 {
		t.Errorf("Expected confidence >= 0.5 for real MCP server, got %.2f", result.Confidence)
	}
	t.Logf("Result: Status=%s, Role=%s, Confidence=%.2f, Evidence=%d", result.Status, result.MCPRole, result.Confidence, len(result.Evidence))
	for _, ev := range result.Evidence {
		t.Logf("  Evidence: %s (rule: %s)", ev.Type, ev.Rule)
	}
}

func TestDetectMCPIdentity_MCPClientOnly(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)
func main() {
	client := mcp.NewClient(&mcp.ClientOptions{})
	client.Initialize(context.Background())
	client.CallTool(context.Background(), "test_tool", map[string]any{})
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-client",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-client",
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleClient {
		t.Errorf("Expected MCPRole CLIENT, got %s", result.MCPRole)
	}
	if result.Status != models.MCPIdentityStatusStaticVerified {
		t.Errorf("Expected STATIC_VERIFIED for client, got %s", result.Status)
	}
	t.Logf("Client Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_MCPHost(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)
type Host struct {
	clients []*mcp.Client
}
func (h *Host) AddServer(url string) {
	client := mcp.NewClient(&mcp.ClientOptions{})
	h.clients = append(h.clients, client)
}
func (h *Host) StartAll() {
	for _, c := range h.clients {
		c.Initialize(context.Background())
	}
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-host",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-host",
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleHost {
		t.Errorf("Expected MCPRole HOST, got %s", result.MCPRole)
	}
	t.Logf("Host Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_SDKPackage(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	*mcp.Server
}

func NewServer() *Server {
	return &Server{Server: mcp.NewServer(&mcp.ServerOptions{})}
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-go-sdk",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-go-sdk",
				Topics:       []string{"mcp", "sdk"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleSDK {
		t.Errorf("Expected MCPRole SDK, got %s", result.MCPRole)
	}
	t.Logf("SDK Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_LibraryOnly(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
package mcputils

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func ParseToolSchema(json string) (*mcp.ToolSchema, error) {
	return mcp.ParseToolSchema(json)
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-utils",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-utils",
				Topics:       []string{"mcp", "utils"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleLibrary {
		t.Errorf("Expected MCPRole LIBRARY, got %s", result.MCPRole)
	}
	t.Logf("Library Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_Extension(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
package mcpext

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type CustomTransport struct {
	*mcp.BaseTransport
}

func (t *CustomTransport) Send(msg []byte) error {
	// Custom transport implementation
	return nil
}

func NewCustomTransport() *CustomTransport {
	return &CustomTransport{}
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-custom-transport",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-custom-transport",
				Topics:       []string{"mcp", "transport", "extension"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleExtension {
		t.Errorf("Expected MCPRole EXTENSION, got %s", result.MCPRole)
	}
	t.Logf("Extension Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_Skill(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
package mcpskill

type Skill struct {
	Name        string
	Description string
	Tools       []Tool
}

func NewSkill() *Skill {
	return &Skill{
		Name: "test-skill",
		Tools: []Tool{
			{Name: "skill_tool", Description: "A skill tool"},
		},
	}
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-skill",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-skill",
				Topics:       []string{"mcp", "skill"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.MCPRole != models.MCPRoleSkill {
		t.Errorf("Expected MCPRole SKILL, got %s", result.MCPRole)
	}
	t.Logf("Skill Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestDetectMCPIdentity_NotMCP_Tutorial(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
# MCP Tutorial

This tutorial shows how to use MCP.

import "github.com/modelcontextprotocol/go-sdk/mcp"
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-tutorial",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-tutorial",
				Topics:       []string{"tutorial", "example"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Expected NOT_MCP for tutorial, got %s", result.Status)
	}
	if result.MCPRole != models.MCPRoleNone {
		t.Errorf("Expected MCPRole NONE for tutorial, got %s", result.MCPRole)
	}
	t.Logf("Tutorial Result: Status=%s, Role=%s", result.Status, result.MCPRole)
}

func TestDetectMCPIdentity_NotMCP_Collection(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
# Awesome MCP Servers

A curated list of MCP servers.

- server1
- server2
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/awesome-mcp",
				Host:         "github.com",
				Owner:        "test",
				Name:         "awesome-mcp",
				Topics:       []string{"awesome-list", "collection"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Expected NOT_MCP for collection, got %s", result.Status)
	}
	if result.MCPRole != models.MCPRoleNone {
		t.Errorf("Expected MCPRole NONE for collection, got %s", result.MCPRole)
	}
	t.Logf("Collection Result: Status=%s, Role=%s", result.Status, result.MCPRole)
}

func TestDetectMCPIdentity_NotMCP_ReadmeOnly(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
# My Project

This project uses MCP for communication.
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/my-project",
				Host:         "github.com",
				Owner:        "test",
				Name:         "my-project",
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Expected NOT_MCP for README-only mention, got %s", result.Status)
	}
	if result.MCPRole != models.MCPRoleNone {
		t.Errorf("Expected MCPRole NONE for README-only, got %s", result.MCPRole)
	}
	t.Logf("Readme-only Result: Status=%s, Role=%s", result.Status, result.MCPRole)
}

func TestDetectMCPIdentity_NotMCP_DocEndpointOnly(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `Just a regular project.`
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://github.com/test/project", Transport: "http"}, Type: models.EndpointTypeDocumentation},
			}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/project",
				Host:         "github.com",
				Owner:        "test",
				Name:         "project",
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Expected NOT_MCP for doc endpoint only, got %s", result.Status)
	}
	t.Logf("Doc endpoint Result: Status=%s, Role=%s", result.Status, result.MCPRole)
}

func TestDetectMCPIdentity_Candidate_OnlyDependency(t *testing.T) {
	engine := NewMCPIdentityEngine()

	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `import "github.com/modelcontextprotocol/go-sdk/mcp"`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/uses-mcp",
				Host:         "github.com",
				Owner:        "test",
				Name:         "uses-mcp",
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	if result.Status != models.MCPIdentityStatusCandidate {
		t.Errorf("Expected CANDIDATE for only dependency, got %s", result.Status)
	}
	t.Logf("Dependency-only Result: Status=%s, Role=%s", result.Status, result.MCPRole)
}

func TestDetectMCPIdentity_MultipleRoles(t *testing.T) {
	engine := NewMCPIdentityEngine()

	// SDK + Extension combination
	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	*mcp.Server
}

func NewServer() *Server {
	return &Server{Server: mcp.NewServer(&mcp.ServerOptions{})}
}

type CustomTransport struct {
	*mcp.BaseTransport
}

func (t *CustomTransport) Send(msg []byte) error {
	return nil
}

func NewCustomTransport() *CustomTransport {
	return &CustomTransport{}
}
`
			e.Endpoints = []models.EndpointWithType{}
			e.Tools = []models.Tool{}
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/test/mcp-sdk-ext",
				Host:         "github.com",
				Owner:        "test",
				Name:         "mcp-sdk-ext",
				Topics:       []string{"mcp", "sdk", "transport", "extension"},
			}
		},
	)

	result := engine.DetectMCPIdentity(entity)

	// Primary should be SDK (strongest signal)
	if result.MCPRole != models.MCPRoleSDK {
		t.Errorf("Expected primary MCPRole SDK for SDK+Extension, got %s", result.MCPRole)
	}
	t.Logf("Multi-role Result: Status=%s, Role=%s, Confidence=%.2f", result.Status, result.MCPRole, result.Confidence)
}

func TestMCPIdentityEngine_CalculateConfidence(t *testing.T) {
	engine := NewMCPIdentityEngine()

	// Empty evidence -> low confidence
	confidence := engine.calculateConfidence([]models.Evidence{})
	if confidence != 0.1 {
		t.Errorf("Expected 0.1 for empty evidence, got %.2f", confidence)
	}

	// With evidence -> higher confidence
	evidence := []models.Evidence{
		{Type: "mcp_import", Source: "source.go", Location: "import", Rule: "mcp_import", Score: 1.0, Confidence: 0.9},
		{Type: "mcp_server_impl", Source: "main.go", Location: "main", Rule: "mcp_server_impl", Score: 1.5, Confidence: 0.95},
	}
	confidence = engine.calculateConfidence(evidence)
	if confidence <= 0.1 {
		t.Errorf("Expected confidence > 0.1 with evidence, got %.2f", confidence)
	}
	if confidence > 1.0 {
		t.Errorf("Expected confidence <= 1.0, got %.2f", confidence)
	}
}

func TestMCPIdentityEngine_StatusTransitions(t *testing.T) {
	// Test status transition validity
	if !models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusCandidate, models.MCPIdentityStatusStaticVerified) {
		t.Error("CANDIDATE -> STATIC_VERIFIED should be valid")
	}
	if !models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusCandidate, models.MCPIdentityStatusNotMCP) {
		t.Error("CANDIDATE -> NOT_MCP should be valid")
	}
	if !models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusStaticVerified, models.MCPIdentityStatusRuntimeVerified) {
		t.Error("STATIC_VERIFIED -> RUNTIME_VERIFIED should be valid")
	}
	if !models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusStaticVerified, models.MCPIdentityStatusNotMCP) {
		t.Error("STATIC_VERIFIED -> NOT_MCP should be valid")
	}
	if !models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusRuntimeVerified, models.MCPIdentityStatusNotMCP) {
		t.Error("RUNTIME_VERIFIED -> NOT_MCP should be valid")
	}

	// Invalid transitions
	if models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusNotMCP, models.MCPIdentityStatusCandidate) {
		t.Error("NOT_MCP -> CANDIDATE should be invalid")
	}
	if models.CanTransitionMCPIdentityStatus(models.MCPIdentityStatusRuntimeVerified, models.MCPIdentityStatusStaticVerified) {
		t.Error("RUNTIME_VERIFIED -> STATIC_VERIFIED should be invalid")
	}
}

// Acceptance test cases from spec §56
func TestAcceptance_MCPServer_RealImplementation(t *testing.T) {
	engine := NewMCPIdentityEngine()
	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `package main
import "github.com/modelcontextprotocol/go-sdk/mcp"
func main() {
	server := mcp.NewServer(&mcp.ServerOptions{})
	server.AddTool(mcp.NewTool("echo", mcp.ToolOptions{}))
	server.Run(context.Background(), mcp.NewStdioTransport())
}`
			e.Endpoints = []models.EndpointWithType{{Endpoint: models.Endpoint{URL: "http://localhost:8080/mcp", Transport: "streamable-http"}, Type: models.EndpointTypeMCPRuntime}}
			e.Tools = []models.Tool{{Name: "echo", Description: "Echo tool", InputSchema: map[string]any{"type": "object"}}}
			e.Repository = models.RepositoryInfo{URL: "https://github.com/test/real-mcp-server", Host: "github.com", Owner: "test", Name: "real-mcp-server"}
		},
	)
	result := engine.DetectMCPIdentity(entity)
	if result.MCPRole != models.MCPRoleServer {
		t.Errorf("§56 Test 1: Expected SERVER, got %s", result.MCPRole)
	}
}

func TestAcceptance_MCPClientOnly(t *testing.T) {
	engine := NewMCPIdentityEngine()
	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `package main
import "github.com/modelcontextprotocol/go-sdk/mcp"
func main() {
	client := mcp.NewClient(&mcp.ClientOptions{})
	client.Initialize(context.Background())
	client.CallTool(context.Background(), "tool", nil)
}`
			e.Repository = models.RepositoryInfo{URL: "https://github.com/test/client-only", Host: "github.com", Owner: "test", Name: "client-only"}
		},
	)
	result := engine.DetectMCPIdentity(entity)
	if result.MCPRole != models.MCPRoleClient {
		t.Errorf("§56 Test 2: Expected CLIENT, got %s", result.MCPRole)
	}
}

func TestAcceptance_MCPCollection(t *testing.T) {
	engine := NewMCPIdentityEngine()
	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `# Awesome MCP Servers\n- server1\n- server2`
			e.Repository = models.RepositoryInfo{URL: "https://github.com/test/awesome", Host: "github.com", Owner: "test", Name: "awesome", Topics: []string{"awesome-list"}}
		},
	)
	result := engine.DetectMCPIdentity(entity)
	if result.Status != models.MCPIdentityStatusNotMCP || result.MCPRole != models.MCPRoleNone {
		t.Errorf("§56 Test 8: Expected NOT_MCP/NONE for collection, got %s/%s", result.Status, result.MCPRole)
	}
}

func TestAcceptance_AI_Agent_With_MCPClient(t *testing.T) {
	engine := NewMCPIdentityEngine()
	entity := createTestEntity(
		func(e *models.Entity) {
			e.RawContent = `package main
import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"context"
)
func main() {
	agent := NewAgent()
	client := mcp.NewClient(&mcp.ClientOptions{})
	client.Initialize(context.Background())
	agent.Run(client)
}`
			e.Repository = models.RepositoryInfo{URL: "https://github.com/test/ai-agent", Host: "github.com", Owner: "test", Name: "ai-agent"}
			e.Classification = models.ClassificationResult{Primary: "ai_agent", Confidence: 0.9}
		},
	)
	result := engine.DetectMCPIdentity(entity)
	// AI agent using MCP -> CLIENT role (spec §49)
	if result.MCPRole != models.MCPRoleClient {
		t.Errorf("§56 Test 49: Expected CLIENT for AI agent using MCP, got %s", result.MCPRole)
	}
}