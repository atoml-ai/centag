package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"centag/core/pkg/backend"
)

// OpenAIEmbeddingService OpenAI 嵌入服务实现
type OpenAIEmbeddingService struct {
	config    *EmbeddingConfig
	client    *http.Client
	similarity Similarity
	dimension int
}

// NewOpenAIEmbeddingService 创建 OpenAI 兼容嵌入服务（支持 OpenAI、Kimi 等）
func NewOpenAIEmbeddingService(config *EmbeddingConfig) (*OpenAIEmbeddingService, error) {
	if config == nil {
		config = DefaultEmbeddingConfig()
	}

	// 确定维度
	dimension := 1536 // text-embedding-3-small 默认维度
	switch config.Model {
	case "text-embedding-3-large":
		dimension = 3072
	case "text-embedding-ada-002":
		dimension = 1536
	case "text-embedding":
		// Kimi embedding (bge_m3_embed) 1024维
		dimension = 1024
	}

	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	return &OpenAIEmbeddingService{
		config:     config,
		client:     client,
		similarity: &DefaultSimilarity{},
		dimension:  dimension,
	}, nil
}

// GetEmbedding 获取文本的嵌入向量
func (s *OpenAIEmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// 标准化文本
	text = NormalizeText(text)

	// 构造请求
	reqBody := map[string]interface{}{
		"input": text,
		"model": s.config.Model,
	}

	reqBodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	url := fmt.Sprintf("%s/embeddings", s.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(s.config.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

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
	var result OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned from API")
	}

	return result.Data[0].Embedding, nil
}

// GetBatchEmbeddings 批量获取嵌入向量
func (s *OpenAIEmbeddingService) GetBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	// 标准化文本
	normalizedTexts := make([]string, len(texts))
	for i, text := range texts {
		normalizedTexts[i] = NormalizeText(text)
	}

	// 分批处理
	batchSize := s.config.BatchSize
	if batchSize <= 0 {
		batchSize = 16
	}

	results := make([][]float32, 0, len(texts))

	for i := 0; i < len(normalizedTexts); i += batchSize {
		end := i + batchSize
		if end > len(normalizedTexts) {
			end = len(normalizedTexts)
		}

		batch := normalizedTexts[i:end]

		// 构造请求
		reqBody := map[string]interface{}{
			"input": batch,
			"model": s.config.Model,
		}

		reqBodyJSON, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		// 创建HTTP请求
		url := fmt.Sprintf("%s/embeddings", s.config.BaseURL)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyJSON))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		apiKey := backend.NormalizeOpenAICompatibleAPIKey(s.config.APIKey)
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// 发送请求
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// 解析响应
		var result OpenAIEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		resp.Body.Close()

		// 添加结果
		for _, data := range result.Data {
			results = append(results, data.Embedding)
		}
	}

	return results, nil
}

// GetDimension 获取向量维度
func (s *OpenAIEmbeddingService) GetDimension() int {
	return s.dimension
}

// GetProviderInfo 获取提供者信息
func (s *OpenAIEmbeddingService) GetProviderInfo() ProviderInfo {
	return ProviderInfo{
		Provider:  "openai",
		Model:     s.config.Model,
		BaseURL:   s.config.BaseURL,
		Dimension: s.dimension,
	}
}

// OpenAIEmbeddingResponse OpenAI 嵌入API响应
type OpenAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
