package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaEmbeddingService Ollama 嵌入服务实现
type OllamaEmbeddingService struct {
	config    *EmbeddingConfig
	client    *http.Client
	similarity Similarity
	dimension int
}

// NewOllamaEmbeddingService 创建 Ollama 嵌入服务
func NewOllamaEmbeddingService(config *EmbeddingConfig) (*OllamaEmbeddingService, error) {
	if config == nil {
		config = DefaultEmbeddingConfig()
		config.Provider = "ollama"
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:21434"
	}

	if config.Model == "" {
		config.Model = "nomic-embed-text" // 默认模型
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &OllamaEmbeddingService{
		config:    config,
		client:    client,
		similarity: &DefaultSimilarity{},
		dimension: 768, // nomic-embed-text 默认维度
	}, nil
}

// GetEmbedding 获取文本的嵌入向量
func (s *OllamaEmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// 标准化文本
	text = NormalizeText(text)

	// 构造请求
	reqBody := map[string]interface{}{
		"model": s.config.Model,
		"prompt": text,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("%s/api/embeddings", s.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result OllamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Embedding == nil {
		return nil, fmt.Errorf("no embedding returned from API")
	}

	// 更新维度
	if s.dimension == 0 {
		s.dimension = len(result.Embedding)
	}

	return result.Embedding, nil
}

// GetBatchEmbeddings 批量获取嵌入向量
func (s *OllamaEmbeddingService) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	// Ollama 不支持批量API,需要逐个请求
	results := make([][]float32, 0, len(texts))

	for i, text := range texts {
		embedding, err := s.GetEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to get embedding for text %d: %w", i, err)
		}
		results = append(results, embedding)
	}

	return results, nil
}

// GetDimension 获取向量维度
func (s *OllamaEmbeddingService) GetDimension() int {
	return s.dimension
}

// GetProviderInfo 获取提供者信息
func (s *OllamaEmbeddingService) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Provider:  "ollama",
		Model:     s.config.Model,
		BaseURL:   s.config.BaseURL,
		Dimension: s.dimension,
	}
}

// OllamaEmbeddingResponse Ollama 嵌入API响应
type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}
