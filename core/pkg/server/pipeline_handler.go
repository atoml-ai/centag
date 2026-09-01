package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"centag/core/internal/auth"
	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/proxymode"
	"centag/core/pkg/scheduler"
	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// PipelineHandler 流水线 API 处理器
type PipelineHandler struct {
	nodeRegistry        *pipeline.NodeRegistry
	pipelineRegistry    *pipeline.PipelineRegistry
	engine              *pipeline.PipelineEngine
	templates           []pipeline.PatternTemplate
	pluginRegistryStore pipeline.PluginRegistryStore // 插件注册表存储（可为 nil）
	pipelineStore       pipeline.PipelineStore       // 流水线执行历史存储（可为 nil）
	modeManager         *proxymode.ModeManager       // 快捷码与 ModeManager 同步（可为 nil）
	edition             edition.Edition
	autoBuildScheduler  autoBuildScheduler
	autoBuildBackendMgr autoBuildBackendManager
	autoBuildMu         sync.Mutex
	autoBuildHistory    map[string][]autoBuildRevision
	autoBuildStop       context.CancelFunc
}

type autoBuildScheduler interface {
	ScheduleWithStrategy(question string, requestedModel string, strategy string) (*scheduler.ScheduleDecision, error)
	ScheduleWithCategory(category string, strategy string) (*scheduler.ScheduleDecision, error)
	UpdateBackendCache(backends []*backend.BackendConfig)
}

type autoBuildBackendManager interface {
	ProbeAllBackends(ctx context.Context, fetchModels bool) ([]*backend.ProbeResult, error)
	Save() error
	List() []*backend.BackendConfig
}

// NewPipelineHandler 创建流水线处理器
func NewPipelineHandler(engine *pipeline.PipelineEngine, nodeRegistry *pipeline.NodeRegistry, pipelineRegistry *pipeline.PipelineRegistry, templates []pipeline.PatternTemplate, pluginRegistryStore pipeline.PluginRegistryStore) *PipelineHandler {
	return &PipelineHandler{
		nodeRegistry:        nodeRegistry,
		pipelineRegistry:    pipelineRegistry,
		engine:              engine,
		templates:           templates,
		pluginRegistryStore: pluginRegistryStore,
	}
}

// SetPipelineStore 注入流水线执行历史存储
func (h *PipelineHandler) SetPipelineStore(store pipeline.PipelineStore) {
	h.pipelineStore = store
}

// SetModeManager 注入模式管理器，使流水线快捷码与 ModeManager 保持同步。
func (h *PipelineHandler) SetModeManager(mgr *proxymode.ModeManager) {
	h.modeManager = mgr
}

// ReloadTemplates reloads pipeline templates from configsync snapshot.
// Called after configsync completes to update the in-memory template list.
func (h *PipelineHandler) ReloadTemplates(edition string) {
	if h == nil {
		return
	}
	newTemplates := resolvePipelineTemplatesWithEdition(edition)
	h.templates = newTemplates
	logger.Infof("pipeline handler: templates reloaded from configsync (count=%d)", len(newTemplates))
}

// SeedIfEmpty creates pipelines from templates if the registry is empty.
// Called after configsync completes on first startup to ensure pipelines exist.
func (h *PipelineHandler) SeedIfEmpty() {
	if h == nil || h.pipelineRegistry == nil {
		return
	}
	if len(h.templates) == 0 {
		return
	}
	registered := 0
	skipped := 0
	for _, tmpl := range h.templates {
		id := strings.TrimSpace(tmpl.ID)
		if id == "" {
			continue
		}
		// Skip if pipeline already exists
		if h.pipelineRegistry.Exists(id) {
			skipped++
			continue
		}
		p := pipeline.CreatePipelineFromTemplate(tmpl, nil)
		if err := h.pipelineRegistry.Register(p); err != nil {
			logger.Warnf("pipeline seed: failed to register %s: %v", p.ID, err)
		} else {
			registered++
		}
	}
	if registered > 0 || skipped > 0 {
		logger.Infof("pipeline seed: registered=%d skipped=%d (already exists) total=%d", registered, skipped, len(h.templates))
	}
}

// Templates returns the current template list (for other subsystems).
func (h *PipelineHandler) Templates() []pipeline.PatternTemplate {
	if h == nil {
		return nil
	}
	return h.templates
}

// SetEdition configures product edition for Team resource access rules.
func (h *PipelineHandler) SetEdition(ed edition.Edition) {
	if h != nil {
		h.edition = ed
	}
}

func (h *PipelineHandler) accessUser(c *gin.Context) *database.User {
	return loadTeamNormalUser(c, h.edition)
}

// SetAutoBuildScheduler 注入用于 auto-build 的调度器。
func (h *PipelineHandler) SetAutoBuildScheduler(s autoBuildScheduler) {
	h.autoBuildScheduler = s
}

// SetAutoBuildBackendManager 注入用于 auto-build 的后端管理器（用于探测）。
func (h *PipelineHandler) SetAutoBuildBackendManager(mgr autoBuildBackendManager) {
	h.autoBuildBackendMgr = mgr
}

// GetPipelineRegistry 获取流水线注册表
func (h *PipelineHandler) GetPipelineRegistry() *pipeline.PipelineRegistry {
	return h.pipelineRegistry
}

// syncModesFromRegistry 将注册表中全部流水线快捷码同步到 ModeManager。
func (h *PipelineHandler) syncModesFromRegistry() {
	if h == nil || h.modeManager == nil || h.pipelineRegistry == nil {
		return
	}
	n := h.modeManager.SyncFromPipelines(h.pipelineRegistry.ListAll())
	logger.Infof("Pipeline modes synced to ModeManager: %d shortcuts", n)
}

