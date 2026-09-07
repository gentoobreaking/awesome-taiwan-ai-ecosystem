package mcpmarket

import (
	"context"
	"errors"
	"fmt"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// Adapter is a skeleton for mcpmarket.com.
// The site is currently protected by Vercel Bot Protection (Security Checkpoint)
// which returns HTTP 403/429 for all automated requests including robots.txt and
// any /api/* endpoints. No public JSON API is available and SSR requires passing
// a JS challenge (x-vercel-mitigated: challenge). Therefore automated crawling is
// blocked until the operator provides an official API or allowlist.
//
// T105: when Enabled is false the adapter short-circuits in Discover
// and Fetch, returning ErrSourceDisabled. The coordinator recognises
// this and logs an INFO 'source_skipped' event instead of a WARN
// 'source_error', so the permanent WAF failure no longer pollutes
// the crawler log.
//
// See local://audit-sources.md §2.1 for full probe details.
type Adapter struct {
	Enabled   bool
	BaseURL   string
	HTTPClient interface{}
}

// ErrSourceDisabled indicates the adapter was constructed with
// Enabled=false. (T105)
var ErrSourceDisabled = errors.New("mcpmarket: source disabled (set Adapter.Enabled = true to opt-in)")

// New creates a new mcpmarket adapter. Default disabled (T105).
// Pass a pointer to the returned value and set Enabled=true to opt in.
func New() *Adapter {
	return &Adapter{
		Enabled: false,
		BaseURL: "https://mcpmarket.com",
	}
}

func (a *Adapter) Name() string { return "mcpmarket" }

// TrustScore returns the trust score for MCPMarket source (registry-based, lower than GitHub).
func (a *Adapter) TrustScore() float64 { return 0.7 }

var _ sources.SourceAdapter = (*Adapter)(nil)

// IsSourceDisabled reports whether the error is ErrSourceDisabled.
// Exposed so the coordinator can decide between INFO (skipped) and
// WARN (real error) log levels. (T105)
func IsSourceDisabled(err error) bool {
	return errors.Is(err, ErrSourceDisabled)
}

// Discover short-circuits when disabled. Otherwise returns
// ErrNotAvailable because Vercel WAF blocks all automated discovery.
// TODO: activate when mcpmarket provides official API or Vercel allowlist.
// Do not attempt to bypass Cloudflare/Vercel challenge without authorization.
func (a *Adapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	if !a.Enabled {
		return nil, ErrSourceDisabled
	}
	return nil, fmt.Errorf("mcpmarket: %w — Vercel WAF challenge required, waiting for official API cooperation", sources.ErrNotAvailable)
}

// Fetch short-circuits when disabled. Otherwise returns
// ErrNotAvailable for the same reason.
func (a *Adapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	if !a.Enabled {
		return nil, ErrSourceDisabled
	}
	return nil, fmt.Errorf("mcpmarket: %w — not implemented (blocked by Vercel WAF)", sources.ErrNotAvailable)
}
