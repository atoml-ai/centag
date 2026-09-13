// Package mcp 以标准 MCP 协议（go-sdk：Streamable HTTP 与 SSE 传输）对外暴露
// centag 的只读观测面（mcp-interface-layer）：centag_info / read_log / read_database
// （read_metrics 由 A-T3 注入 provider 后追加，见 metrics_tool.go）。
//
// 安全边界（开发风险评估 R01）：
//   - mcp.enabled 默认关闭，未启用时不实例化（挂载方不注册路由 → 端点 404）；
//   - 端点复用外层 Bearer/admin 中间件链；未鉴权 401；
//   - 工具集仅只读（无写类工具）；mcp.allowed_tools 非空时按白名单裁剪
//     （tools/list 不可见 + tools/call 返回 403）；
//   - 数据面复用 agent.database 表白名单与只读 SQL 校验（单一真源，
//     见 core/internal/agent/tools）。
//
// 命名以 core/internal/agent/tools 工具正本的 Name() 为单一真源
// （规格中的 centag_status 对应工具正本 centag_info）。
//
// 注意：本包是 centag 自对外暴露的 MCP server，与既有入站反向代理
// MCPProxyHandler（core/pkg/server/mcp_proxy.go）是不同物，勿混淆。
package mcp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/atoml-ai/edgeag/pkg/agentcore"

	"centag/core/internal/agent/tools"
)

// Deps 与内置 agent 引擎共用同一注入面（单一真源，装配点 core/pkg/server/server.go）。
type Deps struct {
	DataDir       string                   // centag 数据目录（read_log / centag_info）
	DBPath        string                   // SQLite 数据库文件路径（centag_info 展示用）
	AllowedTables []string                 // 数据库表白名单（复用 agent.database）
	Version       string                   // centag 版本号（Implementation.Version）
	DB            *sql.DB                  // 元数据库句柄（read_database）
	Metrics       MetricsProvider          // 可选：A-T3 注入后启用 read_metrics；nil 时不注册
	Authorize     func(*http.Request) bool // 可选鉴权钩子：非 nil 且返回 false 时 401
}

// ObservationServer centag 自有的 MCP 只读观测面 server。
type ObservationServer struct {
	deps Deps

	mu          sync.Mutex // 保护 on/allowedList/sdk（前两者支持运行时热切换）
	on          bool       // mcp.enabled 运行时开关；false → 端点 404
	allowedList []string   // mcp.allowed_tools 配置片段；nil/空 = 全部只读工具
	sdk         *gomcp.Server
}

// NewObservationServer 装配 MCP server。enabled 为启动初始值；运行时可通过
// SetEnabled 热切换（系统配置保存后生效，无需重启），关闭时端点统一 404。
func NewObservationServer(enabled bool, deps Deps, allowedTools []string) *ObservationServer {
	return &ObservationServer{deps: deps, on: enabled, allowedList: allowedTools}
}

// SetEnabled 运行时热切换 MCP 服务启停（系统配置保存后调用，无需重启）。
func (s *ObservationServer) SetEnabled(on bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.on = on
}

// SetAllowedTools 运行时热更新白名单；已建立的 SDK 会话沿用旧工具面，
// 新会话按新白名单注册（懒初始化缓存置空触发重建）。
func (s *ObservationServer) SetAllowedTools(tools []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedList = tools
	s.sdk = nil
}

