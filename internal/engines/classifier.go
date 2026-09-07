package engines

import (
	"context"
	"fmt"
	"strings"

	"github.com/david/awesome-taiwan-mcp/internal/config"
	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Classifier performs rule-based primary classification of entities,
// with an optional LLM fallback that runs when rule confidence is low
// (T108).
type Classifier struct {
	signals     *config.TaiwanSignals
	llmFallback *LLMClassifierFallback // optional; nil = pure rule-based
}

// ClassifierOption configures a Classifier.
type ClassifierOption func(*Classifier)

// WithLLMFallback attaches an LLM fallback. If nil is passed the
// option is a no-op.
func WithLLMFallback(fb *LLMClassifierFallback) ClassifierOption {
	return func(c *Classifier) {
		if fb != nil {
			c.llmFallback = fb
		}
	}
}

// NewClassifier creates a new entity classifier.
func NewClassifier(opts ...ClassifierOption) *Classifier {
	c := &Classifier{
		signals: config.DefaultTaiwanSignals(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Classify is the legacy no-ctx entry point. It runs rule-based
// classification only. New callers should prefer ClassifyWithCtx.
func (c *Classifier) Classify(entity *models.Entity) models.ClassificationResult {
	return c.ClassifyWithCtx(context.Background(), entity)
}

// ClassifyWithCtx runs rule-based classification, then falls back to
// the LLM classifier (if attached) when the rule confidence is below
// the LLM trigger threshold. (T108)
func (c *Classifier) ClassifyWithCtx(ctx context.Context, entity *models.Entity) models.ClassificationResult {
	result := c.classifyByRules(entity)

	if c.llmFallback == nil {
		return result
	}
	if !ShouldTriggerLLM(result, entity.TaiwanRelevance.Score, entity.AIRelevance.Score) {
		return result
	}

	llmResult := c.llmFallback.ClassifyWithLLM(ctx, entity, result)
	// Merge rule evidence + LLM evidence, and prepend an LLM-fallback
	// note to the reasoning string.
	llmResult.Evidence = append(llmResult.Evidence, result.Evidence...)
	llmResult.Reasoning = fmt.Sprintf(
		"[LLM fallback triggered; rule confidence %.2f] %s",
		result.Confidence,
		llmResult.Reasoning,
	)
	return llmResult
}

// classifyByRules contains the original rule-based logic.
func (c *Classifier) classifyByRules(entity *models.Entity) models.ClassificationResult {
	var evidence []models.ClassificationEvidence
	reasoning := []string{}

	// Priority -0.5: Data Library / Data Infrastructure guard
	// (spec §22, §23, §48, T111). A pure data SDK or database
	// schema that happens to expose an MCP-style API must not be
	// promoted to MCP_SERVER. Run before Priority 1 so the
	// isMCPServer rule never gets a chance to fire on these.
	if c.isDataLibraryOrInfrastructure(entity) {
		if c.isDataLibraryDescription(entity) {
			return c.buildResult(models.PrimaryClassificationAIDataset, evidence, reasoning, models.MCPRoleNone)
		}
		if c.isDataInfrastructureDescription(entity) {
			return c.buildResult(models.PrimaryClassificationAIInfrastructure, evidence, reasoning, models.MCPRoleNone)
		}
	}

	// Priority 1: MCP_SERVER - Source code contains MCP server implementation
	if c.isMCPServer(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPServer, evidence, reasoning, models.MCPRoleServer)
	}

	// Priority 2: MCP_CLIENT - Depends on MCP SDK but implements client logic
	if c.isMCPClient(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPClient, evidence, reasoning, models.MCPRoleClient)
	}

	// Priority 3: MCP_HOST - Implements MCP host managing multiple server connections
	if c.isMCPHost(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPHost, evidence, reasoning, models.MCPRoleHost)
	}

	// Priority 4: MCP_SDK - Publishes MCP SDK/package
	if c.isMCPSDK(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPSDK, evidence, reasoning, models.MCPRoleSDK)
	}

	// Priority 5: MCP_LIBRARY - MCP related library (non-SDK)
	if c.isMCPLibrary(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPLibrary, evidence, reasoning, models.MCPRoleLibrary)
	}

	// Priority 6: MCP_EXTENSION - MCP extension/plugin
	if c.isMCPExtension(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPExtension, evidence, reasoning, models.MCPRoleExtension)
	}

	// Priority 7: MCP_SKILL - MCP skill
	if c.isMCPSkill(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPSkill, evidence, reasoning, models.MCPRoleSkill)
	}

	// Priority 8: MCP_COLLECTION - awesome-list, registry, collection
	if c.isMCPCollection(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationMCPCollection, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 9: AI_AGENT - Implements AI agent
	if c.isAIAgent(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIAgent, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 10: AI_TOOL - AI tool/function
	if c.isAITool(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAITool, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 11: AI_SDK - AI SDK
	if c.isAISDK(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAISDK, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 12: AI_FRAMEWORK - AI framework
	if c.isAIFramework(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIFramework, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 13: AI_SKILL - Generic AI skill (non-MCP specific)
	if c.isAISkill(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAISkill, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 14: AI_KNOWLEDGE_BASE - Knowledge base
	if c.isAIKnowledgeBase(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIKnowledgeBase, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 15: AI_DATASET / DATA_LIBRARY - Dataset/database
	if c.isAIDataset(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIDataset, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 16: AI_API - AI API wrapper
	if c.isAIAPI(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIAPI, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 17: AI_APPLICATION - AI application
	if c.isAIApplication(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIApplication, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 18: AI_INFRASTRUCTURE - AI infrastructure
	if c.isAIInfrastructure(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIInfrastructure, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 19: AI_PLUGIN - AI plugin
	if c.isAIPlugin(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIPlugin, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 20: AI_TUTORIAL - Tutorial
	if c.isAITutorial(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAITutorial, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 21: AI_EXAMPLE - Example code
	if c.isAIExample(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIExample, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 22: AI_COLLECTION - AI related collection
	if c.isAICollection(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAICollection, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 23: AI_REGISTRY - AI registry
	if c.isAIRegistry(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIRegistry, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 24: AI_RELATED_PROJECT - AI related but not above
	if c.isAIRelatedProject(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationAIRelatedProject, evidence, reasoning, models.MCPRoleNone)
	}

	// Priority 25: NON_AI_PROJECT - No AI relevance
	if c.isNonAIProject(entity, &evidence, &reasoning) {
		return c.buildResult(models.PrimaryClassificationNonAIProject, evidence, reasoning, models.MCPRoleNone)
	}

	// Default: UNKNOWN
	return c.buildResult(models.PrimaryClassificationUnknown, evidence, reasoning, models.MCPRoleNone)
}

// Helper functions for classification rules

func (c *Classifier) isMCPServer(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check for MCP server implementation patterns
	hasMCPServerImpl := false
	hasStdioTransport := false
	hasToolDefs := false
	hasExecutableEntry := false

	// Check source code patterns
	sourceCode := c.getSourceCodeText(entity)
	if strings.Contains(sourceCode, "McpServer") || strings.Contains(sourceCode, "mcp.Server") {
		hasMCPServerImpl = true
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "mcp_server_impl", "McpServer", 0.9))
		*reasoning = append(*reasoning, "Source code contains McpServer implementation")
	}

	if strings.Contains(sourceCode, "StdioServerTransport") || strings.Contains(sourceCode, "stdio.ServerTransport") {
		hasStdioTransport = true
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "stdio_transport", "StdioServerTransport", 0.85))
		*reasoning = append(*reasoning, "Uses StdioServerTransport")
	}

	// Check for tool definitions
	if strings.Contains(sourceCode, "tools.List") || strings.Contains(sourceCode, "tool.Definition") ||
		strings.Contains(sourceCode, "tools.ListChanged") {
		hasToolDefs = true
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "tool_definitions", "tools.List", 0.9))
		*reasoning = append(*reasoning, "Contains tool definitions")
	}

	// Check package.json for bin entrypoint
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "\"bin\"") || strings.Contains(pkg, "bin:") {
			hasExecutableEntry = true
			*evidence = append(*evidence, c.newEvidence("package_manifest", "package.json", "executable_entrypoint", "bin", 0.8))
			*reasoning = append(*reasoning, "Package manifest has executable entrypoint")
			break
		}
		// Check go.mod for main package
		if strings.Contains(pkg, "package main") {
			hasExecutableEntry = true
			*evidence = append(*evidence, c.newEvidence("source_code", "go.mod/main.go", "executable_entrypoint", "package main", 0.8))
			*reasoning = append(*reasoning, "Go package has main function")
			break
		}
	}

	// Runtime verification status
	if entity.MCPIdentity.Status == models.MCPIdentityStatusRuntimeVerified {
		*evidence = append(*evidence, c.newEvidence("runtime_handshake", "mcp_verification", "runtime_verified", "MCPIdentityStatusRuntimeVerified", 1.0))
		*reasoning = append(*reasoning, "Runtime verification passed")
		return true
	}

	// Need at least 2 indicators for MCP_SERVER classification
	matches := 0
	if hasMCPServerImpl {
		matches++
	}
	if hasStdioTransport {
		matches++
	}
	if hasToolDefs {
		matches++
	}
	if hasExecutableEntry {
		matches++
	}

	return matches >= 2
}

func (c *Classifier) isMCPClient(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	sourceCode := c.getSourceCodeText(entity)
	hasClientImpl := false

	// Check for MCP client patterns
	if strings.Contains(sourceCode, "mcp.Client") || strings.Contains(sourceCode, "McpClient") {
		hasClientImpl = true
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "mcp_client_impl", "mcp.Client", 0.9))
		*reasoning = append(*reasoning, "Source code contains MCP client implementation")
	}

	// Check for client dependencies without server patterns
	hasServerPatterns := strings.Contains(sourceCode, "McpServer") || strings.Contains(sourceCode, "StdioServerTransport")
	if hasClientImpl && !hasServerPatterns {
		return true
	}

	return false
}

func (c *Classifier) isMCPHost(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	sourceCode := c.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpHost") || strings.Contains(sourceCode, "mcp.Host") ||
		strings.Contains(sourceCode, "host.Manager") || strings.Contains(sourceCode, "MultiServer") {
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "mcp_host_impl", "McpHost", 0.9))
		*reasoning = append(*reasoning, "Source code contains MCP host implementation")
		return true
	}

	return false
}

func (c *Classifier) isMCPSDK(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check package manifest for SDK patterns
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "@modelcontextprotocol/sdk") ||
			strings.Contains(pkg, "mcp-sdk") ||
			strings.Contains(pkg, "mcp_sdk") {
			*evidence = append(*evidence, c.newEvidence("package_manifest", "package.json/go.mod", "mcp_sdk_dependency", "@modelcontextprotocol/sdk", 0.95))
			*reasoning = append(*reasoning, "Depends on MCP SDK package")
			return true
		}
	}

	// Check topics for SDK keywords
	for _, topic := range entity.Repository.Topics {
		if strings.Contains(strings.ToLower(topic), "mcp-sdk") || strings.Contains(strings.ToLower(topic), "mcp_sdk") {
			*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "mcp_sdk_topic", topic, 0.8))
			*reasoning = append(*reasoning, "Repository topics indicate MCP SDK")
			return true
		}
	}

	return false
}

