// Package evidence provides evidence collection for scoring rules.
package evidence

import (
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// maxSnippetLen is the cap on the Snippet field (spec §4.4 / §39).
const maxSnippetLen = 200

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

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

// AddSourceCode records source-code evidence (spec §39 SOURCE_CODE).
// T109: snippet is truncated to 200 chars.
func (c *Collector) AddSourceCode(file, signal, snippet string, confidence float64) {
	c.Add(models.Evidence{
		Type:       "SOURCE_CODE",
		Location:   file,
		Source:     "source_code",
		MatchedText: signal,
		Snippet:    truncate(snippet, maxSnippetLen),
		Rule:       "source_code_match",
		Confidence: confidence,
	})
}

// AddPackageManifest records a package-manifest evidence row
// (spec §39 PACKAGE_MANIFEST).
func (c *Collector) AddPackageManifest(file, signal string, confidence float64) {
	c.Add(models.Evidence{
		Type:       "PACKAGE_MANIFEST",
		Location:   file,
		Source:     "package_manifest",
		MatchedText: signal,
		Rule:       "package_manifest_match",
		Confidence: confidence,
	})
}

// AddReadme records a README evidence row (spec §39 README).
func (c *Collector) AddReadme(signal, snippet string, confidence float64) {
	c.Add(models.Evidence{
		Type:       "README",
		Source:     "readme",
		MatchedText: signal,
		Snippet:    truncate(snippet, maxSnippetLen),
		Rule:       "readme_match",
		Confidence: confidence,
	})
}

// AddRuntime records a runtime-verification evidence row
// (spec §39 RUNTIME).
func (c *Collector) AddRuntime(signal string, confidence float64) {
	c.Add(models.Evidence{
		Type:       "RUNTIME",
		Source:     "runtime",
		MatchedText: signal,
		Rule:       "runtime_match",
		Confidence: confidence,
	})
}

// AddRuleMatch records a low-weight rule evidence row.
func (c *Collector) AddRuleMatch(rule, signal string, confidence float64) {
	c.Add(models.Evidence{
		Type:       "RULE_MATCH",
		MatchedText: signal,
		Rule:       rule,
		Confidence: confidence,
	})
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
