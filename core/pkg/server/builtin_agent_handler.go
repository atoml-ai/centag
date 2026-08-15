package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"centag/core/internal/agent"
	"centag/core/internal/agent/skills"
	"centag/core/internal/agent/tools"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"

	"edgeag/pkg/agentcore"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BuiltinAgentHandler 内置Agent处理器
type BuiltinAgentHandler struct {
	config              *agent.AgentConfig
	dataDir             string
	db                  *sql.DB
	skillRegistry       *skills.SkillRegistry
	skillPluginRegistry *skills.SkillPluginRegistry
	manifestStore       *skills.FileManifestStore
	pipelineRegistry    *pipeline.PipelineRegistry
	defaultBackend      string
	defaultModel        string
	toolRegistry        *tools.ToolRegistry
	provider            AgentDataProvider
	engine              *agent.RuntimeEngine
	baseURL             string
	sessions            map[string]*AgentSession
	sessionsMu          sync.RWMutex
	messages            map[string][]*AgentMessage // sessionID -> messages
	messagesMu          sync.RWMutex
}

// AgentSession Agent会话
type AgentSession struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Skill     string    `json:"skill"`
	BackendID string    `json:"backend_id"`
	Model     string    `json:"model"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	agentCore *agentcore.Agent
}

// AgentMessage Agent消息
type AgentMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Skill     string    `json:"skill"`
	ToolName  string    `json:"tool_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NewBuiltinAgentHandler 创建内置Agent处理器
func NewBuiltinAgentHandler(config *agent.AgentConfig, dataDir string, db *sql.DB, provider AgentDataProvider, baseURL, dbPath string, skillPluginRegistry *skills.SkillPluginRegistry, pipelineRegistry *pipeline.PipelineRegistry, defaultBackend, defaultModel string) *BuiltinAgentHandler {
	// 创建Skill注册表
	skillRegistry := skills.NewSkillRegistry()

	// 内置 manifest 加载成功时以 manifest 为权威来源注册 Skill；
	// 否则回退硬编码构造器（保持行为不变）。
	if skillPluginRegistry != nil {
		for _, p := range skillPluginRegistry.ListAll() {
			skillRegistry.RegisterSkill(skills.SkillFromPlugin(p))
		}
	}
	if len(skillRegistry.ListSkills()) == 0 {
		skills.LoadBuiltinSkills(skillRegistry)
	}
	
	// 创建工具注册表
	toolRegistry := tools.NewToolRegistry(dataDir, db, config.Database.AllowedTables)

	// 创建 Agent 运行时引擎
	engine := agent.NewRuntimeEngine(config, dataDir, db)
	engine.SetDBPath(dbPath)
	
	return &BuiltinAgentHandler{
		config:              config,
		dataDir:             dataDir,
		db:                  db,
		skillRegistry:       skillRegistry,
		skillPluginRegistry: skillPluginRegistry,
		manifestStore:       skills.NewFileManifestStore(filepath.Join(dataDir, "agent-skills")),
		pipelineRegistry:    pipelineRegistry,
		defaultBackend:      defaultBackend,
		defaultModel:        defaultModel,
		toolRegistry:        toolRegistry,
		provider:            provider,
		engine:              engine,
		baseURL:             baseURL,
		sessions:            make(map[string]*AgentSession),
		messages:            make(map[string][]*AgentMessage),
	}
}

// Health 健康检查
func (h *BuiltinAgentHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"enabled": h.config.Enabled,
	})
}

// loadBuiltinSkillPlugins 从内置 initdata 目录加载 agent-skill-*.yaml manifest。
// 目录不存在或全部解析失败时返回 nil（调用方回退硬编码构造器）。
func loadBuiltinSkillPlugins() *skills.SkillPluginRegistry {
	registry := skills.NewSkillPluginRegistry()
	sources := builtinSkillManifestSources()
	if len(sources) == 0 {
		return nil
	}
	if err := registry.LoadFromSources(sources); err != nil {
		logger.Warnf("agent: 内置 skill manifest 加载失败: %v", err)
		return nil
	}
	if len(registry.ListAll()) == 0 {
		return nil
	}
	logger.Infof("agent: 内置 skill manifest 注册 %d 个 skill", len(registry.ListAll()))
	return registry
}