func (c *Classifier) isMCPLibrary(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// MCP library has MCP dependencies but not full SDK
	hasMCPDep := false
	for _, pkg := range entity.Repository.PackageFiles {
		if strings.Contains(pkg, "modelcontextprotocol") && !strings.Contains(pkg, "sdk") {
			hasMCPDep = true
			break
		}
	}

	// Check topics
	for _, topic := range entity.Repository.Topics {
		if strings.Contains(strings.ToLower(topic), "mcp") && !strings.Contains(strings.ToLower(topic), "sdk") {
			hasMCPDep = true
			break
		}
	}

	if hasMCPDep {
		*evidence = append(*evidence, c.newEvidence("package_manifest", "package_manifest", "mcp_library_dependency", "modelcontextprotocol", 0.8))
		*reasoning = append(*reasoning, "Depends on MCP library but not full SDK")
		return true
	}

	return false
}

func (c *Classifier) isMCPExtension(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	sourceCode := c.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpExtension") || strings.Contains(sourceCode, "mcp.Extension") ||
		strings.Contains(sourceCode, "extension.Plugin") {
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "mcp_extension_impl", "McpExtension", 0.9))
		*reasoning = append(*reasoning, "Source code contains MCP extension implementation")
		return true
	}

	return false
}

func (c *Classifier) isMCPSkill(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	sourceCode := c.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "McpSkill") || strings.Contains(sourceCode, "mcp.Skill") ||
		strings.Contains(sourceCode, "skill.Definition") {
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "mcp_skill_impl", "McpSkill", 0.9))
		*reasoning = append(*reasoning, "Source code contains MCP skill implementation")
		return true
	}

	return false
}

