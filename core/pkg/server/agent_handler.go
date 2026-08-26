package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"centag/core/internal/agent"
	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"

	"github.com/gin-gonic/gin"
)

// AgentHandler Agent 配置快速接入处理器
type AgentHandler struct {
	registry   *agent.TemplateRegistry
	backendMgr *backend.Manager
}

var errNoUsableProxyAPIKey = errors.New("no usable proxy api key")

// NewAgentHandler 创建 Agent 处理器
func NewAgentHandler(registry *agent.TemplateRegistry, backendMgr *backend.Manager) *AgentHandler {
	return &AgentHandler{
		registry:   registry,
		backendMgr: backendMgr,
	}
}

// resolveModelName 构建最终模型名：
// - 有流水线时固定使用 centag/<id>（具体模型由流水线内部节点决定）
// - 无流水线时使用显式模型或后端首个支持模型
func resolveModelName(model string, pipelineID string, supportedModels []backend.ModelMapping) string {
	if pipelineID != "" {
		return "centag/" + pipelineID
	}
	if model == "" && len(supportedModels) > 0 {
		model = supportedModels[0].ActualModel
	}
	return model
}

// ListAgentTypes 列出所有支持的 Agent 工具类型（含配置方法与安装指引）
func (h *AgentHandler) ListAgentTypes(c *gin.Context) {
	type agentInfo struct {
		Type           string               `json:"type"`
		DisplayName    string               `json:"display_name"`
		Description    string               `json:"description"`
		Category       agent.AgentCategory  `json:"category"`
		Vendor         string               `json:"vendor"`
		WriteMode      string               `json:"write_mode"`
		ConfigPaths    []string             `json:"config_paths"`
		KeyFields      []string             `json:"key_fields"`
		ConfigMethod   string               `json:"config_method"`
		InstallURL     string               `json:"install_url"`
		InstallHint    string               `json:"install_hint"`
		AccessMethods  []agent.AccessMethod `json:"access_methods,omitempty"`
		CompanionCLI   *agent.CompanionCLI  `json:"companion_cli,omitempty"`
		UIGuide        *agent.UIGuide       `json:"ui_guide,omitempty"`
		VerifiedWrite  bool                 `json:"verified_write"`
		VerifiedWrap   bool                 `json:"verified_wrap"`
		VerifiedUI     bool                 `json:"verified_ui"`
		Verified       bool                 `json:"verified"` // 任一方式已验证（兼容/排序）
		WrapCommand    string               `json:"wrap_command,omitempty"`
		GuideOnly      bool                 `json:"guide_only,omitempty"`
	}
	var list []agentInfo
	for _, at := range h.registry.List() {
		t, ok := h.registry.Get(at)
		if !ok {
			continue
		}
		meta := t.Meta().Normalize()
		info := agentInfo{
			Type:          string(at),
			DisplayName:   t.DisplayName(),
			Description:   t.Description(),
			Category:      meta.Category,
			Vendor:        meta.Vendor,
			WriteMode:     meta.WriteMode,
			ConfigPaths:   meta.ConfigPaths,
			KeyFields:     meta.KeyFields,
			ConfigMethod:  meta.ConfigMethod,
			InstallURL:    meta.InstallURL,
			InstallHint:   meta.InstallHint,
			AccessMethods: meta.AccessMethods,
			CompanionCLI:  meta.CompanionCLI,
			UIGuide:       meta.UIGuide,
			VerifiedWrite: meta.VerifiedWrite,
			VerifiedWrap:  meta.VerifiedWrap,
			VerifiedUI:    meta.VerifiedUI,
			Verified:      meta.AnyVerified(),
			GuideOnly:     meta.GuideOnly(),
		}
		if meta.HasAccess(agent.AccessWrapCLI) {
			if argv := meta.WrapArgv(); len(argv) > 0 {
				if cmd, err := buildWrapRunUserCommand("", "", argv); err == nil {
					info.WrapCommand = cmd
				}
			}
		}
		list = append(list, info)
	}
	// 已验证方式更多的靠前，其次写配置已验证，再按 type
	sort.Slice(list, func(i, j int) bool {
		score := func(a agentInfo) int {
			n := 0
			if a.VerifiedWrite {
				n += 2
			}
			if a.VerifiedUI {
				n += 2
			}
			if a.VerifiedWrap {
				n++
			}
			return n
		}
		if si, sj := score(list[i]), score(list[j]); si != sj {
			return si > sj
		}
		return list[i].Type < list[j].Type
	})
	c.JSON(http.StatusOK, gin.H{"agent_types": list})
}

