package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/config"
	"centag/core/internal/httpclient"
	"centag/core/pkg/logger"
)

// BackendHealthStatus 后端健康状态
type BackendHealthStatus struct {
	Status       string `json:"status"`        // healthy, unhealthy, unknown, checking
	LastCheckAt  string `json:"last_check_at"` // 最后检查时间
	LastError    string `json:"last_error"`    // 最后错误信息
	ResponseTime int64  `json:"response_time"` // 响应时间（毫秒）
	ModelsCount  int    `json:"models_count"`  // 获取到的模型数量
}

// BackendAccount 账户池中的单个凭证
type BackendAccount struct {
	ID        string `json:"id"`                  // 池内唯一，如 "key-1"
	Label     string `json:"label,omitempty"`     // 显示名，如 "免费 Key A"
	APIKey    string `json:"api_key,omitempty"`   // 明文仅写入；响应用 has_api_key
	Enabled   bool   `json:"enabled"`             // 默认 true
	Weight    int    `json:"weight,omitempty"`     // 加权轮询，默认 1
	CreatedAt string `json:"created_at,omitempty"`
}

// AccountPoolConfig 账户池配置（非空时优先于 BackendConfig.APIKey）
type AccountPoolConfig struct {
	Strategy string           `json:"strategy"` // round_robin | least_usage | sticky_session
	Accounts []BackendAccount `json:"accounts"`
}

// BackendConfig 后端服务配置
// 注意：Weight 和 Priority 是调度策略参数，由 scheduler/preset 模块管理
// 后端管理页面不展示和编辑这些字段，但内部存储需要保留
type BackendConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // openai, ollama, anthropic
	BaseURL     string            `json:"base_url"`
	APIKey      string            `json:"api_key,omitempty"`
	Enabled     bool              `json:"enabled"`
	Timeout     int               `json:"timeout"`     // 请求超时（秒）
	MaxRetries  int               `json:"max_retries"` // 最大重试次数
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// 模型管理
	SupportedModels []ModelMapping    `json:"supported_models,omitempty"`  // 支持的模型列表
	AutoFetchModels bool              `json:"auto_fetch_models,omitempty"` // 是否自动获取模型列表
	ProbeModel      string            `json:"probe_model,omitempty"`       // 默认探测模型（用于连通性/可用性探测）
	Capabilities    ModelCapabilities `json:"capabilities,omitempty"`      // 模型能力限制

	// 健康状态（由探测功能更新）
	HealthStatus *BackendHealthStatus `json:"health_status,omitempty"`

	// 元数据（只读）
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`

	// 调度策略参数（后端管理页面不展示，由 scheduler/preset 模块管理）
	Weight   int `json:"weight,omitempty"`   // 负载均衡权重 / 严格度
	Priority int `json:"priority,omitempty"` // 优先级

	// 故障转移：降级后端列表（按优先级排序）
	FallbackBackends []string `json:"fallback_backends,omitempty"`

	// 账户池：多凭证轮转（非空时优先于 APIKey）
	AccountPool *AccountPoolConfig `json:"account_pool,omitempty"`

	// 租户隔离（新增）
	TenantID string `json:"tenant_id,omitempty"` // 空字符串表示系统默认后端，非空表示租户私有后端
}

// BackendConfigResponse 用于 API 响应的后端配置（包含 has_api_key 标记）
type BackendConfigResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	BaseURL     string            `json:"base_url"`
	HasAPIKey   bool              `json:"has_api_key"` // 是否已设置 API Key（不返回实际值）
	Enabled     bool              `json:"enabled"`
	Timeout     int               `json:"timeout"`
	MaxRetries  int               `json:"max_retries"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`

	// 模型管理
	SupportedModels []ModelMapping    `json:"supported_models,omitempty"`
	AutoFetchModels bool              `json:"auto_fetch_models,omitempty"`
	ProbeModel      string            `json:"probe_model,omitempty"`
	DefaultModel    string            `json:"default_model,omitempty"` // 首选对话模型（ProbeModel 或 SupportedModels[0]）
	Capabilities    ModelCapabilities `json:"capabilities,omitempty"`

	// 健康状态
	HealthStatus *BackendHealthStatus `json:"health_status,omitempty"`

	// 元数据（只读）
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`

	// 调度策略参数
	Weight   int `json:"weight,omitempty"`
	Priority int `json:"priority,omitempty"`

	// 空 = 系统后端；非空 = 用户私有后端（ownTenantID）
	TenantID string `json:"tenant_id,omitempty"`

	// 账户池信息（只读，用于 UI 展示）
	AccountPoolSummary *AccountPoolSummary `json:"account_pool_summary,omitempty"`
}

// AccountPoolSummary 账户池摘要信息（用于 API 响应）
type AccountPoolSummary struct {
	TotalAccounts  int    `json:"total_accounts"`
	EnabledAccounts int   `json:"enabled_accounts"`
	Strategy       string `json:"strategy"`
	HealthStatus   string `json:"health_status"` // healthy, partial, unhealthy
}

