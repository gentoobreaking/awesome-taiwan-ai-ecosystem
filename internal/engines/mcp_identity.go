package engines

import (
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// MCPIdentityEngine performs static analysis to detect MCP server implementation.
type MCPIdentityEngine struct{}

// NewMCPIdentityEngine creates a new MCP identity engine.
func NewMCPIdentityEngine() *MCPIdentityEngine {
	return &MCPIdentityEngine{}
}

// MCPIdentityResult holds the result of MCP identity detection.
type MCPIdentityResult struct {
	Status    models.MCPIdentityStatus `json:"status"`
	Evidence  []models.Evidence        `json:"evidence"`
	Confidence float64                 `json:"confidence"`
	MCPRole   models.MCPRole           `json:"mcp_role"`
	Reasoning string                   `json:"reasoning"`
}

// DetectMCPIdentity analyzes an entity for MCP server/client/host implementation.
func (e *MCPIdentityEngine) DetectMCPIdentity(entity *models.Entity) MCPIdentityResult {
	var evidence []models.Evidence
	reasoning := []string{}

	// Check for positive indicators
	hasMCPImport := e.checkMCPImport(entity, &evidence, &reasoning)
	hasMCPServerImpl := e.checkMCPServerImpl(entity, &evidence, &reasoning)
	hasTransport := e.checkTransport(entity, &evidence, &reasoning)
	hasToolDefs := e.checkToolDefinitions(entity, &evidence, &reasoning)
	hasExecutableEntry := e.checkExecutableEntry(entity, &evidence, &reasoning)
	hasMCPDep := e.checkMCPDependency(entity, &evidence, &reasoning)
	hasClientImpl := e.checkClientImpl(entity, &evidence, &reasoning)
	hasHostImpl := e.checkHostImpl(entity, &evidence, &reasoning)
	hasSDKPackage := e.checkSDKPackage(entity, &evidence, &reasoning)
	hasLibraryOnly := e.checkLibraryOnly(entity, &evidence, &reasoning)
	hasExtension := e.checkExtension(entity, &evidence, &reasoning)
	hasSkill := e.checkSkill(entity, &evidence, &reasoning)

	// Check for negative indicators
	hasOnlyReadmeMention := e.checkOnlyReadmeMention(entity, &evidence, &reasoning)
	isTutorialOrExample := e.checkTutorialOrExample(entity, &evidence, &reasoning)
	isCollectionOrRegistry := e.checkCollectionOrRegistry(entity, &evidence, &reasoning)
	hasOnlyDocEndpoint := e.checkOnlyDocEndpoint(entity, &evidence, &reasoning)

	// Determine status based on evidence
	status, mcpRole := e.determineStatus(
		hasMCPImport, hasMCPServerImpl, hasTransport, hasToolDefs,
		hasExecutableEntry, hasMCPDep, hasClientImpl, hasHostImpl,
		hasSDKPackage, hasLibraryOnly, hasExtension, hasSkill,
		hasOnlyReadmeMention, isTutorialOrExample, isCollectionOrRegistry,
		hasOnlyDocEndpoint,
	)

	// Calculate confidence
	confidence := e.calculateConfidence(evidence)

	return MCPIdentityResult{
		Status:     status,
		Evidence:   evidence,
		Confidence: confidence,
		MCPRole:    mcpRole,
		Reasoning:  strings.Join(reasoning, "; "),
	}
}

// checkMCPImport checks for MCP SDK imports.
func (e *MCPIdentityEngine) checkMCPImport(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	goImports := []string{
		"github.com/modelcontextprotocol/go-sdk/mcp",
		"modelcontextprotocol/go-sdk/mcp",
	}
	jsImports := []string{
		"@modelcontextprotocol/sdk",
		"@modelcontextprotocol/sdk/client",
		"@modelcontextprotocol/sdk/server",
	}
	pyImports := []string{
		"mcp",
		"mcp.server",
		"mcp.client",
	}

	found := false
	for _, imp := range goImports {
		if strings.Contains(sourceCode, imp) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "mcp_import",
				MatchedText: imp,
				Score:       10,
				Confidence:  0.95,
			})
			*reasoning = append(*reasoning, "Found Go MCP SDK import: "+imp)
			found = true
		}
	}
	for _, imp := range jsImports {
		if strings.Contains(sourceCode, imp) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "mcp_import",
				MatchedText: imp,
				Score:       10,
				Confidence:  0.95,
			})
			*reasoning = append(*reasoning, "Found JS/TS MCP SDK import: "+imp)
			found = true
		}
	}
	for _, imp := range pyImports {
		if strings.Contains(sourceCode, imp) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "mcp_import",
				MatchedText: imp,
				Score:       10,
				Confidence:  0.9,
			})
			*reasoning = append(*reasoning, "Found Python MCP import: "+imp)
			found = true
		}
	}

	return found
}

