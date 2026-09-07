package engines

import (
	"context"
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// T108: NewClassifier with WithLLMFallback attaches a fallback.
func TestNewClassifier_WithLLMFallback(t *testing.T) {
	// Pass a non-nil fallback by constructing one directly. Without an
	// API key the factory returns nil, so we need to bypass the env
	// check. Use a struct literal that satisfies the same shape.
	fb := &LLMClassifierFallback{models: []string{"m"}}
	c := NewClassifier(WithLLMFallback(fb))
	if c.llmFallback != fb {
		t.Error("WithLLMFallback should attach the fallback")
	}
}

func TestNewClassifier_WithoutFallback(t *testing.T) {
	c := NewClassifier()
	if c.llmFallback != nil {
		t.Error("default Classifier should have nil llmFallback")
	}
}

// T108: ClassifyWithCtx with no fallback delegates to classifyByRules
// (legacy Classify still works).
func TestClassifyWithCtx_NoFallback(t *testing.T) {
	c := NewClassifier()
	e := &models.Entity{
		Name:        "twstock-mcp",
		Description: "Taiwan stock MCP server",
		Repository:  models.RepositoryInfo{URL: "https://github.com/twstock/mcp"},
		RawContent:  "package main; func main() {} // McpServer",
	}
	got := c.ClassifyWithCtx(context.Background(), e)
	if got.Primary == "" {
		t.Error("ClassifyWithCtx should still produce a primary classification")
	}
}
