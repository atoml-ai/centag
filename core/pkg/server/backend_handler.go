package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"centag/core/internal/auth"
	"centag/core/internal/edition"
	"centag/core/pkg/backend"
	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/useraccess"

	"github.com/gin-gonic/gin"
)

// BackendHandler 后端配置处理器
type BackendHandler struct {
	backendManager *backend.Manager
	edition        edition.Edition
}

// NewBackendHandler 创建后端配置处理器
func NewBackendHandler(manager *backend.Manager) *BackendHandler {
	return &BackendHandler{
		backendManager: manager,
	}
}

// SetEdition configures product edition for Team resource access rules.
func (h *BackendHandler) SetEdition(ed edition.Edition) {
	if h != nil {
		h.edition = ed
	}
}

func (h *BackendHandler) accessUser(c *gin.Context) *database.User {
	return loadTeamNormalUser(c, h.edition)
}

// getTenantID 返回当前请求的资源作用域：
// - Team 普通用户 → ownTenantID（遗留 tenant_id 或合成 user:{id}）
// - 管理员 / Personal → ""（系统级）
func (h *BackendHandler) getTenantID(c *gin.Context) string {
	if user := h.accessUser(c); user != nil {
		return ownTenantID(user)
	}
	return ""
}

// ListBackends 列出后端配置（角色感知）
// - 管理员: 仅系统后端（TenantID 为空），不暴露用户私有后端
// - 普通用户: 自有后端 + 组模型 policy 允许且启用的系统后端
func (h *BackendHandler) ListBackends(c *gin.Context) {
	backends := h.backendManager.List()
	if user := h.accessUser(c); user != nil {
		backends = useraccess.FilterBackendsFor(user, backends, policyForUser(c.Request.Context(), user))
	} else if h.edition.IsTeam() && auth.IsAdmin(c) {
		backends = filterSystemBackends(backends)
	}

	// 转换为响应格式（包含 has_api_key 标记，但不暴露实际 api_key）
	responses := make([]*backend.BackendConfigResponse, 0, len(backends))
	breakerStates := circuitbreaker.GetAllStates() // P0-T2：列表暴露熔断状态
	for _, b := range backends {
		resp := b.ToResponse()
		if st, ok := breakerStates[b.ID]; ok {
			resp.CircuitState = string(st)
		}
		responses = append(responses, resp)
	}
	RespondSuccess(c, responses)
}

func filterSystemBackends(list []*backend.BackendConfig) []*backend.BackendConfig {
	out := make([]*backend.BackendConfig, 0, len(list))
	for _, b := range list {
		if b != nil && b.TenantID == "" {
			out = append(out, b)
		}
	}
	return out
}

// ExportBackends 导出所有后端配置
// 支持 ?desensitize=true 参数，导出时脱敏 api_key（仅返回 has_api_key 标记）
func (h *BackendHandler) ExportBackends(c *gin.Context) {
	desensitize := c.Query("desensitize") == "true"

	backends := h.backendManager.List()
	if user := h.accessUser(c); user != nil {
		backends = useraccess.FilterBackendsFor(user, backends, policyForUser(c.Request.Context(), user))
	} else if h.edition.IsTeam() && auth.IsAdmin(c) {
		backends = filterSystemBackends(backends)
	}

	// 脱敏处理
	if desensitize {
		masked := make([]*backend.BackendConfig, 0, len(backends))
		for _, b := range backends {
			m := *b
			m.APIKey = ""
			// 掩码账户池中的 api_key
			if m.AccountPool != nil {
				for i := range m.AccountPool.Accounts {
					m.AccountPool.Accounts[i].APIKey = ""
				}
			}
			masked = append(masked, &m)
		}
		RespondSuccess(c, masked)
		return
	}

	// 返回完整配置，包含 api_key
	RespondSuccess(c, backends)
}

// ImportBackends 批量导入后端（同名 id 已存在则跳过）
func (h *BackendHandler) ImportBackends(c *gin.Context) {
	if user := h.accessUser(c); user != nil && !user.CanAddOwnBackends {
		RespondError(c, http.StatusForbidden, "adding or modifying own backends is disabled for this user")
		return
	}
	var req struct {
		Backends []*backend.BackendConfig `json:"backends"`
	}
	if !BindJSON(c, &req) {
		return
	}
	if len(req.Backends) == 0 {
		RespondBadRequest(c, "backends is empty")
		return
	}

	own := ""
	if user := h.accessUser(c); user != nil {
		own = ownTenantID(user)
	}
	imported := 0
	skipped := 0
	for _, cfg := range req.Backends {
		if cfg == nil {
			continue
		}
		if cfg.ID == "" {
			cfg.ID = generateBackendID(cfg.Type, cfg.Name, cfg.BaseURL)
		}
		cfg.TenantID = own
		if _, err := h.backendManager.Get(cfg.ID); err == nil {
			skipped++
			continue
		}
		if err := h.backendManager.Add(cfg); err != nil {
			skipped++
			continue
		}
		imported++
	}
	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}
	RespondSuccess(c, gin.H{"imported": imported, "skipped": skipped})
}