// ToResponse 将 BackendConfig 转换为 BackendConfigResponse（用于 API 响应）
func (c *BackendConfig) ToResponse() *BackendConfigResponse {
	resp := &BackendConfigResponse{
		ID:              c.ID,
		Name:            c.Name,
		Type:            c.Type,
		BaseURL:         c.BaseURL,
		HasAPIKey:       c.APIKey != "" || (c.AccountPool != nil && len(c.AccountPool.Accounts) > 0),
		Enabled:         c.Enabled,
		Timeout:         c.Timeout,
		MaxRetries:      c.MaxRetries,
		Description:     c.Description,
		Metadata:        c.Metadata,
		SupportedModels: c.SupportedModels,
		AutoFetchModels: c.AutoFetchModels,
		ProbeModel:      c.ProbeModel,
		DefaultModel:    PreferredDefaultModel(c),
		Capabilities:    c.Capabilities,
		HealthStatus:    c.HealthStatus,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		Weight:          c.Weight,
		Priority:        c.Priority,
		TenantID:        c.TenantID,
	}

	// 账户池摘要
	if c.AccountPool != nil && len(c.AccountPool.Accounts) > 0 {
		enabledCount := 0
		for _, acc := range c.AccountPool.Accounts {
			if acc.Enabled {
				enabledCount++
			}
		}
		healthStatus := "healthy"
		if enabledCount == 0 {
			healthStatus = "unhealthy"
		} else if enabledCount < len(c.AccountPool.Accounts) {
			healthStatus = "partial"
		}
		resp.AccountPoolSummary = &AccountPoolSummary{
			TotalAccounts:   len(c.AccountPool.Accounts),
			EnabledAccounts: enabledCount,
			Strategy:        c.AccountPool.Strategy,
			HealthStatus:    healthStatus,
		}
	}

	return resp
}

// Manager 后端管理器
type Manager struct {
	mu       sync.RWMutex
	backends map[string]*BackendConfig // id -> config
	store    BackendStore              // optional persistence layer (nil for backward compat)
}

// SetStore configures the optional persistence layer.
// When set, Load() and Save() delegate to the store instead of using the
// legacy config.Get() / config.SaveBackendsToDB() code paths.
func (m *Manager) SetStore(store BackendStore) {
	m.store = store
}

// NewManager 创建一个新的后端管理器实例（用于测试）
func NewManager() *Manager {
	return &Manager{
		backends: make(map[string]*BackendConfig),
	}
}

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager 获取全局后端管理器
func GetManager() *Manager {
	once.Do(func() {
		globalManager = &Manager{
			backends: make(map[string]*BackendConfig),
		}
	})
	return globalManager
}

// SetManagerForTest replaces the process-wide manager (tests only).
func SetManagerForTest(m *Manager) {
	once.Do(func() {
		globalManager = m
	})
	globalManager = m
}

// Load 从持久化存储加载后端配置。
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store != nil {
		backends, err := m.store.Load()
		if err != nil {
			return err
		}
		m.backends = make(map[string]*BackendConfig, len(backends))
		for _, b := range backends {
			m.backends[b.ID] = b
		}
		logger.Info("Loaded backends via store", logger.GetField("count", len(m.backends)))
		return nil
	}

	// 传统路径：从全局配置（database 已加载）中读取
	cfg := config.Get()
	if cfg == nil || len(cfg.Backends) == 0 {
		logger.Info("No backend configs found in database, starting with empty list")
		return nil
	}

	// 用当前环境重新解析 initial-backends.json，补全 DB 里为空的 api_key，并写回库（与 seed 时序无关）
	enriched, ok := MergeAPIKeysFromInitialFile(cfg.Backends)
	if ok {
		cfg.Backends = enriched
		if err := config.SaveConfig(cfg); err != nil {
			logger.Warnf("写入补全后的 API Key 到数据库失败: %v（内存仍使用补全后的值）", err)
		}
	}

	m.backends = make(map[string]*BackendConfig)
	for i := range cfg.Backends {
		b := configBackendToBackend(&cfg.Backends[i])
		m.backends[b.ID] = b
	}
	logger.Info("Loaded backend configs from database", logger.GetField("count", len(m.backends)))
	return nil
}

// Save 将后端配置持久化。
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store != nil {
		backends := make([]*BackendConfig, 0, len(m.backends))
		for _, b := range m.backends {
			backends = append(backends, b)
		}
		return m.store.Save(backends)
	}

	// 传统路径：持久化到数据库
	globalCfg := config.Get()
	if globalCfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	backendConfigs := make([]config.BackendConfig, 0, len(m.backends))
	for _, b := range m.backends {
		backendConfigs = append(backendConfigs, backendToConfigBackend(b))
	}

	globalCfg.Backends = backendConfigs

	if err := config.SaveBackendsToDB(backendConfigs); err != nil {
		return fmt.Errorf("failed to save backend config to database: %w", err)
	}

	logger.Info("Saved backend configs to database", logger.GetField("count", len(m.backends)))
	return nil
}

// Add 添加后端配置
func (m *Manager) Add(cfg *BackendConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.ID == "" {
		return fmt.Errorf("backend id is required")
	}

	if _, exists := m.backends[cfg.ID]; exists {
		return fmt.Errorf("backend with id %s already exists", cfg.ID)
	}

	cfg.APIKey = NormalizeOpenAICompatibleAPIKey(cfg.APIKey)

	if cfg.Weight <= 0 {
		cfg.Weight = 1
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 60
	}

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	m.backends[cfg.ID] = cfg
	return nil
}