func (c *Classifier) isMCPCollection(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check for collection indicators
	isCollection := false

	// Repository name/topics indicate collection
	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	collectionKeywords := []string{"awesome", "list", "collection", "registry", "curated", "index", "catalog"}
	for _, kw := range collectionKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			isCollection = true
			*evidence = append(*evidence, c.newEvidence("repository_metadata", "github_repo", "collection_indicator", kw, 0.85))
			break
		}
	}

	// Check topics
	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, kw := range collectionKeywords {
			if strings.Contains(topicLower, kw) {
				isCollection = true
				*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "collection_topic", topic, 0.8))
				break
			}
		}
	}

	// Check if it has multiple repositories listed (README with many links)
	readme := c.getReadmeText(entity)
	if strings.Count(readme, "github.com/") > 10 {
		isCollection = true
		*evidence = append(*evidence, c.newEvidence("readme", "README", "many_repo_links", "Multiple github.com links", 0.7))
	}

	if isCollection {
		*reasoning = append(*reasoning, "Repository appears to be a collection/registry of MCP servers")
		return true
	}

	return false
}

func (c *Classifier) isAIAgent(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	hasAgentImpl := false
	sourceCode := c.getSourceCodeText(entity)

	// Check for agent patterns
	agentPatterns := []string{"Agent", "agent.Executor", "autonomous", "planner", "reasoning", "chain-of-thought"}
	for _, pattern := range agentPatterns {
		if strings.Contains(sourceCode, pattern) {
			hasAgentImpl = true
			*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "agent_impl", pattern, 0.85))
			break
		}
	}

	// Check tools for agent-like functions
	for _, tool := range entity.Tools {
		toolDesc := strings.ToLower(tool.Description)
		toolName := strings.ToLower(tool.Name)
		if strings.Contains(toolDesc, "agent") || strings.Contains(toolName, "agent") ||
			strings.Contains(toolDesc, "autonomous") || strings.Contains(toolDesc, "plan") {
			hasAgentImpl = true
			*evidence = append(*evidence, c.newEvidence("mcp_tool", "tools_list", "agent_tool", tool.Name, 0.8))
			break
		}
	}

	// Check AI relevance for agent
	if entity.AIRelevance.Level == models.AIRelevanceLevelA5 || entity.AIRelevance.Level == models.AIRelevanceLevelA4 {
		*evidence = append(*evidence, c.newEvidence("ai_relevance", "ai_scoring", "high_ai_relevance", string(entity.AIRelevance.Level), 0.7))
	}

	if hasAgentImpl {
		*reasoning = append(*reasoning, "Source code or tools indicate AI agent implementation")
		return true
	}

	return false
}

