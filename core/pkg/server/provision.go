// Package server 提供租户生命周期管理：创建、初始化、资源复制
// 核心原则：一用户一租户，注册即自动创建租户并复制系统预设资源
package server

import (
	"context"
	"fmt"
	"time"

	"centag/core/internal/agent"
	"centag/core/internal/auth"
	"centag/core/pkg/backend"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"
	"centag/core/pkg/proxymode"
)

// TenantProvisioner 租户资源供应器
type TenantProvisioner struct {
	db               *database.Manager
	backendManager   *backend.Manager
	pipelineRegistry *pipeline.PipelineRegistry
	modeManager      *proxymode.ModeManager
}

// NewTenantProvisioner 创建租户供应器
func NewTenantProvisioner(db *database.Manager, bm *backend.Manager, pr *pipeline.PipelineRegistry) *TenantProvisioner {
	return &TenantProvisioner{
		db:               db,
		backendManager:   bm,
		pipelineRegistry: pr,
	}
}

// SetModeManager 注入模式管理器，租户复制流水线后同步快捷码。
func (p *TenantProvisioner) SetModeManager(mgr *proxymode.ModeManager) {
	if p != nil {
		p.modeManager = mgr
	}
}

func (p *TenantProvisioner) syncPipelineModes() {
	if p == nil || p.modeManager == nil || p.pipelineRegistry == nil {
		return
	}
	if n := p.modeManager.SyncFromPipelines(p.pipelineRegistry.ListAll()); n > 0 {
		logger.Debug("Tenant provision synced pipeline shortcuts to ModeManager",
			logger.GetField("count", n))
	}
}

// ── 3.5 新用户注册流程 ───────────────────────────────────────────────────────

// ProvisionForUser 为新用户创建租户并复制系统预设资源
// 调用时机：UserHandler.CreateUser 成功创建用户后
// 行为：
//   1. 生成租户 ID（格式：t_<user_id>_<timestamp>）
//   2. 创建租户记录
//   3. 设置默认配额
//   4. 复制系统预设后端到租户空间
//   5. 复制系统预设流水线到租户空间
//   6. 创建默认 API Key
//   7. 更新用户的 tenant_id 字段
func (p *TenantProvisioner) ProvisionForUser(ctx context.Context, user *database.User) (*database.Tenant, string, error) {
	if user == nil || user.ID == 0 {
		return nil, "", fmt.Errorf("invalid user")
	}

	// 1. 创建租户（与 bootstrap 共用逻辑）
	tenant, err := database.ProvisionUserTenant(ctx, p.db, user)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create tenant: %w", err)
	}
	tenantID := tenant.ID

	// 2. 复制系统预设后端到租户空间
	if err := p.copySystemBackends(ctx, tenantID); err != nil {
		logger.Warnf("Failed to copy system backends to tenant %s: %v", tenantID, err)
		// 不阻塞注册，后端可以在管理界面手动添加
	}

	// 3. 复制系统预设流水线到租户空间
	if err := p.copySystemPipelines(ctx, tenantID); err != nil {
		logger.Warnf("Failed to copy system pipelines to tenant %s: %v", tenantID, err)
		// 不阻塞注册
	}

	// 4. 创建默认 API Key
	rawKey, err := p.createDefaultAPIKey(ctx, user.ID, tenantID)
	if err != nil {
		logger.Warnf("Failed to create default API key for tenant %s: %v", tenantID, err)
	}

	// 5. 复制系统 Agent 供应商配置到租户空间
	if err := p.copySystemAgentProviders(ctx, user.ID, tenantID); err != nil {
		logger.Warnf("Failed to copy system agent providers to tenant %s: %v", tenantID, err)
		// 不阻塞注册
	}

	logger.Info("Tenant provisioned successfully",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("user_id", user.ID))

	return tenant, rawKey, nil
}

// ── 3.6 预设复制工具 ─────────────────────────────────────────────────────────

