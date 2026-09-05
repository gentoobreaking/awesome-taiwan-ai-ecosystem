package mcpmarket

import (
	"context"
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
// See local://audit-sources.md §2.1 for full probe details.
type Adapter struct {
	BaseURL    string
	HTTPClient interface{}
}

// New creates a new mcpmarket adapter (blocked).
func New() *Adapter {
	return &Adapter{
		BaseURL: "https://mcpmarket.com",
	}
}

func (a *Adapter) Name() string { return "mcpmarket" }

var _ sources.SourceAdapter = (*Adapter)(nil)

// Discover returns ErrNotAvailable because Vercel WAF blocks all automated discovery.
// TODO: activate when mcpmarket provides official API or Vercel allowlist.
// Do not attempt to bypass Cloudflare/Vercel challenge without authorization.
func (a *Adapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	return nil, fmt.Errorf("mcpmarket: %w — Vercel WAF challenge required, waiting for official API cooperation", sources.ErrNotAvailable)
}

// Fetch returns ErrNotAvailable for the same reason.
func (a *Adapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	return nil, fmt.Errorf("mcpmarket: %w — not implemented (blocked by Vercel WAF)", sources.ErrNotAvailable)
}
