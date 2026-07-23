package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

const KeyFallbackPolicies = "fallback_policies"

// FallbackPolicyStore 策略持久化（DB 为主，文件为兼容）。
type FallbackPolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*GlobalFallbackPolicy
}

var (
	globalPolicyStore     *FallbackPolicyStore
	globalPolicyStoreOnce sync.Once
)

// GetFallbackPolicyStore 获取全局策略存储单例。
func GetFallbackPolicyStore() *FallbackPolicyStore {
	globalPolicyStoreOnce.Do(func() {
		globalPolicyStore = &FallbackPolicyStore{
			policies: make(map[string]*GlobalFallbackPolicy),
		}
	})
	return globalPolicyStore
}

// Load 从数据库加载所有策略（启动时调用，热生效）。
// 若数据库中无策略，自动创建默认策略。
func (s *FallbackPolicyStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	db := database.Get()
	if db == nil {
		logger.Warn("FallbackPolicyStore: database not initialized, skipping load")
		s.initDefaultPolicies()
		return nil
	}
	scs := db.SystemConfigStore()
	ctx := context.Background()

	raw, err := scs.Get(ctx, KeyFallbackPolicies)
	if err != nil || raw == "" {
		logger.Info("FallbackPolicyStore: no policies found in DB, initializing defaults")
		s.initDefaultPolicies()
		return s.saveLocked()
	}

	var policies []*GlobalFallbackPolicy
	if err := json.Unmarshal([]byte(raw), &policies); err != nil {
		return err
	}

	s.policies = make(map[string]*GlobalFallbackPolicy, len(policies))
	for _, p := range policies {
		s.policies[p.ID] = p
	}

	// 如果数据库为空（反序列化后无策略），也初始化默认值
	if len(s.policies) == 0 {
		logger.Info("FallbackPolicyStore: empty policies in DB, initializing defaults")
		s.initDefaultPolicies()
		return s.saveLocked()
	}

	// 内置策略规则由代码维护：覆盖同 ID 旧规则（例如仍含 {{requested_model}} 的过期配置）
	if s.mergeBuiltinPoliciesLocked() {
		logger.Info("FallbackPolicyStore: builtin policies refreshed from code defaults")
		return s.saveLocked()
	}

	logger.Infof("FallbackPolicyStore: loaded %d policies", len(policies))
	return nil
}

// mergeBuiltinPoliciesLocked 用代码内默认规则刷新内置策略，保留用户对 Enabled 的选择。
func (s *FallbackPolicyStore) mergeBuiltinPoliciesLocked() bool {
	defaults := builtinFallbackPolicies()
	changed := false
	for id, def := range defaults {
		existing := s.policies[id]
		if existing == nil {
			cp := *def
			s.policies[id] = &cp
			changed = true
			continue
		}
		if !fallbackRulesEqual(existing.Rules, def.Rules) ||
			existing.Strategy != def.Strategy ||
			existing.Name != def.Name ||
			existing.Description != def.Description {
			enabled := existing.Enabled
			cp := *def
			cp.Enabled = enabled
			cp.CreatedAt = existing.CreatedAt
			cp.UpdatedAt = time.Now()
			s.policies[id] = &cp
			changed = true
		}
	}
	return changed
}

func fallbackRulesEqual(a, b []FallbackRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Priority != b[i].Priority ||
			a[i].BackendID != b[i].BackendID ||
			a[i].Model != b[i].Model ||
			a[i].TimeoutSec != b[i].TimeoutSec ||
			a[i].MaxRetries != b[i].MaxRetries {
			return false
		}
	}
	return true
}

// Save 持久化所有策略到数据库。
func (s *FallbackPolicyStore) Save() error {
	s.mu.RLock()
	policies := make([]*GlobalFallbackPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	s.mu.RUnlock()

	data, err := json.Marshal(policies)
	if err != nil {
		return err
	}

	db := database.Get()
	if db == nil {
		return nil
	}
	scs := db.SystemConfigStore()
	ctx := context.Background()
	return scs.Set(ctx, KeyFallbackPolicies, string(data))
}

// List 返回所有策略（按创建时间排序）。
func (s *FallbackPolicyStore) List() []*GlobalFallbackPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*GlobalFallbackPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// Get 按 ID 获取策略。
func (s *FallbackPolicyStore) Get(id string) *GlobalFallbackPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policies[id]
}

// GetEnabled 获取启用的策略。
func (s *FallbackPolicyStore) GetEnabled(id string) *GlobalFallbackPolicy {
	p := s.Get(id)
	if p != nil && p.Enabled {
		return p
	}
	return nil
}

// Create 创建策略。
func (s *FallbackPolicyStore) Create(p *GlobalFallbackPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.policies[p.ID]; exists {
		return &FallbackPolicyError{"policy already exists: " + p.ID}
	}
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	s.policies[p.ID] = p
	return s.saveLocked()
}

// Update 更新策略。
func (s *FallbackPolicyStore) Update(p *GlobalFallbackPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.policies[p.ID]
	if !ok {
		return ErrFallbackPolicyNotFound
	}
	p.CreatedAt = existing.CreatedAt
	p.UpdatedAt = time.Now()
	s.policies[p.ID] = p
	return s.saveLocked()
}

// Delete 删除策略。
func (s *FallbackPolicyStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[id]; !ok {
		return ErrFallbackPolicyNotFound
	}
	delete(s.policies, id)
	return s.saveLocked()
}

