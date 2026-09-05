package registry

import (
	"context"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// Adapter implements SourceAdapter for the official MCP registry.
type Adapter struct {
	BaseURL string
	Token   string
}

func New() *Adapter {
	return &Adapter{BaseURL: "https://api.mcp-servers.dev"}
}

func (a *Adapter) Name() string { return "registry" }

func (a *Adapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	return nil, nil
}

func (a *Adapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*models.RawRecord, error) {
	return &models.RawRecord{
		RawCandidate: candidate,
	}, nil
}

var _ sources.SourceAdapter = (*Adapter)(nil)