// GetBackend 获取单个后端配置
func (h *BackendHandler) GetBackend(c *gin.Context) {
	id := c.Param("id")

	cfg, err := h.backendManager.Get(id)
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}
	if user := h.accessUser(c); user != nil {
		filtered := useraccess.FilterBackendsFor(user, []*backend.BackendConfig{cfg}, policyForUser(c.Request.Context(), user))
		if len(filtered) == 0 {
			RespondError(c, http.StatusForbidden, "backend not found or access denied")
			return
		}
		cfg = filtered[0]
	}

	RespondSuccess(c, cfg.ToResponse())
}

// CreateBackend 创建后端配置
func (h *BackendHandler) CreateBackend(c *gin.Context) {
	if user := h.accessUser(c); user != nil && !user.CanAddOwnBackends {
		RespondError(c, http.StatusForbidden, "adding or modifying own backends is disabled for this user")
		return
	}
	var cfg backend.BackendConfig
	if !BindJSON(c, &cfg) {
		return
	}

	// 自动生成ID；若客户端传入的 id 已存在则改用唯一 id（避免预设 provider id 与种子数据冲突）
	if cfg.ID == "" {
		cfg.ID = generateBackendID(cfg.Type, cfg.Name, cfg.BaseURL)
	}
	cfg.ID = ensureUniqueBackendID(h.backendManager, cfg.ID)

	// 新建后端默认启用
	cfg.Enabled = true

	// 组模型（036）：普通用户新建后端归属到自有遗留租户别名（ownTenantID），
	// 与 Update/Delete 的自有校验及 policy 过滤保持一致；
	// 管理员新建后端为系统后端（TenantID 为空）。
	cfg.TenantID = ""
	if user := h.accessUser(c); user != nil {
		cfg.TenantID = ownTenantID(user)
	}

	if err := h.backendManager.Add(&cfg); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}

	RespondCreated(c, cfg)
}

// UpdateBackend 更新后端配置（角色感知：普通用户仅可改自有后端）
func (h *BackendHandler) UpdateBackend(c *gin.Context) {
	id := c.Param("id")

	var cfg backend.BackendConfig
	if !BindJSON(c, &cfg) {
		return
	}

	cfg.ID = id // 确保ID匹配

	// 组模型（036）：普通用户仅可修改自有（遗留租户别名）后端，禁止改系统/他人后端
	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnBackends {
			RespondError(c, http.StatusForbidden, "adding or modifying own backends is disabled for this user")
			return
		}
		existing, err := h.backendManager.Get(id)
		if err != nil {
			RespondError(c, http.StatusForbidden, "backend not found or access denied: "+err.Error())
			return
		}
		if own := ownTenantID(user); own == "" || existing.TenantID != own {
			RespondError(c, http.StatusForbidden, "cannot modify system or another user's backend")
			return
		}
		cfg.TenantID = existing.TenantID
		// 保留原有的 API Key 如果请求中没有提供新的
		if cfg.APIKey == "" && existing.APIKey != "" {
			cfg.APIKey = existing.APIKey
		}
	}

	if err := h.backendManager.Update(&cfg); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}

	// 若该后端是系统默认，同步 proxy 默认模型（minimal: proxy-config.yaml）
	syncProxyDefaultModelFromBackend(id)

	RespondSuccess(c, cfg)
}

// DeleteBackend 删除后端配置（角色感知：普通用户仅可删自有后端）
func (h *BackendHandler) DeleteBackend(c *gin.Context) {
	id := c.Param("id")

	if user := h.accessUser(c); user != nil {
		if !user.CanAddOwnBackends {
			RespondError(c, http.StatusForbidden, "adding or modifying own backends is disabled for this user")
			return
		}
		existing, err := h.backendManager.Get(id)
		if err != nil {
			RespondNotFound(c, err.Error())
			return
		}
		if own := ownTenantID(user); own == "" || existing.TenantID != own {
			RespondError(c, http.StatusForbidden, "cannot delete system or another user's backend")
			return
		}
	}

	// 删除前记录该后端是否当前系统默认
	wasDefault := isCurrentDefaultBackend(id)

	err := h.backendManager.Delete(id)
	if err != nil {
		if strings.Contains(err.Error(), "cannot delete system backend") {
			RespondError(c, http.StatusForbidden, err.Error())
			return
		}
		RespondNotFound(c, err.Error())
		return
	}

	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}

	// 若删除的是默认后端，自动切换到下一个后端/模型；无剩余后端则置空。
	if wasDefault {
		resyncDefaultAfterBackendDelete()
	}

	RespondSuccessWithMessage(c, "Backend deleted successfully")
}

// SetDefaultBackend 设置默认后端（通过权重调整）
func (h *BackendHandler) SetDefaultBackend(c *gin.Context) {
	id := c.Param("id")

	if err := h.backendManager.SetDefault(id); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}

	RespondSuccessWithMessage(c, "Default backend updated successfully")
}