// Update 更新后端配置
func (m *Manager) Update(cfg *BackendConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.ID == "" {
		return fmt.Errorf("backend id is required")
	}

	old, exists := m.backends[cfg.ID]
	if !exists {
		return fmt.Errorf("backend with id %s not found", cfg.ID)
	}

	// 列表/详情对 api_key 使用 omitempty：前端常拿不到密钥，保存时会带上 api_key: ""。
	// 部分更新（如仅切换 enabled）也可能不传 api_key。空字符串表示「不修改」而非「清空」。
	if cfg.APIKey == "" && old.APIKey != "" {
		cfg.APIKey = old.APIKey
	}

	// 前端保存配置时通常不携带 health_status，避免覆盖掉刚探测得到的健康状态。
	if cfg.HealthStatus == nil && old.HealthStatus != nil {
		cfg.HealthStatus = old.HealthStatus
	}
	cfg.APIKey = NormalizeOpenAICompatibleAPIKey(cfg.APIKey)

	// 账户池：未传则原样保留（不重跑 Validate，避免历史空 key 脏数据阻塞改模型/探测等无关字段）。
	// 传入时对空 api_key 的账户从旧池按 id 补全，避免保存策略/权重时把密钥冲掉。
	preservePool := cfg.AccountPool == nil && old.AccountPool != nil
	if preservePool {
		cfg.AccountPool = old.AccountPool
	} else if cfg.AccountPool != nil && old.AccountPool != nil {
		for i := range cfg.AccountPool.Accounts {
			if strings.TrimSpace(cfg.AccountPool.Accounts[i].APIKey) != "" {
				cfg.AccountPool.Accounts[i].APIKey = NormalizeOpenAICompatibleAPIKey(cfg.AccountPool.Accounts[i].APIKey)
				continue
			}
			if oldAcc, err := GetAccountByID(old.AccountPool, cfg.AccountPool.Accounts[i].ID); err == nil && oldAcc != nil {
				cfg.AccountPool.Accounts[i].APIKey = oldAcc.APIKey
			}
		}
	}
	if cfg.AccountPool != nil && !preservePool {
		NormalizeAccountPool(cfg.AccountPool)
		if err := ValidateAccountPool(cfg.AccountPool); err != nil {
			return err
		}
	}

	m.backends[cfg.ID] = cfg
	return nil
}

// Delete 删除后端配置
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.backends[id]; !exists {
		return fmt.Errorf("backend with id %s not found", id)
	}

	delete(m.backends, id)
	return nil
}

// DeleteByTenant 租户用户删除本租户私有后端（不可删除系统共享后端）
func (m *Manager) DeleteByTenant(tenantID, id string) error {
	cfg, err := m.GetByTenant(tenantID, id)
	if err != nil {
		return err
	}
	if cfg.TenantID == "" {
		return fmt.Errorf("cannot delete system backend")
	}
	if cfg.TenantID != tenantID {
		return fmt.Errorf("backend with id %s not found", id)
	}
	return m.Delete(id)
}

// SetDefault 设置默认后端（通过权重调整）
func (m *Manager) SetDefault(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查后端是否存在
	target, exists := m.backends[id]
	if !exists {
		return fmt.Errorf("backend with id %s not found", id)
	}

	// 检查后端是否启用
	if !target.Enabled {
		return fmt.Errorf("backend %s is not enabled", target.Name)
	}

	// 将目标后端权重设为最高
	target.Weight = 100

	// 将其他启用的后端权重设为较低值
	for _, backend := range m.backends {
		if backend.ID != id && backend.Enabled {
			backend.Weight = 1
		}
	}

	logger.Info("Set default backend",
		logger.GetField("id", id),
		logger.GetField("name", target.Name),
		logger.GetField("weight", target.Weight))

	return nil
}

// Get 获取后端配置
func (m *Manager) Get(id string) (*BackendConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.backends[id]
	if !exists {
		return nil, fmt.Errorf("backend with id %s not found", id)
	}

	// 调试日志
	return cfg, nil
}

// GetByName 通过名称获取后端配置
func (m *Manager) GetByName(name string) (*BackendConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cfg := range m.backends {
		if cfg.Name == name {
			return cfg, nil
		}
	}
	return nil, fmt.Errorf("backend with name %s not found", name)
}

// GetAll 获取所有后端配置
func (m *Manager) GetAll() []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backends := make([]*BackendConfig, 0, len(m.backends))
	for _, cfg := range m.backends {
		backends = append(backends, cfg)
	}
	return backends
}

// List 列出所有后端配置（按 ID 字母顺序排序）
func (m *Manager) List() []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0, len(m.backends))
	for _, cfg := range m.backends {
		list = append(list, cfg)
	}

	// 按 ID 字母顺序排序，确保列表顺序稳定
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// GetEnabled 获取所有启用的后端（按 ID 字母顺序排序）
func (m *Manager) GetEnabled() []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0)
	for _, cfg := range m.backends {
		if cfg.Enabled {
			list = append(list, cfg)
		}
	}

	// 按 ID 字母顺序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// GetByType 按类型获取后端（按 ID 字母顺序排序）
func (m *Manager) GetByType(backendType string) []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0)
	for _, cfg := range m.backends {
		if cfg.Type == backendType && cfg.Enabled {
			list = append(list, cfg)
		}
	}

	// 按 ID 字母顺序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// GetByTenant 按租户获取后端配置（租户隔离）
func (m *Manager) GetByTenant(tenantID, id string) (*BackendConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.backends[id]
	if !exists {
		return nil, fmt.Errorf("backend with id %s not found", id)
	}

	// 租户隔离检查：
	// 1. 如果请求的是系统默认后端（tenantID == ""），只能访问系统默认后端（cfg.TenantID == ""）
	// 2. 如果请求的是租户后端，只能访问该租户的后端或系统默认后端
	if cfg.TenantID != "" && cfg.TenantID != tenantID {
		return nil, fmt.Errorf("backend with id %s not found", id)
	}

	return cfg, nil
}

