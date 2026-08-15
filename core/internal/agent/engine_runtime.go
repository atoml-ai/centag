package agent

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"centag/core/internal/agent/tools"

	"edgeag/pkg/agentcore"
)

// RuntimeEngine centag 内置 Agent 运行时引擎：负责构建 backend、注册工具并运行多轮推理。
type RuntimeEngine struct {
	config  *AgentConfig
	dataDir string
	db      *sql.DB
	dbPath  string // 数据库文件路径（用于 centag_info 工具）
	// BackendConfig 由 server 层注入（指向 centag 自身代理）
	backendHTTP *agentcore.HTTPBackend
	backendID   string // 当前 backend 的 ID（用于检测切换后重建）
	// currentPipelineID 当前透传的 pipeline ID（用于检测变化后更新 X-Pipeline-ID）
	currentPipelineID string
	// currentSessionID 当前透传的 session ID（用于检测变化后更新 X-Session-ID）
	currentSessionID string
}

// AgentEngineOptions 构造 Agent 运行时所需的 backend 信息
type AgentEngineOptions struct {
	// BaseURL centag 自身服务地址，如 http://127.0.0.1:20060
	BaseURL string
	// Token 当前请求的 JWT（用于通过代理鉴权）
	Token string
	// BackendID 指定后端（空=使用系统默认）
	BackendID string
	// Model 指定模型（空=使用系统默认）
	Model string
	// PipelineID 指定 skill 挂接的 pipeline（空=透传模式，不注入 X-Pipeline-ID）
	PipelineID string
	// SessionID 指定代理侧会话 ID（透传 X-Session-ID）。
	// 同一 agent 会话的所有 LLM 调用共享同一 session，使多轮推理合并为一条对话记录。
	SessionID string
}

// NewRuntimeEngine 创建 Agent 运行时引擎
func NewRuntimeEngine(config *AgentConfig, dataDir string, db *sql.DB) *RuntimeEngine {
	return &RuntimeEngine{
		config:  config,
		dataDir: dataDir,
		db:      db,
	}
}

// SetDBPath 设置数据库文件路径（供 centag_info 工具展示）
func (e *RuntimeEngine) SetDBPath(p string) {
	e.dbPath = p
}

// EnsureBackend 初始化 HTTP backend（指向 centag 自身代理）。
// 指定后端变化时重建，以更新 X-Backend-ID。
// PipelineID 非空时透传 X-Pipeline-ID（skill pipeline 路由）。
// SessionID 非空时透传 X-Session-ID（agent 会话 → 代理侧对话记录）。
func (e *RuntimeEngine) EnsureBackend(opts AgentEngineOptions) {
	if e.backendHTTP == nil || e.backendID != opts.BackendID {
		e.backendHTTP = agentcore.NewHTTPBackend(agentcore.HTTPBackendConfig{
			BaseURL:   opts.BaseURL,
			APIPath:   "/v1/chat/completions",
			Token:     opts.Token,
			Timeout:   e.effectiveTimeout(),
			Model:     opts.Model,
			BackendID: opts.BackendID,
			ProxyMode: "transparent",
			SessionID: opts.SessionID,
		})
		e.backendID = opts.BackendID
		e.currentPipelineID = opts.PipelineID
		e.currentSessionID = opts.SessionID
		e.backendHTTP.SetStrategy("", opts.PipelineID)
		return
	}
	if opts.Token != "" {
		e.backendHTTP.SetToken(opts.Token)
	}
	if opts.PipelineID != e.currentPipelineID {
		e.backendHTTP.SetStrategy("", opts.PipelineID)
		e.currentPipelineID = opts.PipelineID
	}
	if opts.SessionID != e.currentSessionID {
		e.backendHTTP.SetSessionID(opts.SessionID)
		e.currentSessionID = opts.SessionID
	}
}

// effectiveTimeout 返回有效的会话超时（0 或过小值回退到默认）
func (e *RuntimeEngine) effectiveTimeout() time.Duration {
	const defaultTimeout = 10 * time.Minute
	if e.config == nil || e.config.Timeout <= 0 {
		return defaultTimeout
	}
	return e.config.Timeout
}

// effectiveMaxTurns 返回有效最大轮次
func (e *RuntimeEngine) effectiveMaxTurns() int {
	if e.config == nil || e.config.MaxTurns <= 0 {
		return 20
	}
	return e.config.MaxTurns
}

// effectiveMaxTokens 返回有效最大 token
func (e *RuntimeEngine) effectiveMaxTokens() int {
	if e.config == nil || e.config.MaxTokens <= 0 {
		return 8192
	}
	return e.config.MaxTokens
}

// SetBackend 设置 backend（server 层可复用已构造的 HTTPBackend）
func (e *RuntimeEngine) SetBackend(b *agentcore.HTTPBackend) {
	e.backendHTTP = b
}

// RefreshToken 更新 backend 的 JWT token（refresh 后 JWT 轮换需要）
func (e *RuntimeEngine) RefreshToken(token string) {
	if e.backendHTTP != nil && token != "" {
		e.backendHTTP.SetToken(token)
	}
}