// getTenantID 返回当前请求的资源作用域：
// - Team 普通用户 → ownTenantID（遗留 tenant_id 或合成 user:{id}）
// - 管理员 → ""（系统级）
func (h *PipelineHandler) getTenantID(c *gin.Context) string {
	if user := h.accessUser(c); user != nil {
		return ownTenantID(user)
	}
	return ""
}

// requirePipelineAccess validates pipeline access for the given scope.
// On success returns (pipeline, nil); on failure writes HTTP error and returns (nil, err).
func (h *PipelineHandler) requirePipelineAccess(c *gin.Context, pipelineID string) (*pipeline.AgentPatternPipeline, error) {
	scope := auth.GetScopedAccess(c)

	lookup := func(id string) *pipeline.AgentPatternPipeline {
		if user := h.accessUser(c); user != nil {
			return h.pipelineRegistry.GetByTenant(ownTenantID(user), id)
		}
		if scope == auth.AccessGlobal {
			// Admin: 仅系统级流水线
			return h.pipelineRegistry.Get(id)
		}
		return h.pipelineRegistry.GetByTenant("", id)
	}

	// 先按请求原值精确查找（兼容以旧 ID 注册的存量流水线），
	// 未命中再走执行层同源的别名归一（P1-T5：direct-backend → transparent），
	// 消除「执行可用、详情 403」裂缝。
	p := lookup(pipelineID)
	if p == nil {
		if norm := pipeline.NormalizePipelineID(pipelineID); norm != pipelineID {
			p = lookup(norm)
			if p != nil {
				pipelineID = norm
			}
		}
	}

	if p == nil {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   fmt.Sprintf("pipeline %s not found or access denied", pipelineID),
		})
		return nil, fmt.Errorf("access denied")
	}
	if user := h.accessUser(c); user != nil {
		filtered := useraccess.FilterPipelinesFor(user, []*pipeline.AgentPatternPipeline{p}, policyForUser(c.Request.Context(), user))
		if len(filtered) == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   fmt.Sprintf("pipeline %s not found or access denied", pipelineID),
			})
			return nil, fmt.Errorf("access denied")
		}
	}
	return p, nil
}

func (h *PipelineHandler) Stop() {
	if h == nil {
		return
	}
	h.autoBuildMu.Lock()
	if h.autoBuildStop != nil {
		h.autoBuildStop()
		h.autoBuildStop = nil
	}
	h.autoBuildMu.Unlock()
	if h.nodeRegistry == nil {
		return
	}
	h.nodeRegistry.StopRemoteHealthChecks()
}

// recordExecution 异步记录执行历史到 PipelineStore
func (h *PipelineHandler) recordExecution(pipelineID string, input *pipeline.PipelineInput, output *pipeline.PipelineOutput, execErr error) {
	if h.pipelineStore == nil {
		return
	}

	record := &pipeline.ExecutionRecord{
		PipelineID:   pipelineID,
		InputContent: input.Content,
		Status:       "success",
	}

	if execErr != nil {
		record.Status = "failed"
		record.ErrorMessage = execErr.Error()
	}

	if output != nil {
		record.OutputContent = output.Content
		if output.ExecutionLog != nil {
			record.DurationMs = output.ExecutionLog.Duration
			record.TotalTokens = output.ExecutionLog.TotalTokens
			// 序列化节点审计摘要
			auditSummaries := make([]pipeline.NodeAuditSummary, 0, len(output.ExecutionLog.NodeLogs))
			for _, nl := range output.ExecutionLog.NodeLogs {
				auditSummaries = append(auditSummaries, pipeline.NodeAuditSummary{
					NodeID:         nl.NodeID,
					Implementation: nl.Implementation,
					Kind:           nl.Kind,
					Success:        nl.Success,
					DurationMs:     nl.Duration,
					ErrorCode:      nl.ErrorCode,
				})
			}
			if len(auditSummaries) > 0 {
				if data, err := json.Marshal(auditSummaries); err == nil {
					record.NodeAuditLog = string(data)
				}
			}
		}
	}

	// 异步写入，不阻塞请求返回
	go func() {
		if err := h.pipelineStore.RecordExecution(record); err != nil {
			logger.Debugf("failed to record pipeline execution: %v", err)
		}
	}()
}

// ListPipelines 列出流水线（组模型 policy 过滤）
// GET /api/v1/pipelines
// - 管理员：仅系统流水线（不暴露用户私有副本）
// - 普通用户：自有流水线 + policy 允许的系统预设
func (h *PipelineHandler) ListPipelines(c *gin.Context) {
	var pipelines []*pipeline.AgentPatternPipeline

	if user := h.accessUser(c); user != nil {
		pipelines = h.pipelineRegistry.ListByTenant(ownTenantID(user))
		pipelines = useraccess.FilterPipelinesFor(user, pipelines, policyForUser(c.Request.Context(), user))
	} else {
		// Admin / 其它：仅系统级
		pipelines = h.pipelineRegistry.List()
	}

	// 注入 RouteConfig：使前端能获取到完整的 RouteConfig 信息
	if len(h.templates) > 0 {
		for i, p := range pipelines {
			pipelines[i] = injectRouteConfigFromTemplate(p, h.templates)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    pipelines,
	})
}

// GetPipeline 获取单个流水线（租户隔离）
// GET /api/v1/pipelines/:id
func (h *PipelineHandler) GetPipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	p, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return // error already written
	}

	// 注入 RouteConfig：如果已存储的流水线节点缺少 RouteConfig，
	// 从模板中补全，使前端能获取到完整的 RouteConfig 信息
	if len(h.templates) > 0 {
		p = injectRouteConfigFromTemplate(p, h.templates)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    p,
	})
}