// TestConnection 测试后端连接（可选择是否更新健康状态和模型列表并保存）
func (h *BackendHandler) TestConnection(c *gin.Context) {
	var cfg backend.BackendConfig
	if !BindJSON(c, &cfg) {
		return
	}

	// 是否更新并保存状态（默认 true，与自动探测行为一致）
	updateAndSave := c.DefaultQuery("update_and_save", "true") == "true"

	// 若带 id，以已保存的后端为准合并字段，避免列表里未带出 api_key 等导致测试必失败
	if cfg.ID != "" {
		// 内存与 DB 偶发不同步时，先从 admin_backends 补全密钥再合并
		h.backendManager.RepairAPIKeyFromDBIfEmpty(c.Request.Context(), cfg.ID)
		// DB 仍为空时：从 config/initdata/initial-backends.json 读非空 api_key 并写入 DB（与首次保存效果一致，无需用户先点保存）
		if err := h.backendManager.ApplyAPIKeyFromInitialFileIfEmpty(cfg.ID); err != nil {
			logger.Warnf("从 initial-backends.json 补全 API Key 失败: %v", err)
		}

		tenantID := h.getTenantID(c)
		var stored *backend.BackendConfig
		var err error
		if tenantID != "" {
			stored, err = h.backendManager.GetByTenant(tenantID, cfg.ID)
		} else {
			stored, err = h.backendManager.Get(cfg.ID)
		}
		if err != nil {
			RespondBadRequest(c, err.Error())
			return
		}
		merged := *stored
		if cfg.BaseURL != "" {
			merged.BaseURL = cfg.BaseURL
		}
		if cfg.Type != "" {
			merged.Type = cfg.Type
		}
		if cfg.APIKey != "" {
			merged.APIKey = cfg.APIKey
		}
		if cfg.Timeout > 0 {
			merged.Timeout = cfg.Timeout
		}
		if strings.TrimSpace(cfg.ProbeModel) != "" {
			merged.ProbeModel = strings.TrimSpace(cfg.ProbeModel)
		}
		cfg = merged
	}
	cfg.APIKey = backend.NormalizeOpenAICompatibleAPIKey(cfg.APIKey)

	// 方式一：仅测试连接（向后兼容）
	if !updateAndSave {
		if err := h.backendManager.TestConnectionWithContext(c.Request.Context(), &cfg); err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondSuccessWithMessage(c, "Connection successful")
		return
	}

	// 方式二：探测并更新状态和模型列表（与自动探测一致）
	// 使用 context 带超时，确保探测不会永久阻塞
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	// 是否获取模型列表（默认 true）
	fetchModels := c.DefaultQuery("fetch_models", "true") == "true"

	// 先测试连接（此时不修改状态），确认后端可达后再探测和更新
	if err := h.backendManager.TestConnectionWithContext(ctx, &cfg); err != nil {
		logger.Warn("连接测试失败", logger.GetField("id", cfg.ID), logger.GetField("error", err.Error()))
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 连接可达，执行完整探测（更新健康状态、模型列表、启用状态）
	if _, err := h.backendManager.ProbeAndUpdateBackend(ctx, cfg.ID, fetchModels); err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 保存更新后的配置（包括健康状态、启用状态和模型列表）
	if err := h.backendManager.Save(); err != nil {
		logger.Error("测试连接后保存配置失败", logger.GetField("id", cfg.ID), logger.GetField("error", err.Error()))
		RespondError(c, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	// 返回更新后的完整后端配置（包含最新的 health_status），而非仅返回探测结果
	tenantID := h.getTenantID(c)
	var updated *backend.BackendConfig
	var getErr error
	if tenantID != "" {
		updated, getErr = h.backendManager.GetByTenant(tenantID, cfg.ID)
	} else {
		updated, getErr = h.backendManager.Get(cfg.ID)
	}
	if getErr != nil {
		RespondError(c, http.StatusInternalServerError, "获取更新后的配置失败: "+getErr.Error())
		return
	}

	RespondSuccess(c, updated.ToResponse())
}

// ProbeBackend 探测单个后端并更新状态（租户隔离）
func (h *BackendHandler) ProbeBackend(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.getTenantID(c)

	// 租户隔离：验证当前用户是否有权限访问该后端
	if tenantID != "" {
		if _, err := h.backendManager.GetByTenant(tenantID, id); err != nil {
			RespondError(c, http.StatusForbidden, "backend not found or access denied: "+err.Error())
			return
		}
	}

	fetchModels := c.DefaultQuery("fetch_models", "true") == "true"

	result, err := h.backendManager.ProbeAndUpdateBackend(c.Request.Context(), id, fetchModels)
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	// 保存更新后的配置（包括启用状态、健康状态和模型列表）
	if err := h.backendManager.Save(); err != nil {
		logger.Error("Failed to save backend config after probe", logger.GetField("id", id), logger.GetField("error", err.Error()))
	}

	RespondSuccess(c, result)
}

// ProbeAllBackends 批量探测所有启用的后端（租户隔离）
func (h *BackendHandler) ProbeAllBackends(c *gin.Context) {
	fetchModels := c.DefaultQuery("fetch_models", "true") == "true"
	tenantID := h.getTenantID(c)

	// 租户隔离：只探测当前租户可访问的后端
	var backends []*backend.BackendConfig
	if tenantID != "" {
		backends = h.backendManager.GetEnabledByTenant(tenantID)
	} else {
		backends = h.backendManager.GetEnabled()
	}

	resChan := make(chan backend.ProbeResult, len(backends))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	for _, cfg := range backends {
		go func(b *backend.BackendConfig) {
			r, err := h.backendManager.ProbeAndUpdateBackend(ctx, b.ID, fetchModels)
			if err != nil {
				resChan <- backend.ProbeResult{BackendID: b.ID, Success: false, Error: err.Error()}
				return
			}
			resChan <- *r
		}(cfg)
	}

	var results []backend.ProbeResult
	successCount := 0
	failCount := 0
	for i := 0; i < len(backends); i++ {
		select {
		case r := <-resChan:
			results = append(results, r)
			if r.Success {
				successCount++
			} else {
				failCount++
			}
		case <-ctx.Done():
			results = append(results, backend.ProbeResult{
				Success: false,
				Error:   "timeout",
			})
			failCount++
		}
	}

	// 保存更新后的配置
	if err := h.backendManager.Save(); err != nil {
		logger.Error("Failed to save backend configs after probe all", logger.GetField("error", err.Error()))
	}

	RespondSuccess(c, gin.H{
		"results":       results,
		"total":         len(results),
		"success_count": successCount,
		"fail_count":    failCount,
	})
}

// ProbeAllBackendsSSE 批量探测所有后端，使用 SSE 流式推送结果（租户隔离）
func (h *BackendHandler) ProbeAllBackendsSSE(c *gin.Context) {
	fetchModels := c.DefaultQuery("fetch_models", "true") == "true"
	tenantID := h.getTenantID(c)

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 缓冲

	// 租户隔离：只获取当前租户可访问的后端
	var backends []*backend.BackendConfig
	if tenantID != "" {
		backends = h.backendManager.GetEnabledByTenant(tenantID)
	} else {
		backends = append(h.backendManager.GetEnabled(), h.backendManager.List()...)
		// 去重
		seen := make(map[string]bool)
		unique := backends[:0]
		for _, b := range backends {
			if !seen[b.ID] {
				seen[b.ID] = true
				unique = append(unique, b)
			}
		}
		backends = unique
	}

	// 发送总数
	c.SSEvent("total", len(backends))
	c.Writer.Flush()

	successCount := 0
	failCount := 0

	for i, cfg := range backends {
		// 创建独立的 context，带单个后端的超时
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		result, err := h.backendManager.ProbeAndUpdateBackend(ctx, cfg.ID, fetchModels)
		cancel()

		if err != nil {
			failCount++
			c.SSEvent("result", gin.H{
				"index":    i,
				"id":       cfg.ID,
				"success":  false,
				"error":    err.Error(),
				"finished": false,
			})
		} else {
			if result.Success {
				successCount++
			} else {
				failCount++
			}
			c.SSEvent("result", gin.H{
				"index":    i,
				"id":       cfg.ID,
				"success":  result.Success,
				"finished": false,
			})
		}
		c.Writer.Flush()
	}

	// 发送完成信号
	c.SSEvent("done", gin.H{
		"total":         len(backends),
		"success_count": successCount,
		"fail_count":    failCount,
	})
	c.Writer.Flush()

	// 保存更新后的配置（包括健康状态、启用状态和模型列表）
	if err := h.backendManager.Save(); err != nil {
		logger.Error("Failed to save backend configs after SSE probe", logger.GetField("error", err.Error()))
	}
}

// GetModels 获取后端的模型列表（租户隔离）
func (h *BackendHandler) GetModels(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.getTenantID(c)

	// 获取过滤参数
	modelType := c.DefaultQuery("type", "all") // all, chat, embedding

	var cfg *backend.BackendConfig
	var err error
	if tenantID != "" {
		cfg, err = h.backendManager.GetByTenant(tenantID, id)
	} else {
		cfg, err = h.backendManager.Get(id)
	}
	if err != nil {
		logger.Warn("GetModels: backend not found", logger.GetField("id", id), logger.GetField("error", err.Error()))
		RespondNotFound(c, err.Error())
		return
	}
	if user := h.accessUser(c); user != nil {
		filtered := useraccess.FilterBackendsFor(user, []*backend.BackendConfig{cfg}, policyForUser(c.Request.Context(), user))
		if len(filtered) == 0 {
			RespondError(c, http.StatusForbidden, "backend not found or access denied")
			return
		}
		cfg = filtered[0]
	}

	logger.Info("GetModels: fetching models for backend",
		logger.GetField("id", cfg.ID),
		logger.GetField("name", cfg.Name),
		logger.GetField("type", cfg.Type),
		logger.GetField("auto_fetch_models", cfg.AutoFetchModels),
		logger.GetField("supported_models_count", len(cfg.SupportedModels)))

	models, err := h.backendManager.GetModelsWithContext(c.Request.Context(), cfg)
	if err != nil {
		logger.Error("GetModels: failed to get models",
			logger.GetField("id", id),
			logger.GetField("error", err.Error()))
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	logger.Info("GetModels: successfully retrieved models",
		logger.GetField("id", id),
		logger.GetField("count", len(models)))

	// 根据类型过滤模型
	filteredModels := filterModelsByType(models, modelType)
	if user := h.accessUser(c); user != nil && cfg.TenantID == "" {
		pol := policyForUser(c.Request.Context(), user)
		kept := make([]string, 0, len(filteredModels))
		for _, m := range filteredModels {
			if pol != nil && pol.HasPlan {
				if pol.IsAllowedModel(m) {
					kept = append(kept, m)
				}
			} else if useraccess.CanUseSharedModel(user, m) {
				kept = append(kept, m)
			}
		}
		filteredModels = kept
	}

	RespondSuccess(c, filteredModels)
}

// FetchModels 用前端提交的配置直接探测远端模型列表
// 用于编辑对话框中「刷新支持的模型」按钮（新建/编辑、各发行版共用）。
// 若提供了 backend_id 且后端已存在：编辑态可复用已存 API Key；探测成功后立即更新 supported_models 并持久化。
func (h *BackendHandler) FetchModels(c *gin.Context) {
	var req struct {
		BaseURL   string `json:"base_url" binding:"required"`
		APIKey    string `json:"api_key"`
		Type      string `json:"type"`    // openai | ollama | anthropic | gemini
		Timeout   int    `json:"timeout"` // seconds
		BackendID string `json:"backend_id,omitempty"`
		Replace   *bool  `json:"replace,omitempty"` // 默认 true：以远端列表覆盖
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "缺少必要参数："+err.Error())
		return
	}
	replace := true
	if req.Replace != nil {
		replace = *req.Replace
	}

	apiKey := backend.NormalizeOpenAICompatibleAPIKey(req.APIKey)
	baseURL := strings.TrimSpace(req.BaseURL)
	backendType := strings.TrimSpace(req.Type)
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30
	}

	// 编辑态：与 TestConnection 一致，未提交 Key 时复用已存密钥，并允许补全 URL/Type
	if req.BackendID != "" {
		h.backendManager.RepairAPIKeyFromDBIfEmpty(c.Request.Context(), req.BackendID)
		_ = h.backendManager.ApplyAPIKeyFromInitialFileIfEmpty(req.BackendID)

		tenantID := h.getTenantID(c)
		var stored *backend.BackendConfig
		var getErr error
		if tenantID != "" {
			stored, getErr = h.backendManager.GetByTenant(tenantID, req.BackendID)
		} else {
			stored, getErr = h.backendManager.Get(req.BackendID)
		}
		if getErr == nil && stored != nil {
			if apiKey == "" && stored.APIKey != "" {
				apiKey = backend.NormalizeOpenAICompatibleAPIKey(stored.APIKey)
			}
			if baseURL == "" {
				baseURL = stored.BaseURL
			}
			if backendType == "" && stored.Type != "" {
				backendType = stored.Type
			}
			if timeout == 30 && stored.Timeout > 0 {
				timeout = stored.Timeout
			}
		}
	}
	if backendType == "" {
		backendType = "openai"
	}

	if backendType != "ollama" && apiKey == "" {
		RespondError(c, http.StatusBadRequest, "未配置 API Key，无法拉取模型列表")
		return
	}

	tempCfg := &backend.BackendConfig{
		Name:            "__fetch_probe__",
		Type:            backendType,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		Timeout:         timeout,
		AutoFetchModels: true,
	}

	// 强制走远端，忽略本地 supported_models 缓存
	models, err := h.backendManager.FetchModelsFromRemote(c.Request.Context(), tempCfg)
	if err != nil {
		RespondError(c, http.StatusBadGateway, "探测失败："+err.Error())
		return
	}

	if req.BackendID != "" {
		tenantID := h.getTenantID(c)
		existing, getErr := h.backendManager.Get(req.BackendID)
		if getErr == nil && existing != nil {
			if tenantID == "" || existing.TenantID == tenantID || existing.TenantID == "" {
				if replace {
					h.backendManager.UpdateSupportedModels(req.BackendID, models)
				} else {
					merged := mergeModelNameLists(getSupportedModelNameStrings(existing), models)
					h.backendManager.UpdateSupportedModels(req.BackendID, merged)
				}
				if saveErr := h.backendManager.Save(); saveErr != nil {
					logger.Warn("FetchModels: 模型列表已获取但持久化失败", logger.GetField("backend_id", req.BackendID), logger.GetField("error", saveErr.Error()))
				} else {
					logger.Info("FetchModels: 模型列表已获取并自动持久化", logger.GetField("backend_id", req.BackendID), logger.GetField("count", len(models)))
				}
			}
		}
	}

	RespondSuccess(c, models)
}

func getSupportedModelNameStrings(cfg *backend.BackendConfig) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.SupportedModels))
	for _, sm := range cfg.SupportedModels {
		name := sm.ActualModel
		if name == "" {
			name = sm.RequestedModel
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func mergeModelNameLists(existing, remote []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(remote))
	out := make([]string, 0, len(existing)+len(remote))
	for _, name := range existing {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, name := range remote {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// filterModelsByType 根据类型过滤模型
func filterModelsByType(models []string, modelType string) []string {
	if modelType == "all" {
		return models
	}

	filtered := make([]string, 0)
	for _, model := range models {
		if modelType == "embedding" && isEmbeddingModel(model) {
			filtered = append(filtered, model)
		} else if modelType == "chat" && !isEmbeddingModel(model) {
			filtered = append(filtered, model)
		}
	}

	return filtered
}

// isEmbeddingModel 判断是否是向量化模型
func isEmbeddingModel(modelName string) bool {
	modelLower := strings.ToLower(modelName)

	// 常见的向量化模型关键词
	embeddingKeywords := []string{
		"embedding",
		"embed",
		"bge",         // BAAI general embedding
		"gte",         // General Text Embeddings
		"e5",          // Microsoft E5
		"sentence",    // sentence transformers
		"nomic-embed", // nomic embedding
		"mxbai-embed", // mixedbread.ai embedding
		"all-minilm",  // all-MiniLM sentence transformer
	}

	for _, keyword := range embeddingKeywords {
		if strings.Contains(modelLower, keyword) {
			return true
		}
	}

	return false
}

// generateBackendID 生成可读且有辨识度的后端 ID。
// 规则（按优先级）：
//  1. 名称 slug 有辨识度（非空、且不等于 type/custom/backend）→ 直接用名称 slug
//  2. 否则用 Base URL 主机名 slug（如 api.deepseek.com → api-deepseek-com）
//  3. 再不行 → {type}-{4位短码}
// 不再使用「type-name」硬拼接，避免选 OpenAI 预设变成 openai-openai。
func generateBackendID(backendType, name, baseURL string) string {
	typePart := strings.ToLower(strings.TrimSpace(backendType))
	if typePart == "" {
		typePart = "backend"
	}
	nameSlug := slugifyBackendIDPart(name)
	hostSlug := slugifyBackendHost(baseURL)

	genericNames := map[string]bool{
		"": true, typePart: true, "backend": true, "custom": true, "provider": true,
	}
	if !genericNames[nameSlug] {
		return nameSlug
	}
	if hostSlug != "" && hostSlug != typePart {
		return hostSlug
	}
	return typePart + "-" + shortBackendIDToken()
}

func slugifyBackendIDPart(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '_' || r == '-' || r == '.' || r == '(' || r == ')':
			return '-'
		default:
			return -1
		}
	}, slug)
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}

func slugifyBackendHost(baseURL string) string {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}
	// localhost / 纯 IP 时带上端口，避免都变成 localhost
	if (host == "localhost" || net.ParseIP(host) != nil) && u.Port() != "" {
		host = host + "-" + u.Port()
	}
	return slugifyBackendIDPart(host)
}

