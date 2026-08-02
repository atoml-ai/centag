package billing

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// PersonalPricingConfig 用于 centag Personal 版的定价配置
type PersonalPricingConfig struct {
	Version          string        `yaml:"version"`
	Currency         string        `yaml:"currency"`
	USDToCNY         float64       `yaml:"usd_to_cny,omitempty"`
	AllowedBackends  []string      `yaml:"allowed_backends,omitempty"`
	AllowedModels    []string      `yaml:"allowed_models,omitempty"`
	AllowedPipelines []string      `yaml:"allowed_pipelines,omitempty"`
	Rules            []PricingRule `yaml:"rules"`
}

// ParsePersonalPricingConfig 解析 YAML 配置
func ParsePersonalPricingConfig(data []byte) (*PersonalPricingConfig, error) {
	var cfg PersonalPricingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse pricing config: %w", err)
	}

	// 设置默认值
	if cfg.Currency == "" {
		cfg.Currency = DefaultPricingCurrency
	}

	// 为每条规则设置默认值
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		if rule.PriceType == "" {
			rule.PriceType = PriceTypeCost
		}
		if rule.Currency == "" {
			rule.Currency = cfg.Currency
		}
		if rule.Source == "" {
			rule.Source = PriceSourceConfig
		}
	}

	return &cfg, nil
}

// LoadPersonalPricingConfig 从文件加载配置
func LoadPersonalPricingConfig(path string) (*PersonalPricingConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		log.Printf("pricing config not found at %s, using empty defaults", path)
		return &PersonalPricingConfig{
			Version:  "2.0",
			Currency: DefaultPricingCurrency,
			Rules:    []PricingRule{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read pricing config: %w", err)
	}

	return ParsePersonalPricingConfig(data)
}

// IsAllowed 检查资源是否在允许列表中
// 如果 allowed_* 列表为空，则默认全部可用
func (c *PersonalPricingConfig) IsAllowed(backendID, model, pipelineID string) bool {
	if len(c.AllowedBackends) > 0 {
		allowed := false
		for _, id := range c.AllowedBackends {
			if id == backendID {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	if len(c.AllowedModels) > 0 {
		allowed := false
		for _, m := range c.AllowedModels {
			if m == model || m == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	if len(c.AllowedPipelines) > 0 && pipelineID != "" {
		allowed := false
		for _, id := range c.AllowedPipelines {
			if id == pipelineID {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	return true
}

// GetRulesByModel 获取指定模型的规则
func (c *PersonalPricingConfig) GetRulesByModel(backendID, model string) []*PricingRule {
	var result []*PricingRule
	for i := range c.Rules {
		rule := &c.Rules[i]
		if rule.BackendID == backendID && rule.Enabled {
			if rule.Model == model || rule.Model == "*" {
				result = append(result, rule)
			}
		}
	}
	return result
}

// GetRulesByType 获取指定价格类型的规则
func (c *PersonalPricingConfig) GetRulesByType(priceType PriceType) []*PricingRule {
	var result []*PricingRule
	for i := range c.Rules {
		rule := &c.Rules[i]
		if rule.PriceType == priceType && rule.Enabled {
			result = append(result, rule)
		}
	}
	return result
}