// CreatePipeline 创建流水线
// POST /api/v1/pipelines?overwrite=true
// - overwrite=true（默认）：允许覆盖已有流水线
// - overwrite=false：如果流水线已存在则返回冲突错误
// - 管理员：创建全局（系统级）流水线
// - 普通用户：自动绑定到 ownTenantID（遗留 tenant_id 或合成 user:{id}）
func (h *PipelineHandler) CreatePipeline(c *gin.Context) {
	tenantID := ""
	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnPipelines {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "adding or modifying own pipelines is disabled for this user"})
			return
		}
		tenantID = ownTenantID(user)
	}

	var req pipeline.AgentPatternPipeline
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 归一化所有节点：将顶层 Backend/Model 归入 Config，统一出口
	for i := range req.Nodes {
		req.Nodes[i].Normalize()
	}

	if req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	if req.Version == "" {
		req.Version = "1.0"
	}

	// 检查 overwrite 参数：false 时拒绝覆盖已有流水线
	overwrite := c.DefaultQuery("overwrite", "true") == "true"
	if !overwrite {
		var exists bool
		if tenantID != "" {
			exists = h.pipelineRegistry.ExistsInTenant(tenantID, req.ID)
		} else {
			exists = h.pipelineRegistry.Exists(req.ID)
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": fmt.Sprintf("pipeline '%s' already exists", req.ID)})
			return
		}
	}

	var err error
	if tenantID != "" {
		// 普通用户：注册到租户空间
		err = h.pipelineRegistry.RegisterForTenant(tenantID, &req)
	} else {
		// 管理员：全局注册
		err = h.pipelineRegistry.Register(&req)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 清除存储钩子缓存，确保下次执行使用最新配置
	if h.engine != nil {
		h.engine.InvalidateStorageHookCache(req.ID)
	}

	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
	})
}

// UpdatePipeline 更新流水线（角色感知）
// PUT /api/v1/pipelines/:id
func (h *PipelineHandler) UpdatePipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	// 验证访问权限
	existing, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return
	}
	tenantID := ""
	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnPipelines {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "adding or modifying own pipelines is disabled for this user"})
			return
		}
		tenantID = ownTenantID(user)
		if existing.TenantID == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "cannot modify system pipeline"})
			return
		}
	}

	var req pipeline.AgentPatternPipeline
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Warnf("[UpdatePipeline] id=%s ShouldBindJSON failed: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 归一化所有节点：将顶层 Backend/Model 归入 Config，统一出口
	for i := range req.Nodes {
		req.Nodes[i].Normalize()
	}

	// 确保ID一致
	req.ID = id

	// 先验证新数据，确保不会破坏现有流水线
	if err := req.Validate(); err != nil {
		logger.Warnf("[UpdatePipeline] id=%s validation failed: %v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 保存旧数据的深拷贝，用于注册失败时回滚
	oldCopy := *existing
	oldCopy.Nodes = make([]pipeline.PipelineNodeConfig, len(existing.Nodes))
	copy(oldCopy.Nodes, existing.Nodes)

	if tenantID != "" {
		// 普通用户：先移除旧的租户流水线，再注册新的
		h.pipelineRegistry.RemoveFromTenant(tenantID, id)
		if err := h.pipelineRegistry.RegisterForTenant(tenantID, &req); err != nil {
			logger.Warnf("[UpdatePipeline] tenant=%s id=%s RegisterForTenant failed: %v", tenantID, id, err)
			// 回滚：恢复旧的流水线
			_ = h.pipelineRegistry.RegisterForTenant(tenantID, &oldCopy)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	} else {
		// 管理员：全局更新
		h.pipelineRegistry.Remove(id)
		if err := h.pipelineRegistry.Register(&req); err != nil {
			logger.Warnf("[UpdatePipeline] id=%s Register failed: %v", id, err)
			// 回滚：恢复旧的流水线
			_ = h.pipelineRegistry.Register(&oldCopy)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
	}

	_ = existing // 已验证存在

	// 清除存储钩子缓存，确保下次执行使用最新配置
	if h.engine != nil {
		h.engine.InvalidateStorageHookCache(id)
	}

	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    req,
	})
}

// DeletePipeline 删除流水线（角色感知）
// DELETE /api/v1/pipelines/:id
func (h *PipelineHandler) DeletePipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	// 验证访问权限
	existing, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return
	}

	tenantID := ""
	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnPipelines {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "adding or modifying own pipelines is disabled for this user"})
			return
		}
		tenantID = ownTenantID(user)
		if existing != nil && existing.TenantID == "" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "cannot delete system pipeline"})
			return
		}
	}

	if err := h.pipelineRegistry.DeleteScoped(tenantID, id); err != nil {
		logger.Warnf("[DeletePipeline] tenant=%s id=%s failed: %v", tenantID, id, err)
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "cannot delete system pipeline") {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 清除存储钩子缓存
	if h.engine != nil {
		h.engine.InvalidateStorageHookCache(id)
	}

	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "pipeline deleted",
	})
}