func (c *Classifier) isAITool(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check for tool-like implementations
	sourceCode := c.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "tool.Function") || strings.Contains(sourceCode, "ToolDefinition") {
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "tool_definition", "ToolDefinition", 0.85))
		*reasoning = append(*reasoning, "Source code contains tool/function definitions")
		return true
	}

	// Check tools list
	if len(entity.Tools) > 0 && len(entity.Tools) <= 5 {
		*evidence = append(*evidence, c.newEvidence("mcp_tool", "tools_list", "few_tools", "Tools list", 0.75))
		*reasoning = append(*reasoning, "Small number of tools suggests tool-oriented project")
		return true
	}

	return false
}

func (c *Classifier) isAISDK(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check for AI SDK patterns in dependencies
	aiSDKKeywords := []string{"langchain", "llamaindex", "autogen", "crewai", "semantic-kernel", "langgraph",
		"openai-sdk", "anthropic-sdk", "google-generativeai", "cohere-sdk"}

	for _, pkg := range entity.Repository.PackageFiles {
		for _, kw := range aiSDKKeywords {
			if strings.Contains(strings.ToLower(pkg), kw) {
				*evidence = append(*evidence, c.newEvidence("package_manifest", "package_manifest", "ai_sdk_dependency", kw, 0.9))
				*reasoning = append(*reasoning, "Depends on AI SDK")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAIFramework(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	frameworkKeywords := []string{"langchain", "llamaindex", "autogen", "crewai", "semantic-kernel", "langgraph", "haystack"}

	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, fw := range frameworkKeywords {
			if strings.Contains(topicLower, fw) {
				*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "ai_framework_topic", topic, 0.85))
				*reasoning = append(*reasoning, "Repository topics indicate AI framework")
				return true
			}
		}
	}

	for _, pkg := range entity.Repository.PackageFiles {
		for _, fw := range frameworkKeywords {
			if strings.Contains(strings.ToLower(pkg), fw) {
				*evidence = append(*evidence, c.newEvidence("package_manifest", "package_manifest", "ai_framework_dependency", fw, 0.9))
				*reasoning = append(*reasoning, "Depends on AI framework")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAISkill(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	sourceCode := c.getSourceCodeText(entity)

	if strings.Contains(sourceCode, "Skill") && (strings.Contains(sourceCode, "skill.Definition") || strings.Contains(sourceCode, "Skill(")) {
		*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "skill_definition", "Skill", 0.85))
		*reasoning = append(*reasoning, "Source code contains skill definition")
		return true
	}

	return false
}

func (c *Classifier) isAIKnowledgeBase(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	kbKeywords := []string{"knowledge", "knowledge-base", "vector-db", "vector-database", "embeddings", "rag", "retrieval"}

	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, kw := range kbKeywords {
			if strings.Contains(topicLower, kw) {
				*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "knowledge_base_topic", topic, 0.85))
				*reasoning = append(*reasoning, "Repository topics indicate knowledge base")
				return true
			}
		}
	}

	descLower := strings.ToLower(entity.Description)
	for _, kw := range kbKeywords {
		if strings.Contains(descLower, kw) {
			*evidence = append(*evidence, c.newEvidence("readme", "README", "knowledge_base_description", kw, 0.75))
			*reasoning = append(*reasoning, "Description indicates knowledge base")
			return true
		}
	}

	return false
}

