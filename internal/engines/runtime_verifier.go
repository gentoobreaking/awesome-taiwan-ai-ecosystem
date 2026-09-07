package engines

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

type MCPTransport string

const (
	TransportStdio          MCPTransport = "stdio"
	TransportSSE            MCPTransport = "sse"
	TransportStreamableHTTP MCPTransport = "streamable-http"
)

type RuntimeVerifier struct {
	InitializeTimeout time.Duration
	ToolsListTimeout  time.Duration
	TotalTimeout      time.Duration
}

func NewRuntimeVerifier() *RuntimeVerifier {
	return &RuntimeVerifier{
		InitializeTimeout: 10 * time.Second,
		ToolsListTimeout:  10 * time.Second,
		TotalTimeout:      30 * time.Second,
	}
}

type RuntimeVerificationResult struct {
	Status           RuntimeVerificationStatus `json:"status"`
	InitializeResult *InitializeResult         `json:"initialize_result,omitempty"`
	ToolsListResult  *ToolsListResult          `json:"tools_list_result,omitempty"`
	Timestamp        models.RFC3339Time        `json:"timestamp"`
	Evidence         []models.Evidence         `json:"evidence"`
	ServerCommand    string                    `json:"server_command,omitempty"`
	Transport        MCPTransport              `json:"transport,omitempty"`
}

type InitializeResult struct {
	Success       bool   `json:"success"`
	Response      string `json:"response,omitempty"`
	LatencyMs     int    `json:"latency_ms"`
	Error         string `json:"error,omitempty"`
	ServerInfo    string `json:"server_info,omitempty"`
	ProtocolVer   string `json:"protocol_version,omitempty"`
	Capabilities  string `json:"capabilities,omitempty"`
}

type ToolsListResult struct {
	Success       bool   `json:"success"`
	ToolCount     int    `json:"tool_count"`
	ToolsSummary  string `json:"tools_summary,omitempty"`
	LatencyMs     int    `json:"latency_ms"`
	Error         string `json:"error,omitempty"`
}

type RuntimeVerificationStatus string

const (
	RuntimeVerificationStatusPassed  RuntimeVerificationStatus = "PASSED"
	RuntimeVerificationStatusFailed  RuntimeVerificationStatus = "FAILED"
	RuntimeVerificationStatusTimeout RuntimeVerificationStatus = "TIMEOUT"
	RuntimeVerificationStatusError   RuntimeVerificationStatus = "ERROR"
)