// checkMCPServerImpl checks for MCP server implementation patterns.
func (e *MCPIdentityEngine) checkMCPServerImpl(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	patterns := []string{
		"McpServer",
		"mcp.Server",
		"server.McpServer",
		"NewMcpServer",
	}

	for _, pattern := range patterns {
		if strings.Contains(sourceCode, pattern) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "mcp_server_impl",
				MatchedText: pattern,
				Score:       15,
				Confidence:  0.9,
			})
			*reasoning = append(*reasoning, "Found MCP server implementation: "+pattern)
			return true
		}
	}
	return false
}

// checkTransport checks for MCP transport implementations.
func (e *MCPIdentityEngine) checkTransport(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	transports := []string{
		"StdioServerTransport",
		"stdio.ServerTransport",
		"SSEServerTransport",
		"sse.ServerTransport",
		"StreamableHTTPServerTransport",
		"streamablehttp.ServerTransport",
	}

	for _, transport := range transports {
		if strings.Contains(sourceCode, transport) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "mcp_transport",
				MatchedText: transport,
				Score:       15,
				Confidence:  0.95,
			})
			*reasoning = append(*reasoning, "Found MCP transport: "+transport)
			return true
		}
	}

	// Check endpoints for MCP runtime transport
	for _, ep := range entity.Endpoints {
		if ep.Type == models.EndpointTypeMCPRuntime {
			*evidence = append(*evidence, models.Evidence{
				Type:        "mcp_protocol",
				Source:      "endpoints",
				Location:    ep.Endpoint.URL,
				Rule:        "mcp_runtime_endpoint",
				MatchedText: "MCP_RUNTIME_ENDPOINT",
				Score:       20,
				Confidence:  1.0,
			})
			*reasoning = append(*reasoning, "Found MCP runtime endpoint")
			return true
		}
	}
	return false
}

// checkToolDefinitions checks for tool definitions.
func (e *MCPIdentityEngine) checkToolDefinitions(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	patterns := []string{
		"tools.List",
		"tool.Definition",
		"tools.ListChanged",
		"mcp.Tool",
		"server.Tool",
	}

	for _, pattern := range patterns {
		if strings.Contains(sourceCode, pattern) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      "repository_source",
				Location:    entity.Repository.URL,
				Rule:        "tool_definitions",
				MatchedText: pattern,
				Score:       10,
				Confidence:  0.9,
			})
			*reasoning = append(*reasoning, "Found tool definitions: "+pattern)
			return true
		}
	}

	// Check tools list
	if len(entity.Tools) > 0 {
		*evidence = append(*evidence, models.Evidence{
			Type:        "mcp_protocol",
			Source:      "tools_list",
			Location:    entity.Repository.URL,
			Rule:        "tool_definitions",
			MatchedText: "tools list populated",
			Score:       10,
			Confidence:  0.85,
		})
		*reasoning = append(*reasoning, "Tools list is populated")
		return true
	}
	return false
}

// checkExecutableEntry checks for executable entrypoint.
func (e *MCPIdentityEngine) checkExecutableEntry(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	// Check package files for bin entrypoint
	for filename, content := range entity.Repository.PackageFiles {
		if strings.Contains(content, "\"bin\"") || strings.Contains(content, "bin:") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "package_manifest",
				Source:      filename,
				Location:    entity.Repository.URL,
				Rule:        "executable_entrypoint",
				MatchedText: "bin entrypoint",
				Score:       10,
				Confidence:  0.8,
			})
			return true
		}
		// Check go.mod for main package
		if strings.Contains(content, "package main") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "source_code",
				Source:      filename,
				Location:    entity.Repository.URL,
				Rule:        "executable_entrypoint",
				MatchedText: "package main",
				Score:       10,
				Confidence:  0.8,
			})
			return true
		}
		// Check Python entrypoint
		if strings.Contains(filename, "setup.py") || strings.Contains(filename, "pyproject.toml") {
			if strings.Contains(content, "entry_points") || strings.Contains(content, "console_scripts") {
				*evidence = append(*evidence, models.Evidence{
					Type:        "package_manifest",
					Source:      filename,
					Location:    entity.Repository.URL,
					Rule:        "executable_entrypoint",
					MatchedText: "entry_points/console_scripts",
					Score:       10,
					Confidence:  0.8,
				})
				return true
			}
		}
		// Check Rust Cargo.toml
		if strings.Contains(filename, "Cargo.toml") {
			if strings.Contains(content, "[[bin]]") || strings.Contains(content, "crate-type") {
				*evidence = append(*evidence, models.Evidence{
					Type:        "package_manifest",
					Source:      filename,
					Location:    entity.Repository.URL,
					Rule:        "executable_entrypoint",
					MatchedText: "Cargo.toml bin",
					Score:       10,
					Confidence:  0.8,
				})
				return true
			}
		}
	}
	return false
}

