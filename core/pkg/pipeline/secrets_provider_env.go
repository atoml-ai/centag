package pipeline

import (
	"fmt"
	"os"
	"strings"
)

// EnvSecretsProvider 从环境变量解析密钥
type EnvSecretsProvider struct {
	prefix string
}

// NewEnvSecretsProvider 创建环境变量密钥提供者
// prefix: 环境变量前缀，例如 "LLM_PROXY_" 或 "SECRET_"
func NewEnvSecretsProvider(prefix string) *EnvSecretsProvider {
	return &EnvSecretsProvider{prefix: prefix}
}

// ResolveSecret 解析密钥引用
// ref 格式:
//   - "API_KEY" -> 查找环境变量 API_KEY 或 {prefix}API_KEY
//   - "env:API_KEY" -> 查找环境变量 API_KEY
//   - "vault:secret_name" -> 暂不支持，返回错误
func (p *EnvSecretsProvider) ResolveSecret(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("empty secret ref")
	}

	// 处理不同格式的引用
	if strings.HasPrefix(ref, "env:") {
		// env:API_KEY 格式
		key := strings.TrimPrefix(ref, "env:")
		return os.Getenv(key), nil
	}

	if strings.HasPrefix(ref, "vault:") {
		// vault:secret_name 格式暂不支持
		return "", fmt.Errorf("vault secrets not supported yet: %s", ref)
	}

	// 直接查找环境变量，先尝试带前缀，再尝试原始值
	if p.prefix != "" {
		if val := os.Getenv(p.prefix + ref); val != "" {
			return val, nil
		}
	}

	// 尝试原始值
	if val := os.Getenv(ref); val != "" {
		return val, nil
	}

	return "", fmt.Errorf("secret not found: %s", ref)
}
