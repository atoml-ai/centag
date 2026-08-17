package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// RoutingStrategy 路由策略接口
type RoutingStrategy interface {
	// Classify 分类用户输入，返回路由目标和置信度
	Classify(ctx context.Context, content string) (string, float64, error)

	// GetStrategyName 获取策略名称
	GetStrategyName() string

	// Validate 验证配置
	Validate() error
}

// KeywordStrategy 关键词策略
type KeywordStrategy struct {
	// Routes 路由规则映射
	Routes map[string]string
	// MatchMode 匹配模式：contains（包含）/ prefix（前缀）/ regex（正则）
	MatchMode string
}

// NewKeywordStrategy 创建关键词策略
func NewKeywordStrategy(routes map[string]string, matchMode string) *KeywordStrategy {
	if matchMode == "" {
		matchMode = "contains"
	}
	return &KeywordStrategy{
		Routes:    routes,
		MatchMode: matchMode,
	}
}

// Classify 分类用户输入
func (s *KeywordStrategy) Classify(ctx context.Context, content string) (string, float64, error) {
	if s.Routes == nil {
		return "", 0, fmt.Errorf("routes not configured")
	}

	content = strings.ToLower(content)

	for keyword, target := range s.Routes {
		keyword = strings.ToLower(keyword)

		switch s.MatchMode {
		case "contains":
			if strings.Contains(content, keyword) {
				return target, 1.0, nil
			}
		case "prefix":
			if strings.HasPrefix(content, keyword) {
				return target, 1.0, nil
			}
		case "regex":
			matched, err := regexp.MatchString(keyword, content)
			if err != nil {
				continue
			}
			if matched {
				return target, 1.0, nil
			}
		}
	}

	return "", 0, nil
}

// GetStrategyName 获取策略名称
func (s *KeywordStrategy) GetStrategyName() string {
	return "keyword_" + s.MatchMode
}

// Validate 验证配置
func (s *KeywordStrategy) Validate() error {
	if s.Routes == nil || len(s.Routes) == 0 {
		return fmt.Errorf("routes cannot be empty")
	}

	validModes := map[string]bool{
		"contains": true,
		"prefix":   true,
		"regex":    true,
	}
	if !validModes[s.MatchMode] {
		return fmt.Errorf("invalid match mode: %s", s.MatchMode)
	}

	return nil
}

// LLMClassifyStrategy LLM分类策略
type LLMClassifyStrategy struct {
	// Backend LLM后端
	Backend string
	// Model LLM模型
	Model string
	// Prompt 分类提示词
	Prompt string
	// Routes 路由规则映射
	Routes map[string]string
}

// NewLLMClassifyStrategy 创建LLM分类策略
func NewLLMClassifyStrategy(backend, model, prompt string, routes map[string]string) *LLMClassifyStrategy {
	if prompt == "" {
		prompt = "请根据用户输入判断意图，返回对应的路由目标。"
	}
	return &LLMClassifyStrategy{
		Backend: backend,
		Model:   model,
		Prompt:  prompt,
		Routes:  routes,
	}
}

// Classify 分类用户输入
func (s *LLMClassifyStrategy) Classify(ctx context.Context, content string) (string, float64, error) {
	// TODO: 实现LLM分类逻辑
	// 这里只是占位符，实际实现需要调用LLM API
	return "", 0, fmt.Errorf("LLM classify not implemented yet")
}

// GetStrategyName 获取策略名称
func (s *LLMClassifyStrategy) GetStrategyName() string {
	return "llm_classify"
}

// Validate 验证配置
func (s *LLMClassifyStrategy) Validate() error {
	if s.Backend == "" {
		return fmt.Errorf("backend cannot be empty")
	}
	if s.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}
	if s.Routes == nil || len(s.Routes) == 0 {
		return fmt.Errorf("routes cannot be empty")
	}

	return nil
}

// HybridStrategy 混合策略
type HybridStrategy struct {
	// KeywordStrategy 关键词策略
	KeywordStrategy *KeywordStrategy
	// LLMStrategy LLM分类策略
	LLMStrategy *LLMClassifyStrategy
	// ConfidenceThreshold 置信度阈值（低于此值时使用LLM分类）
	ConfidenceThreshold float64
}

