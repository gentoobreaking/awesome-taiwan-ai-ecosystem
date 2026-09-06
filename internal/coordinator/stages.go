package coordinator

import (
	"context"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/metrics"
	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Stage represents a single pipeline stage that transforms entities.
// Each stage receives entities from the previous stage and returns modified entities.
type Stage interface {
	// Name returns the human-readable name of this stage.
	Name() string

	// Execute runs the stage on the given entities and returns the result.
	Execute(ctx context.Context, entities []*models.Entity) ([]*models.Entity, error)
}

// StageFunc is a function adapter for Stage.
type StageFunc struct {
	NameValue string
	Run       func(ctx context.Context, entities []*models.Entity) ([]*models.Entity, error)
}

// Name returns the stage name.
func (s *StageFunc) Name() string { return s.NameValue }

// Execute runs the stage function.
func (s *StageFunc) Execute(ctx context.Context, entities []*models.Entity) ([]*models.Entity, error) {
	return s.Run(ctx, entities)
}

// Pipeline manages the ordered execution of pipeline stages.
// It handles stage registration, execution order, error handling, and progress reporting.
type Pipeline struct {
	stages []Stage
	logger *metrics.Logger
}

// NewPipeline creates a new Pipeline with the given logger.
func NewPipeline(logger *metrics.Logger) *Pipeline {
	return &Pipeline{
		stages: make([]Stage, 0),
		logger: logger,
	}
}

// Register adds a stage to the pipeline.
func (p *Pipeline) Register(stage Stage) {
	p.stages = append(p.stages, stage)
}

// Run executes all registered stages in order, passing entities through each stage.
func (p *Pipeline) Run(ctx context.Context, entities []*models.Entity) ([][]*models.Entity, error) {
	results := make([][]*models.Entity, 0, len(p.stages))
	for _, stage := range p.stages {
		start := time.Now()
		entities, err := stage.Execute(ctx, entities)
		elapsed := time.Since(start)
		if p.logger != nil {
			if err != nil {
				p.logger.Info(ctx, "", stage.Name(), "stage_failed",
					"duration", elapsed.String(),
					"error", err.Error(),
				)
			} else {
				p.logger.Info(ctx, "", stage.Name(), "stage_complete",
					"items", len(entities),
					"duration", elapsed.String(),
				)
			}
		}
		if err != nil {
			return results, err
		}
		results = append(results, entities)
	}
	return results, nil
}

// Stages returns the registered stages in order.
func (p *Pipeline) Stages() []Stage {
	return p.stages
}
