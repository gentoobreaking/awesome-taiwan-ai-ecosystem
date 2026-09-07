package evidence

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestCollector_AddSourceCode_TruncatesSnippet(t *testing.T) {
	// T109: snippet capped at 200 chars.
	c := New()
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	c.AddSourceCode("src/server.ts", "McpServer class", string(long), 0.9)
	e := c.All()[0]
	if got := len(e.Snippet); got > 200 {
		t.Errorf("snippet length %d > 200", got)
	}
}

func TestCollector_Types(t *testing.T) {
	// T109: the 5 evidence types from spec §4.4 / §39 are all settable.
	c := New()
	c.AddSourceCode("src/server.ts", "McpServer", "class McpServer {}", 0.95)
	c.AddPackageManifest("package.json", "@modelcontextprotocol/sdk", 0.7)
	c.AddReadme("MCP server for Taiwan stock", "A MCP server", 0.8)
	c.AddRuntime("initialize_success", 0.95)
	c.AddRuleMatch("has_mcp_sdk", "sdk found", 0.5)
	if got := c.Len(); got != 5 {
		t.Errorf("expected 5 evidence items, got %d", got)
	}
	types := map[string]int{}
	for _, e := range c.All() {
		types[e.Type]++
	}
	for _, want := range []string{"SOURCE_CODE", "PACKAGE_MANIFEST", "README", "RUNTIME", "RULE_MATCH"} {
		if types[want] != 1 {
			t.Errorf("expected 1 of type %s, got %d", want, types[want])
		}
	}
}

func TestCollector_ByType_Filter(t *testing.T) {
	c := New()
	c.AddSourceCode("a", "x", "y", 0.9)
	c.AddSourceCode("b", "x", "y", 0.9)
	c.AddReadme("c", "d", 0.7)
	if got := len(c.ByType("SOURCE_CODE")); got != 2 {
		t.Errorf("expected 2 SOURCE_CODE, got %d", got)
	}
}

// Models-level: Snippet field is present in JSON output.
func TestEvidence_SnippetField(t *testing.T) {
	_ = models.Evidence{Snippet: "abc"}
}
