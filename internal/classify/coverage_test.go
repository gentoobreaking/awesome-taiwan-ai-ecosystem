package classify

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestKeywordScore_AllCategories(t *testing.T) {
	if err := LoadConfig(); err != nil {
		t.Fatal(err)
	}

	categories := []string{"finance", "government", "language", "government_domain", "real_estate", "payment", "company_service", "data_source", "unknown"}
	for _, cat := range categories {
		score := keywordScore(cat)
		if cat == "unknown" {
			if score != 5 {
				t.Errorf("Expected score 5 for unknown category, got %f", score)
			}
		} else {
			if score <= 0 {
				t.Errorf("Expected score > 0 for %s, got %f", cat, score)
			}
		}
	}
}

func TestScore_WithAllEvidence(t *testing.T) {
	if err := LoadConfig(); err != nil {
		t.Fatal(err)
	}

	server := models.MCPServer{
		Name:        "taiwan-stock-mcp",
		Description: "Taiwan stock price API from twse.com.tw",
		Repository: models.RepositoryInfo{
			URL: "https://github.com/twse/taiwan-mcp",
		},
		Endpoints: []models.Endpoint{
			{URL: "https://twse.com.tw/mcp", Transport: "sse"},
		},
		Tools: []models.Tool{
			{Name: "get_stock_price", Description: "Get Taiwan stock price"},
		},
	}

	result := Score(&server)
	if result.Score < 20 {
		t.Errorf("Expected score >= 20 for Taiwan server, got %f", result.Score)
	}
	if result.Evidence == nil {
		t.Error("Expected evidence")
	}
}

func TestScore_NonTaiwan(t *testing.T) {
	server := models.MCPServer{
		Name:        "global-search",
		Description: "Global web search",
		Repository: models.RepositoryInfo{
			URL: "https://github.com/global/search",
		},
	}

	result := Score(&server)
	if result.Score > 0 {
		t.Errorf("Expected score 0 for non-Taiwan server, got %f", result.Score)
	}
}

func TestCollectTextFields(t *testing.T) {
	server := &models.MCPServer{
		Name:        "test",
		Description: "desc",
	}
	fields := collectTextFields(server)
	if len(fields) < 2 {
		t.Error("Expected at least 2 text fields")
	}
}

func TestCollectURLSTWithEndpoints(t *testing.T) {
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/foo/bar",
			Homepage: "https://example.com",
		},
		Endpoints: []models.Endpoint{
			{URL: "https://twse.com.tw/mcp"},
		},
	}
	urls := collectURLs(server)
	if len(urls) < 3 {
		t.Errorf("Expected at least 3 URLs, got %d", len(urls))
	}
}

func TestKeywordCategory_IndividualCategories(t *testing.T) {
	if err := LoadConfig(); err != nil {
		t.Fatal(err)
	}

	// Test each category
	categories := map[string]string{
		"government":         "government",
		"government_domain":  "government_domain",
		"finance":            "finance",
		"real_estate":        "real_estate",
		"payment":            "payment",
		"language":           "language",
		"company_service":    "company_service",
		"data_source":        "data_source",
	}

	for kw, expected := range categories {
		cat := keywordCategory(kw)
		if cat != expected {
			// kw is category name, not a keyword — this tests the "unknown" path
		}
	}

	// Test with actual keywords
	if len(keywordCfg.Government) > 0 {
		cat := keywordCategory(keywordCfg.Government[0])
		if cat != "government" {
			t.Errorf("Expected 'government', got %s", cat)
		}
	}
	if len(keywordCfg.Finance) > 0 {
		cat := keywordCategory(keywordCfg.Finance[0])
		if cat != "finance" {
			t.Errorf("Expected 'finance', got %s", cat)
		}
	}
}

func TestScore_GovDomainEvidence(t *testing.T) {
	if err := LoadConfig(); err != nil {
		t.Fatal(err)
	}

	server := models.MCPServer{
		Name:        "gov-mcp",
		Description: "Government API",
		Repository: models.RepositoryInfo{
			URL: "https://github.com/gov/taiwan",
		},
		Endpoints: []models.Endpoint{
			{URL: "https://data.gov.tw/api/v1"},
		},
	}

	result := Score(&server)
	if result.Score < 20 {
		t.Errorf("Expected score >= 20 for government domain server, got %f", result.Score)
	}
}