func (c *Classifier) isAIDataset(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Check data sources for dataset types
	for _, ds := range entity.DataSources {
		if ds.Type == models.DataSourceOfficialGovAPI || ds.Type == models.DataSourceOpenData ||
			ds.Type == models.DataSourceOfficial || ds.Type == models.DataSourceCommunity {
			*evidence = append(*evidence, c.newEvidence("data_source", "data_sources", "dataset_type", string(ds.Type), 0.9))
			*reasoning = append(*reasoning, "Data source indicates dataset/database")
			return true
		}
	}
	// Check topics
	datasetKeywords := []string{"dataset", "data", "database", "csv", "parquet", "sql"}
	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, kw := range datasetKeywords {
			if strings.Contains(topicLower, kw) {
				*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "dataset_topic", topic, 0.8))
				*reasoning = append(*reasoning, "Repository topics indicate dataset")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAIAPI(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	apiKeywords := []string{"api", "wrapper", "client", "sdk", "rest", "graphql"}

	// Check for API patterns in source code
	sourceCode := c.getSourceCodeText(entity)
	for _, kw := range apiKeywords {
		if strings.Contains(strings.ToLower(sourceCode), kw) {
			// Must also have AI relevance
			if entity.AIRelevance.Score > 15 {
				*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "api_wrapper", kw, 0.75))
				*reasoning = append(*reasoning, "Source code indicates API wrapper with AI relevance")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAIApplication(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	appKeywords := []string{"app", "application", "web", "ui", "frontend", "dashboard", "chat", "bot"}

	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	for _, kw := range appKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			if entity.AIRelevance.Score > 10 {
				*evidence = append(*evidence, c.newEvidence("repository_metadata", "github_repo", "ai_app_indicator", kw, 0.8))
				*reasoning = append(*reasoning, "Repository name/description indicates AI application")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAIInfrastructure(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	infraKeywords := []string{"infrastructure", "platform", "deploy", "kubernetes", "k8s", "operator", "orchestration"}

	for _, topic := range entity.Repository.Topics {
		topicLower := strings.ToLower(topic)
		for _, kw := range infraKeywords {
			if strings.Contains(topicLower, kw) {
				if entity.AIRelevance.Score > 10 {
					*evidence = append(*evidence, c.newEvidence("repository_topics", "github_topics", "ai_infrastructure_topic", topic, 0.85))
					*reasoning = append(*reasoning, "Repository topics indicate AI infrastructure")
					return true
				}
			}
		}
	}

	return false
}

func (c *Classifier) isAIPlugin(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	pluginKeywords := []string{"plugin", "extension", "addon", "integration", "hook"}

	sourceCode := c.getSourceCodeText(entity)
	for _, kw := range pluginKeywords {
		if strings.Contains(strings.ToLower(sourceCode), kw) && strings.Contains(sourceCode, "AI") {
			*evidence = append(*evidence, c.newEvidence("source_code", "repository_source", "ai_plugin", kw, 0.8))
			*reasoning = append(*reasoning, "Source code indicates AI plugin")
			return true
		}
	}

	return false
}

func (c *Classifier) isAITutorial(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	tutorialKeywords := []string{"tutorial", "guide", "howto", "walkthrough", "example", "learn", "getting-started"}

	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)
	readme := strings.ToLower(c.getReadmeText(entity))

	for _, kw := range tutorialKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) || strings.Contains(readme, kw) {
			*evidence = append(*evidence, c.newEvidence("readme", "README", "tutorial_indicator", kw, 0.8))
			*reasoning = append(*reasoning, "Repository indicates tutorial/guide")
			return true
		}
	}

	return false
}

func (c *Classifier) isAIExample(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	exampleKeywords := []string{"example", "demo", "sample", "showcase", "starter", "boilerplate"}

	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	for _, kw := range exampleKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			*evidence = append(*evidence, c.newEvidence("repository_metadata", "github_repo", "example_indicator", kw, 0.8))
			*reasoning = append(*reasoning, "Repository name/description indicates example")
			return true
		}
	}

	return false
}

