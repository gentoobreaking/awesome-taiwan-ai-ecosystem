// Package evidence provides evidence collection for scoring rules.
package evidence

import (
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// Collector gathers evidence for each scoring rule (§16, §66).
type Collector struct {
	items []models.Evidence
}

// New creates a new evidence Collector.
func New() *Collector {
	return &Collector{}
}

func (c *Collector) Add(ev models.Evidence) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = models.RFC3339Time(time.Now().UTC())
	}
	c.items = append(c.items, ev)
}

// All returns all collected evidence.
func (c *Collector) All() []models.Evidence {
	return c.items
}

// Len returns the number of evidence items.
func (c *Collector) Len() int {
	return len(c.items)
}

// ByType returns evidence items matching a type.
func (c *Collector) ByType(t string) []models.Evidence {
	var result []models.Evidence
	for _, e := range c.items {
		if e.Type == t {
			result = append(result, e)
		}
	}
	return result
}