func (rv *RuntimeVerifier) Verify(ctx context.Context, entity *models.Entity) *RuntimeVerificationResult {
	startTime := time.Now()

	result := &RuntimeVerificationResult{
		Status:    RuntimeVerificationStatusError,
		Timestamp: models.RFC3339Time(startTime.UTC()),
		Evidence:  []models.Evidence{},
	}

	if entity == nil {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "eligibility_check",
			Source:       "runtime_verifier",
			Location:     "entity",
			Rule:         "nil_entity",
			MatchedText:  "Entity is nil",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	if entity.MCPIdentity.Status != models.MCPIdentityStatusStaticVerified {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "eligibility_check",
			Source:       "runtime_verifier",
			Location:     "entity",
			Rule:         "static_verified_required",
			MatchedText:  "Entity not eligible for runtime verification",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	runtimeEndpoint := rv.findRuntimeEndpoint(entity)
	if runtimeEndpoint == nil {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "eligibility_check",
			Source:       "runtime_verifier",
			Location:     "entity.endpoints",
			Rule:         "runtime_endpoint_required",
			MatchedText:  "No MCP_RUNTIME_ENDPOINT found",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	result.Transport = rv.detectTransport(runtimeEndpoint)
	result.ServerCommand = rv.getServerCommand(entity, runtimeEndpoint)

	verifyCtx, cancel := context.WithTimeout(ctx, rv.TotalTimeout)
	defer cancel()

	switch result.Transport {
	case TransportStdio:
		return rv.verifyStdio(verifyCtx, entity, runtimeEndpoint, result)
	case TransportSSE:
		return rv.verifySSE(verifyCtx, entity, runtimeEndpoint, result)
	case TransportStreamableHTTP:
		return rv.verifyStreamableHTTP(verifyCtx, entity, runtimeEndpoint, result)
	default:
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "transport_error",
			Source:       "runtime_verifier",
			Location:     "endpoint",
			Rule:         "unsupported_transport",
			MatchedText:  fmt.Sprintf("Unsupported transport: %s", result.Transport),
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}
}

func (rv *RuntimeVerifier) isEligibleForVerification(entity *models.Entity) bool {
	if entity == nil {
		return false
	}
	if entity.MCPIdentity.Status != models.MCPIdentityStatusStaticVerified {
		return false
	}
	for _, ep := range entity.Endpoints {
		if ep.Type == models.EndpointTypeMCPRuntime {
			return true
		}
	}
	return false
}

func (rv *RuntimeVerifier) findRuntimeEndpoint(entity *models.Entity) *models.EndpointWithType {
	for _, ep := range entity.Endpoints {
		if ep.Type == models.EndpointTypeMCPRuntime {
			return &ep
		}
	}
	return nil
}

func (rv *RuntimeVerifier) detectTransport(ep *models.EndpointWithType) MCPTransport {
	url := strings.ToLower(ep.Endpoint.URL)
	if strings.HasPrefix(url, "stdio:") || strings.Contains(url, "stdio") {
		return TransportStdio
	}
	if strings.Contains(url, "sse") {
		return TransportSSE
	}
	if strings.Contains(url, "streamable-http") || strings.Contains(url, "http://") || strings.Contains(url, "https://") {
		return TransportStreamableHTTP
	}
	return TransportStdio
}

func (rv *RuntimeVerifier) getServerCommand(entity *models.Entity, ep *models.EndpointWithType) string {
	return rv.getServerCommandFromEntity(entity)
}

func (rv *RuntimeVerifier) getServerCommandFromEntity(entity *models.Entity) string {
	if entity == nil || entity.RawContent == "" {
		return ""
	}

	content := entity.RawContent

	if strings.HasPrefix(strings.TrimSpace(content), "module ") {
		return "go run ."
	}

	if strings.Contains(content, "Cargo.toml") || strings.HasPrefix(strings.TrimSpace(content), "[package]") {
		return "cargo run"
	}

	if strings.Contains(content, "pyproject.toml") || strings.HasPrefix(strings.TrimSpace(content), "[project]") || strings.HasPrefix(strings.TrimSpace(content), "[build-system]") {
		return "python -m mcp_server"
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal([]byte(content), &pkg); err == nil && len(pkg.Scripts) > 0 {
		if cmd, ok := pkg.Scripts["mcp"]; ok {
			return cmd
		}
		if cmd, ok := pkg.Scripts["server"]; ok {
			return cmd
		}
		if cmd, ok := pkg.Scripts["start"]; ok {
			return cmd
		}
	}

	return ""
}

func (rv *RuntimeVerifier) verifyStdio(ctx context.Context, entity *models.Entity, ep *models.EndpointWithType, result *RuntimeVerificationResult) *RuntimeVerificationResult {
	cmdStr := result.ServerCommand

	if cmdStr == "" {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "command_error",
			Source:       "runtime_verifier",
			Location:     "endpoint",
			Rule:         "missing_command",
			MatchedText:  "No server command found for stdio transport",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "command_error",
			Source:       "runtime_verifier",
			Location:     "endpoint",
			Rule:         "invalid_command",
			MatchedText:  cmdStr,
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "process_error",
			Source:       "runtime_verifier",
			Location:     "stdin_pipe",
			Rule:         "pipe_creation_failed",
			MatchedText:  err.Error(),
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "process_error",
			Source:       "runtime_verifier",
			Location:     "stdout_pipe",
			Rule:         "pipe_creation_failed",
			MatchedText:  err.Error(),
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "process_error",
			Source:       "runtime_verifier",
			Location:     "stderr_pipe",
			Rule:         "pipe_creation_failed",
			MatchedText:  err.Error(),
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	if err := cmd.Start(); err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "process_error",
			Source:       "runtime_verifier",
			Location:     "process_start",
			Rule:         "process_start_failed",
			MatchedText:  err.Error(),
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	scanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)
	go func() {
		for stderrScanner.Scan() {
			_ = stderrScanner.Text()
		}
	}()

	initResult := rv.sendStdioRequest(ctx, stdin, scanner, 1, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]bool{},
		ClientInfo:      clientInfo{Name: "taiwan-mcp-crawler", Version: "1.0.0"},
	}, rv.InitializeTimeout)

	var ir *InitializeResult
	var ok bool
	if ir, ok = initResult.(*InitializeResult); !ok {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "initialize_error",
			Source:       "runtime_verifier",
			Location:     "initialize",
			Rule:         "type_assertion_failed",
			MatchedText:  "initResult type assertion failed",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		result.Status = RuntimeVerificationStatusFailed
		return result
	}
	result.InitializeResult = ir
	if !ir.Success {
		result.Status = RuntimeVerificationStatusFailed
		return result
	}

	toolsResult := rv.sendStdioRequest(ctx, stdin, scanner, 2, "tools/list", nil, rv.ToolsListTimeout)
	var tlr *ToolsListResult
	if tlr, ok = toolsResult.(*ToolsListResult); !ok {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:         "tools_list_error",
			Source:       "runtime_verifier",
			Location:     "tools_list",
			Rule:         "type_assertion_failed",
			MatchedText:  "toolsResult type assertion failed",
			Confidence:   1.0,
			Timestamp:    models.RFC3339Time(time.Now().UTC()),
		})
		result.Status = RuntimeVerificationStatusFailed
		return result
	}
	result.ToolsListResult = tlr

	if !result.ToolsListResult.Success {
		result.Status = RuntimeVerificationStatusFailed
		return result
	}

	result.Status = RuntimeVerificationStatusPassed
	return result
}

