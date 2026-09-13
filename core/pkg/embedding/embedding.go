package embedding

import (
	"context"
	"fmt"
	"strings"
)

// EmbeddingService 嵌入服务接口
type EmbeddingService interface {
	// GetEmbedding 获取文本的嵌入向量
	GetEmbedding(ctx context.Context, text string) ([]float32, error)

	// GetBatchEmbeddings 批量获取嵌入向量
	GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)

	// GetDimension 获取向量维度
	GetDimension() int

	// GetProviderInfo 获取提供者信息
	GetProviderInfo() ProviderInfo
}

// ProviderInfo 嵌入服务提供者信息
type ProviderInfo struct {
	Provider string // openai, ollama
	Model    string // 模型名称
	BaseURL  string // API地址
	Dimension int  // 向量维度
}

// EmbeddingConfig 嵌入配置
type EmbeddingConfig struct {
	// Provider 服务提供商 (openai, ollama, local)
	Provider string `json:"provider" yaml:"provider"`

	// Model 模型名称
	Model string `json:"model" yaml:"model"`

	// BaseURL API 基础URL (仅用于远程API)
	BaseURL string `json:"base_url" yaml:"base_url"`

	// APIKey API密钥 (仅用于远程API)
	APIKey string `json:"api_key" yaml:"api_key"`

	// BatchSize 批量处理大小
	BatchSize int `json:"batch_size" yaml:"batch_size"`

	// Timeout 请求超时时间
	Timeout int `json:"timeout" yaml:"timeout"`

	// Enabled 是否启用
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// DefaultEmbeddingConfig 返回默认配置
func DefaultEmbeddingConfig() *EmbeddingConfig {
	return &EmbeddingConfig{
		Provider:  "openai", // 默认使用OpenAI
		Model:     "text-embedding-3-small",
		BaseURL:   "https://api.openai.com/v1",
		BatchSize: 16,
		Timeout:   30,
		Enabled:   true,
	}
}

// NormalizeText 文本标准化
func NormalizeText(text string) string {
	// 去除多余空格
	text = strings.TrimSpace(text)

	// 去除特殊字符(可选)
	// text = strings.Map(func(r rune) rune {
	// 	if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
	// 		return r
	// 	}
	// 	return -1
	// }, text)

	return text
}

// NewEmbeddingService 根据配置创建 Embedding 服务（工厂函数）
func NewEmbeddingService(config *EmbeddingConfig) (EmbeddingService, error) {
	if config == nil {
		config = DefaultEmbeddingConfig()
	}

	switch config.Provider {
	case "ollama":
		return NewOllamaEmbeddingService(config)
	case "openai", "kimi":
		return NewOpenAIEmbeddingService(config)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.Provider)
	}
}