// registerTools 注册 centag 内置工具。
// skillTools 非空时按「skill 工具集 ∩ 全局白名单」收敛（T8）；为空时仅受全局白名单约束。
func (e *RuntimeEngine) registerTools(registry agentcore.ToolRegistry, skillTools ...[]string) error {
	allowed := make(map[string]bool)
	for _, t := range e.config.Tools.Allowed {
		allowed[t] = true
	}

	// skill 工具集（存在时）与全局白名单求交集，作为实际注册集
	var effective []string
	if len(skillTools) > 0 {
		effective = tools.IntersectAllowedTools(skillTools[0], e.config.Tools.Allowed)
	}
	skillSet := make(map[string]bool, len(effective))
	for _, t := range effective {
		skillSet[t] = true
	}

	readConfig := tools.NewReadConfigTool(e.dataDir)
	readLog := tools.NewReadLogTool(e.dataDir)
	readDB := tools.NewReadDatabaseTool(e.db, e.config.Database.AllowedTables)
	writeConfig := tools.NewWriteConfigTool(e.dataDir)
	analyze := tools.NewAnalyzeTool()
	systemInfo := tools.NewSystemInfoTool()
	centagInfo := tools.NewCentagInfoTool(e.dataDir, e.dbPath)

	for _, tool := range []agentcore.Tool{readConfig, readLog, readDB, writeConfig, analyze, systemInfo, centagInfo} {
		if !allowed[tool.Name()] {
			continue
		}
		if len(effective) > 0 && !skillSet[tool.Name()] {
			continue
		}
		registry.Register(tool)
	}
	return nil
}

// NewSession 创建一次会话对应的 agentcore.Agent 实例。
// systemPrompt 非空时作为 system 消息注入。
// skillTools 非空时按 skill 工具集 ∩ 全局白名单注册工具（T8）。
func (e *RuntimeEngine) NewSession(systemPrompt string, skillTools ...[]string) (*agentcore.Agent, error) {
	if e.backendHTTP == nil {
		return nil, fmt.Errorf("agent backend not initialized")
	}

	registry := agentcore.NewToolRegistry()
	if err := e.registerTools(registry, skillTools...); err != nil {
		return nil, err
	}

	opts := agentcore.AgentOptions{
		MaxTurns:      e.effectiveMaxTurns(),
		MaxTokens:     e.effectiveMaxTokens(),
		ToolExecution: "sequential",
	}

	// 需要确认的工具（write_config 等写操作）；无确认通道时自动拒绝（保持安全）
	policy := agentcore.NewPermissionPolicy(e.config.Tools.RequireConfirm)
	ag := agentcore.NewAgent(e.backendHTTP, registry, opts, policy, &agentcore.RuntimeConfig{
		Timeout: e.effectiveTimeout(),
	})

	// 同步聚合模式无 SSE 确认通道：需确认的工具自动拒绝，避免阻塞死锁
	ag.SetAutoDeny(true)

	if systemPrompt != "" {
		ag.LoadMessages([]*agentcore.AgentMessage{
			{Role: agentcore.RoleSystem, Content: systemPrompt},
		})
	}

	return ag, nil
}

// RunPrompt 运行一次完整 prompt（多轮推理 + 工具调用），返回最终文本。
func (e *RuntimeEngine) RunPrompt(ctx context.Context, ag *agentcore.Agent, input string, logf func(format string, args ...any)) (string, error) {
	if ag == nil {
		return "", fmt.Errorf("agent is nil")
	}

	events := ag.Subscribe()
	defer ag.Unsubscribe(events)

	timeout := e.effectiveTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var sb strings.Builder
	done := make(chan struct{})
	var promptErr error

	go func() {
		defer close(done)
		promptErr = ag.Prompt(ctx, input)
	}()

	for {
		select {
		case <-done:
			// Prompt 完成：仅收集当前已缓冲事件，不等待通道关闭（避免死锁）
			for {
				select {
				case evt, ok := <-events:
					if !ok {
						goto collected
					}
					e.collectEvent(evt, &sb, logf)
				default:
					goto collected
				}
			}
		collected:
			if promptErr != nil {
				return strings.TrimSpace(sb.String()), promptErr
			}
			if sb.Len() == 0 {
				return "Agent 未生成有效回复", nil
			}
			return strings.TrimSpace(sb.String()), nil
		case evt, ok := <-events:
			if !ok {
				continue
			}
			e.collectEvent(evt, &sb, logf)
		}
	}
}

// collectEvent 收集事件文本到输出。
// 仅收集正式回答 Content；ReasoningContent 为模型思考过程，不出现在最终回复中（记日志即可）。
func (e *RuntimeEngine) collectEvent(evt agentcore.AgentEvent, sb *strings.Builder, logf func(format string, args ...any)) {
	switch evt.Type {
	case agentcore.EventMessageUpdate:
		if evt.Message != nil {
			if evt.Message.Content != "" {
				sb.WriteString(evt.Message.Content)
			}
		}
	case agentcore.EventToolExecutionStart:
		if evt.ToolCall != nil && logf != nil {
			logf("[agent] tool start: %s input=%s", evt.ToolCall.ToolName, truncate(evt.ToolCall.Input, 200))
		}
	case agentcore.EventToolExecutionEnd:
		if evt.ToolResult != nil && logf != nil {
			logf("[agent] tool end: %s err=%v", evt.ToolResult.ToolName, evt.ToolResult.IsError)
		}
	case agentcore.EventError:
		if logf != nil && evt.Error != "" {
			logf("[agent] error: %s", evt.Error)
		}
	case agentcore.EventAgentStart:
		if evt.RouteInfo != nil && logf != nil {
			logf("[agent] start backend=%s model=%s", evt.RouteInfo.Backend, evt.RouteInfo.Model)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