func shortBackendIDToken() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	n := time.Now().UnixNano()
	buf := make([]byte, 4)
	for i := 0; i < 4; i++ {
		buf[i] = alphabet[n%int64(len(alphabet))]
		n /= int64(len(alphabet))
	}
	return string(buf)
}

// ensureUniqueBackendID 若 id 已被占用则追加 -2/-3...，保证创建成功。
func ensureUniqueBackendID(mgr *backend.Manager, id string) string {
	if id == "" {
		id = fmt.Sprintf("backend-%d", time.Now().Unix()%100000)
	}
	if _, err := mgr.Get(id); err != nil {
		return id
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", id, i)
		if _, err := mgr.Get(candidate); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", id, time.Now().UnixNano()%100000)
}

// GetCircuitBreakerStatus 获取所有后端的熔断器状态
func (h *BackendHandler) GetCircuitBreakerStatus(c *gin.Context) {
	states := circuitbreaker.GetAllStates()

	type breakerInfo struct {
		State  string `json:"state"`
		IsOpen bool   `json:"is_open"`
	}

	result := make(map[string]breakerInfo, len(states))
	for id, state := range states {
		result[id] = breakerInfo{
			State:  string(state),
			IsOpen: state == "open",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"circuit_breakers": result,
		"count":            len(result),
	})
}

// ResetCircuitBreaker 重置指定后端的熔断器
func (h *BackendHandler) ResetCircuitBreaker(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backend id is required"})
		return
	}

	circuitbreaker.Reset(id)
	logger.Infof("[CircuitBreaker] Reset breaker for backend: %s", id)

	c.JSON(http.StatusOK, gin.H{
		"message":    "circuit breaker reset",
		"backend_id": id,
	})
}