// checkMCPDependency checks for MCP-related dependencies.
func (e *MCPIdentityEngine) checkMCPDependency(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "modelcontextprotocol") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "package_manifest",
				Source:      "package_manifest",
				Location:    entity.Repository.URL,
				Rule:        "mcp_dependency",
				MatchedText: "modelcontextprotocol",
				Score:       5,
				Confidence:  0.8,
			})
			return true
		}
	}

	// Check topics
	for _, topic := range entity.Repository.Topics {
		if strings.Contains(strings.ToLower(topic), "mcp") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "repository_topics",
				Source:      "github_topics",
				Location:    entity.Repository.URL,
				Rule:        "mcp_topic",
				MatchedText: topic,
				Score:       5,
				Confidence:  0.7,
			})
			return true
		}
	}
	return false
}

// checkClientImpl checks for MCP client implementation.
func (e *MCPIdentityEngine) checkClientImpl(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "mcp.Client") || strings.Contains(sourceCode, "McpClient") ||
		strings.Contains(sourceCode, "client.Initialize") || strings.Contains(sourceCode, "ClientTransport") {
		*evidence = append(*evidence, models.Evidence{
			Type:        "source_code",
			Source:      "repository_source",
			Location:    entity.Repository.URL,
			Rule:        "mcp_client_impl",
			MatchedText: "McpClient",
			Score:       10,
			Confidence:  0.9,
		})
		*reasoning = append(*reasoning, "Found MCP client implementation")
		return true
	}
	return false
}

// checkHostImpl checks for MCP host implementation.
func (e *MCPIdentityEngine) checkHostImpl(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpHost") || strings.Contains(sourceCode, "mcp.Host") ||
		strings.Contains(sourceCode, "host.Manager") || strings.Contains(sourceCode, "MultiServer") ||
		strings.Contains(sourceCode, "host.Manager") {
		*evidence = append(*evidence, models.Evidence{
			Type:        "source_code",
			Source:      "repository_source",
			Location:    entity.Repository.URL,
			Rule:        "mcp_host_impl",
			MatchedText: "McpHost",
			Score:       10,
			Confidence:  0.9,
		})
		*reasoning = append(*reasoning, "Found MCP host implementation")
		return true
	}
	return false
}

// checkSDKPackage checks for SDK package publishing.
func (e *MCPIdentityEngine) checkSDKPackage(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "@modelcontextprotocol/sdk") ||
			strings.Contains(pkg, "mcp-sdk") ||
			strings.Contains(pkg, "mcp_sdk") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "package_manifest",
				Source:      "package_manifest",
				Location:    entity.Repository.URL,
				Rule:        "mcp_sdk_dependency",
				MatchedText: "@modelcontextprotocol/sdk",
				Score:       10,
				Confidence:  0.95,
			})
			*reasoning = append(*reasoning, "Depends on MCP SDK package")
			return true
		}
	}

	// Check topics for SDK keywords
	for _, topic := range entity.Repository.Topics {
		if strings.Contains(strings.ToLower(topic), "mcp-sdk") || strings.Contains(strings.ToLower(topic), "mcp_sdk") {
			*evidence = append(*evidence, models.Evidence{
				Type:        "repository_topics",
				Source:      "github_topics",
				Location:    entity.Repository.URL,
				Rule:        "mcp_sdk_topic",
				MatchedText: topic,
				Score:       5,
				Confidence:  0.8,
			})
			*reasoning = append(*reasoning, "Repository topics indicate MCP SDK")
			return true
		}
	}
	return false
}
// checkLibraryOnly checks for MCP library (not SDK).
func (e *MCPIdentityEngine) checkLibraryOnly(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	hasMCPDep := false
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "modelcontextprotocol") && !strings.Contains(pkg, "sdk") {
			hasMCPDep = true
			break
		}
	}

	for _, topic := range entity.Repository.Topics {
		if strings.Contains(strings.ToLower(topic), "mcp") && !strings.Contains(strings.ToLower(topic), "sdk") {
			hasMCPDep = true
			break
		}
	}

	if hasMCPDep {
		*evidence = append(*evidence, models.Evidence{
			Type:        "package_manifest",
			Source:      "package_manifest",
			Location:    entity.Repository.URL,
			Rule:        "mcp_library_dependency",
			MatchedText: "modelcontextprotocol (no sdk)",
			Score:       5,
			Confidence:  0.8,
		})
		*reasoning = append(*reasoning, "Depends on MCP library but not full SDK")
		return true
	}
	return false
}

