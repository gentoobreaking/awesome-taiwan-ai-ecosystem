// Package health implements endpoint health checking for the crawler pipeline.
package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// HTTPClient is the minimal HTTP interface for health checks.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HealthChecker verifies MCP endpoint availability.
type HealthChecker struct {
	httpClient HTTPClient
	timeout    time.Duration
}

// HealthResult holds the result of an endpoint health check.
type HealthResult struct {
	EndpointURL string
	Health      models.HealthStatus
	HTTPStatus  int
	LatencyMs   int
	Error       string
	LastChecked time.Time
}

// New creates a HealthChecker.
func New(client HTTPClient, timeout time.Duration) *HealthChecker {
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &HealthChecker{
		httpClient: client,
		timeout:    timeout,
	}
}

// CheckEndpoint performs health verification of an MCP endpoint (§23).
func (hc *HealthChecker) CheckEndpoint(ctx context.Context, endpoint models.Endpoint) HealthResult {
	result := HealthResult{
		EndpointURL: endpoint.URL,
		LastChecked: time.Now().UTC(),
	}

	if endpoint.URL == "" {
		result.Health = models.HealthUnknown
		result.Error = "no endpoint URL"
		return result
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.URL, nil)
	if err != nil {
		result.Health = models.HealthInvalid
		result.Error = fmt.Sprintf("request error: %v", err)
		return result
	}
	req.Header.Set("Accept", "application/json, text/event-stream")

	start := time.Now()
	resp, err := hc.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	result.LatencyMs = int(latency)

	if err != nil {
		result.Health = models.HealthUnavailable
		result.Error = fmt.Sprintf("network error: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.HTTPStatus = resp.StatusCode
	_, _ = io.Copy(io.Discard, resp.Body) // drain body

	switch {
	case resp.StatusCode == http.StatusOK:
		result.Health = models.HealthHealthy
	case resp.StatusCode >= 500:
		result.Health = models.HealthUnavailable
	case resp.StatusCode >= 400:
		result.Health = models.HealthDegraded
	default:
		result.Health = models.HealthDegraded
	}

	return result
}

// CheckServer checks all endpoints of a server and returns aggregate health.
func (hc *HealthChecker) CheckServer(ctx context.Context, server *models.MCPServer) models.HealthStatus {
	if len(server.Endpoints) == 0 {
		// stdio transport — can't check, assume healthy from manifest
		return models.HealthHealthy
	}

	var worst models.HealthStatus = models.HealthHealthy
	for _, ep := range server.Endpoints {
		result := hc.CheckEndpoint(ctx, ep)
		if rank(result.Health) > rank(worst) {
			worst = result.Health
		}
	}
	return worst
}

func rank(h models.HealthStatus) int {
	switch h {
	case models.HealthHealthy:
		return 1
	case models.HealthDegraded:
		return 2
	case models.HealthUnavailable, models.HealthInvalid:
		return 3
	default:
		return 4
	}
}

// IsStreamingTransport checks if endpoint uses SSE or streamable-http.
func IsStreamingTransport(transport string) bool {
	t := strings.ToLower(transport)
	return t == "sse" || t == "streamable-http"
}