// ── v2.1: Backend Health Check ──────────────────────────────────────────────

// GetBackendHealth 获取后端健康状态
func (h *BackendHandler) GetBackendHealth(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.getTenantID(c)

	var cfg *backend.BackendConfig
	var err error
	if tenantID != "" {
		cfg, err = h.backendManager.GetByTenant(tenantID, id)
	} else {
		cfg, err = h.backendManager.Get(id)
	}
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	if cfg.HealthStatus == nil {
		RespondSuccess(c, gin.H{
			"backend_id": id,
			"status":     "unknown",
		})
		return
	}

	RespondSuccess(c, gin.H{
		"backend_id":    id,
		"status":        cfg.HealthStatus.Status,
		"last_check_at": cfg.HealthStatus.LastCheckAt,
		"last_error":    cfg.HealthStatus.LastError,
		"response_time": cfg.HealthStatus.ResponseTime,
		"models_count":  cfg.HealthStatus.ModelsCount,
	})
}

// ── v2.1: Priority/Weight Management ────────────────────────────────────────

type updatePriorityWeightRequest struct {
	Priority *int `json:"priority"`
	Weight   *int `json:"weight"`
}

// UpdateBackendPriorityWeight 更新后端优先级和权重
func (h *BackendHandler) UpdateBackendPriorityWeight(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.getTenantID(c)

	var req updatePriorityWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	var cfg *backend.BackendConfig
	var err error
	if tenantID != "" {
		cfg, err = h.backendManager.GetByTenant(tenantID, id)
	} else {
		cfg, err = h.backendManager.Get(id)
	}
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	if req.Priority != nil {
		cfg.Priority = *req.Priority
	}
	if req.Weight != nil {
		cfg.Weight = *req.Weight
	}

	if err := h.backendManager.Save(); err != nil {
		logger.Errorf("update backend priority/weight %s: %v", id, err)
		RespondInternalError(c, "failed to update backend")
		return
	}

	RespondSuccess(c, cfg.ToResponse())
}