// copySystemBackends 复制系统预设后端到租户空间
// 策略：
//   - 只复制系统默认后端（TenantID == ""）
//   - 复制时清空 API Key（安全：不泄露系统级 API Key）
//   - 设置 TenantID 为目标租户
//   - 添加 metadata 标记来源
//   - 后端 ID 添加租户前缀避免冲突
func (p *TenantProvisioner) copySystemBackends(ctx context.Context, tenantID string) error {
	if p.backendManager == nil {
		return fmt.Errorf("backend manager not initialized")
	}

	// 获取系统默认后端（TenantID == ""）
	systemBackends := p.backendManager.ListByTenant("")
	if len(systemBackends) == 0 {
		logger.Info("No system backends to copy", logger.GetField("tenant_id", tenantID))
		return nil
	}

	copiedCount := 0
	for _, cfg := range systemBackends {
		// 跳过已属于其他租户的后端（防御性）
		if cfg.TenantID != "" {
			continue
		}

		// 深拷贝后端配置
		tenantCfg := deepCopyBackendConfig(cfg)

		// 清空 API Key（安全关键：不泄露系统级 API Key）
		tenantCfg.APIKey = ""

		// 设置租户归属
		tenantCfg.TenantID = tenantID

		// 修改 ID 避免冲突：原 ID 前加租户前缀
		originalID := tenantCfg.ID
		tenantCfg.ID = fmt.Sprintf("%s_%s", tenantID, originalID)

		// 添加元数据标记
		if tenantCfg.Metadata == nil {
			tenantCfg.Metadata = make(map[string]string)
		}
		tenantCfg.Metadata["copied_from_system"] = "true"
		tenantCfg.Metadata["original_id"] = originalID
		tenantCfg.Metadata["tenant_id"] = tenantID

		// 添加到后端管理器
		if err := p.backendManager.Add(tenantCfg); err != nil {
			logger.Warnf("Failed to add copied backend %s for tenant %s: %v", originalID, tenantID, err)
			continue
		}

		copiedCount++
		logger.Debug("Backend copied to tenant",
			logger.GetField("tenant_id", tenantID),
			logger.GetField("original_id", originalID),
			logger.GetField("new_id", tenantCfg.ID))
	}

	// 持久化到数据库
	if err := p.backendManager.Save(); err != nil {
		return fmt.Errorf("failed to save copied backends: %w", err)
	}

	logger.Info("System backends copied to tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("copied_count", copiedCount),
		logger.GetField("total_system", len(systemBackends)))

	return nil
}

// copySystemPipelines 复制系统预设流水线到租户空间
// 策略：
//   - 复制系统预设流水线（未标记租户专属的版本）
//   - 设置 TenantID 为目标租户
//   - 流水线 ID 添加租户前缀避免冲突
func (p *TenantProvisioner) copySystemPipelines(ctx context.Context, tenantID string) error {
	if p.pipelineRegistry == nil {
		return fmt.Errorf("pipeline registry not initialized")
	}

	// 获取系统预设流水线
	systemPipelines := p.pipelineRegistry.ListByTenant("")
	if len(systemPipelines) == 0 {
		logger.Info("No system pipelines to copy", logger.GetField("tenant_id", tenantID))
		return nil
	}

	copiedCount := 0
	for _, pipe := range systemPipelines {
		// 深拷贝流水线
		tenantPipe := deepCopyPipeline(pipe)
		tenantPipe.TenantID = tenantID

		// 修改 ID 避免冲突
		originalID := tenantPipe.ID
		tenantPipe.ID = fmt.Sprintf("%s_%s", tenantID, originalID)

		// 添加元数据标记
		if tenantPipe.Metadata == nil {
			tenantPipe.Metadata = make(map[string]interface{})
		}
		tenantPipe.Metadata["copied_from_system"] = true
		tenantPipe.Metadata["original_id"] = originalID
		tenantPipe.Metadata["tenant_id"] = tenantID

		// 注册到租户空间
		if err := p.pipelineRegistry.RegisterForTenant(tenantID, tenantPipe); err != nil {
			logger.Warnf("Failed to register copied pipeline %s for tenant %s: %v", originalID, tenantID, err)
			continue
		}

		copiedCount++
		logger.Debug("Pipeline copied to tenant",
			logger.GetField("tenant_id", tenantID),
			logger.GetField("original_id", originalID),
			logger.GetField("new_id", tenantPipe.ID))
	}

	logger.Info("System pipelines copied to tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("copied_count", copiedCount),
		logger.GetField("total_system", len(systemPipelines)))

	p.syncPipelineModes()
	return nil
}