// builtinSkillManifestSources 返回内置 skill manifest 的扫描来源（initdata pipeline-templates/common 与 team 目录）。
func builtinSkillManifestSources() []skills.ManifestSource {
	globalRoot, profileRoot := bootstrap.InitdataRoots()
	var sources []skills.ManifestSource
	seen := make(map[string]bool)
	for _, root := range []string{profileRoot, globalRoot} {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		for _, sub := range []string{"common", "team"} {
			dir := filepath.Join(root, "pipeline-templates", sub)
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				sources = append(sources, skills.ManifestSource{Dir: dir, Custom: false})
			}
		}
	}
	return sources
}

// CreateSession 创建会话
func (h *BuiltinAgentHandler) CreateSession(c *gin.Context) {
	var req struct {
		Skill     string `json:"skill"`
		BackendID string `json:"backend_id"`
		Model     string `json:"model"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// 检查skill是否允许
	if req.Skill != "" && !h.skillRegistry.IsSkillAllowed(req.Skill, h.config.Skills.InternalOnly) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill not allowed"})
		return
	}
	
	// 创建会话
	session := &AgentSession{
		ID:        uuid.New().String(),
		UserID:    0,
		TenantID:  "",
		Title:     "New Session",
		Skill:     req.Skill,
		BackendID: req.BackendID,
		Model:     req.Model,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	h.sessionsMu.Lock()
	h.sessions[session.ID] = session
	h.sessionsMu.Unlock()
	
	c.JSON(http.StatusOK, session)
}

// ListSessions 获取会话列表
func (h *BuiltinAgentHandler) ListSessions(c *gin.Context) {
	h.sessionsMu.RLock()
	defer h.sessionsMu.RUnlock()
	
	var sessions []*AgentSession
	for _, session := range h.sessions {
		sessions = append(sessions, session)
	}
	
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// GetSession 获取会话详情
func (h *BuiltinAgentHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	h.sessionsMu.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	
	c.JSON(http.StatusOK, session)
}

// DeleteSession 删除会话
func (h *BuiltinAgentHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()
	
	if _, ok := h.sessions[sessionID]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	
	delete(h.sessions, sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "session deleted"})
}

// SendMessage 发送消息
func (h *BuiltinAgentHandler) SendMessage(c *gin.Context) {
	sessionID := c.Param("id")
	
	var req struct {
		Content   string `json:"content"`
		Skill     string `json:"skill"`
		BackendID string `json:"backend_id"`
		Model     string `json:"model"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	h.sessionsMu.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	
	// 允许请求更新会话的后端/模型
	if req.BackendID != "" {
		h.sessionsMu.Lock()
		session.BackendID = req.BackendID
		h.sessionsMu.Unlock()
	}
	if req.Model != "" {
		h.sessionsMu.Lock()
		session.Model = req.Model
		h.sessionsMu.Unlock()
	}
	
	// 检查skill是否允许
	skillName := req.Skill
	if skillName == "" {
		skillName = session.Skill
	}
	
	if skillName != "" && !h.skillRegistry.IsSkillAllowed(skillName, h.config.Skills.InternalOnly) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "skill not allowed"})
		return
	}
	
	// 创建用户消息
	userMessage := &AgentMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Content,
		Skill:     skillName,
		CreatedAt: time.Now(),
	}

	// 保存消息到内存（重启即失效；后续接入 DB）
	h.messagesMu.Lock()
	h.messages[sessionID] = append(h.messages[sessionID], userMessage)
	h.messagesMu.Unlock()

	// 执行真实 agent（多轮推理 + 工具调用）
	reply, err := h.runAgent(c, session, skillName, req.Content)
	if err != nil {
		logger.Warnf("[builtin-agent] run failed: %v", err)
		reply = fmt.Sprintf("Agent 执行失败: %v\n\n用户问题: %s", err, req.Content)
	}

	// agent 响应
	agentMessage := &AgentMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   reply,
		Skill:     skillName,
		CreatedAt: time.Now(),
	}

	// 保存 assistant 消息
	h.messagesMu.Lock()
	h.messages[sessionID] = append(h.messages[sessionID], agentMessage)
	h.messagesMu.Unlock()

	// 更新会话时间
	h.sessionsMu.Lock()
	if sess, ok := h.sessions[sessionID]; ok {
		sess.UpdatedAt = time.Now()
	}
	h.sessionsMu.Unlock()

	c.JSON(http.StatusOK, agentMessage)
}

