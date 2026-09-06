package models

import (
	"testing"
)

func TestEntity_IsVerifiedMCPServer(t *testing.T) {
	tests := []struct {
		name       string
		entity     *Entity
		want       bool
	}{
		{
			name: "Verified MCP Server",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusRuntimeVerified,
				},
			},
			want: true,
		},
		{
			name: "MCP Server but not runtime verified (static verified)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusStaticVerified,
				},
			},
			want: false,
		},
		{
			name: "MCP Server but not runtime verified (candidate)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusCandidate,
				},
			},
			want: false,
		},
		{
			name: "Not MCP Server (AI Agent)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationAIAgent,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusRuntimeVerified,
				},
			},
			want: false,
		},
		{
			name: "Not MCP Server (Non-AI Project)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationNonAIProject,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusRuntimeVerified,
				},
			},
			want: false,
		},
		{
			name: "MCP Server with NOT_MCP status",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusNotMCP,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entity.IsVerifiedMCPServer()
			if got != tt.want {
				t.Errorf("IsVerifiedMCPServer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntity_IsMCPServerCandidate(t *testing.T) {
	tests := []struct {
		name       string
		entity     *Entity
		want       bool
	}{
		{
			name: "MCP Server Candidate (Static Verified)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusStaticVerified,
				},
			},
			want: true,
		},
		{
			name: "MCP Server Candidate",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusCandidate,
				},
			},
			want: true,
		},
		{
			name: "Verified MCP Server (also candidate)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusRuntimeVerified,
				},
			},
			want: false, // RuntimeVerified is not in candidate list
		},
		{
			name: "Not MCP Server (AI Agent)",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationAIAgent,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusRuntimeVerified,
				},
			},
			want: false,
		},
		{
			name: "MCP Server with NOT_MCP status",
			entity: &Entity{
				Classification: ClassificationResult{
					Primary: PrimaryClassificationMCPServer,
				},
				MCPIdentity: MCPIdentity{
					Status: MCPIdentityStatusNotMCP,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entity.IsMCPServerCandidate()
			if got != tt.want {
				t.Errorf("IsMCPServerCandidate() = %v, want %v", got, tt.want)
			}
		})
	}
}