package main

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestHeuristicPrimary(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want models.PrimaryClassification
	}{
		{"pydantic-ai-tutorial", "Pydantic AI tutorial for beginners", models.PrimaryClassificationAITutorial},
		{"claude-skills", "Awesome collection of Claude skills", models.PrimaryClassificationMCPCollection},
		{"fugle-agent", "AI agent for Taiwan stock", models.PrimaryClassificationAIAgent},
		{"langchain-sdk", "Python SDK for langchain", models.PrimaryClassificationAISDK},
		{"autogen-framework", "Multi-agent orchestration framework", models.PrimaryClassificationAIFramework},
		{"twmarketdata", "Taiwan stock market data Python SDK", models.PrimaryClassificationAIDataset},
		{"tubecli", "CLI for youtube", models.PrimaryClassificationAITool},
		{"mcp-server-foo", "MCP server for foo", models.PrimaryClassificationMCPServer},
		{"", "", models.PrimaryClassificationMCPServer},
		// Some extra coverage for the name-pass vs desc-pass split.
		{"awesome-llms", "List of LLM projects", models.PrimaryClassificationMCPCollection},
		{"fugle-stock-cli", "Stock data command line tool", models.PrimaryClassificationAITool},
	}
	for _, tc := range cases {
		if got := heuristicPrimary(tc.name, tc.desc); got != tc.want {
			t.Errorf("name=%q desc=%q: got %s, want %s", tc.name, tc.desc, got, tc.want)
		}
	}
}