// ClonePipeline 从已有流水线复制一份到当前用户空间
// POST /api/v1/pipelines/:id/clone
// - 管理员：复制为全局流水线（新 ID，不覆盖原流水线）
// - 普通用户：复制为用户自有流水线（TenantID = own）
// - 原流水线可以是系统预设或其他用户可见的流水线
// - 新流水线 ID 默认为 "{原ID}-copy-{时间戳}"，也可通过请求体自定义
func (h *PipelineHandler) ClonePipeline(c *gin.Context) {
	srcID := c.Param("id")
	if srcID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	// 1. 读取源流水线（需要有访问权限）
	src, err := h.requirePipelineAccess(c, srcID)
	if err != nil {
		return // error already written
	}

	// 2. 深拷贝源流水线
	data, err := json.Marshal(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to clone pipeline"})
		return
	}
	var clone pipeline.AgentPatternPipeline
	if err := json.Unmarshal(data, &clone); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to clone pipeline"})
		return
	}

	// 3. 生成新 ID（支持请求体自定义）
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)

	if req.ID != "" {
		clone.ID = req.ID
	} else {
		clone.ID = fmt.Sprintf("%s-copy-%d", srcID, time.Now().UnixMilli())
	}
	if req.Name != "" {
		clone.Name = req.Name
	} else {
		clone.Name = fmt.Sprintf("%s (副本)", src.Name)
	}

	// 4. 设置租户作用域
	tenantID := ""
	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnPipelines {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "adding or modifying own pipelines is disabled for this user"})
			return
		}
		tenantID = ownTenantID(user)
	}

	// 清除系统流水线的 TenantID；快捷码需合法 # 前缀，才能同步进 ModeManager
	clone.TenantID = ""
	clone.ShortcutCode = fmt.Sprintf("#u%d", time.Now().UnixMilli()%1_000_000_000)
	clone.Version = "1.0"

	// 5. 注册
	if tenantID != "" {
		err = h.pipelineRegistry.RegisterForTenant(tenantID, &clone)
	} else {
		err = h.pipelineRegistry.Register(&clone)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 清除存储钩子缓存
	if h.engine != nil {
		h.engine.InvalidateStorageHookCache(clone.ID)
	}
	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    &clone,
	})
}

// ExecutePipeline 测试执行流水线（租户隔离）
// POST /api/v1/pipelines/:id/execute
func (h *PipelineHandler) ExecutePipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	// 验证访问权限
	_, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return
	}

	var req struct {
		Content  string                 `json:"content"`
		Messages []pipeline.Message     `json:"messages,omitempty"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// A-修复（P1-4 回归）：空输入不应透传给上游（上游会 400 并被包成 500），
	// 直接以 400 返回清晰的客户端错误。
	if !hasUsableInput(req.Content, req.Messages) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "messages must not be empty"})
		return
	}

	if h.engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "pipeline engine not initialized"})
		return
	}

	// P1-T9：结构化入口补齐 transparent_forward 所需的原始请求体。
	// 节点只消费 metadata["raw_request_body"]；chat 入口由 attachTransparentRequestMetadata
	// 注入，而本入口此前缺失，导致 messages-only 请求被节点回退为空 content 单条消息，
	// 上游报 "Input must have at least 1 token."。这里把 messages/content 规范化为
	// chat-completions 形态注入；metadata.model 作为客户端模型提示一并写入。
	// 同时对齐键名：backend → backend_id（transparent_forward 的 pinning 只读 backend_id）。
	meta := req.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
	}
	if _, ok := meta["raw_request_body"]; !ok {
		msgs := req.Messages
		if len(msgs) == 0 && strings.TrimSpace(req.Content) != "" {
			msgs = []pipeline.Message{{Role: "user", Content: req.Content}}
		}
		if len(msgs) == 0 {
			msgs = []pipeline.Message{{Role: "user", Content: ""}}
		}
		synth := map[string]interface{}{"stream": false, "messages": msgs}
		if m, ok := meta["model"].(string); ok && strings.TrimSpace(m) != "" {
			synth["model"] = strings.TrimSpace(m)
		}
		if raw, err := json.Marshal(synth); err == nil {
			meta["raw_request_body"] = string(raw)
		}
	}
	if b, ok := meta["backend"].(string); ok && b != "" {
		if _, exists := meta["backend_id"]; !exists {
			meta["backend_id"] = b
		}
	}

	input := &pipeline.PipelineInput{
		Content:   req.Content,
		Messages:  req.Messages,
		Metadata:  meta,
		UserID:    "",
		SessionID: c.GetHeader("X-Request-ID"),
	}

	// 流水线执行使用独立 context，不与 HTTP 请求绑定。
	// 当浏览器超时断开连接时，HTTP request context 会被取消，
	// 但流水线应继续执行直到自身超时（global_config.timeout）。
	// 获取流水线配置的全局超时，默认 10 分钟兜底。
	const defaultPipelineTimeout = 10 * time.Minute
	execTimeout := defaultPipelineTimeout
	lookupCtx := context.Background()
	if user := h.accessUser(c); user != nil {
		lookupCtx = pipeline.WithOwnerScope(lookupCtx, ownTenantID(user))
	}
	// 将用户 ID 注入 context，供 FilterAllowedBackend 等 hook 回调使用。
	if uid, exists := c.Get(auth.CtxKeyUserID); exists {
		if uidInt64, ok := uid.(int64); ok {
			lookupCtx = pipeline.WithUserID(lookupCtx, uidInt64)
		}
	}
	if p := h.engine.LookupPipeline(lookupCtx, id); p != nil && p.GlobalConfig.Timeout > 0 {
		execTimeout = time.Duration(p.GlobalConfig.Timeout) * time.Second
	}
	execCtx, cancel := context.WithTimeout(lookupCtx, execTimeout)
	defer cancel()

	output, err := h.engine.Execute(execCtx, id, input)
	if err != nil {
		h.recordExecution(id, input, nil, err)
		c.JSON(pipelineErrorStatus(err), gin.H{"success": false, "error": fmt.Sprintf("pipeline execution failed: %v", err)})
		return
	}

	// P1-T3：上游 2xx+错误体已被标记为 upstream_error，按映射状态返回规范错误体。
	// P1-T10：透明转发状态码透传。
	if !handleTransparentStatusError(c, output, id, input, h) {
		return
	}

	h.recordExecution(id, input, output, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    output,
	})
}

// ExecutePipelineDirect 直接执行流水线定义（无需预先保存到注册表）。
// 用于前端画布编辑时的"测试"场景：发送当前画布中的完整流水线定义，后端执行后返回结果。
// POST /api/v1/pipelines/execute-direct
func (h *PipelineHandler) ExecutePipelineDirect(c *gin.Context) {
	var req struct {
		Pipeline pipeline.AgentPatternPipeline `json:"pipeline"`
		Content  string                        `json:"content"`
		Metadata map[string]interface{}        `json:"metadata,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if h.engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "pipeline engine not initialized"})
		return
	}

	input := &pipeline.PipelineInput{
		Content:   req.Content,
		Metadata:  req.Metadata,
		UserID:    "",
		SessionID: c.GetHeader("X-Request-ID"),
	}

	// 注入 RouteConfig，防止前端传递的流水线缺少 RouteConfig
	if len(h.templates) > 0 {
		injected := injectRouteConfigFromTemplate(&req.Pipeline, h.templates)
		if injected != &req.Pipeline {
			req.Pipeline = *injected
		}
	}

	const defaultPipelineTimeout = 10 * time.Minute
	execTimeout := defaultPipelineTimeout
	if req.Pipeline.GlobalConfig.Timeout > 0 {
		execTimeout = time.Duration(req.Pipeline.GlobalConfig.Timeout) * time.Second
	}
	// 将用户 ID 注入 context，供 FilterAllowedBackend 等 hook 回调使用。
	execCtx := context.Background()
	if uid, exists := c.Get(auth.CtxKeyUserID); exists {
		if uidInt64, ok := uid.(int64); ok {
			execCtx = pipeline.WithUserID(execCtx, uidInt64)
		}
	}
	execCtx, cancel := context.WithTimeout(execCtx, execTimeout)
	defer cancel()

	output, err := h.engine.ExecutePipelineDefinition(execCtx, &req.Pipeline, input)
	if err != nil {
		h.recordExecution(req.Pipeline.ID, input, nil, err)
		c.JSON(pipelineErrorStatus(err), gin.H{"success": false, "error": fmt.Sprintf("pipeline execution failed: %v", err)})
		return
	}

	// P1-T10：透明转发状态码透传（与 ExecutePipeline 同逻辑）。
	if !handleTransparentStatusError(c, output, req.Pipeline.ID, input, h) {
		return
	}

	h.recordExecution(req.Pipeline.ID, input, output, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    output,
	})
}