// GenerateConfig 生成 Agent 配置（团队版/桌面版通用）
func (h *AgentHandler) GenerateConfig(c *gin.Context) {
	var req agent.GenerateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentType := agent.AgentType(req.AgentType)
	tmpl, ok := h.registry.Get(agentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "unsupported agent type: " + req.AgentType + ". supported: " + strings.Join(supportedTypes(h.registry), ", "),
		})
		return
	}

	info, routeName, err := h.buildAgentBackendInfo(c, req.BackendID, req.PipelineID, req.Model, req.Host, req.Port)
	if err != nil {
		if errors.Is(err, errNoUsableProxyAPIKey) {
			h.respondProxyAPIKeyError(c, err)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := agent.GenerateConfigResponse{
		AgentType:   req.AgentType,
		BackendName: routeName,
		Description: tmpl.Description(),
		Commands:    tmpl.PlatformCommands(info),
		Files:       files,
		Steps:       tmpl.Steps(info),
		VerifyCmd:   tmpl.VerifyCommand(info),
	}

	c.JSON(http.StatusOK, resp)
}

// WriteConfig 桌面版一键写入配置到本地文件
func (h *AgentHandler) WriteConfig(c *gin.Context) {
	var req agent.WriteConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentType := agent.AgentType(req.AgentType)
	tmpl, ok := h.registry.Get(agentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported agent type: " + req.AgentType})
		return
	}

	info, _, err := h.buildAgentBackendInfo(c, req.BackendID, req.PipelineID, req.Model, req.Host, req.Port)
	if err != nil {
		if errors.Is(err, errNoUsableProxyAPIKey) {
			h.respondProxyAPIKeyError(c, err)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	files, err := tmpl.ConfigFiles(info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tmpl.WriteConfig(info); err != nil {
		c.JSON(http.StatusInternalServerError, agent.WriteConfigResponse{
			AgentType: req.AgentType,
			Success:   false,
			Message:   err.Error(),
		})
		return
	}

	meta := tmpl.Meta().Normalize()
	msg := "配置已写入本地文件"
	restart := meta.Category == agent.AgentCategoryDesktop && meta.HasAccess(agent.AccessWriteConfig)
	guideExported := meta.GuideOnly()
	if guideExported {
		msg = "已导出接入说明（未改写代理配置）。请按 UI 指引在客户端内添加模型。"
		if meta.UIGuide != nil && meta.UIGuide.ExportHint != "" {
			msg = meta.UIGuide.ExportHint + " 已写入本地说明文件。"
		}
		if agentType == agent.AgentTrae {
			msg = "已导出 TRAE 接入说明。请在 IDE「设置 → 模型 → 添加模型 → 自定义配置」按说明填写（完整 URL …/v1/chat/completions），然后完全退出并重启 TRAE。"
		}
		restart = true
	} else if restart {
		msg = "配置已写入本地文件。请完全退出并重启客户端后生效。"
	}

	c.JSON(http.StatusOK, agent.WriteConfigResponse{
		AgentType:       req.AgentType,
		Success:         true,
		Written:         files,
		Message:         msg,
		RestartRequired: restart,
		GuideExported:   guideExported,
	})
}

// RestoreConfig 恢复 Agent 本地配置为接入 Centag 之前的状态
func (h *AgentHandler) RestoreConfig(c *gin.Context) {
	var req agent.RestoreConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentType := agent.AgentType(req.AgentType)
	tmpl, ok := h.registry.Get(agentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported agent type: " + req.AgentType})
		return
	}

	// 仅需路径；用占位 BackendInfo 生成 ConfigFiles 列表
	files, err := tmpl.ConfigFiles(&agent.BackendInfo{
		Name:  "restore",
		Model: "centag/transparent",
		Host:  "localhost",
		Port:  20060,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results, err := agent.RestoreConfigFiles(files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, agent.RestoreConfigResponse{
			AgentType: req.AgentType,
			Success:   false,
			Results:   results,
			Message:   err.Error(),
		})
		return
	}

	changed := 0
	for _, r := range results {
		if r.Action == "restored" || r.Action == "removed" {
			changed++
		}
	}
	msg := "未找到可恢复的配置（可能尚未写入过 Centag 配置）"
	if changed > 0 {
		msg = fmt.Sprintf("已恢复 %d 个配置文件", changed)
	}

	c.JSON(http.StatusOK, agent.RestoreConfigResponse{
		AgentType: req.AgentType,
		Success:   true,
		Results:   results,
		Message:   msg,
	})
}

// GetConfigPreview 获取配置预览（无需后端 ID）
func (h *AgentHandler) GetConfigPreview(c *gin.Context) {
	agentType := agent.AgentType(c.Query("agent_type"))
	baseURL := c.Query("base_url")
	apiKey := c.Query("api_key")
	model := c.Query("model")
	pipelineID := c.Query("pipeline_id")

	tmpl, ok := h.registry.Get(agentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported agent type: " + string(agentType)})
		return
	}

	model = resolveModelName(model, pipelineID, nil)
	if model == "" {
		model = "gpt-4o"
	}

	info := &agent.BackendInfo{
		Name:  "custom",
		Model: model,
		Host:  "localhost",
		Port:  20060,
	}
	if baseURL != "" {
		info.BaseURL = baseURL
	}
	if apiKey != "" {
		info.APIKey = apiKey
	}

	files, _ := tmpl.ConfigFiles(info)
	resp := agent.GenerateConfigResponse{
		AgentType:   string(agentType),
		BackendName: info.Name,
		Description: tmpl.Description(),
		Commands:    tmpl.PlatformCommands(info),
		Files:       files,
		Steps:       tmpl.Steps(info),
		VerifyCmd:   tmpl.VerifyCommand(info),
	}

	c.JSON(http.StatusOK, resp)
}

// GenerateScriptRequest 一键脚本生成请求
type GenerateScriptRequest struct {
	AgentType  string `json:"agent_type" binding:"required"`
	BackendID  string `json:"backend_id,omitempty"`
	Model      string `json:"model,omitempty"`
	PipelineID string `json:"pipeline_id,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
}

// GenerateScript 生成一键配置脚本（Shell / PowerShell）
func (h *AgentHandler) GenerateScript(c *gin.Context) {
	var req GenerateScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentType := agent.AgentType(req.AgentType)
	tmpl, ok := h.registry.Get(agentType)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported agent type: " + req.AgentType})
		return
	}

	info, routeName, err := h.buildAgentBackendInfo(c, req.BackendID, req.PipelineID, req.Model, req.Host, req.Port)
	if err != nil {
		if errors.Is(err, errNoUsableProxyAPIKey) {
			h.respondProxyAPIKeyError(c, err)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	commands := tmpl.PlatformCommands(info)
	steps := tmpl.Steps(info)
	verifyCmd := tmpl.VerifyCommand(info)

	// 生成 macOS/Linux shell 脚本
	shellScript := "#!/bin/bash\n"
	shellScript += "# Centag Agent 一键配置脚本\n"
	shellScript += "# Agent: " + string(agentType) + "\n"
	shellScript += "# Route: " + routeName + "\n"
	shellScript += "# Generated by Centag\n\n"
	shellScript += "set -e\n\n"

	shellScript += "echo \"=== Centag Agent 配置 ===\"\n"
	shellScript += "echo \"Agent: " + string(agentType) + "\"\n"
	shellScript += "echo \"Route: " + routeName + "\"\n"
	shellScript += "echo \"\"\n\n"

	// macOS/Linux 命令
	if commands.MacOS != "" {
		shellScript += "echo \"[1/3] 配置环境变量...\"\n"
		shellScript += commands.MacOS + "\n\n"
	} else if commands.Linux != "" {
		shellScript += "echo \"[1/3] 配置环境变量...\"\n"
		shellScript += commands.Linux + "\n\n"
	}

	// 配置文件内容（写入文件的命令）
	if len(steps) > 0 {
		shellScript += "echo \"[2/3] 写入配置文件...\"\n"
		for _, step := range steps {
			shellScript += "echo \"" + step.Title + "\"\n"
			if step.Code != "" {
				shellScript += step.Code + "\n"
			}
		}
		shellScript += "\n"
	}

	// 验证命令
	if verifyCmd != "" {
		shellScript += "echo \"[3/3] 验证配置...\"\n"
		shellScript += "echo \"运行以下命令验证: " + verifyCmd + "\"\n"
	}

	shellScript += "\necho \"\"\necho \"配置完成! 请重启 Agent 工具使配置生效。\"\n"

	// 生成 Windows PowerShell 脚本
	psScript := "# Centag Agent 一键配置脚本 (PowerShell)\n"
	psScript += "# Agent: " + string(agentType) + "\n"
	psScript += "# Route: " + routeName + "\n"
	psScript += "# Generated by Centag\n\n"
	psScript += "$ErrorActionPreference = 'Stop'\n\n"

	psScript += "Write-Host '=== Centag Agent 配置 ===' -ForegroundColor Cyan\n"
	psScript += "Write-Host 'Agent: " + string(agentType) + "'\n"
	psScript += "Write-Host 'Route: " + routeName + "'\n"
	psScript += "Write-Host ''\n"

	if commands.Windows != "" {
		psScript += "Write-Host '[1/3] 配置环境变量...'\n"
		psScript += commands.Windows + "\n\n"
	}

	if len(steps) > 0 {
		psScript += "Write-Host '[2/3] 写入配置文件...'\n"
		for _, step := range steps {
			psScript += "Write-Host '" + step.Title + "'\n"
			if step.Code != "" {
				psScript += step.Code + "\n"
			}
		}
		psScript += "\n"
	}

	if verifyCmd != "" {
		psScript += "Write-Host '[3/3] 验证配置...'\n"
		psScript += "Write-Host '运行以下命令验证: " + verifyCmd + "'\n"
	}

	psScript += "\nWrite-Host ''\nWrite-Host '配置完成! 请重启 Agent 工具使配置生效。' -ForegroundColor Green\n"

	c.JSON(http.StatusOK, gin.H{
		"agent_type":    req.AgentType,
		"backend_name":  routeName,
		"description":   tmpl.Description(),
		"shell_script":  shellScript,
		"ps_script":     psScript,
		"verify_cmd":    verifyCmd,
		"steps":         steps,
	})
}

func supportedTypes(r *agent.TemplateRegistry) []string {
	var types []string
	for _, at := range r.List() {
		types = append(types, string(at))
	}
	return types
}

func (h *AgentHandler) respondProxyAPIKeyError(c *gin.Context, err error) {
	if errors.Is(err, errNoUsableProxyAPIKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "未找到可用于 Agent 接入的 Centag API Key（llmproxy_*）。请先在「个人设置 → API Keys」创建；若开启了 LLM_PROXY_API_KEY_REVEAL_ONCE，历史密钥可能无法解密，请新建。",
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

func (h *AgentHandler) resolveProxyAPIKey(c *gin.Context) (string, error) {
	userID, err := auth.GetUserID(c)
	if err != nil {
		return "", fmt.Errorf("resolve proxy api key: unauthenticated: %w", err)
	}

	if !database.IsInitialized() {
		return "", fmt.Errorf("resolve proxy api key: database is not initialized")
	}
	db := database.Get()
	if db == nil {
		return "", fmt.Errorf("resolve proxy api key: database manager unavailable")
	}

	keys, err := db.APIKeyStore().ListByUserID(c.Request.Context(), userID)
	if err != nil {
		return "", fmt.Errorf("resolve proxy api key: list user API keys: %w", err)
	}
	if key, ok := selectDecryptableProxyAPIKey(keys, time.Now(), auth.APIKeyStorageKey()); ok {
		return key, nil
	}

	if auth.IsAdmin(c) {
		if key, ok := loadDefaultProxyAdminAPIKey(); ok {
			return key, nil
		}
	}

	return "", errNoUsableProxyAPIKey
}

func selectDecryptableProxyAPIKey(keys []*database.APIKey, now time.Time, storageKey []byte) (string, bool) {
	if len(storageKey) == 0 || len(keys) == 0 {
		return "", false
	}

	candidates := make([]*database.APIKey, 0, len(keys))
	for _, key := range keys {
		if key == nil || !key.Enabled || key.KeySecretEnc == "" {
			continue
		}
		if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
			continue
		}
		candidates = append(candidates, key)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].ID > candidates[j].ID
		}
		return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
	})

	for _, key := range candidates {
		plain, err := auth.DecryptAPIKeyPlaintext(key.KeySecretEnc, storageKey)
		if err != nil {
			continue
		}
		plain = strings.TrimSpace(plain)
		if strings.HasPrefix(plain, "llmproxy_") {
			return plain, true
		}
	}

	return "", false
}

func loadDefaultProxyAdminAPIKey() (string, bool) {
	for _, envKey := range []string{"LLM_PROXY_DEFAULT_ADMIN_API_KEY", "LLM_PROXY_ADMIN_API_KEY"} {
		if val := strings.TrimSpace(os.Getenv(envKey)); strings.HasPrefix(val, "llmproxy_") {
			return val, true
		}
	}
	return "", false
}

func (h *AgentHandler) resolveBackendForAgent(backendID, pipelineID string) (*backend.BackendConfig, string, error) {
	// 流水线模式：后端与模型由流水线内部定义，接入时不再强制要求 backend_id。
	if strings.TrimSpace(pipelineID) != "" {
		routeName := "pipeline." + strings.TrimSpace(pipelineID)
		return &backend.BackendConfig{
			ID:   routeName,
			Name: routeName,
			Type: "openai",
		}, routeName, nil
	}

	if strings.TrimSpace(backendID) != "" {
		be, err := h.backendMgr.Get(strings.TrimSpace(backendID))
		if err != nil {
			return nil, "", fmt.Errorf("backend not found: %s", backendID)
		}
		return be, be.Name, nil
	}

	return nil, "", fmt.Errorf("pipeline_id is required for agent quick setup")
}

// effectiveDirectAPIKey 解析直连接入用的真实密钥：优先主密钥，其次账户池中首个启用的账户密钥。
func effectiveDirectAPIKey(be *backend.BackendConfig) string {
	if k := strings.TrimSpace(be.APIKey); k != "" {
		return k
	}
	if be.AccountPool != nil {
		for _, acc := range be.AccountPool.Accounts {
			if acc.Enabled && strings.TrimSpace(acc.APIKey) != "" {
				return strings.TrimSpace(acc.APIKey)
			}
		}
	}
	return ""
}

// buildAgentBackendInfo 构建接入用的 BackendInfo，并按模式选择密钥来源：
//   - 流水线模式（pipeline_id 非空）：使用 Centag 代理密钥（llmproxy_*），BaseURL 留空由模板回退到代理地址。
//   - 直连模式（backend_id 非空）：使用后端真实密钥与真实 BaseURL，跳过代理密钥解析（绕过 Centag 代理）。
//
// 直连模式要求后端已启用且具备可用 API Key（主密钥或账户池中任一启用账户），否则返回错误引导用户改用代理模式。
func (h *AgentHandler) buildAgentBackendInfo(c *gin.Context, backendID, pipelineID, model, host string, port int) (*agent.BackendInfo, string, error) {
	be, routeName, err := h.resolveBackendForAgent(backendID, pipelineID)
	if err != nil {
		return nil, "", err
	}

	modelName := resolveModelName(model, pipelineID, be.SupportedModels)

	// 直连模式：使用后端真实密钥与真实 BaseURL（绕过 Centag 代理，不走计量/配额/failover/语义缓存）
	if strings.TrimSpace(backendID) != "" {
		apiKey := effectiveDirectAPIKey(be)
		if !be.Enabled {
			return nil, "", fmt.Errorf("后端 %q 已禁用，无法用于直连接入；请改用 Centag 代理模式，或在后端管理中启用它", be.Name)
		}
		if apiKey == "" {
			return nil, "", fmt.Errorf("后端 %q 未配置可用 API Key，无法直连接入；请改用 Centag 代理模式，或在后端管理中填写密钥", be.Name)
		}
		return &agent.BackendInfo{
			ID:      be.ID,
			Name:    be.Name,
			BaseURL: be.BaseURL,
			APIKey:  apiKey,
			Type:    be.Type,
			Model:   modelName,
			Host:    host,
			Port:    port,
		}, routeName, nil
	}

	// 代理模式：解析 Centag 代理密钥
	proxyAPIKey, err := h.resolveProxyAPIKey(c)
	if err != nil {
		return nil, "", err
	}
	return &agent.BackendInfo{
		ID:      be.ID,
		Name:    be.Name,
		BaseURL: be.BaseURL,
		APIKey:  proxyAPIKey,
		Type:    be.Type,
		Model:   modelName,
		Host:    host,
		Port:    port,
	}, routeName, nil
}