// saveLocked 在持有写锁时持久化（内部方法）。
func (s *FallbackPolicyStore) saveLocked() error {
	policies := make([]*GlobalFallbackPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	data, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	db := database.Get()
	if db == nil {
		return nil
	}
	scs := db.SystemConfigStore()
	ctx := context.Background()
	return scs.Set(ctx, KeyFallbackPolicies, string(data))
}

func builtinFallbackPolicies() map[string]*GlobalFallbackPolicy {
	now := time.Now()
	return map[string]*GlobalFallbackPolicy{
		"same-model-cross-backend": {
			ID:          "same-model-cross-backend",
			Name:        "同模型跨后端降级",
			Description: "当主后端返回 429/5xx 错误时，按优先级切换到其他后端，模型名保持不变",
			Strategy:    StrategySameModelDifferentBackend,
			Rules: []FallbackRule{
				{Priority: 1, BackendID: "{{system.default_backend}}", Model: "{{requested_model}}"},
			},
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		"same-backend-cross-model": {
			ID:          "same-backend-cross-model",
			Name:        "同后端跨模型降级",
			Description: "主模型失败（含余额不足）时优先 system.fallback_model，再试 fallback_backend",
			Strategy:    StrategySameBackendDifferentModel,
			Rules: []FallbackRule{
				{Priority: 1, BackendID: "{{system.default_backend}}", Model: "{{system.fallback_model}}"},
				{Priority: 2, BackendID: "{{system.fallback_backend}}", Model: "{{system.fallback_model}}"},
				{Priority: 3, BackendID: "{{system.default_backend}}", Model: "{{system.default_model}}"},
			},
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		},
		"openai-series": {
			ID:          "openai-series",
			Name:        "OpenAI 系列降级",
			Description: "GPT-4o → GPT-4o-mini → GPT-3.5-turbo 逐级降级",
			Strategy:    StrategySameBackendDifferentModel,
			Rules: []FallbackRule{
				{Priority: 1, BackendID: "openai", Model: "gpt-4o"},
				{Priority: 2, BackendID: "openai", Model: "gpt-4o-mini"},
				{Priority: 3, BackendID: "openai", Model: "gpt-3.5-turbo"},
			},
			Enabled:   false,
			CreatedAt: now,
			UpdatedAt: now,
		},
		"claude-series": {
			ID:          "claude-series",
			Name:        "Claude 系列降级",
			Description: "Claude Sonnet → Claude Haiku 逐级降级",
			Strategy:    StrategySameBackendDifferentModel,
			Rules: []FallbackRule{
				{Priority: 1, BackendID: "anthropic", Model: "claude-sonnet-4-20250514"},
				{Priority: 2, BackendID: "anthropic", Model: "claude-3-5-haiku-20241022"},
			},
			Enabled:   false,
			CreatedAt: now,
			UpdatedAt: now,
		},
		"local-fallback": {
			ID:          "local-fallback",
			Name:        "本地 Ollama 兜底",
			Description: "远程后端全部不可用时，降级到本地 Ollama 模型",
			Strategy:    StrategyCustomChain,
			Rules: []FallbackRule{
				{Priority: 1, BackendID: "{{system.default_backend}}", Model: "{{requested_model}}"},
				{Priority: 2, BackendID: "ollama-local", Model: "llama3.1:latest"},
			},
			Enabled:   false,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// initDefaultPolicies 初始化默认降级策略（在持有写锁时调用）。
func (s *FallbackPolicyStore) initDefaultPolicies() {
	s.policies = make(map[string]*GlobalFallbackPolicy)
	for id, p := range builtinFallbackPolicies() {
		cp := *p
		s.policies[id] = &cp
	}
	logger.Infof("FallbackPolicyStore: initialized %d default policies", len(s.policies))
}

// BuildAutoPolicy 为指定节点自动构建降级策略（同模型跨后端）。
func (s *FallbackPolicyStore) BuildAutoPolicy(requestedModel string) *GlobalFallbackPolicy {
	rules := make([]FallbackRule, 0)
	// 遍历所有启用的后端，按 priority 排序
	if bm := GetBackendManager(); bm != nil {
		backends := bm.List()
		// 按 Priority 排序
		sort.Slice(backends, func(i, j int) bool {
			return backends[i].Priority < backends[j].Priority
		})
		for i, b := range backends {
			if !b.Enabled {
				continue
			}
			model := requestedModel
			// 使用 SupportedModels 中的第一个模型作为默认
			if model == "" && len(b.SupportedModels) > 0 {
				model = b.SupportedModels[0].RequestedModel
			}
			if model == "" {
				continue // 跳过无模型的后端
			}
			rules = append(rules, FallbackRule{
				Priority:  i + 1,
				BackendID: b.ID,
				Model:     model,
			})
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return &GlobalFallbackPolicy{
		ID:       "__auto__",
		Name:     "自动降级（同模型跨后端）",
		Strategy: StrategySameModelDifferentBackend,
		Rules:    rules,
		Enabled:  true,
	}
}

// ── 兼容旧文件存储 ──────────────────────────────────────────────────────────

// LoadPoliciesFromFile 从文件加载策略（向后兼容旧版配置文件）。
func (s *FallbackPolicyStore) LoadPoliciesFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var policies []*GlobalFallbackPolicy
	if err := json.Unmarshal(data, &policies); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range policies {
		if _, exists := s.policies[p.ID]; !exists {
			s.policies[p.ID] = p
		}
	}
	return nil
}

// SaveToFile 持久化策略到文件（兼容）。
func (s *FallbackPolicyStore) SaveToFile(dir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	policies := make([]*GlobalFallbackPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	data, err := json.MarshalIndent(policies, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "fallback-policies.json"), data, 0600)
}
