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

func TestPromote_StateMachine(t *testing.T) {
	// Candidate + staticChecked -> StaticVerified
	got := MCPIdentityStatusCandidate.Promote(true, RuntimeVerificationStatusFailed, SecurityStatusClean)
	if got != MCPIdentityStatusStaticVerified {
		t.Errorf("Candidate + static -> want StaticVerified, got %s", got)
	}
	// Candidate without staticChecked -> stays Candidate
	got = MCPIdentityStatusCandidate.Promote(false, RuntimeVerificationStatusPassed, SecurityStatusClean)
	if got != MCPIdentityStatusCandidate {
		t.Errorf("Candidate no static -> want Candidate, got %s", got)
	}
	// StaticVerified + runtime pass -> RuntimeVerified
	got = MCPIdentityStatusStaticVerified.Promote(true, RuntimeVerificationStatusPassed, SecurityStatusClean)
	if got != MCPIdentityStatusRuntimeVerified {
		t.Errorf("StaticVerified + runtime -> want RuntimeVerified, got %s", got)
	}
	// RuntimeVerified + static + runtime + security clean -> Verified
	got = MCPIdentityStatusRuntimeVerified.Promote(true, RuntimeVerificationStatusPassed, SecurityStatusClean)
	if got != MCPIdentityStatusVerified {
		t.Errorf("RuntimeVerified all pass -> want Verified, got %s", got)
	}
	// RuntimeVerified + security BLOCKED -> stays RuntimeVerified
	got = MCPIdentityStatusRuntimeVerified.Promote(true, RuntimeVerificationStatusPassed, SecurityStatusBlocked)
	if got != MCPIdentityStatusRuntimeVerified {
		t.Errorf("RuntimeVerified BLOCKED -> want RuntimeVerified, got %s", got)
	}
	// NotMCP never moves
	got = MCPIdentityStatusNotMCP.Promote(true, RuntimeVerificationStatusPassed, SecurityStatusClean)
	if got != MCPIdentityStatusNotMCP {
		t.Errorf("NotMCP should not transition, got %s", got)
	}
	// Verified stays Verified
	got = MCPIdentityStatusVerified.Promote(true, RuntimeVerificationStatusPassed, SecurityStatusClean)
	if got != MCPIdentityStatusVerified {
		t.Errorf("Verified should not regress, got %s", got)
	}
}

func TestShouldPromoteToVerified_Gating(t *testing.T) {
	if MCPIdentityStatusStaticVerified.ShouldPromoteToVerified(false, true, RuntimeVerificationStatusPassed, SecurityStatusClean) {
		t.Error("should not promote without staticChecked")
	}
	if MCPIdentityStatusStaticVerified.ShouldPromoteToVerified(true, false, RuntimeVerificationStatusPassed, SecurityStatusClean) {
		t.Error("should not promote without runtimeChecked")
	}
	if MCPIdentityStatusStaticVerified.ShouldPromoteToVerified(true, true, RuntimeVerificationStatusFailed, SecurityStatusClean) {
		t.Error("should not promote with runtime not Passed")
	}
	if MCPIdentityStatusStaticVerified.ShouldPromoteToVerified(true, true, RuntimeVerificationStatusPassed, SecurityStatusBlocked) {
		t.Error("should not promote with security BLOCKED")
	}
	if !MCPIdentityStatusStaticVerified.ShouldPromoteToVerified(true, true, RuntimeVerificationStatusPassed, SecurityStatusClean) {
		t.Error("should promote when all conditions met")
	}
}

func TestValidMCPIdentityStatuses_IncludesVerified(t *testing.T) {
	found := false
	for _, s := range ValidMCPIdentityStatuses {
		if s == MCPIdentityStatusVerified {
			found = true
			break
		}
	}
	if !found {
		t.Error("MCPIdentityStatusVerified missing from ValidMCPIdentityStatuses (T102)")
	}
}