func (rv *RuntimeVerifier) sendStdioRequest(ctx context.Context, stdin io.Writer, scanner *bufio.Scanner, id int, method string, params interface{}, timeout time.Duration) interface{} {
	startTime := time.Now()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("marshal request: %v", err),
			LatencyMs: int(time.Since(startTime).Milliseconds()),
		}
	}

	if _, err := stdin.Write(append(reqBytes, '\n')); err != nil {
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("write request: %v", err),
			LatencyMs: int(time.Since(startTime).Milliseconds()),
		}
	}

	responseChan := make(chan *jsonRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				errChan <- fmt.Errorf("unmarshal response: %w", err)
				return
			}
			if resp.ID == id {
				responseChan <- &resp
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errChan <- fmt.Errorf("scanner error: %w", err)
		}
	}()

	select {
	case resp := <-responseChan:
		latency := int(time.Since(startTime).Milliseconds())
		if resp.Error != nil {
			return &InitializeResult{
				Success:   false,
				Error:     fmt.Sprintf("MCP error: %s", resp.Error.Message),
				LatencyMs: latency,
			}
		}
		if method == "initialize" {
			var initResp initializeResult
			if err := json.Unmarshal(resp.Result, &initResp); err != nil {
				return &InitializeResult{
					Success:   false,
					Error:     fmt.Sprintf("unmarshal initialize: %v", err),
					LatencyMs: latency,
				}
			}
			return &InitializeResult{
				Success:    true,
				Response:   string(resp.Result),
				LatencyMs:  latency,
			}
		}
		if method == "tools/list" {
			var toolsResp toolsListResult
			if err := json.Unmarshal(resp.Result, &toolsResp); err != nil {
				return &ToolsListResult{
					Success:   false,
					Error:     fmt.Sprintf("unmarshal tools/list: %v", err),
					LatencyMs: latency,
				}
			}
			return &ToolsListResult{
				Success:      true,
				ToolCount:    len(toolsResp.Tools),
				ToolsSummary: rv.summarizeTools(toolsResp.Tools),
				LatencyMs:    latency,
			}
		}
		return &InitializeResult{
			Success:    true,
			Response:   string(resp.Result),
			LatencyMs:  latency,
		}
	case err := <-errChan:
		return &InitializeResult{
			Success:   false,
			Error:     err.Error(),
			LatencyMs: int(time.Since(startTime).Milliseconds()),
		}
	case <-time.After(timeout):
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("timeout after %v", timeout),
			LatencyMs: int(time.Since(startTime).Milliseconds()),
		}
	case <-ctx.Done():
		return &InitializeResult{
			Success:   false,
			Error:     ctx.Err().Error(),
			LatencyMs: int(time.Since(startTime).Milliseconds()),
		}
	}
}