// ListByTenant 按租户列出后端配置（租户隔离）
func (m *Manager) ListByTenant(tenantID string) []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0)
	for _, cfg := range m.backends {
		// 租户隔离：只能访问自己租户的后端或系统默认后端
		if cfg.TenantID == "" || cfg.TenantID == tenantID {
			list = append(list, cfg)
		}
	}

	// 按 ID 字母顺序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// GetEnabledByTenant 按租户获取启用的后端（租户隔离）
func (m *Manager) GetEnabledByTenant(tenantID string) []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0)
	for _, cfg := range m.backends {
		if cfg.Enabled && (cfg.TenantID == "" || cfg.TenantID == tenantID) {
			list = append(list, cfg)
		}
	}

	// 按 ID 字母顺序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// SelectDefaultBackendByTenant 按租户选择默认后端（租户隔离）
func (m *Manager) SelectDefaultBackendByTenant(tenantID string) (*BackendConfig, error) {
	backends := m.GetEnabledByTenant(tenantID)
	if len(backends) == 0 {
		return nil, NewNoUsableBackendError(fmt.Errorf("no enabled backends found for tenant %s", tenantID))
	}

	// 按权重排序，权重高的排在前面
	sortedBackends := make([]*BackendConfig, len(backends))
	copy(sortedBackends, backends)

	// 简单排序：按权重降序
	for i := 0; i < len(sortedBackends)-1; i++ {
		for j := i + 1; j < len(sortedBackends); j++ {
			if sortedBackends[i].Weight < sortedBackends[j].Weight {
				sortedBackends[i], sortedBackends[j] = sortedBackends[j], sortedBackends[i]
			}
		}
	}

	selected := sortedBackends[0]
	logger.Info("Default backend selected for tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("id", selected.ID),
		logger.GetField("name", selected.Name),
		logger.GetField("type", selected.Type),
		logger.GetField("weight", selected.Weight))

	return selected, nil
}

// GetByTypeAndTenant 按类型和租户获取后端（租户隔离）
func (m *Manager) GetByTypeAndTenant(backendType, tenantID string) []*BackendConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*BackendConfig, 0)
	for _, cfg := range m.backends {
		if cfg.Type == backendType && cfg.Enabled && (cfg.TenantID == "" || cfg.TenantID == tenantID) {
			list = append(list, cfg)
		}
	}

	// 按 ID 字母顺序排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list
}

// SelectDefaultBackend 选择默认后端(基于权重)
// 注意：此方法不执行租户隔离，所有租户共享同一套后端配置
// 建议使用 SelectDefaultBackendByTenant 进行租户隔离
func (m *Manager) SelectDefaultBackend() (*BackendConfig, error) {
	// 获取所有启用的后端
	backends := m.GetEnabled()
	if len(backends) == 0 {
		return nil, NewNoUsableBackendError(errors.New("no enabled backends found"))
	}

	// 记录候选后端数量
	logger.Debug("Selecting default backend", logger.GetField("candidates", len(backends)))

	// 按权重排序，权重高的排在前面
	sortedBackends := make([]*BackendConfig, len(backends))
	copy(sortedBackends, backends)

	// 简单排序：按权重降序
	for i := 0; i < len(sortedBackends)-1; i++ {
		for j := i + 1; j < len(sortedBackends); j++ {
			if sortedBackends[i].Weight < sortedBackends[j].Weight {
				sortedBackends[i], sortedBackends[j] = sortedBackends[j], sortedBackends[i]
			}
		}
	}

	// 返回权重最高的后端作为默认
	selected := sortedBackends[0]
	logger.Info("Default backend selected",
		logger.GetField("id", selected.ID),
		logger.GetField("name", selected.Name),
		logger.GetField("type", selected.Type),
		logger.GetField("weight", selected.Weight))

	return selected, nil
}

// SelectBackend 选择后端(负载均衡)
// 注意：此方法不执行租户隔离，所有租户共享同一套后端配置
// 建议使用 SelectBackendByTenant 进行租户隔离
func (m *Manager) SelectBackend(backendType string) (*BackendConfig, error) {
	backends := m.GetByType(backendType)
	if len(backends) == 0 {
		return nil, fmt.Errorf("no enabled backend of type %s found", backendType)
	}

	// 根据权重选择后端
	totalWeight := 0
	for _, b := range backends {
		totalWeight += b.Weight
	}

	// 如果总权重为0，返回第一个
	if totalWeight <= 0 {
		return backends[0], nil
	}

	// 按权重排序，权重高的排在前面
	sortedBackends := make([]*BackendConfig, len(backends))
	copy(sortedBackends, backends)

	// 简单排序：按权重降序
	for i := 0; i < len(sortedBackends)-1; i++ {
		for j := i + 1; j < len(sortedBackends); j++ {
			if sortedBackends[i].Weight < sortedBackends[j].Weight {
				sortedBackends[i], sortedBackends[j] = sortedBackends[j], sortedBackends[i]
			}
		}
	}

	// 返回权重最高的后端
	return sortedBackends[0], nil
}