func (c *Classifier) isAICollection(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Similar to MCP_COLLECTION but for AI
	collectionKeywords := []string{"awesome", "list", "collection", "curated", "resources", "links"}

	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	for _, kw := range collectionKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			if entity.AIRelevance.Score > 15 {
				*evidence = append(*evidence, c.newEvidence("repository_metadata", "github_repo", "ai_collection_indicator", kw, 0.85))
				*reasoning = append(*reasoning, "Repository indicates AI collection with AI relevance")
				return true
			}
		}
	}

	return false
}

func (c *Classifier) isAIRegistry(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	registryKeywords := []string{"registry", "catalog", "index", "directory", "marketplace"}

	repoName := strings.ToLower(entity.Repository.Name)
	repoDesc := strings.ToLower(entity.Description)

	for _, kw := range registryKeywords {
		if strings.Contains(repoName, kw) || strings.Contains(repoDesc, kw) {
			*evidence = append(*evidence, c.newEvidence("repository_metadata", "github_repo", "registry_indicator", kw, 0.85))
			*reasoning = append(*reasoning, "Repository indicates AI registry")
			return true
		}
	}

	return false
}

func (c *Classifier) isAIRelatedProject(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	// Has AI relevance but doesn't match specific categories
	if entity.AIRelevance.Score > 5 {
		*evidence = append(*evidence, c.newEvidence("ai_relevance", "ai_scoring", "ai_relevant", string(entity.AIRelevance.Level), 0.7))
		*reasoning = append(*reasoning, "Entity has AI relevance but doesn't match specific categories")
		return true
	}

	return false
}

func (c *Classifier) isNonAIProject(entity *models.Entity, evidence *[]models.ClassificationEvidence, reasoning *[]string) bool {
	if entity.AIRelevance.Score == 0 && entity.TaiwanRelevance.Score == 0 {
		*evidence = append(*evidence, c.newEvidence("ai_relevance", "ai_scoring", "no_ai_signals", "AI relevance = 0", 1.0))
		*reasoning = append(*reasoning, "No AI or Taiwan relevance signals detected")
		return true
	}

	return false
}

