package models
import (
	"testing"
)

func TestIsValidSecurityStatus(t *testing.T) {
	valid := []SecurityStatus{
		SecurityStatusClean,
		SecurityStatusSuspicious,
		SecurityStatusQuarantined,
		SecurityStatusBlocked,
	}
	for _, s := range valid {
		if !IsValidSecurityStatus(s) {
			t.Errorf("Expected %s to be valid", s)
		}
	}
	if IsValidSecurityStatus("INVALID_STATUS") {
		t.Error("Expected INVALID_STATUS to be invalid")
	}
}

func TestValidSecurityStatuses_ContainsAll(t *testing.T) {
	expected := map[SecurityStatus]bool{
		SecurityStatusClean:       false,
		SecurityStatusSuspicious:  false,
		SecurityStatusQuarantined: false,
		SecurityStatusBlocked:     false,
	}
	for _, s := range ValidSecurityStatuses {
		expected[s] = true
	}
	for s, found := range expected {
		if !found {
			t.Errorf("SecurityStatus %s missing from ValidSecurityStatuses", s)
		}
	}
}

func TestCanTransitionSecurityStatus_Transitions(t *testing.T) {
	tests := []struct {
		name string
		from SecurityStatus
		to   SecurityStatus
		want bool
	}{
		// CLEAN → SUSPICIOUS, CLEAN → BLOCKED
		{"CLEAN -> SUSPICIOUS", SecurityStatusClean, SecurityStatusSuspicious, true},
		{"CLEAN -> BLOCKED", SecurityStatusClean, SecurityStatusBlocked, true},
		{"CLEAN -> CLEAN", SecurityStatusClean, SecurityStatusClean, false},
		{"CLEAN -> QUARANTINED", SecurityStatusClean, SecurityStatusQuarantined, false},

		// SUSPICIOUS → QUARANTINED, SUSPICIOUS → BLOCKED, SUSPICIOUS → CLEAN
		{"SUSPICIOUS -> QUARANTINED", SecurityStatusSuspicious, SecurityStatusQuarantined, true},
		{"SUSPICIOUS -> BLOCKED", SecurityStatusSuspicious, SecurityStatusBlocked, true},
		{"SUSPICIOUS -> CLEAN", SecurityStatusSuspicious, SecurityStatusClean, true},
		{"SUSPICIOUS -> SUSPICIOUS", SecurityStatusSuspicious, SecurityStatusSuspicious, false},

		// QUARANTINED → BLOCKED, QUARANTINED → CLEAN
		{"QUARANTINED -> BLOCKED", SecurityStatusQuarantined, SecurityStatusBlocked, true},
		{"QUARANTINED -> CLEAN", SecurityStatusQuarantined, SecurityStatusClean, true},
		{"QUARANTINED -> SUSPICIOUS", SecurityStatusQuarantined, SecurityStatusSuspicious, false},
		{"QUARANTINED -> QUARANTINED", SecurityStatusQuarantined, SecurityStatusQuarantined, false},

		// BLOCKED → nothing (terminal)
		{"BLOCKED -> CLEAN", SecurityStatusBlocked, SecurityStatusClean, false},
		{"BLOCKED -> BLOCKED", SecurityStatusBlocked, SecurityStatusBlocked, false},
		{"BLOCKED -> SUSPICIOUS", SecurityStatusBlocked, SecurityStatusSuspicious, false},

		// Invalid
		{"invalid -> CLEAN", SecurityStatus("INVALID"), SecurityStatusClean, false},
		{"CLEAN -> invalid", SecurityStatusClean, SecurityStatus("INVALID"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitionSecurityStatus(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionSecurityStatus(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestCanTransitionSecurity_Alias(t *testing.T) {
	// Verify the alias produces the same result
	if !CanTransitionSecurity(SecurityStatusClean, SecurityStatusSuspicious) {
		t.Error("Expected CanTransitionSecurity(CLEAN, SUSPICIOUS) = true")
	}
	if CanTransitionSecurity(SecurityStatusBlocked, SecurityStatusClean) {
		t.Error("Expected CanTransitionSecurity(BLOCKED, CLEAN) = false")
	}
}

func TestEntity_IsSafeForRegistry(t *testing.T) {
	tests := []struct {
		name    string
		status  SecurityStatus
		want    bool
	}{
		{"CLEAN", SecurityStatusClean, true},
		{"SUSPICIOUS", SecurityStatusSuspicious, true},
		{"QUARANTINED", SecurityStatusQuarantined, false},
		{"BLOCKED", SecurityStatusBlocked, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &Entity{
				SecurityStatus: SecurityStatusDetail{
					Status: tt.status,
				},
			}
			if got := entity.IsSafeForRegistry(); got != tt.want {
				t.Errorf("IsSafeForRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEntity_IsVerifiedForRegistry(t *testing.T) {
	tests := []struct {
		name   string
		status SecurityStatus
		want   bool
	}{
		{"CLEAN", SecurityStatusClean, true},
		{"SUSPICIOUS", SecurityStatusSuspicious, true},
		{"QUARANTINED", SecurityStatusQuarantined, true}, // QUARANTINED can still be "verified" (just not in views)
		{"BLOCKED", SecurityStatusBlocked, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &Entity{
				SecurityStatus: SecurityStatusDetail{
					Status: tt.status,
				},
			}
			if got := entity.IsVerifiedForRegistry(); got != tt.want {
				t.Errorf("IsVerifiedForRegistry() = %v, want %v", got, tt.want)
			}
		})
	}
}