// verifyHTTP runs the MCP initialize + tools/list handshake over HTTP
// (SSE or streamable-http). Implemented in T100; the previous versions
// just returned transport_not_implemented evidence.
func (rv *RuntimeVerifier) verifyHTTP(ctx context.Context, ep *models.EndpointWithType, result *RuntimeVerificationResult, transport MCPTransport) *RuntimeVerificationResult {
	url := ep.Endpoint.URL
	if url == "" {
		result.Status = RuntimeVerificationStatusFailed
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:        "missing_endpoint",
			Source:      "runtime_verifier",
			Location:    "http_transport",
			Rule:        "missing_url",
			MatchedText: "No endpoint URL for HTTP transport",
			Confidence:  1.0,
			Timestamp:   models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}

	// Stage 1: initialize
	initRaw, err := rv.sendHTTPRequest(ctx, url, 1, "initialize", initializeParams{
		ProtocolVersion: "2024-11-05",
		Capabilities:    map[string]bool{},
		ClientInfo:      clientInfo{Name: "taiwan-mcp-crawler", Version: "1.0.0"},
	}, transport, rv.InitializeTimeout)
	if err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:        "initialize_error",
			Source:      "runtime_verifier",
			Location:    "http_initialize",
			Rule:        "request_failed",
			MatchedText: err.Error(),
			Confidence:  1.0,
			Timestamp:   models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}
	if jsonResp, ok := initRaw.(*jsonRPCResponse); ok {
		var initParsed initializeResult
		if uerr := json.Unmarshal(jsonResp.Result, &initParsed); uerr != nil {
			result.Status = RuntimeVerificationStatusFailed
			result.Evidence = append(result.Evidence, models.Evidence{
				Type:        "initialize_error",
				Source:      "runtime_verifier",
				Location:    "http_initialize",
				Rule:        "unmarshal_failed",
				MatchedText: uerr.Error(),
				Confidence:  1.0,
				Timestamp:   models.RFC3339Time(time.Now().UTC()),
			})
			return result
		}
		result.InitializeResult = &InitializeResult{
			Success:     true,
			ServerInfo:  initParsed.ServerInfo.Name + "@" + initParsed.ServerInfo.Version,
			ProtocolVer: initParsed.ProtocolVersion,
		}
	} else {
		ir := initRaw.(*InitializeResult)
		result.InitializeResult = ir
		if !ir.Success {
			result.Status = RuntimeVerificationStatusFailed
			return result
		}
	}

	// Stage 2: tools/list
	toolsRaw, err := rv.sendHTTPRequest(ctx, url, 2, "tools/list", nil, transport, rv.ToolsListTimeout)
	if err != nil {
		result.Status = RuntimeVerificationStatusError
		result.Evidence = append(result.Evidence, models.Evidence{
			Type:        "tools_list_error",
			Source:      "runtime_verifier",
			Location:    "http_tools_list",
			Rule:        "request_failed",
			MatchedText: err.Error(),
			Confidence:  1.0,
			Timestamp:   models.RFC3339Time(time.Now().UTC()),
		})
		return result
	}
	if toolsResp, ok := toolsRaw.(*jsonRPCResponse); ok {
		var toolsParsed toolsListResult
		if uerr := json.Unmarshal(toolsResp.Result, &toolsParsed); uerr != nil {
			result.Status = RuntimeVerificationStatusFailed
			result.Evidence = append(result.Evidence, models.Evidence{
				Type:        "tools_list_error",
				Source:      "runtime_verifier",
				Location:    "http_tools_list",
				Rule:        "unmarshal_failed",
				MatchedText: uerr.Error(),
				Confidence:  1.0,
				Timestamp:   models.RFC3339Time(time.Now().UTC()),
			})
			return result
		}
		result.ToolsListResult = &ToolsListResult{
			Success:      true,
			ToolCount:    len(toolsParsed.Tools),
			ToolsSummary: rv.summarizeTools(toolsParsed.Tools),
		}
	} else {
		tlr := toolsRaw.(*ToolsListResult)
		result.ToolsListResult = tlr
		if !tlr.Success {
			result.Status = RuntimeVerificationStatusFailed
			return result
		}
	}

	result.Status = RuntimeVerificationStatusPassed
	return result
}

