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

// T111: Data Library / Infrastructure guard.

func TestIsDataLibraryDescription(t *testing.T) {
	c := NewClassifier()
	cases := []struct {
		desc string
		want bool
	}{
		{"Taiwan financial data Python SDK for TWSE stock data", true},
		{"market data API for Taiwan investors", true},
		{"MCP server for Taiwan stock data", false}, // 已被 isMCPServer 抓
		{"A CLI for searching arxiv papers", false},
	}
	for _, tc := range cases {
		e := &models.Entity{Name: "x", Description: tc.desc}
		if got := c.isDataLibraryDescription(e); got != tc.want {
			t.Errorf("desc=%q: got %v want %v", tc.desc, got, tc.want)
		}
	}
}

func TestIsDataInfrastructureDescription(t *testing.T) {
	c := NewClassifier()
	cases := []struct {
		desc   string
		topics []string
		want   bool
	}{
		{"Shared PostgreSQL schema/data layer for Taiwan quant trading", nil, true},
		{"ETL pipeline for stock data", nil, true},
		{"Database migration system using Alembic", nil, true},
		{"MCP server backed by postgres", nil, false},
		{"Some tool", []string{"database"}, true},
		{"Some tool", []string{"mcp"}, false},
	}
	for _, tc := range cases {
		e := &models.Entity{
			Name:        "x",
			Description: tc.desc,
			Repository:  models.RepositoryInfo{Topics: tc.topics},
		}
		if got := c.isDataInfrastructureDescription(e); got != tc.want {
			t.Errorf("desc=%q topics=%v: got %v want %v", tc.desc, tc.topics, got, tc.want)
		}
	}
}

func TestClassify_DataLibrary_NotMCPServer(t *testing.T) {
	// T111 / spec §22: twmarketdata should be DATA_LIBRARY, not MCP_SERVER.
	c := NewClassifier()
	e := &models.Entity{
		Name:        "twmarketdata",
		Description: "Taiwan stock market data Python SDK",
		Repository:  models.RepositoryInfo{Topics: []string{"taiwan", "stock", "data"}},
		RawContent:  "from .client import StockClient  # uses @modelcontextprotocol/sdk style API",
	}
	got := c.classifyByRules(e)
	if string(got.Primary) == "MCP_SERVER" {
		t.Errorf("twmarketdata should not be MCP_SERVER, got %s", got.Primary)
	}
}

func TestClassify_TwQuantDB_DataInfrastructure(t *testing.T) {
	// T111 / spec §48: tw-quant-db should be AI_INFRASTRUCTURE.
	c := NewClassifier()
	e := &models.Entity{
		Name:        "tw-quant-db",
		Description: "Shared PostgreSQL schema/data layer for Taiwan quant trading",
		Repository:  models.RepositoryInfo{Topics: []string{"database", "postgresql", "taiwan"}},
	}
	got := c.classifyByRules(e)
	if string(got.Primary) == "MCP_SERVER" {
		t.Errorf("tw-quant-db should not be MCP_SERVER, got %s", got.Primary)
	}
}
