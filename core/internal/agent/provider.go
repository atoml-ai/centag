package agent

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

const (
	// ConfigKeyAgentProviders system_config 中存储 Agent 供应商配置的 key
	ConfigKeyAgentProviders = "agent_providers"
)

// AgentProviderConfig Agent 供应商配置
// 定义某种 Agent 类型的路由策略：使用哪个后端、哪个流水线、API Key 等
type AgentProviderConfig struct {
	ID          string `json:"id"`
	AgentType   string `json:"agent_type"`
	DisplayName string `json:"display_name,omitempty"`

	// 路由目标
	BackendID  string `json:"backend_id,omitempty"`  // 指定后端 ID（空=使用默认后端）
	PipelineID string `json:"pipeline_id,omitempty"` // 指定流水线 ID（空=使用默认流水线）

	// 认证覆盖（可选：覆盖后端的 API Key）
	APIKey string `json:"api_key,omitempty"`

	// 模型覆盖（可选：覆盖后端的默认模型）
	Model string `json:"model,omitempty"`

	// 启用/禁用
	Enabled bool `json:"enabled"`

	// 多租户
	TenantID string `json:"tenant_id,omitempty"`

	// 元数据
	Description string `json:"description,omitempty"`
}

// AgentProviderManager Agent 供应商配置管理器
type AgentProviderManager struct {
	mu           sync.RWMutex
	providers    map[string]*AgentProviderConfig // id -> config
	specialization *SpecializedAgentRegistry      // Agent 专业化注册表
}

// globalManager 全局单例
var (
	globalManager     *AgentProviderManager
	globalManagerOnce sync.Once
)

// GetProviderManager 获取全局 Agent 供应商管理器
func GetProviderManager() *AgentProviderManager {
	globalManagerOnce.Do(func() {
		globalManager = NewAgentProviderManager()
	})
	return globalManager
}

// NewAgentProviderManager 创建管理器
func NewAgentProviderManager() *AgentProviderManager {
	m := &AgentProviderManager{
		providers:      make(map[string]*AgentProviderConfig),
		specialization: NewSpecializedAgentRegistry(),
	}
	m.specialization.SeedDefaults()
	return m
}

// GetSpecializationRegistry 返回 Agent 专业化注册表
func (m *AgentProviderManager) GetSpecializationRegistry() *SpecializedAgentRegistry {
	return m.specialization
}

// Load 从 system_config 加载配置
func (m *AgentProviderManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	var configs []AgentProviderConfig
	if err := config.LoadSystemConfigFromDB(ctx, ConfigKeyAgentProviders, &configs); err != nil {
		// 首次启动或 key 不存在时，使用默认配置
		logger.Infof("[AgentProvider] No saved provider configs, using defaults")
		m.seedDefaultsLocked()
		return nil
	}

	m.providers = make(map[string]*AgentProviderConfig, len(configs))
	for i := range configs {
		m.providers[configs[i].ID] = &configs[i]
	}

	logger.Infof("[AgentProvider] Loaded %d provider configs", len(configs))
	return nil
}

// Save 持久化到 system_config
func (m *AgentProviderManager) Save() error {
	m.mu.RLock()
	configs := make([]AgentProviderConfig, 0, len(m.providers))
	for _, cfg := range m.providers {
		configs = append(configs, *cfg)
	}
	m.mu.RUnlock()

	// 按 ID 排序保证稳定输出
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].ID < configs[j].ID
	})

	ctx := context.Background()
	if err := config.SaveSystemConfigToDB(ctx, ConfigKeyAgentProviders, configs); err != nil {
		return fmt.Errorf("failed to save agent_providers: %w", err)
	}

	logger.Infof("[AgentProvider] Saved %d provider configs", len(configs))
	return nil
}

// Add 添加配置
func (m *AgentProviderManager) Add(cfg *AgentProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.ID == "" {
		cfg.ID = cfg.AgentType
	}
	if _, exists := m.providers[cfg.ID]; exists {
		return fmt.Errorf("agent provider config already exists: %s", cfg.ID)
	}

	m.providers[cfg.ID] = cfg
	return nil
}

// Update 更新配置
func (m *AgentProviderManager) Update(cfg *AgentProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[cfg.ID]; !exists {
		return fmt.Errorf("agent provider config not found: %s", cfg.ID)
	}

	m.providers[cfg.ID] = cfg
	return nil
}

// Delete 删除配置
func (m *AgentProviderManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.providers[id]; !exists {
		return fmt.Errorf("agent provider config not found: %s", id)
	}

	delete(m.providers, id)
	return nil
}

// Get 获取单个配置
func (m *AgentProviderManager) Get(id string) (*AgentProviderConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.providers[id]
	return cfg, ok
}

