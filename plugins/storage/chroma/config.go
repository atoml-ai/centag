package chroma

import (
	"fmt"
	"time"
)

// LoadConfig 从map加载配置
func LoadConfig(cfgMap map[string]interface{}) (*Config, error) {
	cfg := &Config{
		Collection: "llm_cache",
		Timeout:    30 * time.Second,
	}

	// 解析地址
	if addr, ok := cfgMap["addr"].(string); ok && addr != "" {
		cfg.Addr = addr
	} else {
		return nil, fmt.Errorf("missing required field: addr")
	}

	// 解析集合名称
	if collection, ok := cfgMap["collection"].(string); ok && collection != "" {
		cfg.Collection = collection
	}

	// 解析超时
	if timeout, ok := cfgMap["timeout"].(float64); ok && timeout > 0 {
		cfg.Timeout = time.Duration(timeout) * time.Second
	}

	// 解析认证token
	if token, ok := cfgMap["token"].(string); ok && token != "" {
		cfg.Token = token
	}

	return cfg, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("address cannot be empty")
	}
	if c.Collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}