// SelectBackendByTenant 按租户选择后端(负载均衡 + 租户隔离)
func (m *Manager) SelectBackendByTenant(backendType, tenantID string) (*BackendConfig, error) {
	backends := m.GetByTypeAndTenant(backendType, tenantID)
	if len(backends) == 0 {
		return nil, fmt.Errorf("no enabled backend of type %s found for tenant %s", backendType, tenantID)
	}

	// 根据权重选择后端
	totalWeight := 0
	for _, b := range backends {
		totalWeight += b.Weight
	}

	// 如果总权重为0，返回第一个
	if totalWeight <= 0 {
		return backends[0], nil
	}

	// 按权重排序，权重高的排在前面
	sortedBackends := make([]*BackendConfig, len(backends))
	copy(sortedBackends, backends)

	// 简单排序：按权重降序
	for i := 0; i < len(sortedBackends)-1; i++ {
		for j := i + 1; j < len(sortedBackends); j++ {
			if sortedBackends[i].Weight < sortedBackends[j].Weight {
				sortedBackends[i], sortedBackends[j] = sortedBackends[j], sortedBackends[i]
			}
		}
	}

	// 返回权重最高的后端
	return sortedBackends[0], nil
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context, id string) error {
	cfg, err := m.Get(id)
	if err != nil {
		return err
	}

	// 使用 HTTP 客户端工具
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 // 默认30秒
	}
	client := httpclient.NewClient(time.Duration(timeout) * time.Second)

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"

	normKey := NormalizeOpenAICompatibleAPIKey(cfg.APIKey)
	if normKey != "" {
		headers["Authorization"] = "Bearer " + normKey
	}

	// 记录开始时间
	startTime := time.Now()
	var checkErr error

	// 根据后端类型选择不同的健康检查端点
	if cfg.Type == "ollama" {
		checkErr = healthCheckOllama(client, cfg, headers)
	} else {
		checkErr = healthCheckOpenAI(client, cfg, headers)
	}

	// 计算响应时间
	responseTime := time.Since(startTime).Milliseconds()

	// 更新健康状态
	if cfg.HealthStatus == nil {
		cfg.HealthStatus = &BackendHealthStatus{}
	}

	cfg.HealthStatus.LastCheckAt = time.Now().Format(time.RFC3339)
	cfg.HealthStatus.ResponseTime = responseTime

	if checkErr != nil {
		cfg.HealthStatus.Status = "unhealthy"
		cfg.HealthStatus.LastError = checkErr.Error()
		logger.Warnf("Health check failed for backend %s (%s): %v", cfg.ID, cfg.Name, checkErr)
	} else {
		cfg.HealthStatus.Status = "healthy"
		cfg.HealthStatus.LastError = ""
		logger.Infof("Health check for backend %s (%s): OK (response time: %dms)", cfg.ID, cfg.Name, responseTime)
	}

	// 保存更新后的配置
	return m.Update(cfg)
}

// healthCheckOllama 检查 Ollama 后端健康状态
func healthCheckOllama(client *httpclient.Client, cfg *BackendConfig, headers map[string]string) error {
	baseURL := NormalizeOllamaAPIBase(cfg.BaseURL)
	healthURL := baseURL + "/api/tags"

	resp, err := client.Do(context.Background(), &httpclient.RequestConfig{
		Method:  "GET",
		URL:     healthURL,
		Headers: headers,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ollama health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// healthCheckOpenAI 检查 OpenAI 兼容后端健康状态
func healthCheckOpenAI(client *httpclient.Client, cfg *BackendConfig, headers map[string]string) error {
	// 使用 CandidateOpenAIAPIRoots 避免 BaseURL 末尾 /v1 导致路径重复
	roots := CandidateOpenAIAPIRoots(cfg.BaseURL)
	var lastErr error
	for _, root := range roots {
		healthURL := root + "/models"
		resp, err := client.Do(context.Background(), &httpclient.RequestConfig{
			Method:  "GET",
			URL:     healthURL,
			Headers: headers,
		})
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("authentication failed (HTTP %d), please check API key", resp.StatusCode)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("health check failed with status %d", resp.StatusCode)
		continue
	}
	if lastErr != nil {
		return fmt.Errorf("failed to connect to OpenAI compatible backend: %w", lastErr)
	}
	return fmt.Errorf("health check failed")
}

// TestConnection 测试后端连接
func (m *Manager) TestConnection(cfg *BackendConfig) error {
	return m.TestConnectionWithContext(context.Background(), cfg)
}

// TestConnectionWithContext 测试后端连接（支持上层 context 超时/取消）
func (m *Manager) TestConnectionWithContext(ctx context.Context, cfg *BackendConfig) error {
	if cfg.Type == "" {
		return fmt.Errorf("backend type is required (ollama or openai)")
	}
	logger.Info("Testing connection", logger.GetField("name", cfg.Name), logger.GetField("url", cfg.BaseURL), logger.GetField("type", cfg.Type))

	// 验证URL格式
	_, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return fmt.Errorf("base URL is empty")
	}

	normKey := NormalizeOpenAICompatibleAPIKey(cfg.APIKey)
	logger.Infof("TestConnection 检查: id=%s, type=%s, api_key_len=%d",
		cfg.ID, cfg.Type, len(normKey))
	// Ollama 可无密钥；OpenAI 兼容 / 其他云端接口必须先配置 API Key
	if cfg.Type != "ollama" && normKey == "" {
		return fmt.Errorf("未配置 API Key，无法探测。请在 WebUI 保存密钥，或在 config/initdata/initial-backends.json 对应后端填写非空 \"api_key\" 后重启；令牌勿含 \"Bearer \" 前缀")
	}

	// 使用 HTTP 客户端工具
	// 使用不走代理的客户端直连远端，避免系统 http_proxy/https_proxy 环境变量干扰
	// （本地 Ollama 和外部 API 都应直连）
	client := httpclient.NewClientNoProxy(time.Duration(cfg.Timeout) * time.Second)

	headers := make(map[string]string)
	headers["Content-Type"] = "application/json"
	if normKey != "" {
		headers["Authorization"] = "Bearer " + normKey
	}

	// 根据后端类型选择不同的测试端点
	if cfg.Type == "ollama" {
		// Ollama 后端测试逻辑
		return testOllamaConnection(ctx, client, cfg, headers)
	}

	// OpenAI 兼容后端测试逻辑
	return testOpenAIConnectionWithContext(ctx, client, cfg, headers)
}

// testOllamaConnection 测试 Ollama 后端连接（仅探测 /api/tags，避免误用模型导致误判）
func testOllamaConnection(ctx context.Context, client *httpclient.Client, cfg *BackendConfig, headers map[string]string) error {
	baseURL := NormalizeOllamaAPIBase(cfg.BaseURL)
	testURL := baseURL + "/api/tags"

	logger.Info("Sending test request to Ollama tags endpoint", logger.GetField("url", testURL))

	resp, err := client.Do(ctx, &httpclient.RequestConfig{
		Method:  "GET",
		URL:     testURL,
		Headers: headers,
	})
	if err != nil {
		return fmt.Errorf("无法连接 Ollama（网络/超时）: %w；请求 URL: %s", err, testURL)
	}

	logger.Info("Received response status from Ollama tags endpoint", logger.GetField("status", resp.StatusCode))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Info("Ollama connection test successful", logger.GetField("name", cfg.Name))
		return nil
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return fmt.Errorf("ollama 返回重定向 HTTP %d，请检查 Base URL 是否需直连且无多余路径: %s", resp.StatusCode, testURL)
	}
	body := truncateErrBody(resp.Body, 800)
	if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
		return fmt.Errorf("GET %s -> HTTP %d（常见原因：反代无法连到 Ollama、端口错误、或 Base URL 误写成 OpenAI 的 /v1 形式）响应: %q",
			testURL, resp.StatusCode, body)
	}
	return fmt.Errorf("GET %s -> HTTP %d 响应: %q", testURL, resp.StatusCode, body)
}