// runAgent 运行内置 agent，返回最终文本回复。
func (h *BuiltinAgentHandler) runAgent(c *gin.Context, session *AgentSession, skillName, userInput string) (string, error) {
	if h.engine == nil {
		return "", fmt.Errorf("agent engine not initialized")
	}

	// skill → pipeline id（空 skill 不注入 X-Pipeline-ID）
	pipelineID := h.resolveSkillPipelineID(skillName)

	// 构造 backend（指向 centag 自身代理）
	token := ""
	if auth := c.GetHeader("Authorization"); auth != "" {
		token = strings.TrimPrefix(auth, "Bearer ")
	}
	h.engine.EnsureBackend(agent.AgentEngineOptions{
		BaseURL:    h.baseURL,
		Token:      token,
		BackendID:  session.BackendID,
		Model:      session.Model,
		PipelineID: pipelineID,
	})
	// 每次刷新 token（refresh 后 JWT 会轮换）
	h.engine.RefreshToken(token)

	// 首次会话：创建 agentcore.Agent 并注入 skill 上下文
	if session.agentCore == nil {
		systemPrompt := h.buildSystemPrompt(skillName)
		ag, err := h.engine.NewSession(systemPrompt, h.skillTools(skillName))
		if err != nil {
			return "", err
		}
		h.sessionsMu.Lock()
		session.agentCore = ag
		h.sessionsMu.Unlock()
	}

	// 设置模型（若会话指定）
	if session.Model != "" {
		session.agentCore.SetModel(session.Model)
	}

	return h.engine.RunPrompt(c.Request.Context(), session.agentCore, userInput, func(format string, args ...any) {
		logger.Infof("[builtin-agent] "+format, args...)
	})
}

// buildSystemPrompt 按 skill 构建 system 提示词
func (h *BuiltinAgentHandler) buildSystemPrompt(skillName string) string {
	if skillName == "" {
		return defaultAgentSystemPrompt()
	}
	if sk, ok := h.skillRegistry.GetSkill(skillName); ok {
		return sk.BuildPrompt("")
	}
	return defaultAgentSystemPrompt()
}

// resolveSkillPipelineID 返回 skill 对应的 X-Pipeline-ID 值。
//   - skill 为空：自动路由，统一走 agent-skill-router（router 节点 LLM 分类决定 skill）。
//   - skill 已注册：显式指定，用 agent-skill-router:<skill> 强制 router 走该分支（跳过 LLM 分类）。
//   - skill 未注册或 skill 插件未初始化：返回空串（不注入 X-Pipeline-ID，回落透传）。
func (h *BuiltinAgentHandler) resolveSkillPipelineID(skillName string) string {
	if h.skillPluginRegistry == nil {
		return ""
	}
	if skillName == "" {
		return skills.AgentSkillRouterPipelineID
	}
	if p, ok := h.skillPluginRegistry.Get(skillName); ok && p.Enabled() {
		return skills.ForcedRoutePipelineID(skillName)
	}
	return ""
}