// hasUsableInput 判定是否存在非空用户输入（content 或任一 message 的 content）。
// 全空时不应透传给上游（上游会 400 并被包成 500），应直接以 400 拒绝。
func hasUsableInput(content string, msgs []pipeline.Message) bool {
	if strings.TrimSpace(content) != "" {
		return true
	}
	for _, m := range msgs {
		if strings.TrimSpace(m.Content) != "" {
			return true
		}
	}
	return false
}

// pipelineErrorStatus 将流水线执行错误映射为合适的 HTTP 状态码：
// 上游 429（限流/配额）与 4xx 透传，便于客户端/观测区分"配额不足"与"服务端错误"；
// 其余（含真正的上游 5xx 与不可归类的错误）回退 500。
func pipelineErrorStatus(err error) int {
	switch code := pipeline.UpstreamStatusCodeOf(err); {
	case code == http.StatusTooManyRequests: // 429 限流/配额
		return http.StatusTooManyRequests
	case code >= 400 && code < 500: // 上游客户端错误透传
		return code
	default:
		return http.StatusInternalServerError
	}
}

// GetTemplates 获取内置模板列表
// GET /api/v1/pipelines/templates
func (h *PipelineHandler) GetTemplates(c *gin.Context) {
	templates := h.templates
	if len(templates) == 0 {
		templates = resolvePipelineTemplates()
	}

	// 转换为前端友好的格式
	result := make(map[string]interface{})
	for _, tmpl := range templates {
		result[tmpl.ID] = map[string]interface{}{
			"id":            tmpl.ID,
			"name":          tmpl.Name,
			"description":   tmpl.Description,
			"shortcut_code": tmpl.ShortcutCode,
			"nodes":         tmpl.Nodes,
			"global_config": tmpl.GlobalConfig,
			"metadata":      tmpl.Metadata,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetNodePlugins 获取可用流水线节点插件描述
// GET /api/v1/pipelines/node-plugins
func (h *PipelineHandler) GetNodePlugins(c *gin.Context) {
	descriptors := h.nodeRegistry.GetPluginDescriptors()
	logger.Infof("GetNodePlugins: returning %d plugins", len(descriptors))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"schema_version": pipeline.PipelinePluginSchemaVersion,
			"plugins":        descriptors,
		},
	})
}