// copySystemAgentProviders 复制系统预设 Agent 供应商配置到租户空间
// 策略：
//   - 复制系统默认配置（tenant_id 为空的）
//   - 更新 tenant_id 和 user_id 为目标租户/用户
//   - 清空 api_key（安全：不泄露系统级 API Key）
func (p *TenantProvisioner) copySystemAgentProviders(_ context.Context, userID int64, tenantID string) error {
	mgr := agent.GetProviderManager()
	systemConfigs := mgr.List()

	copiedCount := 0
	for _, cfg := range systemConfigs {
		// 只复制系统默认配置（未绑定租户的）
		if cfg.TenantID != "" {
			continue
		}

		// 深拷贝
		newCfg := *cfg
		newCfg.ID = fmt.Sprintf("%s_%s", tenantID, cfg.ID)
		newCfg.TenantID = tenantID
		newCfg.APIKey = "" // 清空 API Key

		if err := mgr.Update(&newCfg); err != nil {
			// 不存在则创建
			if err := mgr.Add(&newCfg); err != nil {
				logger.Warnf("Failed to copy agent provider %s for tenant %s: %v", cfg.ID, tenantID, err)
				continue
			}
		}
		copiedCount++
	}

	if err := mgr.Save(); err != nil {
		return fmt.Errorf("failed to save copied agent providers: %w", err)
	}

	logger.Info("System agent providers copied to tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("copied_count", copiedCount))
	return nil
}

// createDefaultAPIKey 为新租户创建默认 API Key
// 返回原始 key（仅一次，需展示给用户）和错误
func (p *TenantProvisioner) createDefaultAPIKey(ctx context.Context, userID int64, tenantID string) (string, error) {
	// 生成随机 API Key
	fullKey, hash, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	enc, encErr := auth.EncryptAPIKeyForStorage(fullKey)
	if encErr != nil {
		return "", fmt.Errorf("failed to encrypt API key: %w", encErr)
	}

	apiKey := &database.APIKey{
		UserID:       userID,
		TenantID:     &tenantID,
		Name:         "default",
		KeyHash:      hash,
		KeyPrefix:    prefix,
		KeySecretEnc: enc,
		Enabled:      true,
		CreatedAt:    time.Now().UTC(),
	}

	if err := p.db.APIKeyStore().Create(ctx, apiKey); err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	logger.Info("Default API key created for tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("user_id", userID),
		logger.GetField("key_id", apiKey.ID))

	return fullKey, nil
}

// ── 深拷贝辅助函数 ───────────────────────────────────────────────────────────

func deepCopyBackendConfig(src *backend.BackendConfig) *backend.BackendConfig {
	if src == nil {
		return nil
	}

	dst := &backend.BackendConfig{
		ID:              src.ID,
		Name:            src.Name,
		Type:            src.Type,
		BaseURL:         src.BaseURL,
		APIKey:          src.APIKey,
		Enabled:         src.Enabled,
		Timeout:         src.Timeout,
		MaxRetries:      src.MaxRetries,
		Description:     src.Description,
		AutoFetchModels: src.AutoFetchModels,
		CreatedAt:       src.CreatedAt,
		UpdatedAt:       src.UpdatedAt,
		Weight:          src.Weight,
		Priority:        src.Priority,
		TenantID:        src.TenantID,
	}

	// 深拷贝 Metadata
	if src.Metadata != nil {
		dst.Metadata = make(map[string]string, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}

	// 深拷贝 SupportedModels
	if src.SupportedModels != nil {
		dst.SupportedModels = make([]backend.ModelMapping, len(src.SupportedModels))
		copy(dst.SupportedModels, src.SupportedModels)
	}

	// 深拷贝 Capabilities
	dst.Capabilities = backend.ModelCapabilities{
		MaxContextTokens: src.Capabilities.MaxContextTokens,
		Features:         append([]string(nil), src.Capabilities.Features...),
		SupportsImages:   src.Capabilities.SupportsImages,
		SupportsTools:    src.Capabilities.SupportsTools,
	}

	// 深拷贝 HealthStatus
	if src.HealthStatus != nil {
		dst.HealthStatus = &backend.BackendHealthStatus{
			Status:       src.HealthStatus.Status,
			LastCheckAt:  src.HealthStatus.LastCheckAt,
			LastError:    src.HealthStatus.LastError,
			ResponseTime: src.HealthStatus.ResponseTime,
			ModelsCount:  src.HealthStatus.ModelsCount,
		}
	}

	return dst
}

func deepCopyPipeline(src *pipeline.AgentPatternPipeline) *pipeline.AgentPatternPipeline {
	if src == nil {
		return nil
	}

	dst := &pipeline.AgentPatternPipeline{
		SchemaVersion: src.SchemaVersion,
		ID:            src.ID,
		Name:          src.Name,
		Description:   src.Description,
		Version:       src.Version,
		ShortcutCode:  src.ShortcutCode,
		GlobalConfig:  src.GlobalConfig,
	}

	// 深拷贝 Nodes
	if src.Nodes != nil {
		dst.Nodes = make([]pipeline.PipelineNodeConfig, len(src.Nodes))
		for i, node := range src.Nodes {
			dst.Nodes[i] = pipeline.PipelineNodeConfig{
				ID:              node.ID,
				Type:            node.Type,
				Kind:            node.Kind,
				Implementation:  node.Implementation,
				Name:            node.Name,
				Backend:         node.Backend,
				Model:           node.Model,
				Config:          node.Config,
				ConfigSchemaRef: node.ConfigSchemaRef,
				Timeout:         node.Timeout,
				Condition:       node.Condition,
				NextNodes:       append([]string(nil), node.NextNodes...),
				DependsOn:       append([]string(nil), node.DependsOn...),
			}

			// 深拷贝 Inputs
			if node.Inputs != nil {
				dst.Nodes[i].Inputs = make(map[string]string, len(node.Inputs))
				for k, v := range node.Inputs {
					dst.Nodes[i].Inputs[k] = v
				}
			}

			// 深拷贝 Outputs
			if node.Outputs != nil {
				dst.Nodes[i].Outputs = make(map[string]interface{}, len(node.Outputs))
				for k, v := range node.Outputs {
					dst.Nodes[i].Outputs[k] = v
				}
			}

			// 深拷贝 SecretsRef
			if node.SecretsRef != nil {
				dst.Nodes[i].SecretsRef = make(map[string]string, len(node.SecretsRef))
				for k, v := range node.SecretsRef {
					dst.Nodes[i].SecretsRef[k] = v
				}
			}

			// 深拷贝 Permissions
			if node.Permissions != nil {
				dst.Nodes[i].Permissions = append([]string(nil), node.Permissions...)
			}

			// 深拷贝 Retry
			if node.Retry != nil {
				dst.Nodes[i].Retry = &pipeline.RetryConfig{
					MaxAttempts:     node.Retry.MaxAttempts,
					BackoffStrategy: node.Retry.BackoffStrategy,
					InitialDelay:    node.Retry.InitialDelay,
					MaxDelay:        node.Retry.MaxDelay,
				}
			}

			// 深拷贝 RouteConfig
			if node.RouteConfig != nil {
				dst.Nodes[i].RouteConfig = &pipeline.RouteConfig{
					RouterNodeID: node.RouteConfig.RouterNodeID,
					RouteValue:   node.RouteConfig.RouteValue,
					IsDefault:    node.RouteConfig.IsDefault,
				}
			}
		}
	}
	// 归一化所有节点（防御性，确保 src 即使未被归一化也能统一出口）
	for i := range dst.Nodes {
		dst.Nodes[i].Normalize()
	}

	// 深拷贝 Metadata
	if src.Metadata != nil {
		dst.Metadata = make(map[string]interface{}, len(src.Metadata))
		for k, v := range src.Metadata {
			dst.Metadata[k] = v
		}
	}

	// 深拷贝 FallbackGroups
	if src.GlobalConfig.FallbackGroups != nil {
		dst.GlobalConfig.FallbackGroups = make([]pipeline.FallbackGroup, len(src.GlobalConfig.FallbackGroups))
		for i, fg := range src.GlobalConfig.FallbackGroups {
			dst.GlobalConfig.FallbackGroups[i] = pipeline.FallbackGroup{
				PrimaryNodeID: fg.PrimaryNodeID,
				FallbackNodes: append([]string(nil), fg.FallbackNodes...),
				MaxAttempts:   fg.MaxAttempts,
			}
		}
	}

	return dst
}