// unionSkillTools 返回全部启用 skill 的工具集并集（会话级）。
// 自动路由下 skill 在消息级才确定，无法在会话创建时按单 skill 收敛，
// 因此改为「全部可用 skill 工具 ∪ 全局白名单」的交集由 registerTools 处理（技术方案 §8.2 演进）。
func (h *BuiltinAgentHandler) unionSkillTools() []string {
	if h.skillPluginRegistry == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, p := range h.skillPluginRegistry.ListAll() {
		if !p.Enabled() {
			continue
		}
		for _, t := range p.GetSkillDefinition().Tools {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// skillTools 返回会话级 skill 工具集（全部启用 skill 工具并集）。
// 兼容旧调用签名：参数不再用于单 skill 收敛。
func (h *BuiltinAgentHandler) skillTools(skillName string) []string {
	return h.unionSkillTools()
}

// defaultAgentSystemPrompt 默认运维助手提示词
func defaultAgentSystemPrompt() string {
	return `你是一个 centag 运维助手，负责管理和诊断 centag 系统。

你可以使用以下工具（必须实际调用，而不是描述你将要做什么）：
- read_config：读取 centag 配置文件。参数 path：配置文件路径（相对于 centag 数据目录）
- read_log：读取 centag 日志文件。参数 path：日志文件路径（相对于 centag 数据目录）；可选 lines：读取行数；filter：过滤关键词
- read_database：查询 centag 数据库（只读）。参数 table：表名；可选 query：SQL
- write_config：写入配置文件（需要用户确认）。参数 path、content
- analyze：分析数据并生成报告。参数 data、type（status/config/error/log/strategy）
- system_info：获取当前操作系统/主机信息（只读）。可选 detail：os/host/arch/env/all
- centag_info：获取 centag 系统信息——数据目录结构、日志文件路径、数据库路径与可用表、配置说明。分析日志/配置/数据库前，建议先调用此工具了解文件位置

执行规则（非常重要）：
1. 必须通过调用工具获取真实数据，禁止凭空编造或只描述计划。
2. 分析日志前先调用 centag_info 或 read_log（无 path 会列出候选）确定日志路径；不要臆测路径。
3. 用户询问操作系统/系统信息时，调用 system_info 工具。
4. 禁止输出"我将开始..."之类的计划性文字；直接调用工具并基于工具结果回答。
5. 如果工具返回错误，报告错误信息并给出建议。`
}

// ListMessages 获取会话消息历史
func (h *BuiltinAgentHandler) ListMessages(c *gin.Context) {
	sessionID := c.Param("id")

	h.sessionsMu.RLock()
	_, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	h.messagesMu.RLock()
	messages := h.messages[sessionID]
	h.messagesMu.RUnlock()

	if messages == nil {
		messages = []*AgentMessage{}
	}
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// replyForSkill 根据 skill 生成 agent 回复。
// status-check 当前为确定性分析（读真实配置/后端/流水线），其余 skill 暂为占位说明。
func (h *BuiltinAgentHandler) replyForSkill(skillName, userInput string) string {
	if h.provider == nil {
		return "Agent 数据源未初始化，无法分析。\n\n您的问题是: " + userInput
	}

	switch skillName {
	case "status-check":
		return statusCheckReport(h.provider)
	case "config-analysis":
		return "config-analysis skill：读取配置并分析。\n\n暂以 status-check 结果代替：\n\n" + statusCheckReport(h.provider)
	case "error-diagnosis":
		return "error-diagnosis skill：诊断错误。\n\n请先运行 status-check 检查后端与流水线状态，或提供具体错误信息。"
	case "log-analysis":
		return "log-analysis skill：分析日志。\n\n日志分析能力待接入，请先运行 status-check。"
	case "strategy-recommend":
		return "strategy-recommend skill：策略建议。\n\n基于当前状态：\n\n" + statusCheckReport(h.provider)
	default:
		return "收到您的消息: " + userInput
	}
}

// ListSkills 获取可用Skills
func (h *BuiltinAgentHandler) ListSkills(c *gin.Context) {
	skillsList := h.skillRegistry.ListSkills()

	// 附加 manifest 元数据（pipeline_id / custom / system_prompt），供前端展示与编辑
	type skillView struct {
		*skills.Skill
		PipelineID   string `json:"pipeline_id"`
		Custom       bool   `json:"custom"`
		SystemPrompt string `json:"system_prompt"`
	}
	views := make([]skillView, 0, len(skillsList))
	for _, sk := range skillsList {
		view := skillView{Skill: sk}
		if h.skillPluginRegistry != nil {
			if p, ok := h.skillPluginRegistry.Get(sk.Name); ok {
				view.PipelineID = p.PipelineID()
				view.Custom = !p.Internal()
				view.SystemPrompt = p.GetSkillDefinition().SystemPrompt
			}
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{"skills": views})
}

// ConfirmTool 确认工具执行
func (h *BuiltinAgentHandler) ConfirmTool(c *gin.Context) {
	sessionID := c.Param("id")
	
	var req struct {
		Confirm bool   `json:"confirm"`
		ToolID  string `json:"tool_id"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	h.sessionsMu.RLock()
	_, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	
	// TODO: 处理工具确认逻辑
	
	c.JSON(http.StatusOK, gin.H{"message": "tool confirmed"})
}

// CancelExecution 取消执行
func (h *BuiltinAgentHandler) CancelExecution(c *gin.Context) {
	sessionID := c.Param("id")
	
	h.sessionsMu.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()
	
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	
	session.Status = "cancelled"
	session.UpdatedAt = time.Now()
	
	c.JSON(http.StatusOK, gin.H{"message": "execution cancelled"})
}