// ── Account Pool CRUD ──────────────────────────────────────────────────────

// ListBackendAccounts 列出后端账户池中的所有账户
func (h *BackendHandler) ListBackendAccounts(c *gin.Context) {
	backendID := c.Param("id")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	if cfg.AccountPool == nil || len(cfg.AccountPool.Accounts) == 0 {
		// 无真实账户池时返回空列表（勿合成假 account id）。
		// 前端凭 has_api_key 展示本地占位 Key；若此处合成 key-1，保存会误走 PUT → 404 account pool not found。
		RespondSuccess(c, gin.H{
			"accounts": []backend.BackendAccount{},
			"strategy": "round_robin",
		})
		return
	}

	// 掩码 api_key，并显式返回 has_api_key（避免前端把空密钥账户当成「已配置」）
	accounts := make([]gin.H, 0, len(cfg.AccountPool.Accounts))
	for _, acc := range cfg.AccountPool.Accounts {
		accounts = append(accounts, gin.H{
			"id":          acc.ID,
			"label":       acc.Label,
			"enabled":     acc.Enabled,
			"weight":      acc.Weight,
			"created_at":  acc.CreatedAt,
			"has_api_key": strings.TrimSpace(acc.APIKey) != "",
		})
	}

	RespondSuccess(c, gin.H{
		"accounts": accounts,
		"strategy": cfg.AccountPool.Strategy,
	})
}