// AllowedToolNames 返回当前白名单过滤后的工具名（不依赖懒初始化状态）。
func (s *ObservationServer) AllowedToolNames() []string {
	if s == nil {
		return nil
	}
	base := []string{"centag_info", "read_log", "read_database"}
	if s.deps.Metrics != nil {
		base = append(base, "read_metrics")
	}
	s.mu.Lock()
	allowed := make([]string, len(s.allowedList))
	copy(allowed, s.allowedList)
	s.mu.Unlock()
	if len(allowed) == 0 {
		sort.Strings(base)
		return base
	}
	keep := map[string]bool{}
	for _, a := range allowed {
		keep[strings.ToLower(strings.TrimSpace(a))] = true
	}
	var out []string
	for _, name := range base {
		if keep[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (s *ObservationServer) version() string {
	if s.deps.Version == "" {
		return "dev"
	}
	return s.deps.Version
}

// sdkServer 懒构造 SDK server（幂等）。装配完成后字段不再写入（只读）。
func (s *ObservationServer) sdkServer() *gomcp.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sdk != nil {
		return s.sdk
	}

	// 工具构造器集合：复用 agent 工具正本（同一依赖注入 → 单一真源）。
	mk := map[string]func() agentcore.Tool{
		"centag_info": func() agentcore.Tool {
			return tools.NewCentagInfoTool(s.deps.DataDir, s.deps.DBPath, s.deps.AllowedTables)
		},
		"read_log": func() agentcore.Tool {
			return tools.NewReadLogTool(s.deps.DataDir)
		},
		"read_database": func() agentcore.Tool {
			return tools.NewReadDatabaseTool(s.deps.DB, s.deps.AllowedTables)
		},
	}
	if s.deps.Metrics != nil {
		mk["read_metrics"] = func() agentcore.Tool {
			return newReadMetricsTool(s.deps.Metrics)
		}
	}

	// 白名单过滤（R01：nil = 全部只读工具）。
	keep := map[string]bool{}
	if len(s.allowedList) > 0 {
		for _, a := range s.allowedList {
			keep[strings.ToLower(strings.TrimSpace(a))] = true
		}
	}

	impl := &gomcp.Implementation{Name: "centag-mcp", Version: s.version()}
	s.sdk = gomcp.NewServer(impl, nil)
	for _, name := range []string{"centag_info", "read_log", "read_database", "read_metrics"} {
		buildTool, ok := mk[name]
		if !ok {
			continue
		}
		if len(keep) > 0 && !keep[name] {
			continue // 白名单外工具不注册（tools/list 不可见）
		}
		t := buildTool()
		s.sdk.AddTool(toSDKTool(t), toSDKHandler(t))
	}
	return s.sdk
}

// getServer 每次 HTTP 会话建立时供 go-sdk 获取 server 实例。
func (s *ObservationServer) getServer(*http.Request) *gomcp.Server {
	return s.sdkServer()
}

// StreamableHandler 返回 MCP 标准 Streamable HTTP 传输 handler（端点建议挂 /mcp）。
// 外层做 Bearer 鉴权（401）与 allowed_tools 拦截（403）。
func (s *ObservationServer) StreamableHandler() http.Handler {
	return s.observe(s.sdkStreamable())
}

// SSEHandler 返回 MCP SSE（2024-11-05 版传输）兼容端点 handler（建议挂 /mcp/sse）。
func (s *ObservationServer) SSEHandler() http.Handler {
	return s.observe(s.sdkSSE())
}

// observe 包装鉴权（TC-MCP-SEC-002）与 allowed_tools 拦截（TC-MCP-SEC-003）。
func (s *ObservationServer) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isEnabled() {
			http.Error(w, "mcp service is disabled", http.StatusNotFound)
			return
		}
		if s.deps.Authorize != nil && !s.deps.Authorize(r) {
			http.Error(w, "unauthorized: mcp requires bearer auth", http.StatusUnauthorized)
			return
		}
		if !s.allowCall(r) {
			http.Error(w, "tool not allowed by mcp.allowed_tools", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isEnabled 读取运行时启停开关。
func (s *ObservationServer) isEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.on
}

// allowCall 拦截 JSON-RPC tools/call 白名单外调用（TC-MCP-SEC-003）。
// 其余方法（initialize / tools/list / notifications 等）直接放行；
// 未识别 method 放行（由 SDK 后续响应）。
func (s *ObservationServer) allowCall(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return true
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return true
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))

	var probe struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &probe) != nil || probe.Method != "tools/call" {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(probe.Params.Name))
	allowed := s.AllowedToolNames()
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if a == name {
			return true
		}
	}
	return false
}

func (s *ObservationServer) sdkStreamable() http.Handler {
	return gomcp.NewStreamableHTTPHandler(s.getServer, nil)
}

func (s *ObservationServer) sdkSSE() http.Handler {
	return gomcp.NewSSEHandler(s.getServer, nil)
}

// toSDKTool 将 agentcore 工具映射为 *gomcp.Tool（InputSchema 原样透传，
// 参数校验由工具自身 Execute 内完成 → 「工具名即契约」单一真源）。
func toSDKTool(t agentcore.Tool) *gomcp.Tool {
	schema, err := json.Marshal(t.ParamSchema())
	if err != nil || len(schema) == 0 {
		schema = []byte(`{"type":"object"}`)
	}
	return &gomcp.Tool{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: json.RawMessage(schema),
	}
}

// toSDKHandler 包装 agentcore Tool.Execute 为 MCP ToolHandler。
func toSDKHandler(t agentcore.Tool) gomcp.ToolHandler {
	return func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		params := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
				return nil, fmt.Errorf("invalid tool arguments: %w", err)
			}
		}
		res, err := t.Execute(ctx, params)
		if err != nil {
			// 工具基础设施失败 → IsError=true（让 LLM 看见），不作为协议错误。
			return &gomcp.CallToolResult{
				IsError: true,
				Content: []gomcp.Content{&gomcp.TextContent{Text: err.Error()}},
			}, nil
		}
		text := res.Content
		if text == "" && res.Details != nil {
			if b, jerr := json.Marshal(res.Details); jerr == nil {
				text = string(b)
			}
		}
		return &gomcp.CallToolResult{
			IsError: res.IsError,
			Content: []gomcp.Content{&gomcp.TextContent{Text: text}},
		}, nil
	}
}