// TestNodePlugin 测试执行节点插件
// POST /api/v1/pipelines/node-plugins/:implementation/test
func (h *PipelineHandler) TestNodePlugin(c *gin.Context) {
	implementation := c.Param("implementation")
	if implementation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "implementation is required"})
		return
	}

	plugin, ok := h.nodeRegistry.GetPlugin(implementation)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": fmt.Sprintf("plugin not found: %s", implementation)})
		return
	}

	var req struct {
		Config map[string]interface{} `json:"config"`
		Input  map[string]interface{} `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	nodeConfig := pipeline.NodeConfig{}
	if req.Config != nil {
		if data, err := json.Marshal(req.Config); err == nil {
			_ = json.Unmarshal(data, &nodeConfig)
		}
	}

	if err := plugin.ValidateConfig(nodeConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("config validation failed: %v", err),
			"code":    "CONFIG_INVALID",
		})
		return
	}

	nodeInput := &pipeline.NodeInput{}
	if req.Input != nil {
		if data, err := json.Marshal(req.Input); err == nil {
			_ = json.Unmarshal(data, nodeInput)
		}
	}

	execReq := &pipeline.NodeExecutionRequest{
		SchemaVersion:  pipeline.PipelinePluginSchemaVersion,
		Implementation: implementation,
		Config:         nodeConfig,
		Input:          nodeInput,
		TraceID:        c.GetHeader("X-Trace-ID"),
		RequestID:      c.GetHeader("X-Request-ID"),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	resp, err := plugin.Execute(ctx, execReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("plugin execution failed: %v", err),
			"code":    "EXECUTION_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"output": resp.Output,
			"events": resp.Events,
		},
	})
}

// GetNodePluginByImplementation 获取指定实现的节点插件描述
// GET /api/v1/pipelines/node-plugins/:implementation
func (h *PipelineHandler) GetNodePluginByImplementation(c *gin.Context) {
	implementation := c.Param("implementation")
	if implementation == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "implementation is required"})
		return
	}

	plugin, ok := h.nodeRegistry.GetPlugin(implementation)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": fmt.Sprintf("plugin not found: %s", implementation)})
		return
	}

	// 准备返回数据
	data := gin.H{
		"schema_version": pipeline.PipelinePluginSchemaVersion,
		"plugin":         plugin.Descriptor(),
	}

	// 如果是 RemoteNodePlugin，添加健康状态和熔断状态
	if rnp, ok := plugin.(*pipeline.RemoteNodePlugin); ok {
		status, lastCheck := rnp.GetHealthStatus()
		data["health_status"] = status
		data["circuit_open"] = rnp.IsCircuitOpen()
		data["failure_count"] = rnp.GetFailureCount()
		data["last_health_check"] = lastCheck
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetPluginMetrics 获取插件指标
// GET /api/v1/pipelines/plugin-metrics
func (h *PipelineHandler) GetPluginMetrics(c *gin.Context) {
	snapshot := pipeline.GlobalPluginMetrics.GetSnapshot()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    snapshot,
	})
}

// DiscoverRemoteNodePlugin 获取远程节点插件 manifest，并注册到当前节点注册表。
// POST /api/v1/pipelines/node-plugins/discover
func (h *PipelineHandler) DiscoverRemoteNodePlugin(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.BaseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "base_url is required"})
		return
	}

	plugin := pipeline.NewRemoteNodePlugin(req.BaseURL)
	if err := h.nodeRegistry.RegisterPlugin(plugin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 注册到插件注册表（如果可用）
	if h.pluginRegistryStore != nil {
		descriptor := plugin.Descriptor()
		// 序列化 descriptor 为 JSON
		descriptorJSON, err := json.Marshal(descriptor)
		if err == nil {
			// 签名验证并确定 signature_status
			sigStatus := "none"
			if sv := h.nodeRegistry.GetSecurityValidator(); sv != nil && sv.IsEnabled() {
				if err := sv.ValidateManifestSignature(descriptor); err != nil {
					sigStatus = "invalid"
					logger.Warnf("Plugin signature invalid (implementation=%s): %v", descriptor.Implementation, err)
				} else if descriptor.Signature != "" {
					sigStatus = "verified"
					logger.Infof("Plugin signature verified (implementation=%s)", descriptor.Implementation)
				}
			} else if descriptor.Signature != "" {
				// 即使安全验证器未启用，如果 manifest 有签名，也尝试验证
				sigStatus = "present"
			}
			reg := &pipeline.PluginRegistration{
				Implementation:  descriptor.Implementation,
				Kind:            descriptor.Kind,
				Version:         descriptor.Version,
				DescriptorJSON:  string(descriptorJSON),
				Source:          "remote",
				Enabled:         true,
				SignatureStatus: sigStatus,
			}
			if err := h.pluginRegistryStore.Register(reg); err != nil {
				// 记录警告，但不阻止插件注册
				logger.Warnf("failed to register plugin to registry (implementation=%s): %v", descriptor.Implementation, err)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plugin.Descriptor(),
	})
}

// ValidatePipeline 验证流水线配置（租户隔离）
// POST /api/v1/pipelines/:id/validate
func (h *PipelineHandler) ValidatePipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	var req pipeline.AgentPatternPipeline
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有请求体，尝试从注册表获取（需验证访问权限）
		p, accessErr := h.requirePipelineAccess(c, id)
		if accessErr != nil {
			return
		}
		req = *p
	}

	// 使用配置验证器进行详细验证
	validator := pipeline.ValidatePipelineConfig(&req)
	if validator.HasErrors() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"data": map[string]interface{}{
				"valid":  false,
				"errors": validator.GetErrors(),
			},
		})
		return
	}

	// 执行完整验证
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"data": map[string]interface{}{
				"valid":  false,
				"errors": []string{err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"valid":  true,
			"errors": []string{},
		},
	})
}

// ExportPipeline 导出单个流水线配置为 YAML（租户隔离）
// GET /api/v1/pipelines/:id/export
func (h *PipelineHandler) ExportPipeline(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	p, err := h.requirePipelineAccess(c, id)
	if err != nil {
		return
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("failed to marshal pipeline: %v", err)})
		return
	}

	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.yaml\"", id))
	c.String(http.StatusOK, string(data))
}

// RegisterPipelineRoutes 注册流水线路由
func (h *PipelineHandler) RegisterPipelineRoutes(router *gin.RouterGroup) {
	pipelines := router.Group("/pipelines")
	{
		pipelines.GET("", h.ListPipelines)
		pipelines.GET("/templates", h.GetTemplates)
		pipelines.GET("/node-plugins", h.GetNodePlugins)
		pipelines.GET("/node-plugins/:implementation", h.GetNodePluginByImplementation)
		pipelines.POST("/node-plugins/:implementation/test", h.TestNodePlugin)
		pipelines.POST("/node-plugins/discover", h.DiscoverRemoteNodePlugin)
		pipelines.GET("/plugin-metrics", h.GetPluginMetrics)
		pipelines.POST("/execute-direct", h.ExecutePipelineDirect)
		pipelines.POST("", h.CreatePipeline)
		pipelines.GET("/executions/:execId", h.GetExecutionDetail)
		pipelines.GET("/:id", h.GetPipeline)
		pipelines.PUT("/:id", h.UpdatePipeline)
		pipelines.DELETE("/:id", h.DeletePipeline)
		pipelines.POST("/:id/auto-build", h.AutoBuildPipeline)
		pipelines.POST("/:id/auto-build/rollback", h.AutoBuildRollback)
		pipelines.POST("/:id/clone", h.ClonePipeline)
		pipelines.POST("/:id/execute", h.ExecutePipeline)
		pipelines.POST("/:id/validate", h.ValidatePipeline)
		pipelines.GET("/:id/export", h.ExportPipeline)
		pipelines.GET("/:id/executions", h.ListExecutionHistory)
		pipelines.GET("/:id/available-variables", h.GetAvailableVariables)
		// 节点配置管理 API
		nodes := pipelines.Group("/:id/nodes/:nodeId")
		{
			nodes.PUT("/config", h.UpdateNodeConfig)
		}
	}

	// 插件注册表管理 API
	registry := pipelines.Group("/plugin-registry")
	{
		registry.GET("", h.ListPluginRegistry)
		registry.GET("/:implementation", h.GetPluginRegistry)
		registry.PUT("/:implementation", h.UpdatePluginRegistry)
		registry.DELETE("/:implementation", h.DeletePluginRegistry)
	}
}

// ListPluginRegistry 列出所有注册的插件
// GET /api/v1/pipelines/plugin-registry
func (h *PipelineHandler) ListPluginRegistry(c *gin.Context) {
	if h.pluginRegistryStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "plugin registry store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	plugins, err := h.pluginRegistryStore.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to list plugins: %v", err),
			"code":    "LIST_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plugins,
	})
}

// GetPluginRegistry 获取单个插件注册信息
// GET /api/v1/pipelines/plugin-registry/:implementation
func (h *PipelineHandler) GetPluginRegistry(c *gin.Context) {
	implementation := c.Param("implementation")
	if implementation == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "implementation is required",
			"code":    "INVALID_PARAM",
		})
		return
	}

	if h.pluginRegistryStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "plugin registry store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	plugin, err := h.pluginRegistryStore.Get(implementation)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("plugin not found: %v", err),
			"code":    "NOT_FOUND",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plugin,
	})
}

// UpdatePluginRegistry 更新插件注册信息
// PUT /api/v1/pipelines/plugin-registry/:implementation
func (h *PipelineHandler) UpdatePluginRegistry(c *gin.Context) {
	implementation := c.Param("implementation")
	if implementation == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "implementation is required",
			"code":    "INVALID_PARAM",
		})
		return
	}

	var req struct {
		Enabled         *bool  `json:"enabled"`
		SignatureStatus string `json:"signature_status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request body",
			"code":    "INVALID_BODY",
		})
		return
	}

	if h.pluginRegistryStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "plugin registry store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	// 获取现有记录
	plugin, err := h.pluginRegistryStore.Get(implementation)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("plugin not found: %v", err),
			"code":    "NOT_FOUND",
		})
		return
	}

	// 更新字段
	if req.Enabled != nil {
		plugin.Enabled = *req.Enabled
	}
	if req.SignatureStatus != "" {
		plugin.SignatureStatus = req.SignatureStatus
	}

	if err := h.pluginRegistryStore.Update(plugin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to update plugin: %v", err),
			"code":    "UPDATE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    plugin,
	})
}

