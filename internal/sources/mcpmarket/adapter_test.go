package mcpmarket

import (
	"context"
	"errors"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestNew_DefaultDisabled(t *testing.T) {
	a := New()
	if a.Enabled {
		t.Error("New() should default Enabled=false (T105)")
	}
}

func TestDiscover_DisabledReturnsErrSourceDisabled(t *testing.T) {
	a := New()
	_, err := a.Discover(context.Background())
	if !errors.Is(err, ErrSourceDisabled) {
		t.Errorf("Disabled Discover should return ErrSourceDisabled, got %v", err)
	}
	if !IsSourceDisabled(err) {
		t.Error("IsSourceDisabled() should detect the error")
	}
}

func TestFetch_DisabledReturnsErrSourceDisabled(t *testing.T) {
	a := New()
	_, err := a.Fetch(context.Background(), models.RawCandidate{Name: "x"})
	if !errors.Is(err, ErrSourceDisabled) {
		t.Errorf("Disabled Fetch should return ErrSourceDisabled, got %v", err)
	}
}

func TestDiscover_EnabledStillNotAvailable(t *testing.T) {
	// Even with Enabled=true, the source remains behind a Vercel WAF
	// and the real Discover still fails. The opt-in only changes the
	// log level from INFO 'source_skipped' to WARN 'source_error'.
	a := New()
	a.Enabled = true
	_, err := a.Discover(context.Background())
	if err == nil || errors.Is(err, ErrSourceDisabled) {
		t.Errorf("Enabled Discover should not return ErrSourceDisabled, got %v", err)
	}
}
