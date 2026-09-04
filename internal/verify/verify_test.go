package verify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestVerifyRepository_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rv := NewRepository(srv.Client())
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			URL: srv.URL,
		},
	}

	result := rv.VerifyRepository(context.Background(), server)
	if !result.Reachable {
		t.Error("Expected reachable=true")
	}
	if result.HTTPStatus != 200 {
		t.Errorf("Expected HTTP 200, got %d", result.HTTPStatus)
	}
	if result.Status == models.StatusDeleted {
		t.Error("Expected non-DELETED status")
	}
}

func TestVerifyRepository_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rv := NewRepository(srv.Client())
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			URL: srv.URL,
		},
	}

	result := rv.VerifyRepository(context.Background(), server)
	if result.Status != models.StatusDeleted {
		t.Errorf("Expected DELETED, got %s", result.Status)
	}
}

func TestVerifyRepository_Archived(t *testing.T) {
	rv := NewRepository(http.DefaultClient)
	server := &models.MCPServer{
		Repository: models.RepositoryInfo{
			URL:      "https://github.com/foo/bar",
			Archived: true,
		},
	}

	result := rv.VerifyRepository(context.Background(), server)
	if result.Status != models.StatusArchived {
		t.Errorf("Expected ARCHIVED, got %s", result.Status)
	}
}

func TestStatusFromPushedAt(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name    string
		pushed  time.Time
		archived bool
		want    models.Status
	}{
		{"archived overrides", now, true, models.StatusArchived},
		{"active <90d", now.AddDate(0, 0, -10), false, models.StatusActive},
		{"maintenance 90-180d", now.AddDate(0, 0, -100), false, models.StatusMaintenance},
		{"stale 180-365d", now.AddDate(0, 0, -200), false, models.StatusStale},
		{"dormant >365d", now.AddDate(0, 0, -400), false, models.StatusDormant},
		{"zero time", time.Time{}, false, models.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusFromPushedAt(tt.pushed, tt.archived)
			if got != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestVerifyMCPProtocol_Stdio(t *testing.T) {
	pv := NewProtocol(http.DefaultClient)
	server := &models.MCPServer{
		// No HTTP endpoints — stdio only
	}
	result := pv.VerifyMCPProtocol(context.Background(), server)
	if result.ToolsListable {
		t.Error("Expected ToolsListable=false for stdio")
	}
}

func TestVerifyMCPProtocol_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))
	}))
	defer srv.Close()

	pv := NewProtocol(srv.Client())
	server := &models.MCPServer{
		Endpoints: []models.Endpoint{
			{URL: srv.URL, Transport: "http"},
		},
	}
	result := pv.VerifyMCPProtocol(context.Background(), server)
	if !result.ToolsListable {
		t.Error("Expected ToolsListable=true")
	}
	if result.ProtocolVersion != "2.0" {
		t.Errorf("Expected protocol 2.0, got %s", result.ProtocolVersion)
	}
	if len(result.Tools) != 0 {
		t.Errorf("Expected 0 tools from empty list, got %d", len(result.Tools))
	}
}

func TestExtractToolsFromToolsList(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"search","description":"Search","inputSchema":{"type":"object"},"annotations":{"readOnly":true}}]}}`)
	tools := extractToolsFromToolsList(body)
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "search" {
		t.Errorf("Expected name 'search', got '%s'", tools[0].Name)
	}
	if tools[0].Description != "Search" {
		t.Errorf("Expected description 'Search', got '%s'", tools[0].Description)
	}
	if !tools[0].Annotations.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
}

func TestExtractToolsFromToolsList_Invalid(t *testing.T) {
	// Tool with empty name, description, and no input schema should be rejected
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"","description":"","inputSchema":null}]}}`)
	tools := extractToolsFromToolsList(body)
	if len(tools) != 0 {
		t.Errorf("Expected 0 valid tools, got %d", len(tools))
	}
}

func TestVerifyMCPProtocol_ToolsExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		jsonResp := `{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"get_data","description":"Get data","inputSchema":{"type":"object"},"annotations":{"readOnly":true,"destructive":false}}]}}`
		_, _ = w.Write([]byte(jsonResp))
	}))
	defer srv.Close()

	pv := NewProtocol(srv.Client())
	server := &models.MCPServer{
		Endpoints: []models.Endpoint{
			{URL: srv.URL, Transport: "http"},
		},
	}
	result := pv.VerifyMCPProtocol(context.Background(), server)
	if !result.ToolsListable {
		t.Error("Expected ToolsListable=true")
	}
	if len(result.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(result.Tools))
	}
	if result.Tools[0].Name != "get_data" {
		t.Errorf("Expected tool name 'get_data', got '%s'", result.Tools[0].Name)
	}
	if !result.Tools[0].Annotations.ReadOnly {
		t.Error("Expected ReadOnly=true")
	}
	if result.Tools[0].Annotations.Destructive {
		t.Error("Expected Destructive=false")
	}
}
