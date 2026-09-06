package models

import "testing"

func TestCanTransitionMCPIdentityStatus_Transitions(t *testing.T) {
	tests := []struct {
		name string
		from MCPIdentityStatus
		to   MCPIdentityStatus
		want bool
	}{
		{"CANDIDATE -> STATIC_VERIFIED", MCPIdentityStatusCandidate, MCPIdentityStatusStaticVerified, true},
		{"CANDIDATE -> NOT_MCP", MCPIdentityStatusCandidate, MCPIdentityStatusNotMCP, true},
		{"STATIC_VERIFIED -> RUNTIME_VERIFIED", MCPIdentityStatusStaticVerified, MCPIdentityStatusRuntimeVerified, true},
		{"STATIC_VERIFIED -> NOT_MCP", MCPIdentityStatusStaticVerified, MCPIdentityStatusNotMCP, true},
		{"RUNTIME_VERIFIED -> NOT_MCP", MCPIdentityStatusRuntimeVerified, MCPIdentityStatusNotMCP, true},
		{"CANDIDATE -> RUNTIME_VERIFIED (invalid)", MCPIdentityStatusCandidate, MCPIdentityStatusRuntimeVerified, false},
		{"NOT_MCP -> CANDIDATE (invalid)", MCPIdentityStatusNotMCP, MCPIdentityStatusCandidate, false},
		{"STATIC_VERIFIED -> CANDIDATE (invalid)", MCPIdentityStatusStaticVerified, MCPIdentityStatusCandidate, false},
		{"RUNTIME_VERIFIED -> STATIC_VERIFIED (invalid)", MCPIdentityStatusRuntimeVerified, MCPIdentityStatusStaticVerified, false},
		{"RUNTIME_VERIFIED -> RUNTIME_VERIFIED (invalid)", MCPIdentityStatusRuntimeVerified, MCPIdentityStatusRuntimeVerified, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitionMCPIdentityStatus(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionMCPIdentityStatus(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
