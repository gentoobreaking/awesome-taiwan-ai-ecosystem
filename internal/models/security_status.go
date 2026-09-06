package models

// SecurityStatus represents the security assessment status.
// Based on spec §12, §54, §56 Test 12, §61 Phase 8.
type SecurityStatus string

const (
	// SecurityStatusClean - No security issues found.
	SecurityStatusClean SecurityStatus = "CLEAN"
	// SecurityStatusSuspicious - Low/medium risk patterns found, needs attention but not blocking.
	SecurityStatusSuspicious SecurityStatus = "SUSPICIOUS"
	// SecurityStatusQuarantined - High risk/suspicious malicious patterns, isolated for manual review (spec §12, §56 Test 12).
	SecurityStatusQuarantined SecurityStatus = "QUARANTINED"
	// SecurityStatusBlocked - Confirmed malicious, permanently blocked.
	SecurityStatusBlocked SecurityStatus = "BLOCKED"
)

// ValidSecurityStatuses contains all valid security status values.
var ValidSecurityStatuses = []SecurityStatus{
	SecurityStatusClean,
	SecurityStatusSuspicious,
	SecurityStatusQuarantined,
	SecurityStatusBlocked,
}

var validSecurityStatusSet = map[SecurityStatus]bool{
	SecurityStatusClean:       true,
	SecurityStatusSuspicious:  true,
	SecurityStatusQuarantined: true,
	SecurityStatusBlocked:     true,
}

// IsValidSecurityStatus returns true if the status is valid.
func IsValidSecurityStatus(s SecurityStatus) bool {
	return validSecurityStatusSet[s]
}

// CanTransitionSecurityStatus returns true if the transition is valid.
// Valid transitions (spec §54):
//   - CLEAN → SUSPICIOUS (new scan finds risk)
//   - SUSPICIOUS → QUARANTINED (risk escalated or manual judgment)
//   - QUARANTINED → BLOCKED (confirmed malicious)
//   - QUARANTINED → CLEAN (false positive, manually confirmed)
//   - Any → BLOCKED (emergency block)
func CanTransitionSecurityStatus(from, to SecurityStatus) bool {
	switch from {
	case SecurityStatusClean:
		return to == SecurityStatusSuspicious || to == SecurityStatusBlocked
	case SecurityStatusSuspicious:
		return to == SecurityStatusQuarantined || to == SecurityStatusBlocked || to == SecurityStatusClean
	case SecurityStatusQuarantined:
		return to == SecurityStatusBlocked || to == SecurityStatusClean
	case SecurityStatusBlocked:
		return false // Once blocked, stays blocked
	default:
		return false
	}
}

// CanTransitionSecurity is an alias for CanTransitionSecurityStatus.
// Spec §64 Definition of Done uses the shorter name.
func CanTransitionSecurity(from, to SecurityStatus) bool {
	return CanTransitionSecurityStatus(from, to)
}

// IsSafeForRegistry returns true if the entity's security status allows it
// to appear in public registry views (spec §54).
// BLOCKED entities are permanently excluded; QUARANTINED entities are excluded
// pending manual review.
func (e *Entity) IsSafeForRegistry() bool {
	return e.SecurityStatus.Status != SecurityStatusBlocked &&
		e.SecurityStatus.Status != SecurityStatusQuarantined
}

// IsVerifiedForRegistry returns true if the entity meets the security
// requirement for Verified MCP Servers (spec §54):
// security_status != BLOCKED.
func (e *Entity) IsVerifiedForRegistry() bool {
	return e.SecurityStatus.Status != SecurityStatusBlocked
}
