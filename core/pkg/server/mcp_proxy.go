package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// MCPProxyHandler handles MCP (Model Context Protocol) JSON-RPC proxy requests.
// MCP uses JSON-RPC 2.0 over HTTP. This handler forwards requests to a
// configurable MCP server endpoint, providing a single gateway for Agent tools.
type MCPProxyHandler struct {
	httpClient *http.Client
	loadConfig func() *MCPProxyConfig
}

// NewMCPProxyHandler creates a new MCP proxy handler.
func NewMCPProxyHandler() *MCPProxyHandler {
	return &MCPProxyHandler{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		loadConfig: loadMCPProxyConfigFromDB,
	}
}

// MCPProxyRequest is a generic JSON-RPC 2.0 request for MCP.
type MCPProxyRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

// MCPProxyResponse is a generic JSON-RPC 2.0 response for MCP.
type MCPProxyResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
	ID      interface{} `json:"id,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error object.
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const configKeyMCPProxy = "mcp_proxy_config"

func defaultMCPProxyConfig() *MCPProxyConfig {
	return &MCPProxyConfig{
		Enabled:        false,
		TimeoutSeconds: 120,
		Targets:        make(map[string]string),
	}
}

func loadMCPProxyConfigFromDB() *MCPProxyConfig {
	cfg := defaultMCPProxyConfig()
	if !database.IsInitialized() {
		return cfg
	}
	if err := config.LoadSystemConfigFromDB(context.Background(), configKeyMCPProxy, cfg); err != nil {
		return cfg
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}
	if cfg.Targets == nil {
		cfg.Targets = make(map[string]string)
	}
	return cfg
}

// HandleMCPProxy handles POST /v1/mcp — forwards JSON-RPC 2.0 requests to the MCP server.
func (h *MCPProxyHandler) HandleMCPProxy(c *gin.Context) {
	start := time.Now()

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32700, Message: "failed to read request body"},
		})
		return
	}

	// Parse JSON-RPC request
	var rpcReq MCPProxyRequest
	if err := json.Unmarshal(body, &rpcReq); err != nil {
		c.JSON(http.StatusBadRequest, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32700, Message: "parse error: " + err.Error()},
		})
		return
	}

	if rpcReq.JSONRPC != "2.0" {
		c.JSON(http.StatusBadRequest, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32600, Message: "invalid request: jsonrpc must be '2.0'"},
			ID:      rpcReq.ID,
		})
		return
	}

	if rpcReq.Method == "" {
		c.JSON(http.StatusBadRequest, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32600, Message: "invalid request: method is required"},
			ID:      rpcReq.ID,
		})
		return
	}

	cfg := h.currentConfig()
	mcpTarget, err := resolveMCPProxyTarget(c, cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32600, Message: err.Error()},
			ID:      rpcReq.ID,
		})
		return
	}

	logger.Infof("[MCP Proxy] %s -> %s (method=%s)", c.ClientIP(), mcpTarget, rpcReq.Method)

	// Forward request to MCP server
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, mcpTarget, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32603, Message: "failed to create upstream request: " + err.Error()},
			ID:      rpcReq.ID,
		})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Forward auth header if present
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := h.clientWithTimeout(cfg.timeout()).Do(req)
	if err != nil {
		logger.Errorf("[MCP Proxy] upstream error: %v", err)
		c.JSON(http.StatusBadGateway, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32603, Message: "upstream error: " + err.Error()},
			ID:      rpcReq.ID,
		})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")

	logger.Infof("[MCP Proxy] %s responded %d (content-type=%s, latency=%v)",
		mcpTarget, resp.StatusCode, contentType, time.Since(start))

	// If upstream returned SSE, stream it back progressively.
	if strings.Contains(contentType, "text/event-stream") {
		if err := streamMCPSSEResponse(c, resp); err != nil {
			logger.Errorf("[MCP Proxy] failed to stream SSE response: %v", err)
		}
		return
	}

	// Otherwise forward as JSON body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("[MCP Proxy] failed to read upstream response: %v", err)
		c.JSON(http.StatusBadGateway, MCPProxyResponse{
			JSONRPC: "2.0",
			Error:   &MCPError{Code: -32603, Message: "failed to read upstream response"},
			ID:      rpcReq.ID,
		})
		return
	}
	c.Data(resp.StatusCode, "application/json", respBody)
}

// HandleMCPHealth handles GET /v1/mcp/health — returns MCP server connectivity status.
func (h *MCPProxyHandler) HandleMCPHealth(c *gin.Context) {
	cfg := h.currentConfig()
	mcpTarget, err := resolveMCPProxyTarget(c, cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "invalid_target",
			"error":  err.Error(),
		})
		return
	}
	healthURL := buildMCPHealthURL(mcpTarget)

	client := h.clientWithTimeout(5 * time.Second)
	resp, err := client.Get(healthURL)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unreachable",
			"target": mcpTarget,
			"error":  err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	c.JSON(http.StatusOK, gin.H{
		"status":      "reachable",
		"target":      mcpTarget,
		"status_code": resp.StatusCode,
	})
}

// HandleMCPCapabilities handles POST /v1/mcp/capabilities — queries MCP server capabilities.
func (h *MCPProxyHandler) HandleMCPCapabilities(c *gin.Context) {
	start := time.Now()

	cfg := h.currentConfig()
	mcpTarget, err := resolveMCPProxyTarget(c, cfg)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Send initialize request to MCP server
	initReq := MCPProxyRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "centag",
				"version": "1.0.0",
			},
		},
		ID: 1,
	}

	body, _ := json.Marshal(initReq)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, mcpTarget, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := h.clientWithTimeout(cfg.timeout()).Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   err.Error(),
			"target":  mcpTarget,
			"latency": time.Since(start).String(),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var rpcResp MCPProxyResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":      "non_json_response",
			"target":      mcpTarget,
			"status_code": resp.StatusCode,
			"body":        string(respBody),
			"latency":     time.Since(start).String(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"target":      mcpTarget,
		"status_code": resp.StatusCode,
		"result":      rpcResp.Result,
		"error":       rpcResp.Error,
		"latency":     time.Since(start).String(),
	})
}

// RegisterMCPRoutes registers MCP proxy routes on the given router group.
func (h *MCPProxyHandler) RegisterMCPRoutes(rg *gin.RouterGroup) {
	rg.POST("", h.HandleMCPProxy)
	rg.POST("/", h.HandleMCPProxy)
	rg.GET("/health", h.HandleMCPHealth)
	rg.POST("/capabilities", h.HandleMCPCapabilities)
}

// MCPProxyConfig holds MCP proxy configuration from system_config.
type MCPProxyConfig struct {
	Enabled        bool              `json:"enabled"`
	DefaultTarget  string            `json:"default_target"`
	Targets        map[string]string `json:"targets"` // name -> URL
	TimeoutSeconds int               `json:"timeout_seconds"`
}

func (c *MCPProxyConfig) timeout() time.Duration {
	if c == nil || c.TimeoutSeconds <= 0 {
		return 120 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// GetMCPProxyConfig retrieves MCP proxy configuration from system_config.
// Falls back to defaults if not configured.
func GetMCPProxyConfig(getter func(string) (string, error)) *MCPProxyConfig {
	cfg := defaultMCPProxyConfig()

	if v, err := getter("mcp_proxy_enabled"); err == nil && v == "true" {
		cfg.Enabled = true
	}
	if v, err := getter("mcp_proxy_default_target"); err == nil && v != "" {
		cfg.DefaultTarget = v
	}
	if v, err := getter("mcp_proxy_timeout_seconds"); err == nil && v != "" {
		var sec int
		if _, parseErr := fmt.Sscanf(v, "%d", &sec); parseErr == nil && sec > 0 {
			cfg.TimeoutSeconds = sec
		}
	}

	return cfg
}

// FormatMCPError formats an error as a JSON-RPC 2.0 error response.
func FormatMCPError(id interface{}, code int, msg string) MCPProxyResponse {
	return MCPProxyResponse{
		JSONRPC: "2.0",
		Error:   &MCPError{Code: code, Message: msg},
		ID:      id,
	}
}

// ValidateMCPMethod checks if a method name is a known MCP method.
func ValidateMCPMethod(method string) error {
	validMethods := map[string]bool{
		"initialize":                true,
		"notifications/initialized": true,
		"tools/list":                true,
		"tools/call":                true,
		"resources/list":            true,
		"resources/read":            true,
		"prompts/list":              true,
		"prompts/get":               true,
		"ping":                      true,
		"completion/complete":       true,
		"logging/setLevel":          true,
		"resources/subscribe":       true,
		"resources/unsubscribe":     true,
	}

	if !validMethods[method] {
		return fmt.Errorf("unknown MCP method: %s", method)
	}
	return nil
}

func (h *MCPProxyHandler) currentConfig() *MCPProxyConfig {
	if h == nil || h.loadConfig == nil {
		return defaultMCPProxyConfig()
	}
	cfg := h.loadConfig()
	if cfg == nil {
		return defaultMCPProxyConfig()
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 120
	}
	if cfg.Targets == nil {
		cfg.Targets = make(map[string]string)
	}
	return cfg
}

func (h *MCPProxyHandler) clientWithTimeout(timeout time.Duration) *http.Client {
	base := h.httpClient
	if base == nil {
		base = &http.Client{}
	}
	client := *base
	client.Timeout = timeout
	return &client
}

func resolveMCPProxyTarget(c *gin.Context, cfg *MCPProxyConfig) (string, error) {
	if cfg == nil {
		cfg = defaultMCPProxyConfig()
	}

	rawTarget := strings.TrimSpace(c.GetHeader("X-MCP-Target"))
	if rawTarget == "" {
		rawTarget = strings.TrimSpace(c.Query("target"))
	}
	if rawTarget == "" {
		rawTarget = strings.TrimSpace(c.GetHeader("X-Target-URL"))
	}
	if rawTarget == "" {
		rawTarget = strings.TrimSpace(cfg.DefaultTarget)
	}
	if rawTarget == "" {
		return "", fmt.Errorf("missing MCP target: set X-MCP-Target header or ?target= query param")
	}

	if mapped, ok := cfg.Targets[rawTarget]; ok {
		rawTarget = mapped
	}

	target, err := normalizeMCPURL(rawTarget)
	if err != nil {
		return "", fmt.Errorf("invalid MCP target: %w", err)
	}

	if cfg.Enabled && !isMCPAllowedTarget(target, cfg) {
		return "", fmt.Errorf("MCP target is not in allowlist")
	}
	return target, nil
}

func normalizeMCPURL(target string) (string, error) {
	target = strings.TrimSpace(strings.TrimRight(target, "/"))
	u, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	if !strings.HasSuffix(u.Path, "/mcp") && !strings.HasSuffix(u.Path, "/sse") {
		u.Path = strings.TrimRight(u.Path, "/") + "/mcp"
	}
	return strings.TrimRight(u.String(), "/"), nil
}

func isMCPAllowedTarget(target string, cfg *MCPProxyConfig) bool {
	allowed := make(map[string]struct{})
	if cfg.DefaultTarget != "" {
		if v, err := normalizeMCPURL(cfg.DefaultTarget); err == nil {
			allowed[v] = struct{}{}
		}
	}
	for _, t := range cfg.Targets {
		if v, err := normalizeMCPURL(t); err == nil {
			allowed[v] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return false
	}
	_, ok := allowed[target]
	return ok
}

func buildMCPHealthURL(mcpTarget string) string {
	base := strings.TrimRight(mcpTarget, "/")
	if strings.HasSuffix(base, "/mcp") {
		return strings.TrimSuffix(base, "/mcp") + "/health"
	}
	if strings.HasSuffix(base, "/sse") {
		return strings.TrimSuffix(base, "/sse") + "/health"
	}
	return base + "/health"
}

func streamMCPSSEResponse(c *gin.Context, resp *http.Response) error {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(resp.StatusCode)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported: response writer does not support flushing")
	}

	reader := bufio.NewReader(resp.Body)
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