// GetBackendAccount 获取单个账户信息
func (h *BackendHandler) GetBackendAccount(c *gin.Context) {
	backendID := c.Param("id")
	accountID := c.Param("accountId")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	if cfg.AccountPool == nil {
		RespondNotFound(c, "account pool not found")
		return
	}

	acc, err := backend.GetAccountByID(cfg.AccountPool, accountID)
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	// 掩码 api_key
	masked := *acc
	masked.APIKey = ""

	RespondSuccess(c, masked)
}

// CreateBackendAccount 添加账户到后端账户池
func (h *BackendHandler) CreateBackendAccount(c *gin.Context) {
	backendID := c.Param("id")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	var acc backend.BackendAccount
	if !BindJSON(c, &acc) {
		return
	}

	// 初始化账户池（如果不存在）
	if cfg.AccountPool == nil {
		cfg.AccountPool = &backend.AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []backend.BackendAccount{},
		}
	}

	// 添加账户
	if err := backend.AddAccount(cfg.AccountPool, acc); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	// 保存
	if err := h.backendManager.Save(); err != nil {
		logger.Errorf("create account for backend %s: %v", backendID, err)
		RespondInternalError(c, "failed to save account")
		return
	}

	// 返回掩码后的账户
	masked := acc
	masked.APIKey = ""
	RespondSuccess(c, masked)
}

// UpdateBackendAccount 更新后端账户池中的账户
func (h *BackendHandler) UpdateBackendAccount(c *gin.Context) {
	backendID := c.Param("id")
	accountID := c.Param("accountId")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	var acc backend.BackendAccount
	if !BindJSON(c, &acc) {
		return
	}
	acc.ID = accountID

	// 无池或账户不存在时按 upsert：先建池/追加，避免前端误 PUT 得到 404
	if cfg.AccountPool == nil {
		cfg.AccountPool = &backend.AccountPoolConfig{
			Strategy: "round_robin",
			Accounts: []backend.BackendAccount{},
		}
	}
	if err := backend.UpdateAccount(cfg.AccountPool, acc); err != nil {
		if addErr := backend.AddAccount(cfg.AccountPool, acc); addErr != nil {
			RespondBadRequest(c, addErr.Error())
			return
		}
	}

	if err := h.backendManager.Save(); err != nil {
		logger.Errorf("update account for backend %s: %v", backendID, err)
		RespondInternalError(c, "failed to save account")
		return
	}

	masked := acc
	masked.APIKey = ""
	RespondSuccess(c, masked)
}