// DeletePluginRegistry 删除插件注册信息
// DELETE /api/v1/pipelines/plugin-registry/:implementation
func (h *PipelineHandler) DeletePluginRegistry(c *gin.Context) {
	implementation := c.Param("implementation")
	if implementation == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "implementation is required",
			"code":    "INVALID_PARAM",
		})
		return
	}

	if h.pluginRegistryStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "plugin registry store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	if err := h.pluginRegistryStore.Delete(implementation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to delete plugin: %v", err),
			"code":    "DELETE_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "plugin deleted successfully",
	})
}

// UpdateNodeConfig 更新特定节点的配置
// PUT /api/v1/pipelines/:id/nodes/:nodeId/config
func (h *PipelineHandler) UpdateNodeConfig(c *gin.Context) {
	pipelineId := c.Param("id")
	nodeId := c.Param("nodeId")

	if pipelineId == "" || nodeId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "pipelineId and nodeId are required",
			"code":    "INVALID_PARAM",
		})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("invalid request body: %v", err),
			"code":    "INVALID_BODY",
		})
		return
	}

	// 获取现有的流水线配置（需验证访问权限）
	existingPipeline, err := h.requirePipelineAccess(c, pipelineId)
	if err != nil {
		return
	}

	// 查找指定的节点
	nodeIndex := -1
	for i, node := range existingPipeline.Nodes {
		if node.ID == nodeId {
			nodeIndex = i
			break
		}
	}

	if nodeIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("node not found: %s in pipeline: %s", nodeId, pipelineId),
			"code":    "NODE_NOT_FOUND",
		})
		return
	}

	// 更新节点配置
	node := &existingPipeline.Nodes[nodeIndex]

	// 处理 custom_config 更新
	if customConfig, ok := req["custom_config"].(map[string]interface{}); ok {
		if node.Config.CustomConfig == nil {
			node.Config.CustomConfig = make(map[string]interface{})
		}

		// 更新 custom_config 字段
		for k, v := range customConfig {
			node.Config.CustomConfig[k] = v
		}
	}

	// 更新其他节点配置字段
	if backend, ok := req["backend"].(string); ok {
		node.Config.Backend = backend
	}
	if model, ok := req["model"].(string); ok {
		node.Config.Model = model
	}
	if promptTemplate, ok := req["prompt_template"].(string); ok {
		node.Config.PromptTemplate = promptTemplate
	}
	if systemPrompt, ok := req["system_prompt"].(string); ok {
		node.Config.SystemPrompt = systemPrompt
	}
	if temperature, ok := req["temperature"].(float64); ok {
		temp := temperature
		node.Config.Temperature = &temp
	}
	if maxTokens, ok := req["max_tokens"].(float64); ok {
		tokens := int(maxTokens)
		node.Config.MaxTokens = &tokens
	}
	if maxInputBytes, ok := req["max_input_bytes"].(float64); ok {
		node.Config.MaxInputBytes = int64(maxInputBytes)
	}

	// 重新注册流水线以应用更改
	if err := h.pipelineRegistry.Register(existingPipeline); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to update pipeline: %v", err),
			"code":    "UPDATE_FAILED",
		})
		return
	}

	h.syncModesFromRegistry()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "node configuration updated successfully",
		"data":    node,
	})
}