// checkExtension checks for MCP extension.
func (e *MCPIdentityEngine) checkExtension(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpExtension") || strings.Contains(sourceCode, "mcp.Extension") ||
		strings.Contains(sourceCode, "extension.Plugin") {
		*evidence = append(*evidence, models.Evidence{
			Type:        "source_code",
			Source:      "repository_source",
			Location:    entity.Repository.URL,
			Rule:        "mcp_extension_impl",
			MatchedText: "McpExtension",
			Score:       10,
			Confidence:  0.9,
		})
		*reasoning = append(*reasoning, "Found MCP extension implementation")
		return true
	}
	return false
}

// checkSkill checks for MCP skill.
func (e *MCPIdentityEngine) checkSkill(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	sourceCode := e.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpSkill") || strings.Contains(sourceCode, "mcp.Skill") ||
		strings.Contains(sourceCode, "skill.Definition") {
		*evidence = append(*evidence, models.Evidence{
			Type:        "source_code",
			Source:      "repository_source",
			Location:    entity.Repository.URL,
			Rule:        "mcp_skill_impl",
			MatchedText: "McpSkill",
			Score:       10,
			Confidence:  0.9,
		})
		*reasoning = append(*reasoning, "Found MCP skill implementation")
		return true
	}
	return false
}

// checkOnlyReadmeMention checks if only README mentions MCP.
func (e *MCPIdentityEngine) checkOnlyReadmeMention(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	readme := strings.ToLower(entity.RawContent)
	hasMCPInReadme := strings.Contains(readme, "mcp") || strings.Contains(readme, "model context protocol")

	if hasMCPInReadme {
		*evidence = append(*evidence, models.Evidence{
			Type:        "readme",
			Source:      "README",
			Location:    entity.Repository.URL,
			Rule:        "only_readme_mention",
			MatchedText: "MCP mentioned in README",
			Score:       -5,
			Confidence:  0.7,
		})
		*reasoning = append(*reasoning, "MCP only mentioned in README")
		return true
	}
	return false
}

// checkTutorialOrExample checks if entity is a tutorial/example.
func (e *MCPIdentityEngine) checkTutorialOrExample(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)
	readme := strings.ToLower(entity.RawContent)

	keywords := []string{"tutorial", "guide", "howto", "walkthrough", "example", "demo", "sample", "showcase", "starter", "boilerplate", "learn", "getting-started"}

	for _, kw := range keywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) || strings.Contains(readme, kw) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "repository_metadata",
				Source:      "github_repo",
				Location:    entity.Repository.URL,
				Rule:        "tutorial_or_example",
				MatchedText: kw,
				Score:       -10,
				Confidence:  0.8,
			})
			*reasoning = append(*reasoning, "Repository appears to be tutorial/example: "+kw)
			return true
		}
	}
	return false
}

// checkCollectionOrRegistry checks if entity is a collection/registry.
func (e *MCPIdentityEngine) checkCollectionOrRegistry(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	keywords := []string{"awesome", "list", "collection", "registry", "curated", "index", "catalog"}

	for _, kw := range keywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			*evidence = append(*evidence, models.Evidence{
				Type:        "repository_metadata",
				Source:      "github_repo",
				Location:    entity.Repository.URL,
				Rule:        "collection_or_registry",
				MatchedText: kw,
				Score:       -10,
				Confidence:  0.85,
			})
			*reasoning = append(*reasoning, "Repository appears to be collection/registry: "+kw)
			return true
		}
	}

	// Check README for many GitHub links
	readme := entity.RawContent
	if strings.Count(readme, "github.com/") > 10 {
		*evidence = append(*evidence, models.Evidence{
			Type:        "readme",
			Source:      "README",
			Location:    entity.Repository.URL,
			Rule:        "many_repo_links",
			MatchedText: "Multiple github.com links",
			Score:       -5,
			Confidence:  0.7,
		})
		*reasoning = append(*reasoning, "README contains many GitHub repository links")
		return true
	}
	return false
}

