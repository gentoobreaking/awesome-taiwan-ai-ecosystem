package models

// PrimaryClassification represents the primary classification of an entity.
// Based on spec §2 entity types list.
type PrimaryClassification string

const (
	// MCP-related classifications
	PrimaryClassificationMCPServer         PrimaryClassification = "MCP_SERVER"
	PrimaryClassificationMCPClient         PrimaryClassification = "MCP_CLIENT"
	PrimaryClassificationMCPHost           PrimaryClassification = "MCP_HOST"
	PrimaryClassificationMCPSDK            PrimaryClassification = "MCP_SDK"
	PrimaryClassificationMCPLibrary        PrimaryClassification = "MCP_LIBRARY"
	PrimaryClassificationMCPExtension      PrimaryClassification = "MCP_EXTENSION"
	PrimaryClassificationMCPSkill          PrimaryClassification = "MCP_SKILL"
	PrimaryClassificationMCPCollection     PrimaryClassification = "MCP_COLLECTION"

	// AI-related classifications
	PrimaryClassificationAIAgent           PrimaryClassification = "AI_AGENT"
	PrimaryClassificationAITool            PrimaryClassification = "AI_TOOL"
	PrimaryClassificationAISDK             PrimaryClassification = "AI_SDK"
	PrimaryClassificationAIFramework       PrimaryClassification = "AI_FRAMEWORK"
	PrimaryClassificationAISkill           PrimaryClassification = "AI_SKILL"
	PrimaryClassificationAIKnowledgeBase   PrimaryClassification = "AI_KNOWLEDGE_BASE"
	PrimaryClassificationAIDataset         PrimaryClassification = "AI_DATASET"
	PrimaryClassificationAIAPI             PrimaryClassification = "AI_API"
	PrimaryClassificationAIApplication     PrimaryClassification = "AI_APPLICATION"
	PrimaryClassificationAIInfrastructure  PrimaryClassification = "AI_INFRASTRUCTURE"
	PrimaryClassificationAIPlugin          PrimaryClassification = "AI_PLUGIN"
	PrimaryClassificationAITutorial        PrimaryClassification = "AI_TUTORIAL"
	PrimaryClassificationAIExample         PrimaryClassification = "AI_EXAMPLE"
	PrimaryClassificationAICollection      PrimaryClassification = "AI_COLLECTION"
	PrimaryClassificationAIRegistry        PrimaryClassification = "AI_REGISTRY"

	// Other classifications
	PrimaryClassificationAIRelatedProject  PrimaryClassification = "AI_RELATED_PROJECT"
	PrimaryClassificationNonAIProject      PrimaryClassification = "NON_AI_PROJECT"
	PrimaryClassificationUnknown           PrimaryClassification = "UNKNOWN"
)

// ValidPrimaryClassifications contains all valid primary classification values.
var ValidPrimaryClassifications = []PrimaryClassification{
	PrimaryClassificationMCPServer,
	PrimaryClassificationMCPClient,
	PrimaryClassificationMCPHost,
	PrimaryClassificationMCPSDK,
	PrimaryClassificationMCPLibrary,
	PrimaryClassificationMCPExtension,
	PrimaryClassificationMCPSkill,
	PrimaryClassificationMCPCollection,
	PrimaryClassificationAIAgent,
	PrimaryClassificationAITool,
	PrimaryClassificationAISDK,
	PrimaryClassificationAIFramework,
	PrimaryClassificationAISkill,
	PrimaryClassificationAIKnowledgeBase,
	PrimaryClassificationAIDataset,
	PrimaryClassificationAIAPI,
	PrimaryClassificationAIApplication,
	PrimaryClassificationAIInfrastructure,
	PrimaryClassificationAIPlugin,
	PrimaryClassificationAITutorial,
	PrimaryClassificationAIExample,
	PrimaryClassificationAICollection,
	PrimaryClassificationAIRegistry,
	PrimaryClassificationAIRelatedProject,
	PrimaryClassificationNonAIProject,
	PrimaryClassificationUnknown,
}

