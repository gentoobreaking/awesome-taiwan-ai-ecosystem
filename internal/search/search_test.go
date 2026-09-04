package search

import (
	"testing"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func testServers() []models.MCPServer {
	return []models.MCPServer{
		{
			ID:          "srv-a",
			Name:        "taiwan-stock-mcp",
			Description: "Taiwan stock price MCP server",
			Category:    []string{"finance", "stock"},
			Transport:   []string{"stdio"},
			Health:      models.HealthHealthy,
			Quality:     models.QualityScore{Score: 90, Grade: "B"},
			TaiwanRelevance: models.TaiwanRelevance{
				Level: "T5",
				Score: 65,
			},
			Tools: []models.Tool{
				{Name: "get_stock_price", Description: "Get Taiwan stock price"},
				{Name: "get_trading_volume", Description: "Get trading volume"},
			},
			DataSources: []models.DataSource{
				{Name: "twse.com.tw", Type: models.DataSourceOfficialGovAPI},
			},
			Sources: []models.SourceReference{
				{Source: "github", URL: "https://github.com/foo/taiwan-stock-mcp", TrustScore: 0.95},
			},
		},
		{
			ID:          "srv-b",
			Name:        "taiwan-weather-mcp",
			Description: "Taiwan weather MCP server",
			Category:    []string{"weather"},
			Transport:   []string{"sse"},
			Health:      models.HealthHealthy,
			Quality:     models.QualityScore{Score: 85, Grade: "B"},
			TaiwanRelevance: models.TaiwanRelevance{
				Level: "T5",
				Score: 50,
			},
			Tools: []models.Tool{
				{Name: "get_weather", Description: "Get weather info"},
			},
			Sources: []models.SourceReference{
				{Source: "github", URL: "https://github.com/foo/taiwan-weather-mcp", TrustScore: 0.95},
			},
		},
		{
			ID:          "srv-c",
			Name:        "global-search-mcp",
			Description: "Global web search",
			Category:    []string{"search"},
			Transport:   []string{"stdio"},
			Health:      models.HealthUnavailable,
			Quality:     models.QualityScore{Score: 95, Grade: "A"},
			TaiwanRelevance: models.TaiwanRelevance{
				Level: "T5",
				Score: 30,
			},
			Tools: []models.Tool{
				{Name: "search", Description: "Search the web"},
			},
		},
	}
}

func TestSearchByText(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{Text: "Taiwan stock price"})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected results for 'Taiwan stock price'")
	}
	// server-A should be in results since it has get_stock_price and "stock" in name
	found := false
	for _, r := range results {
		if r.Server.ID == "srv-a" {
			found = true
		}
	}
	if !found {
		t.Error("Expected server-A in results for 'Taiwan stock price'")
	}
}

func TestSearchByLevel(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{Level: "T5"})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("Expected 3 T5 servers, got %d", len(results))
	}
}

func TestSearchByMinScore(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{MinScore: 90})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, r := range results {
		if r.Server.Quality.Score < 90 {
			t.Errorf("Server %s has score %d < 90", r.Server.ID, r.Server.Quality.Score)
		}
	}
}

func TestSearchByCategory(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{Category: []string{"weather"}})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 weather server, got %d", len(results))
	}
	if results[0].Server.ID != "srv-b" {
		t.Errorf("Expected srv-b, got %s", results[0].Server.ID)
	}
}

func TestSearchRanking(t *testing.T) {
	servers := []models.MCPServer{
		{
			ID: "srv-a",
			Name: "Taiwan Stock",
			TaiwanRelevance: models.TaiwanRelevance{Level: "T5", Score: 65},
			Health: models.HealthHealthy,
			Quality: models.QualityScore{Score: 90},
		},
		{
			ID: "srv-c",
			Name: "Global Search",
			TaiwanRelevance: models.TaiwanRelevance{Level: "T5", Score: 85},
			Health: models.HealthUnavailable,
			Quality: models.QualityScore{Score: 95},
		},
	}
	se := New(servers)
	results, err := se.Search(SearchQuery{Limit: 100})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// TST-059: srv-a (T5+HEALTHY+score 90) should rank above srv-c (T5+UNAVAILABLE+score 95)
	if len(results) < 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	if results[0].Server.ID != "srv-a" {
		t.Errorf("Expected srv-a first (HEALTHY ranks above UNAVAILABLE), got %s", results[0].Server.ID)
	}
}

func TestSearchByCapability(t *testing.T) {
	se := New(testServers())
	results, err := se.SearchByCapability("Taiwan stock price")
	if err != nil {
		t.Fatalf("SearchByCapability error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected results for capability search")
	}
	// srv-a has get_stock_price tool which matches
	found := false
	for _, r := range results {
		if r.Server.ID == "srv-a" {
			found = true
		}
	}
	if !found {
		t.Error("Expected server-A in capability results")
	}
}

func TestSearchOffsetLimit(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result with limit=1, got %d", len(results))
	}

	results, err = se.Search(SearchQuery{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result with limit=1,offset=1, got %d", len(results))
	}
}

func TestSearchByTransport(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{Transport: []string{"sse"}})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 SSE server, got %d", len(results))
	}
	if results[0].Server.ID != "srv-b" {
		t.Errorf("Expected srv-b, got %s", results[0].Server.ID)
	}
}

func TestSearchByHealth(t *testing.T) {
	se := New(testServers())
	results, err := se.Search(SearchQuery{
		Health: []models.HealthStatus{models.HealthUnavailable},
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 unavailable server, got %d", len(results))
	}
	if results[0].Server.ID != "srv-c" {
		t.Errorf("Expected srv-c, got %s", results[0].Server.ID)
	}
}