func (c *Classifier) getSourceCodeText(entity *models.Entity) string {
	// Combine all available source code text
	var parts []string
	parts = append(parts, entity.Description)
	parts = append(parts, entity.RawContent)
	for _, pkg := range entity.Repository.PackageFiles {
		parts = append(parts, pkg)
	}
	return strings.Join(parts, "\n")
}

func (c *Classifier) getReadmeText(entity *models.Entity) string {
	// Use RawContent as README approximation
	return entity.RawContent
}

func (c *Classifier) newEvidence(evidenceType, source, rule, matchedText string, confidence float64) models.ClassificationEvidence {
	return models.ClassificationEvidence{
		Evidence: models.Evidence{
			Type:        evidenceType,
			Source:      source,
			Rule:        rule,
			MatchedText: matchedText,
			Confidence:  confidence,
		},
	}
}

func (c *Classifier) buildResult(primary models.PrimaryClassification, evidence []models.ClassificationEvidence, reasoning []string, mcpRole models.MCPRole) models.ClassificationResult {
	// Calculate confidence based on evidence count and strength
	confidence := 0.0
	if len(evidence) > 0 {
		totalConf := 0.0
		for _, e := range evidence {
			totalConf += e.Confidence
		}
		confidence = totalConf / float64(len(evidence))
	} else {
		confidence = 0.1 // Low confidence for UNKNOWN
	}

	// Cap confidence at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return models.ClassificationResult{
		Primary:      primary,
		Confidence:   confidence,
		Evidence:     evidence,
		MCPRole:      mcpRole,
		Reasoning:    strings.Join(reasoning, "; "),
	}
}
// T111: data library / data infrastructure description signals.
// These short-circuit MCP_SERVER promotion per spec §22, §23, §48.

var dataLibrarySignals = []string{
	"data library", "data sdk", "market data", "stock data", "data pipeline",
	"data feed", "data api", "stock api", "twse api", "tpex api",
	"金融資料", "股市資料", "市場資料",
}

var dataInfrastructureSignals = []string{
	"database schema", "migration system", "etl", "data warehouse",
	"data pipeline", "data layer", "postgres schema", "postgresql schema",
	"alembic", "flyway", "prisma schema", "knex migrations",
}

// isDataLibraryOrInfrastructure reports whether the entity's
// description hints at a data-only project (which the spec forbids
// from being upgraded to MCP_SERVER). (T111)
func (c *Classifier) isDataLibraryOrInfrastructure(entity *models.Entity) bool {
	return c.isDataLibraryDescription(entity) || c.isDataInfrastructureDescription(entity)
}

// isDataLibraryDescription checks description text for SDK/data-feed
// signals. Returns false if the description also claims to be an
// MCP server (then it should be classified as MCP_SERVER, not as a
// data library — T111). (T111)
func (c *Classifier) isDataLibraryDescription(entity *models.Entity) bool {
	desc := strings.ToLower(entity.Description)
	// Suppress: if the description claims MCP server, this is not a
	// pure data library.
	for _, mcp := range []string{"mcp server", "model context protocol server", "fastmcp", "stdio_server"} {
		if strings.Contains(desc, mcp) {
			return false
		}
	}
	for _, s := range dataLibrarySignals {
		if strings.Contains(desc, s) {
			return true
		}
	}
	// Also check name patterns like 'twmarketdata', 'twstock-api'.
	name := strings.ToLower(entity.Name)
	for _, s := range []string{"twmarket", "twstock", "twdata", "taiwandata", "fugle"} {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}

// isDataInfrastructureDescription checks description for db /
// schema / migration / ETL / warehouse signals. Same MCP-server
// suppression as the library rule. (T111)
func (c *Classifier) isDataInfrastructureDescription(entity *models.Entity) bool {
	desc := strings.ToLower(entity.Description)
	for _, mcp := range []string{"mcp server", "model context protocol server", "fastmcp", "stdio_server"} {
		if strings.Contains(desc, mcp) {
			return false
		}
	}
	for _, s := range dataInfrastructureSignals {
		if strings.Contains(desc, s) {
			return true
		}
	}
	for _, t := range entity.Repository.Topics {
		topic := strings.ToLower(t)
		if topic == "database" || topic == "schema" || topic == "etl" ||
			topic == "postgres" || topic == "postgresql" || topic == "sqlalchemy" {
			return true
		}
	}
	return false
}