// DeleteBackendAccount 从后端账户池中删除账户
func (h *BackendHandler) DeleteBackendAccount(c *gin.Context) {
	backendID := c.Param("id")
	accountID := c.Param("accountId")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	if cfg.AccountPool == nil {
		RespondNotFound(c, "account pool not found")
		return
	}

	// 删除账户
	if err := backend.RemoveAccount(cfg.AccountPool, accountID); err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	// 保存
	if err := h.backendManager.Save(); err != nil {
		logger.Errorf("delete account for backend %s: %v", backendID, err)
		RespondInternalError(c, "failed to save account")
		return
	}

	RespondSuccess(c, gin.H{"deleted": true})
}

// ResetAccountBreaker 重置账户级熔断器状态
func (h *BackendHandler) ResetAccountBreaker(c *gin.Context) {
	backendID := c.Param("id")
	accountID := c.Param("accountId")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	if cfg.AccountPool == nil {
		RespondNotFound(c, "account pool not found")
		return
	}

	// 验证账户存在
	_, err = backend.GetAccountByID(cfg.AccountPool, accountID)
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	// 重置账户级熔断器（通过 circuitbreaker 包）
	// 注意：这里只是返回成功，实际熔断器状态由 scheduler 包管理
	// 前端可以调用此 API 来通知后端重置熔断器状态
	logger.Infof("reset breaker for backend %s account %s", backendID, accountID)
	RespondSuccess(c, gin.H{"reset": true})
}

// UpdateAccountPool 更新账户池元数据（目前支持 strategy；不覆盖账户列表）。
func (h *BackendHandler) UpdateAccountPool(c *gin.Context) {
	backendID := c.Param("id")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}
	if cfg.AccountPool == nil || len(cfg.AccountPool.Accounts) == 0 {
		RespondBadRequest(c, "account pool is empty; add accounts first")
		return
	}

	var req struct {
		Strategy string `json:"strategy"`
	}
	if !BindJSON(c, &req) {
		return
	}
	strategy := strings.TrimSpace(req.Strategy)
	if strategy == "" {
		RespondBadRequest(c, "strategy is required")
		return
	}
	cfg.AccountPool.Strategy = strategy
	backend.NormalizeAccountPool(cfg.AccountPool)
	if err := backend.ValidateAccountPool(cfg.AccountPool); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if err := h.backendManager.Update(cfg); err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if err := h.backendManager.Save(); err != nil {
		RespondInternalError(c, "Failed to save config: "+err.Error())
		return
	}
	RespondSuccess(c, gin.H{
		"strategy": cfg.AccountPool.Strategy,
		"accounts": len(cfg.AccountPool.Accounts),
	})
}

// GetAccountPoolStats 获取账户池统计信息
func (h *BackendHandler) GetAccountPoolStats(c *gin.Context) {
	backendID := c.Param("id")
	tenantID := h.getTenantID(c)

	cfg, err := h.getBackendConfig(c, tenantID, backendID)
	if err != nil {
		return
	}

	if cfg.AccountPool == nil {
		RespondNotFound(c, "account pool not found")
		return
	}

	stats := gin.H{
		"total_accounts":  len(cfg.AccountPool.Accounts),
		"enabled_accounts": 0,
		"strategy":         cfg.AccountPool.Strategy,
	}

	for _, acc := range cfg.AccountPool.Accounts {
		if acc.Enabled {
			stats["enabled_accounts"] = stats["enabled_accounts"].(int) + 1
		}
	}

	RespondSuccess(c, stats)
}

// getBackendConfig 获取后端配置的通用方法
func (h *BackendHandler) getBackendConfig(c *gin.Context, tenantID, backendID string) (*backend.BackendConfig, error) {
	var cfg *backend.BackendConfig
	var err error

	if tenantID != "" {
		cfg, err = h.backendManager.GetByTenant(tenantID, backendID)
	} else {
		cfg, err = h.backendManager.Get(backendID)
	}

	if err != nil {
		RespondNotFound(c, err.Error())
		return nil, err
	}

	// 访问控制（组模型 policy 感知）
	if user := h.accessUser(c); user != nil {
		filtered := useraccess.FilterBackendsFor(user, []*backend.BackendConfig{cfg}, policyForUser(c.Request.Context(), user))
		if len(filtered) == 0 {
			RespondError(c, http.StatusForbidden, "backend not found or access denied")
			return nil, fmt.Errorf("access denied")
		}
	}

	return cfg, nil
}

// maskAPIKey 掩码 API Key（仅显示前4位和后4位）
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}
