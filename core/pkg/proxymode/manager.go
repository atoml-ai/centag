package proxymode

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

// ModeConfig 代理模式配置
type ModeConfig struct {
	Key         string                 `json:"key"`          // 模式关键字，如 #d
	Name        string                 `json:"name"`         // 模式名称
	Type        string                 `json:"type"`         // 模式类型：direct|schedule|match|classify|transparent|fallback|custom
	Description string                 `json:"description"`  // 描述
	Enabled     bool                   `json:"enabled"`      // 是否启用
	Config      map[string]interface{} `json:"config"`       // 模式特定配置
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Validate 验证模式配置
func (m *ModeConfig) Validate() error {
	if m.Key == "" {
		return errors.New("mode key is required")
	}
	if m.Name == "" {
		return errors.New("mode name is required")
	}
	if m.Type == "" {
		return errors.New("mode type is required")
	}
	if err := validateModeKey(m.Key); err != nil {
		return err
	}
	return nil
}

// ModeManager 模式关键字管理器
type ModeManager struct {
	mu              sync.RWMutex
	modes           map[string]*ModeConfig
	protected       map[string]bool // 受保护的关键字（默认模式）
	pipelineDerived map[string]bool // 由流水线同步注册、可在全量同步时清理的模式
}

// NewManager 创建新的模式管理器
func NewManager() *ModeManager {
	mgr := &ModeManager{
		modes:           make(map[string]*ModeConfig),
		protected:       make(map[string]bool),
		pipelineDerived: make(map[string]bool),
	}
	mgr.initDefaultModes()
	return mgr
}

// initDefaultModes 初始化默认模式
func (m *ModeManager) initDefaultModes() {
	defaultModes := []ModeConfig{
		{
			Key:         "#d",
			Name:        "直连后端",
			Type:        "direct",
			Description: "直连已配置后端，并注入网关 system prompt",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#s",
			Name:        "智能调度",
			Type:        "schedule",
			Description: "根据负载和权重自动选择后端",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#m",
			Name:        "模型匹配",
			Type:        "match",
			Description: "根据模型名称匹配最佳后端",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#c",
			Name:        "意图分类",
			Type:        "classify",
			Description: "使用小模型分类后路由到合适后端",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#p",
			Name:        "流水线编排",
			Type:        "pipeline",
			Description: "按 X-Pipeline-ID 或首行 /pipeline:id 执行已注册流水线",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#t",
			Name:        "透明模式",
			Type:        "transparent",
			Description: "直连已配置后端，不注入网关 system prompt",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#tf",
			Name:        "透明模式（快）",
			Type:        "transparent-fast",
			Description: "与 #t 相同：不注入 system prompt",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#raw",
			Name:        "原始HTTP转发",
			Type:        "raw-forward",
			Description: "高级：需 X-Target-URL 或 hostproxy，非标准聊天客户端路径",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#f",
			Name:        "降级容错",
			Type:        "fallback",
			Description: "主后端失败时自动切换到备用后端",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#a",
			Name:        "审核模式",
			Type:        "audit",
			Description: "由执行模型完成请求，审核模型对结果进行审核",
			Enabled:     true,
			Config: map[string]interface{}{
				"executor_backend":   "bigmodel",
				"auditor_backend":    "bigmodel",
				"auditor_model":      "glm-5",
				"audit_prompt":       "", // 使用默认 Prompt
				"auto_retry":         true,
				"max_retries":        2,
				"bypass_on_timeout":  true,
				"audit_timeout_sec":  30,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Key:         "#o",
			Name:        "优化模式",
			Type:        "optimize",
			Description: "由执行模型完成请求，优化模型对结果进行优化后返回",
			Enabled:     true,
			Config: map[string]interface{}{
				"executor_backend":     "bigmodel",
				"optimizer_backend":    "bigmodel",
				"optimizer_model":      "glm-5",
				"optimize_prompt":      "", // 使用默认 Prompt
				"auto_retry":           true,
				"max_retries":          2,
				"bypass_on_timeout":    true,
				"optimize_timeout_sec": 30,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Key:         "#ag",
			Name:        "聚合模式",
			Type:        "aggregator",
			Description: "多模型并行生成，聚合器综合输出",
			Enabled:     true,
			Config: map[string]interface{}{
				"parallel_limit": 4,
				"strategy":       "summarize",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Key:         "#r",
			Name:        "路由模式",
			Type:        "router",
			Description: "按关键词/规则路由到不同生成器",
			Enabled:     true,
			Config: map[string]interface{}{
				"default_backend": "bigmodel",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Key:         "#l",
			Name:        "翻译模式",
			Type:        "translate",
			Description: "生成后翻译为目标语言",
			Enabled:     true,
			Config: map[string]interface{}{
				"target_language": "zh",
				"translator_backend": "bigmodel",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Key:         "#ch",
			Name:        "缓存优先模式",
			Type:        "cache-hit",
			Description: "先读缓存，命中则返回；未命中则生成并写入缓存",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "cache-hit",
			Name:        "缓存优先模式",
			Type:        "cache-hit",
			Description: "仅读取缓存，不生成新内容",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#cm",
			Name:        "缓存写入模式",
			Type:        "cache-mode",
			Description: "生成内容并保存到缓存",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "cache-mode",
			Name:        "缓存写入模式",
			Type:        "cache-mode",
			Description: "生成内容并保存到缓存",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#mem0",
			Name:        "Mem0 记忆存储",
			Type:        "mem0-memory",
			Description: "自动保存对话到 Mem0 记忆系统",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#rag",
			Name:        "RAG 知识库网关",
			Type:        "rag-mode",
			Description: "缓存优先，未命中时检索知识库增强生成",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#agent",
			Name:        "Agent 智能分流",
			Type:        "agent-mode",
			Description: "代码任务走 Pi 沙盒，问答走 Mem0 + LLM",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#pi",
			Name:        "Pi Agent",
			Type:        "pi-agent",
			Description: "直连 Pi 沙盒执行代码任务",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#sec",
			Name:        "安全审核防火墙",
			Type:        "security-mode",
			Description: "入站安全审核 → 生成 → 质量审核 → PII 脱敏",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#cs",
			Name:        "多语言客服",
			Type:        "multilingual-support",
			Description: "缓存 + Mem0 记忆 + 翻译，适用于客服场景",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			Key:         "#geo",
			Name:        "地理路由",
			Type:        "geo-routing-mode",
			Description: "按客户端 IP / X-Geo-Region 选择区域 backend",
			Enabled:     true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, mode := range defaultModes {
		modeCopy := mode
		m.modes[mode.Key] = &modeCopy
		m.protected[mode.Key] = true
	}
}

// ListModes 列出所有模式
func (m *ModeManager) ListModes() []ModeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ModeConfig, 0, len(m.modes))
	for _, mode := range m.modes {
		result = append(result, *mode)
	}
	return result
}

// GetMode 获取指定模式
func (m *ModeManager) GetMode(key string) (*ModeConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mode, exists := m.modes[key]
	if !exists {
		return nil, false
	}
	modeCopy := *mode
	return &modeCopy, true
}

// AddMode 添加新模式
func (m *ModeManager) AddMode(mode ModeConfig) error {
	if err := mode.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.modes[mode.Key]; exists {
		return fmt.Errorf("mode key %s already exists", mode.Key)
	}

	mode.CreatedAt = time.Now()
	mode.UpdatedAt = time.Now()
	m.modes[mode.Key] = &mode
	return nil
}

// UpdateMode 更新模式
func (m *ModeManager) UpdateMode(mode ModeConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.modes[mode.Key]
	if !exists {
		return fmt.Errorf("mode key %s not found", mode.Key)
	}

	existing.Name = mode.Name
	existing.Type = mode.Type
	existing.Description = mode.Description
	existing.Enabled = mode.Enabled
	existing.Config = mode.Config
	existing.UpdatedAt = time.Now()
	return nil
}

// DeleteMode 删除模式
func (m *ModeManager) DeleteMode(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.protected[key] {
		return errors.New("cannot delete protected default mode")
	}

	if _, exists := m.modes[key]; !exists {
		return fmt.Errorf("mode key %s not found", key)
	}

	delete(m.modes, key)
	return nil
}

// EnableMode 启用/禁用模式
func (m *ModeManager) EnableMode(key string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mode, exists := m.modes[key]
	if !exists {
		return fmt.Errorf("mode key %s not found", key)
	}

	mode.Enabled = enabled
	mode.UpdatedAt = time.Now()
	return nil
}

// ValidateModeKey 验证模式关键字格式（# 后跟字母或数字，如 #ch、#mem0）。
func ValidateModeKey(key string) error {
	return validateModeKey(key)
}

// validateModeKey 验证模式关键字格式
func validateModeKey(key string) error {
	if key == "" {
		return errors.New("key cannot be empty")
	}
	if key[0] != '#' {
		return errors.New("key must start with #")
	}
	// key 必须是 # 后跟字母、数字或组合（如 #d, #ag, #mem0）
	matched, _ := regexp.MatchString(`^#[a-zA-Z0-9]+$`, key)
	if !matched {
		return errors.New("key must be # followed by letters or numbers")
	}
	return nil
}

// IsProtected 检查关键字是否受保护
func (m *ModeManager) IsProtected(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.protected[key]
}