// checkOnlyDocEndpoint checks if only documentation/installer endpoints.
func (e *MCPIdentityEngine) checkOnlyDocEndpoint(entity *models.Entity, evidence *[]models.Evidence, reasoning *[]string) bool {
	hasRuntime := false
	hasDocOrInstaller := false

	for _, ep := range entity.Endpoints {
		if ep.Type == models.EndpointTypeMCPRuntime {
			hasRuntime = true
		}
		if ep.Type == models.EndpointTypeDocumentation || ep.Type == models.EndpointTypeInstaller || ep.Type == models.EndpointTypeRepositoryURL {
			hasDocOrInstaller = true
		}
	}

	if hasDocOrInstaller && !hasRuntime {
		*evidence = append(*evidence, models.Evidence{
			Type:        "endpoint",
			Source:      "endpoints",
			Location:    entity.Repository.URL,
			Rule:        "only_doc_endpoint",
			MatchedText: "Only documentation/installer endpoints",
			Score:       -5,
			Confidence:  0.8,
		})
		*reasoning = append(*reasoning, "Only documentation/installer endpoints, no MCP runtime endpoint")
		return true
	}
	return false
}

// determineStatus determines the MCP identity status based on evidence.
func (e *MCPIdentityEngine) determineStatus(
	hasMCPImport, hasMCPServerImpl, hasTransport, hasToolDefs,
	hasExecutableEntry, hasMCPDep, hasClientImpl, hasHostImpl,
	hasSDKPackage, hasLibraryOnly, hasExtension, hasSkill,
	hasOnlyReadmeMention, isTutorialOrExample, isCollectionOrRegistry,
	hasOnlyDocEndpoint bool,
) (models.MCPIdentityStatus, models.MCPRole) {

	// Check for negative indicators first
	if isTutorialOrExample || isCollectionOrRegistry {
		return models.MCPIdentityStatusNotMCP, models.MCPRoleNone
	}

	// Count positive server indicators
	serverIndicators := 0
	if hasMCPImport {
		serverIndicators++
	}
	if hasMCPServerImpl {
		serverIndicators++
	}
	if hasTransport {
		serverIndicators++
	}
	if hasToolDefs {
		serverIndicators++
	}
	if hasExecutableEntry {
		serverIndicators++
	}

	// Check for server role
	if serverIndicators >= 2 {
		// Check runtime verification
		if hasTransport { // has runtime endpoint
			return models.MCPIdentityStatusRuntimeVerified, models.MCPRoleServer
		}
		return models.MCPIdentityStatusStaticVerified, models.MCPRoleServer
	}

	// Check for client role
	if hasClientImpl {
		return models.MCPIdentityStatusStaticVerified, models.MCPRoleClient
	}

	// Check for host role
	if hasHostImpl {
		return models.MCPIdentityStatusStaticVerified, models.MCPRoleHost
	}

	// Check for SDK role
	if hasSDKPackage {
		return models.MCPIdentityStatusStaticVerified, models.MCPRoleSDK
	}

	// Check for library role
	if hasLibraryOnly {
		return models.MCPIdentityStatusStaticVerified, models.MCPRoleLibrary
	}

	// Check for extension role
	// Note: would need hasExtension check

	// Check for skill role
	// Note: would need hasSkill check

	// If only negative indicators or only README mention
	if hasOnlyReadmeMention || hasOnlyDocEndpoint {
		return models.MCPIdentityStatusNotMCP, models.MCPRoleNone
	}

	// Has some MCP dependency but not enough for server
	if hasMCPDep {
		return models.MCPIdentityStatusCandidate, models.MCPRoleNone
	}

	return models.MCPIdentityStatusNotMCP, models.MCPRoleNone
}

// calculateConfidence calculates overall confidence from evidence.
func (e *MCPIdentityEngine) calculateConfidence(evidence []models.Evidence) float64 {
	if len(evidence) == 0 {
		return 0.1
	}

	totalScore := 0.0
	totalWeight := 0.0
	for _, ev := range evidence {
		weight := ev.Score
		if weight < 0 {
			weight = 5 // Minimum weight for negative evidence
		}
		totalScore += ev.Confidence * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0.1
	}

	confidence := totalScore / totalWeight

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// getSourceCodeText gets combined source code text from entity.
func (e *MCPIdentityEngine) getSourceCodeText(entity *models.Entity) string {
	var parts []string
	parts = append(parts, entity.Description)
	parts = append(parts, entity.RawContent)
	for _, pkg := range entity.Repository.PackageFiles {
		parts = append(parts, pkg)
	}
	return strings.Join(parts, "\n")
}