// GetByAgentType 按 Agent 类型查找配置（返回第一个匹配的启用配置）
func (m *AgentProviderManager) GetByAgentType(agentType string) (*AgentProviderConfig, bool) {
	return m.GetByAgentTypeAndTenant(agentType, "")
}

// GetByAgentTypeAndTenant 按 Agent 类型和租户查找配置。
// 选择规则：
// 1. 优先租户级配置（tenant_id == tenantID）
// 2. 回退系统级配置（tenant_id == ""）
// 3. 同级冲突按 ID 字典序取最小，保证确定性
func (m *AgentProviderManager) GetByAgentTypeAndTenant(agentType, tenantID string) (*AgentProviderConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tenantMatch *AgentProviderConfig
	var systemMatch *AgentProviderConfig

	for _, cfg := range m.providers {
		if cfg.AgentType != agentType || !cfg.Enabled {
			continue
		}

		if tenantID != "" && cfg.TenantID == tenantID {
			if tenantMatch == nil || cfg.ID < tenantMatch.ID {
				tenantMatch = cfg
			}
			continue
		}

		if cfg.TenantID == "" {
			if systemMatch == nil || cfg.ID < systemMatch.ID {
				systemMatch = cfg
			}
		}
	}

	if tenantMatch != nil {
		return tenantMatch, true
	}
	if systemMatch != nil {
		return systemMatch, true
	}
	return nil, false
}

// List 列出所有配置
func (m *AgentProviderManager) List() []*AgentProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AgentProviderConfig, 0, len(m.providers))
	for _, cfg := range m.providers {
		result = append(result, cfg)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ListByTenant 列出指定租户的配置（系统级 + 租户级）
func (m *AgentProviderManager) ListByTenant(tenantID string) []*AgentProviderConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AgentProviderConfig, 0)
	for _, cfg := range m.providers {
		if cfg.TenantID == "" || cfg.TenantID == tenantID {
			result = append(result, cfg)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// SeedDefaults 内置默认配置（首次启动时写入）
func (m *AgentProviderManager) SeedDefaults() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seedDefaultsLocked()
}

// seedDefaultsLocked 在持有锁的情况下写入默认配置
func (m *AgentProviderManager) seedDefaultsLocked() {
	defaults := []AgentProviderConfig{
		{
			ID:          "claude-code",
			AgentType:   "claude-code",
			DisplayName: "Claude Code",
			Enabled:     true,
			Description: "Anthropic Claude Code CLI 默认配置",
		},
		{
			ID:          "claude-desktop",
			AgentType:   "claude-desktop",
			DisplayName: "Claude Desktop",
			Enabled:     true,
			Description: "Anthropic Claude Desktop 默认配置",
		},
		{
			ID:          "codex",
			AgentType:   "codex",
			DisplayName: "Codex CLI",
			Enabled:     true,
			Description: "OpenAI Codex CLI 默认配置",
		},
		{
			ID:          "gemini-cli",
			AgentType:   "gemini-cli",
			DisplayName: "Gemini CLI",
			Enabled:     true,
			Description: "Google Gemini CLI 默认配置",
		},
		{
			ID:          "opencode",
			AgentType:   "opencode",
			DisplayName: "OpenCode",
			Enabled:     true,
			Description: "OpenCode 默认配置",
		},
		{
			ID:          "openclaw",
			AgentType:   "openclaw",
			DisplayName: "OpenClaw",
			Enabled:     true,
			Description: "OpenClaw 默认配置",
		},
		{
			ID:          "hermes",
			AgentType:   "hermes",
			DisplayName: "Hermes Agent",
			Enabled:     true,
			Description: "Hermes Agent 默认配置",
		},
		// TUI Agents
		{
			ID:          "coding-tui",
			AgentType:   "coding-tui",
			DisplayName: "Coding TUI Agent",
			Enabled:     true,
			Description: "编程场景终端交互 Agent（代码高亮、Diff 视图、进度追踪）",
		},
		{
			ID:          "education-tui",
			AgentType:   "education-tui",
			DisplayName: "Education TUI Agent",
			Enabled:     true,
			Description: "教育场景终端交互 Agent（学习进度、课程导航、互动答题）",
		},
		// Web Agents
		{
			ID:          "coding-web",
			AgentType:   "coding-web",
			DisplayName: "Coding Web Agent",
			Enabled:     true,
			Description: "编程场景 Web 自动化 Agent（代码审查、文档浏览、在线IDE交互）",
		},
		{
			ID:          "education-web",
			AgentType:   "education-web",
			DisplayName: "Education Web Agent",
			Enabled:     true,
			Description: "教育场景 Web 自动化 Agent（在线学习平台、测验答题、课程导航）",
		},
	}

	for i := range defaults {
		if _, exists := m.providers[defaults[i].ID]; !exists {
			m.providers[defaults[i].ID] = &defaults[i]
		}
	}
}