// ListExecutionHistory 获取流水线执行历史
// GET /api/v1/pipelines/:id/executions
func (h *PipelineHandler) ListExecutionHistory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id is required"})
		return
	}

	if h.pipelineStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "pipeline store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	records, err := h.pipelineStore.GetExecutionHistory(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to get execution history: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
	})
}

// GetExecutionDetail 获取单条执行记录详情（含节点审计）
// GET /api/v1/pipelines/executions/:execId
func (h *PipelineHandler) GetExecutionDetail(c *gin.Context) {
	execIDStr := c.Param("execId")
	if execIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "execution id is required"})
		return
	}

	execID, err := strconv.ParseInt(execIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid execution id"})
		return
	}

	if h.pipelineStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "pipeline store not initialized",
			"code":    "STORE_NOT_INITIALIZED",
		})
		return
	}

	record, err := h.pipelineStore.GetExecution(execID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("failed to get execution detail: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    record,
	})
}

// GetAvailableVariables 获取流水线可用变量列表
// GET /api/v1/pipelines/:id/available-variables
func (h *PipelineHandler) GetAvailableVariables(c *gin.Context) {
	cfg := config.Get()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "config not initialized"})
		return
	}

	systemVars := config.ListSystemVariables(cfg)
	userVars := config.ListUserVariables(cfg)

	// Node variables are derived from the pipeline's node definitions
	// For now, return common node variable patterns
	nodeVars := []map[string]string{
		{"name": "{{node.generator.content}}", "description": "生成器节点输出"},
		{"name": "{{node.reviewer.content}}", "description": "审核器节点输出"},
		{"name": "{{node.reviewer.score}}", "description": "审核器评分"},
		{"name": "{{node.reviewer.passed}}", "description": "审核器是否通过"},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"system_variables": systemVars,
			"user_variables":   userVars,
			"node_variables":   nodeVars,
		},
	})
}

// transparentPreserveUpstreamStatus 读取 LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS
// 环境变量，控制 execute 入口是否按上游真实状态码返回（默认 false 保持"永远 200"兼容行为）。
func transparentPreserveUpstreamStatus() bool {
	if v := os.Getenv("LLM_PROXY_TRANSPARENT_EXECUTE_PRESERVE_STATUS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return false
}

// handleTransparentStatusError 处理透明转发状态码透传逻辑。
// 当开关开启时，按上游真实状态码返回错误响应；否则返回 true 表示应继续正常流程。
func handleTransparentStatusError(c *gin.Context, output *pipeline.PipelineOutput, execID string, input *pipeline.PipelineInput, h *PipelineHandler) bool {
	if output == nil || output.Metadata == nil {
		return true
	}

	// P1-T3：上游 2xx+错误体已被标记为 upstream_error，按映射状态返回规范错误体。
	if ue, ok := output.Metadata["upstream_error"].(bool); ok && ue {
		status := http.StatusBadGateway
		if sc, ok := output.Metadata["status_code"].(int); ok && sc >= 400 {
			status = sc
		} else if scf, ok := output.Metadata["status_code"].(float64); ok && scf >= 400 {
			status = int(scf)
		}
		h.recordExecution(execID, input, nil, nil)
		c.JSON(status, gin.H{"success": false, "error": "upstream returned an error payload", "data": output})
		return false
	}

	// P1-T10：透明转发状态码透传。
	if transparentPreserveUpstreamStatus() {
		if sc, ok := output.Metadata["status_code"].(int); ok && sc >= 400 {
			h.recordExecution(execID, input, nil, nil)
			c.JSON(sc, gin.H{"success": false, "error": "upstream error", "data": output})
			return false
		} else if scf, ok := output.Metadata["status_code"].(float64); ok && scf >= 400 {
			status := int(scf)
			h.recordExecution(execID, input, nil, nil)
			c.JSON(status, gin.H{"success": false, "error": "upstream error", "data": output})
			return false
		}
	}

	return true
}