// NewHybridStrategy 创建混合策略
func NewHybridStrategy(keywordStrategy *KeywordStrategy, llmStrategy *LLMClassifyStrategy, threshold float64) *HybridStrategy {
	if threshold <= 0 {
		threshold = 0.55
	}
	return &HybridStrategy{
		KeywordStrategy:     keywordStrategy,
		LLMStrategy:         llmStrategy,
		ConfidenceThreshold: threshold,
	}
}

// Classify 分类用户输入
func (s *HybridStrategy) Classify(ctx context.Context, content string) (string, float64, error) {
	// 先尝试关键词匹配
	target, confidence, err := s.KeywordStrategy.Classify(ctx, content)
	if err == nil && confidence >= s.ConfidenceThreshold {
		return target, confidence, nil
	}

	// 关键词匹配置信度不足，使用LLM分类
	if s.LLMStrategy != nil {
		return s.LLMStrategy.Classify(ctx, content)
	}

	// 没有LLM策略，返回关键词结果（即使置信度不足）
	return target, confidence, nil
}

// GetStrategyName 获取策略名称
func (s *HybridStrategy) GetStrategyName() string {
	return "keyword_then_intent"
}

// Validate 验证配置
func (s *HybridStrategy) Validate() error {
	if s.KeywordStrategy == nil {
		return fmt.Errorf("keyword strategy cannot be nil")
	}
	if err := s.KeywordStrategy.Validate(); err != nil {
		return fmt.Errorf("keyword strategy validation failed: %w", err)
	}

	if s.LLMStrategy != nil {
		if err := s.LLMStrategy.Validate(); err != nil {
			return fmt.Errorf("LLM strategy validation failed: %w", err)
		}
	}

	if s.ConfidenceThreshold < 0 || s.ConfidenceThreshold > 1 {
		return fmt.Errorf("confidence threshold must be between 0 and 1")
	}

	return nil
}

// RoutingStrategyRegistry 路由策略注册表
type RoutingStrategyRegistry struct {
	strategies map[string]RoutingStrategy
}

// NewRoutingStrategyRegistry 创建路由策略注册表
func NewRoutingStrategyRegistry() *RoutingStrategyRegistry {
	return &RoutingStrategyRegistry{
		strategies: make(map[string]RoutingStrategy),
	}
}

// Register 注册策略
func (r *RoutingStrategyRegistry) Register(name string, strategy RoutingStrategy) {
	r.strategies[name] = strategy
}

// Get 获取策略
func (r *RoutingStrategyRegistry) Get(name string) (RoutingStrategy, bool) {
	strategy, ok := r.strategies[name]
	return strategy, ok
}

// GetAll 获取所有策略
func (r *RoutingStrategyRegistry) GetAll() map[string]RoutingStrategy {
	return r.strategies
}

// Validate 验证所有策略
func (r *RoutingStrategyRegistry) Validate() error {
	for name, strategy := range r.strategies {
		if err := strategy.Validate(); err != nil {
			return fmt.Errorf("strategy %s validation failed: %w", name, err)
		}
	}
	return nil
}

// 全局路由策略注册表
var globalRoutingStrategyRegistry *RoutingStrategyRegistry

// init 初始化全局路由策略注册表
func init() {
	globalRoutingStrategyRegistry = NewRoutingStrategyRegistry()

	// 注册内置策略
	globalRoutingStrategyRegistry.Register("keyword_contains", NewKeywordStrategy(nil, "contains"))
	globalRoutingStrategyRegistry.Register("keyword_prefix", NewKeywordStrategy(nil, "prefix"))
	globalRoutingStrategyRegistry.Register("regex_only", NewKeywordStrategy(nil, "regex"))
	globalRoutingStrategyRegistry.Register("llm_classify", NewLLMClassifyStrategy("", "", "", nil))
	globalRoutingStrategyRegistry.Register("keyword_then_intent", NewHybridStrategy(nil, nil, 0))
}

// GetRoutingStrategyRegistry 获取全局路由策略注册表
func GetRoutingStrategyRegistry() *RoutingStrategyRegistry {
	return globalRoutingStrategyRegistry
}