func (rv *RuntimeVerifier) verifySSE(ctx context.Context, entity *models.Entity, ep *models.EndpointWithType, result *RuntimeVerificationResult) *RuntimeVerificationResult {
	return rv.verifyHTTP(ctx, ep, result, TransportSSE)
}

func (rv *RuntimeVerifier) verifyStreamableHTTP(ctx context.Context, entity *models.Entity, ep *models.EndpointWithType, result *RuntimeVerificationResult) *RuntimeVerificationResult {
	return rv.verifyHTTP(ctx, ep, result, TransportStreamableHTTP)
}

// sendHTTPRequest POSTs a JSON-RPC payload to the given URL and returns
// either a *jsonRPCResponse (success) or a *InitializeResult with the
// error reason (status / unmarshal / MCP error). For SSE endpoints the
// function reads the event stream and waits for the matching id.
func (rv *RuntimeVerifier) sendHTTPRequest(
	ctx context.Context,
	url string,
	id int,
	method string,
	params interface{},
	transport MCPTransport,
	timeout time.Duration,
) (interface{}, error) {
	startTime := time.Now()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %v", err)
	}

	httpClient := &http.Client{Timeout: timeout}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http do: %v", err)
	}
	defer resp.Body.Close()

	latency := int(time.Since(startTime).Milliseconds())

	if resp.StatusCode != http.StatusOK {
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("HTTP %d", resp.StatusCode),
			LatencyMs: latency,
		}, nil
	}

	var rawResp []byte
	if transport == TransportSSE {
		rawResp, err = readSSEResponse(ctx, resp.Body, id)
	} else {
		rawResp, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return &InitializeResult{
			Success:   false,
			Error:     err.Error(),
			LatencyMs: latency,
		}, nil
	}

	var jsonResp jsonRPCResponse
	if err := json.Unmarshal(rawResp, &jsonResp); err != nil {
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("unmarshal response: %v", err),
			LatencyMs: latency,
		}, nil
	}

	if jsonResp.Error != nil {
		return &InitializeResult{
			Success:   false,
			Error:     fmt.Sprintf("MCP error: %s", jsonResp.Error.Message),
			LatencyMs: latency,
		}, nil
	}

	return &jsonResp, nil
}

// readSSEResponse reads an SSE event stream and returns the JSON-RPC
// response body for the given request id. Bails out on context done.
func readSSEResponse(ctx context.Context, body io.Reader, id int) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var probe jsonRPCResponse
		if err := json.Unmarshal([]byte(payload), &probe); err != nil {
			continue
		}
		if probe.ID == id {
			return []byte(payload), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("sse scanner: %w", err)
	}
	return nil, fmt.Errorf("sse: no response with id %d", id)
}

type jsonRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  interface{}   `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string            `json:"protocolVersion"`
	Capabilities    map[string]bool   `json:"capabilities"`
	ClientInfo      clientInfo        `json:"clientInfo"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]json.RawMessage `json:"capabilities"`
	ServerInfo      serverInfo           `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}
func (rv *RuntimeVerifier) summarizeTools(tools []toolInfo) string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	if len(names) > 5 {
		return strings.Join(names[:5], ", ") + fmt.Sprintf(" (+ %d more)", len(names)-5)
	}
	return strings.Join(names, ", ")
}
