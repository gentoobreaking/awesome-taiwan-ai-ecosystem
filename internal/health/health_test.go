package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestCheckEndpoint_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hc := New(srv.Client(), 5*time.Second)
	server := &models.MCPServer{
		Endpoints: []models.Endpoint{
			{URL: srv.URL, Transport: "http"},
		},
	}
	result := hc.CheckServer(context.Background(), server)
	if result != models.HealthHealthy {
		t.Errorf("Expected HEALTHY, got %s", result)
	}
}

func TestCheckEndpoint_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	hc := New(srv.Client(), 5*time.Second)
	result := hc.CheckEndpoint(context.Background(), models.Endpoint{
		URL:       srv.URL,
		Transport: "http",
	})
	if result.Health != models.HealthDegraded {
		t.Errorf("Expected DEGRADED for 404, got %s", result.Health)
	}
	if result.HTTPStatus != 404 {
		t.Errorf("Expected HTTP 404, got %d", result.HTTPStatus)
	}
}

func TestCheckEndpoint_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	hc := New(srv.Client(), 5*time.Second)
	result := hc.CheckEndpoint(context.Background(), models.Endpoint{
		URL:       srv.URL,
		Transport: "http",
	})
	if result.Health != models.HealthUnavailable {
		t.Errorf("Expected UNAVAILABLE for 500, got %s", result.Health)
	}
}

func TestCheckEndpoint_NoEndpoints(t *testing.T) {
	hc := New(http.DefaultClient, 5*time.Second)
	server := &models.MCPServer{}
	result := hc.CheckServer(context.Background(), server)
	if result != models.HealthHealthy {
		t.Errorf("Expected HEALTHY for stdio/no endpoints, got %s", result)
	}
}

func TestIsStreamingTransport(t *testing.T) {
	if !IsStreamingTransport("sse") {
		t.Error("Expected true for sse")
	}
	if !IsStreamingTransport("SSE") {
		t.Error("Expected true for SSE")
	}
	if !IsStreamingTransport("streamable-http") {
		t.Error("Expected true for streamable-http")
	}
	if IsStreamingTransport("stdio") {
		t.Error("Expected false for stdio")
	}
}

func TestRank(t *testing.T) {
	tests := []struct {
		health models.HealthStatus
		want   int
	}{
		{models.HealthHealthy, 1},
		{models.HealthDegraded, 2},
		{models.HealthUnavailable, 3},
		{models.HealthInvalid, 3},
		{models.HealthUnknown, 4},
	}
	for _, tt := range tests {
		if got := rank(tt.health); got != tt.want {
			t.Errorf("rank(%s) = %d, want %d", tt.health, got, tt.want)
		}
	}
}

func TestHealthChecker_NilClient(t *testing.T) {
	hc := New(nil, 0)
	if hc == nil {
		t.Error("Expected non-nil HealthChecker")
	}
}

func TestHealthChecker_EndpointNoURL(t *testing.T) {
	hc := New(http.DefaultClient, 5*time.Second)
	result := hc.CheckEndpoint(context.Background(), models.Endpoint{URL: ""})
	if result.Health != models.HealthUnknown {
		t.Errorf("Expected UNKNOWN for empty URL, got %s", result.Health)
	}
}

func TestHealthChecker_CheckServer_MultipleEndpoints(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv2.Close()

	hc := New(srv1.Client(), 5*time.Second)
	server := &models.MCPServer{
		Endpoints: []models.Endpoint{
			{URL: srv1.URL, Transport: "http", TLS: true},
			{URL: srv2.URL, Transport: "http", TLS: true},
		},
	}
	result := hc.CheckServer(context.Background(), server)
	if result != models.HealthUnavailable {
		t.Errorf("Expected UNAVAILABLE (worst endpoint), got %s", result)
	}
}

func TestRank_Unknown(t *testing.T) {
	if r := rank(models.HealthUnknown); r != 4 {
		t.Errorf("Expected rank 4 for UNKNOWN, got %d", r)
	}
}

func TestCheckEndpoint_TransportDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hc := New(srv.Client(), 5*time.Second)
	// Test with different transport types
	result := hc.CheckEndpoint(context.Background(), models.Endpoint{
		URL:       "http://127.0.0.1:9999/mcp",
		Transport: "stdio",
	})
	_ = result
}