// ProbeResult 单个后端探测结果
type ProbeResult struct {
	BackendID    string   `json:"backend_id"`
	Name         string   `json:"name"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
	ModelsCount  int      `json:"models_count"`
	ResponseTime int64    `json:"response_time_ms"`
	Models       []string `json:"models,omitempty"`
}

// ProbeAndUpdateBackend 探测单个后端并更新其健康状态和模型列表
func (m *Manager) ProbeAndUpdateBackend(ctx context.Context, id string, fetchModels bool) (*ProbeResult, error) {
	// 确保 API Key 已补全
	m.RepairAPIKeyFromDBIfEmpty(ctx, id)
	m.ApplyAPIKeyFromInitialFileIfEmpty(id)

	// 重新获取（可能已补全密钥）
	cfg, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	result := &ProbeResult{
		BackendID: id,
		Name:      cfg.Name,
	}

	// 更新状态为检查中
	m.updateHealthStatus(id, &BackendHealthStatus{
		Status:      "checking",
		LastCheckAt: time.Now().Format(time.RFC3339),
	})

	startTime := time.Now()

	// 1. 测试连接（受上层 context 约束）
	if err := m.TestConnectionWithContext(ctx, cfg); err != nil {
		result.Success = false
		result.Error = err.Error()
		result.ResponseTime = time.Since(startTime).Milliseconds()
		m.updateHealthStatus(id, &BackendHealthStatus{
			Status:       "unhealthy",
			LastCheckAt:  time.Now().Format(time.RFC3339),
			LastError:    err.Error(),
			ResponseTime: result.ResponseTime,
		})
		// 探测失败，禁用后端
		m.updateEnabledStatus(id, false)
		return result, nil
	}

	result.ResponseTime = time.Since(startTime).Milliseconds()
	result.Success = true

	// 2. 获取模型列表（如果启用自动获取）——强制远端，避免沿用本地缓存导致「探测不刷新」
	if fetchModels && cfg.AutoFetchModels {
		models, err := m.FetchModelsFromRemote(ctx, cfg)
		if err != nil {
			logger.Warn("Probe backend: failed to fetch models",
				logger.GetField("id", id),
				logger.GetField("error", err.Error()))
			result.Error = "连接成功但获取模型失败: " + err.Error()
		} else {
			result.Models = models
			result.ModelsCount = len(models)

			// 更新 supported_models
			m.updateSupportedModels(id, models)
		}
	}

	// 更新健康状态
	m.updateHealthStatus(id, &BackendHealthStatus{
		Status:       "healthy",
		LastCheckAt:  time.Now().Format(time.RFC3339),
		ResponseTime: result.ResponseTime,
		ModelsCount:  result.ModelsCount,
	})

	// 探测成功，启用后端
	m.updateEnabledStatus(id, true)

	return result, nil
}

// updateHealthStatus 更新后端健康状态（内部方法，需确保调用方已加锁或安全访问）
func (m *Manager) updateHealthStatus(id string, status *BackendHealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, exists := m.backends[id]
	if !exists {
		return
	}
	cfg.HealthStatus = status
}

// updateEnabledStatus 更新后端启用状态（内部方法）
func (m *Manager) updateEnabledStatus(id string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, exists := m.backends[id]
	if !exists {
		return
	}
	cfg.Enabled = enabled
}

// updateSupportedModels 根据获取到的模型更新 supported_models（内部方法）
func (m *Manager) updateSupportedModels(id string, models []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, exists := m.backends[id]
	if !exists {
		return
	}

	// 构建新的模型映射列表
	newModels := make([]ModelMapping, 0, len(models))
	existingMap := make(map[string]bool)

	// 保留已存在的映射关系
	for _, sm := range cfg.SupportedModels {
		existingMap[sm.ActualModel] = true
		existingMap[sm.RequestedModel] = true
	}

	for _, modelName := range models {
		// 检查是否已存在
		found := false
		for _, sm := range cfg.SupportedModels {
			if sm.ActualModel == modelName || sm.RequestedModel == modelName {
				newModels = append(newModels, sm)
				found = true
				break
			}
		}
		if !found {
			// 新增模型
			newModels = append(newModels, ModelMapping{
				RequestedModel: modelName,
				ActualModel:    modelName,
				IsExact:        true,
			})
		}
	}

	cfg.SupportedModels = newModels
}

// UpdateSupportedModels 从获取到的模型名称列表更新后端 supported_models（公开方法，供 FetchModels 等使用）
func (m *Manager) UpdateSupportedModels(id string, models []string) {
	m.updateSupportedModels(id, models)
}

// ProbeAllBackends 批量探测可用后端（有 API Key 的云端后端 + Ollama 本地后端）
func (m *Manager) ProbeAllBackends(ctx context.Context, fetchModels bool) ([]*ProbeResult, error) {
	backends := m.GetAll()
	results := make([]*ProbeResult, 0, len(backends))

	// 过滤：只探测可用的后端（Ollama 无需 Key，其他需要 API Key）
	var probeable []*BackendConfig
	for _, cfg := range backends {
		if cfg.Type == "ollama" {
			probeable = append(probeable, cfg)
			continue
		}
		normKey := NormalizeOpenAICompatibleAPIKey(cfg.APIKey)
		if normKey != "" {
			probeable = append(probeable, cfg)
		}
	}

	logger.Info("Starting batch probe for available backends",
		logger.GetField("total_backends", len(backends)),
		logger.GetField("probeable", len(probeable)))

	type probeItem struct {
		cfg    *BackendConfig
		result *ProbeResult
		err    error
	}
	maxWorkers := 4
	if len(probeable) < maxWorkers {
		maxWorkers = len(probeable)
	}
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	sem := make(chan struct{}, maxWorkers)
	out := make(chan probeItem, len(probeable))
	var wg sync.WaitGroup
	for _, cfg := range probeable {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				out <- probeItem{
					cfg: cfg,
					err: ctx.Err(),
				}
				return
			}
			defer func() { <-sem }()
			result, err := m.ProbeAndUpdateBackend(ctx, cfg.ID, fetchModels)
			out <- probeItem{
				cfg:    cfg,
				result: result,
				err:    err,
			}
		}()
	}
	wg.Wait()
	close(out)

	for item := range out {
		if item.err != nil {
			logger.Error("Probe backend failed",
				logger.GetField("id", item.cfg.ID),
				logger.GetField("error", item.err.Error()))
			results = append(results, &ProbeResult{
				BackendID: item.cfg.ID,
				Name:      item.cfg.Name,
				Success:   false,
				Error:     item.err.Error(),
			})
			continue
		}
		results = append(results, item.result)
	}

	// 保存更新后的配置（包括健康状态和模型列表）
	if err := m.Save(); err != nil {
		logger.Error("Failed to save backend configs after probe", logger.GetField("error", err.Error()))
	}

	logger.Info("Batch probe completed", logger.GetField("total", len(results)), logger.GetField("success", countSuccessful(results)))
	return results, nil
}

// countSuccessful 统计成功的探测结果
func countSuccessful(results []*ProbeResult) int {
	count := 0
	for _, r := range results {
		if r.Success {
			count++
		}
	}
	return count
}

// GetModels 获取后端的模型列表
// Ollama 模型是动态的（用户可随时 pull/rm），每次访问都查询 /api/tags 获取真实列表。
// OpenAI 兼容后端优先使用缓存（模型列表通常不变），缓存为空时才探测远程。
func (m *Manager) GetModels(cfg *BackendConfig) ([]string, error) {
	return m.GetModelsWithContext(context.Background(), cfg)
}

// GetModelsWithContext 获取后端模型列表（支持上层 context 超时/取消）
func (m *Manager) GetModelsWithContext(ctx context.Context, cfg *BackendConfig) ([]string, error) {
	// Ollama 动态模型：始终查询 /api/tags 获取真实列表，不使用缓存
	if cfg.Type == "ollama" {
		return m.FetchModelsFromRemote(ctx, cfg)
	}

	// OpenAI / Gemini 等：优先使用缓存的模型列表（数据库中已保存）
	cachedModels := getSupportedModelNames(cfg)
	if len(cachedModels) > 0 {
		logger.Info("Using cached supported_models", logger.GetField("name", cfg.Name), logger.GetField("count", len(cachedModels)))
		return cachedModels, nil
	}

	// 缓存为空且禁用了自动获取，返回空列表
	if !cfg.AutoFetchModels {
		logger.Info("Auto-fetch disabled and no cached models", logger.GetField("name", cfg.Name))
		return nil, nil
	}

	logger.Info("No cached models, fetching from backend", logger.GetField("name", cfg.Name), logger.GetField("type", cfg.Type))
	return m.FetchModelsFromRemote(ctx, cfg)
}

// FetchModelsFromRemote 强制从远端拉取模型列表（忽略本地 supported_models 缓存）。
// 供编辑对话框「刷新支持的模型」与 Probe 使用，各后端类型统一入口。
func (m *Manager) FetchModelsFromRemote(ctx context.Context, cfg *BackendConfig) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("backend config is nil")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	client := httpclient.NewClientNoProxy(time.Duration(timeout) * time.Second)
	headers := map[string]string{"Content-Type": "application/json"}
	norm := NormalizeOpenAICompatibleAPIKey(cfg.APIKey)

	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "ollama":
		return getOllamaModels(ctx, client, cfg, headers)
	case "gemini":
		if norm != "" {
			headers["x-goog-api-key"] = norm
		}
		return getGeminiModels(ctx, client, cfg, headers)
	default:
		if norm != "" {
			headers["Authorization"] = "Bearer " + norm
		}
		return getOpenAIModels(ctx, client, cfg, headers)
	}
}

// getOllamaModels 获取 Ollama 模型列表
func getOllamaModels(ctx context.Context, client *httpclient.Client, cfg *BackendConfig, headers map[string]string) ([]string, error) {
	baseURL := NormalizeOllamaAPIBase(cfg.BaseURL)
	testURL := baseURL + "/api/tags"

	logger.Info("Fetching Ollama models", logger.GetField("url", testURL))

	resp, err := client.Do(ctx, &httpclient.RequestConfig{
		Method:  "GET",
		URL:     testURL,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}

		if err := json.Unmarshal(resp.Body, &result); err != nil {
			logger.Error("Failed to parse Ollama models response", logger.GetField("error", err))
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		models := make([]string, len(result.Models))
		for i, model := range result.Models {
			models[i] = model.Name
		}

		logger.Info("Successfully fetched Ollama models", logger.GetField("count", len(models)))
		return models, nil
	}

	return nil, fmt.Errorf("failed to fetch models with status %d: %s", resp.StatusCode, string(resp.Body))
}

// getGeminiModels 拉取 Gemini 原生 ListModels（name 形如 models/gemini-2.0-flash）
func getGeminiModels(ctx context.Context, client *httpclient.Client, cfg *BackendConfig, headers map[string]string) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	// 常见误配：写成 …/v1beta/models，统一落到 list models 根
	base = strings.TrimSuffix(base, "/models")
	testURL := base + "/models"
	logger.Info("Fetching Gemini models", logger.GetField("url", testURL))

	resp, err := client.Do(ctx, &httpclient.RequestConfig{
		Method:  "GET",
		URL:     testURL,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gemini models: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch gemini models (HTTP %d): %s", resp.StatusCode, truncateErrBody(resp.Body, 500))
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini models response: %w", err)
	}

	models := make([]string, 0, len(result.Models))
	for _, model := range result.Models {
		name := strings.TrimSpace(model.Name)
		name = strings.TrimPrefix(name, "models/")
		if name != "" && !containsString(models, name) {
			models = append(models, name)
		}
	}
	logger.Info("Successfully fetched Gemini models", logger.GetField("count", len(models)))
	return models, nil
}

// getOpenAIModels 获取 OpenAI 兼容模型列表
func getOpenAIModels(ctx context.Context, client *httpclient.Client, cfg *BackendConfig, headers map[string]string) ([]string, error) {
	var lastStatus int
	var lastBody string
	var lastErr error
	// 与 testOpenAIConnection 一致：部分厂商（如部分阿里云 Coding 接入）不提供 GET /models，仅支持 chat/completions。
	// 若所有候选根对该请求均为 404/405，且配置中有 supported_models，则回退到本地列表而非报错。
	onlyModelsEndpointMissing := true
	gotHTTPResponse := false
	for _, root := range CandidateOpenAIAPIRoots(cfg.BaseURL) {
		testURL := root + "/models"
		logger.Info("Fetching OpenAI models", logger.GetField("url", testURL))

		resp, err := client.Do(ctx, &httpclient.RequestConfig{
			Method:  "GET",
			URL:     testURL,
			Headers: headers,
		})
		if err != nil {
			lastErr = err
			onlyModelsEndpointMissing = false
			continue
		}
		gotHTTPResponse = true
		lastStatus = resp.StatusCode
		lastBody = truncateErrBody(resp.Body, 500)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}

			if err := json.Unmarshal(resp.Body, &result); err != nil {
				logger.Error("Failed to parse OpenAI models response", logger.GetField("error", err))
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			models := make([]string, len(result.Data))
			for i, model := range result.Data {
				models[i] = model.ID
			}

			logger.Info("Successfully fetched OpenAI models", logger.GetField("count", len(models)), logger.GetField("root", root))
			return models, nil
		}
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			onlyModelsEndpointMissing = false
		}
	}
	if onlyModelsEndpointMissing && gotHTTPResponse && len(cfg.SupportedModels) > 0 {
		logger.Warn("GET /models not supported by provider (404/405 on all roots), using supported_models from config",
			logger.GetField("name", cfg.Name),
			logger.GetField("lastStatus", lastStatus))
		return getSupportedModelNames(cfg), nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch models: %w (last HTTP %d: %s)", lastErr, lastStatus, lastBody)
	}
	return nil, fmt.Errorf("failed to fetch models (last HTTP %d): %s", lastStatus, lastBody)
}

// getSupportedModelNames 从配置中提取模型名称列表
func getSupportedModelNames(cfg *BackendConfig) []string {
	if len(cfg.SupportedModels) == 0 {
		return []string{}
	}
	models := make([]string, 0, len(cfg.SupportedModels))
	for _, sm := range cfg.SupportedModels {
		// 优先使用 actual_model，如果没有则使用 requested_model
		modelName := sm.ActualModel
		if modelName == "" {
			modelName = sm.RequestedModel
		}
		if modelName != "" && !containsString(models, modelName) {
			models = append(models, modelName)
		}
	}
	logger.Info("Extracted supported models", logger.GetField("name", cfg.Name), logger.GetField("count", len(models)))
	return models
}

// containsString 检查字符串是否存在于切片中
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