var validPrimaryClassificationSet = map[PrimaryClassification]bool{
	PrimaryClassificationMCPServer:        true,
	PrimaryClassificationMCPClient:        true,
	PrimaryClassificationMCPHost:          true,
	PrimaryClassificationMCPSDK:           true,
	PrimaryClassificationMCPLibrary:       true,
	PrimaryClassificationMCPExtension:     true,
	PrimaryClassificationMCPSkill:         true,
	PrimaryClassificationMCPCollection:    true,
	PrimaryClassificationAIAgent:          true,
	PrimaryClassificationAITool:           true,
	PrimaryClassificationAISDK:            true,
	PrimaryClassificationAIFramework:      true,
	PrimaryClassificationAISkill:          true,
	PrimaryClassificationAIKnowledgeBase:  true,
	PrimaryClassificationAIDataset:        true,
	PrimaryClassificationAIAPI:            true,
	PrimaryClassificationAIApplication:    true,
	PrimaryClassificationAIInfrastructure: true,
	PrimaryClassificationAIPlugin:         true,
	PrimaryClassificationAITutorial:       true,
	PrimaryClassificationAIExample:        true,
	PrimaryClassificationAICollection:     true,
	PrimaryClassificationAIRegistry:       true,
	PrimaryClassificationAIRelatedProject: true,
	PrimaryClassificationNonAIProject:     true,
	PrimaryClassificationUnknown:          true,
}

// IsValidPrimaryClassification returns true if the classification is valid.
func IsValidPrimaryClassification(c PrimaryClassification) bool {
	return validPrimaryClassificationSet[c]
}

// IsMCPRelated returns true if the classification is MCP-related.
func IsMCPRelated(c PrimaryClassification) bool {
	switch c {
	case PrimaryClassificationMCPServer,
		PrimaryClassificationMCPClient,
		PrimaryClassificationMCPHost,
		PrimaryClassificationMCPSDK,
		PrimaryClassificationMCPLibrary,
		PrimaryClassificationMCPExtension,
		PrimaryClassificationMCPSkill,
		PrimaryClassificationMCPCollection:
		return true
	default:
		return false
	}
}

// IsAIRelated returns true if the classification is AI-related.
func IsAIRelated(c PrimaryClassification) bool {
	switch c {
	case PrimaryClassificationAIAgent,
		PrimaryClassificationAITool,
		PrimaryClassificationAISDK,
		PrimaryClassificationAIFramework,
		PrimaryClassificationAISkill,
		PrimaryClassificationAIKnowledgeBase,
		PrimaryClassificationAIDataset,
		PrimaryClassificationAIAPI,
		PrimaryClassificationAIApplication,
		PrimaryClassificationAIInfrastructure,
		PrimaryClassificationAIPlugin,
		PrimaryClassificationAITutorial,
		PrimaryClassificationAIExample,
		PrimaryClassificationAICollection,
		PrimaryClassificationAIRegistry,
		PrimaryClassificationAIRelatedProject:
		return true
	default:
		return false
	}
}

// MCPRole represents the MCP role of an entity.
type MCPRole string

const (
	MCPRoleServer     MCPRole = "SERVER"
	MCPRoleClient     MCPRole = "CLIENT"
	MCPRoleHost       MCPRole = "HOST"
	MCPRoleSDK        MCPRole = "SDK"
	MCPRoleLibrary    MCPRole = "LIBRARY"
	MCPRoleExtension  MCPRole = "EXTENSION"
	MCPRoleSkill      MCPRole = "SKILL"
	MCPRoleNone       MCPRole = "NONE"
)

// ValidMCPRoles contains all valid MCP role values.
var ValidMCPRoles = []MCPRole{
	MCPRoleServer,
	MCPRoleClient,
	MCPRoleHost,
	MCPRoleSDK,
	MCPRoleLibrary,
	MCPRoleExtension,
	MCPRoleSkill,
	MCPRoleNone,
}

var validMCPRoleSet = map[MCPRole]bool{
	MCPRoleServer:    true,
	MCPRoleClient:    true,
	MCPRoleHost:      true,
	MCPRoleSDK:       true,
	MCPRoleLibrary:   true,
	MCPRoleExtension: true,
	MCPRoleSkill:     true,
	MCPRoleNone:      true,
}

// IsValidMCPRole returns true if the role is valid.
func IsValidMCPRole(r MCPRole) bool {
	return validMCPRoleSet[r]
}