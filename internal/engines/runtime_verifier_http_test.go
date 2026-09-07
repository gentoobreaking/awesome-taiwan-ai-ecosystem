package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// mcpJSONResponse builds a JSON-RPC envelope with the given id and a
// pre-marshalled result body.
func mcpJSONResponse(id int, result string) string {
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":%s}`, id, result)
}

// newStreamableHTTPServer is a minimal MCP server that answers
// initialize and tools/list. The server is used by the tests below.
func newStreamableHTTPServer(t *testing.T, initOK, toolsOK bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		// Naive parse — the verifier only ever sends two well-formed calls.
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		body := string(buf[:n])
		if strings.Contains(body, `"initialize"`) {
			if !initOK {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mcpJSONResponse(1, `{"protocolVersion":"2024-11-05","serverInfo":{"name":"test","version":"0.0.1"},"capabilities":{}}`)))
			return
		}
		if strings.Contains(body, `"tools/list"`) {
			_ = req
			if !toolsOK {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(mcpJSONResponse(2, `{"tools":[{"name":"a","description":""},{"name":"b","description":""}]}`)))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
}

func TestVerifyHTTP_StreamableHTTP_Pass(t *testing.T) {
	srv := newStreamableHTTPServer(t, true, true)
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 2 * time.Second
	rv.ToolsListTimeout = 2 * time.Second

	ep := &models.EndpointWithType{
		Endpoint: models.Endpoint{URL: srv.URL + "/mcp"},
		Type:     models.EndpointTypeMCPRuntime,
	}
	result := &RuntimeVerificationResult{Transport: TransportStreamableHTTP}

	rv.verifyHTTP(context.Background(), ep, result, TransportStreamableHTTP)

	if result.Status != RuntimeVerificationStatusPassed {
		t.Errorf("expected Passed, got %s (evidence: %+v)", result.Status, result.Evidence)
	}
	if result.ToolsListResult == nil || result.ToolsListResult.ToolCount != 2 {
		t.Errorf("expected ToolCount=2, got %+v", result.ToolsListResult)
	}
}

func TestVerifyHTTP_StreamableHTTP_HTTP500(t *testing.T) {
	srv := newStreamableHTTPServer(t, false, true) // initialize fails
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 2 * time.Second
	rv.ToolsListTimeout = 2 * time.Second

	ep := &models.EndpointWithType{Endpoint: models.Endpoint{URL: srv.URL}}
	result := &RuntimeVerificationResult{Transport: TransportStreamableHTTP}

	rv.verifyHTTP(context.Background(), ep, result, TransportStreamableHTTP)

	if result.Status == RuntimeVerificationStatusPassed {
		t.Errorf("expected non-Passed, got Passed")
	}
}

func TestVerifyHTTP_StreamableHTTP_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 2 * time.Second
	rv.ToolsListTimeout = 2 * time.Second

	ep := &models.EndpointWithType{Endpoint: models.Endpoint{URL: srv.URL}}
	result := &RuntimeVerificationResult{Transport: TransportStreamableHTTP}

	rv.verifyHTTP(context.Background(), ep, result, TransportStreamableHTTP)

	if result.Status == RuntimeVerificationStatusPassed {
		t.Errorf("expected non-Passed, got Passed")
	}
}

func TestVerifyHTTP_EmptyURL(t *testing.T) {
	rv := NewRuntimeVerifier()
	ep := &models.EndpointWithType{}
	result := &RuntimeVerificationResult{Transport: TransportStreamableHTTP}

	rv.verifyHTTP(context.Background(), ep, result, TransportStreamableHTTP)

	if result.Status != RuntimeVerificationStatusFailed {
		t.Errorf("expected Failed, got %s", result.Status)
	}
}

func TestVerifyHTTP_SSE_Pass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// Send a couple of unrelated events then the matching response.
		fmt.Fprintf(w, ": ping\n\n")
		fmt.Fprintf(w, "data: %s\n\n", mcpJSONResponse(1, `{"protocolVersion":"2024-11-05","serverInfo":{"name":"sse","version":"0.0.1"},"capabilities":{}}`))
		fmt.Fprintf(w, "data: %s\n\n", mcpJSONResponse(2, `{"tools":[{"name":"a"}]}`))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 2 * time.Second
	rv.ToolsListTimeout = 2 * time.Second

	ep := &models.EndpointWithType{Endpoint: models.Endpoint{URL: srv.URL}}
	result := &RuntimeVerificationResult{Transport: TransportSSE}

	rv.verifyHTTP(context.Background(), ep, result, TransportSSE)

	if result.Status != RuntimeVerificationStatusPassed {
		t.Errorf("expected Passed, got %s (evidence: %+v)", result.Status, result.Evidence)
	}
}

func TestVerifyHTTP_SSE_NoMatchingID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Stream never contains a response with id=1.
		fmt.Fprintf(w, "data: %s\n\n", mcpJSONResponse(999, `{"protocolVersion":"x"}`))
	}))
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 1 * time.Second
	rv.ToolsListTimeout = 1 * time.Second

	ep := &models.EndpointWithType{Endpoint: models.Endpoint{URL: srv.URL}}
	result := &RuntimeVerificationResult{Transport: TransportSSE}

	rv.verifyHTTP(context.Background(), ep, result, TransportSSE)

	if result.Status == RuntimeVerificationStatusPassed {
		t.Errorf("expected non-Passed, got Passed")
	}
}

func TestVerifyHTTP_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang until client disconnects.
		<-r.Context().Done()
	}))
	defer srv.Close()

	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 200 * time.Millisecond
	rv.ToolsListTimeout = 200 * time.Millisecond

	ep := &models.EndpointWithType{Endpoint: models.Endpoint{URL: srv.URL}}
	result := &RuntimeVerificationResult{Transport: TransportStreamableHTTP}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the request
	rv.verifyHTTP(ctx, ep, result, TransportStreamableHTTP)

	if result.Status == RuntimeVerificationStatusPassed {
		t.Errorf("expected non-Passed, got Passed")
	}
}

// Ensure the original TestRuntimeVerifier_Verify_SSE_NotImplemented and
// TestRuntimeVerifier_Verify_StreamableHTTP_NotImplemented tests in
// runtime_verifier_test.go don't break by changing their behavior: they
// used a non-resolvable URL and now expect FAILED (network) instead of
// ERROR (transport_not_implemented). The status assertion is updated
// accordingly. This test is here to document the contract change.
func TestVerifyHTTP_NetworkErrorIsFailed(t *testing.T) {
	rv := NewRuntimeVerifier()
	rv.InitializeTimeout = 200 * time.Millisecond
	rv.ToolsListTimeout = 200 * time.Millisecond

	ep := &models.EndpointWithType{
		Endpoint: models.Endpoint{URL: "http://127.0.0.1:1/no-such-server"},
	}
	result := &RuntimeVerificationResult{Transport: TransportSSE}
	rv.verifyHTTP(context.Background(), ep, result, TransportSSE)

	// Network error should be FAILED (or ERROR) — NOT the old
	// transport_not_implemented error.
	for _, e := range result.Evidence {
		if e.Rule == "sse_not_implemented" || e.Rule == "streamable_http_not_implemented" {
			t.Errorf("unexpected transport_not_implemented evidence: %+v", e)
		}
	}
}
