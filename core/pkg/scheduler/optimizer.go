package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
	"centag/core/pkg/utils"

	"go.uber.org/zap"
)

// Optimizer 优化器
type Optimizer struct {
	config     *OptimizeConfig
	backendMgr *backend.Manager
	client     *http.Client
	stats      *OptimizeStats
}

// optimizeRequest 优化请求结构
type optimizeRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Format  string `json:"format,omitempty"`
	Options struct {
		Temperature float64 `json:"temperature,omitempty"`
		TopP        float64 `json:"top_p,omitempty"`
	} `json:"options,omitempty"`
}

// optimizeResponse 优化响应结构（Ollama 格式）
type optimizeResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

// openAIChatRequest OpenAI 格式的聊天请求
type openAIChatRequestForOptimize struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// openAIChatResponse OpenAI 格式的聊天响应
type openAIChatResponseForOptimize struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewOptimizer 创建优化器
func NewOptimizer(config *OptimizeConfig, backendMgr *backend.Manager) *Optimizer {
	if config == nil {
		config = DefaultOptimizeConfig()
	}
	if config.OptimizeTimeoutSec == 0 {
		config.OptimizeTimeoutSec = 60
	}

	return &Optimizer{
		config:     config,
		backendMgr: backendMgr,
		client: &http.Client{
			Timeout: time.Duration(config.OptimizeTimeoutSec) * time.Second,
			Transport: &http.Transport{
				Proxy: func(r *http.Request) (*url.URL, error) {
					return nil, nil // 不使用任何代理，直连目标
				},
			},
		},
		stats: &OptimizeStats{},
	}
}

// Optimize 执行优化
// 返回优化后的答案文本，如果优化失败则返回原始答案和错误
func (o *Optimizer) Optimize(
	ctx context.Context,
	originalQuestion string,
	originalAnswer string,
	executorModel string,
) (*OptimizeResult, error) {
	startTime := time.Now()

	logger.Info("Starting optimization",
		zap.String("question", utils.TruncateString(originalQuestion, 50)),
		zap.String("executor_model", executorModel),
		zap.String("optimizer_backend", o.config.OptimizerBackend),
		zap.String("optimizer_model", o.config.OptimizerModel))

	// 1. 构建优化 Prompt
	prompt := o.buildOptimizePrompt(originalQuestion, originalAnswer)

	// 2. 调用优化模型
	optimizedResponse, err := o.callOptimizerModel(ctx, prompt)
	if err != nil {
		logger.Warn("Optimize call failed", zap.Error(err))
		return nil, fmt.Errorf("optimize call failed: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	// 3. 解析优化结果
	result := &OptimizeResult{
		Optimized:     true,
		Original:      originalAnswer,
		OptimizedText: strings.TrimSpace(optimizedResponse),
		RawResponse:   optimizedResponse,
		DurationMs:    durationMs,
	}

	// 如果优化结果为空，使用原始答案
	if result.OptimizedText == "" {
		logger.Warn("Optimize returned empty result, using original answer")
		result.OptimizedText = originalAnswer
		result.Optimized = false
	}

	// 4. 更新统计
	o.updateStats(result, durationMs)

	logger.Info("Optimization completed",
		zap.Bool("improved", result.Optimized),
		zap.Int64("duration_ms", durationMs))

	return result, nil
}

// buildOptimizePrompt 构建优化 Prompt
func (o *Optimizer) buildOptimizePrompt(question, answer string) string {
	prompt := o.config.OptimizePrompt
	if prompt == "" {
		prompt = DefaultOptimizePrompt
	}

	// 替换模板变量
	prompt = strings.ReplaceAll(prompt, "{{.question}}", question)
	prompt = strings.ReplaceAll(prompt, "{{.answer}}", answer)
	prompt = strings.ReplaceAll(prompt, "{{.timestamp}}", time.Now().Format(time.RFC3339))

	return prompt
}

// callOptimizerModel 调用优化模型
func (o *Optimizer) callOptimizerModel(ctx context.Context, prompt string) (string, error) {
	// 获取优化后端配置
	optimizerBackend := o.getOptimizerBackend()
	if optimizerBackend == nil {
		return "", fmt.Errorf("optimizer backend not found: %s", o.config.OptimizerBackend)
	}

	// 确定优化模型（使用后端的模型映射）
	optimizeModel := o.getActualModelName(optimizerBackend, o.config.OptimizerModel)
	if optimizeModel == "" {
		optimizeModel = o.config.OptimizerModel
	}

	// 获取后端 API 地址
	apiURL := o.buildAPIURL(optimizerBackend)

	logger.Infof("[Optimizer] Making request - backend: %s, type: %s, model: %s, url: %s",
		optimizerBackend.Name, optimizerBackend.Type, optimizeModel, apiURL)

	// 根据后端类型选择不同的请求格式
	var jsonData []byte
	var err error

	if optimizerBackend.Type == "ollama" {
		// Ollama 格式
		reqBody := optimizeRequest{
			Model:  optimizeModel,
			Prompt: prompt,
			Stream: false,
		}
		reqBody.Options.Temperature = 0.3 // 稍微高一点的温度以获得更有创意的优化
		reqBody.Options.TopP = 0.9
		jsonData, err = json.Marshal(reqBody)
	} else {
		// OpenAI 兼容格式 - 使用 messages 数组
		reqBody := openAIChatRequestForOptimize{
			Model: optimizeModel,
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "user", Content: prompt},
			},
			Temperature: 0.3,
		}
		jsonData, err = json.Marshal(reqBody)
	}

	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	logger.Infof("[Optimizer] Request body: %s", string(jsonData))

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(optimizerBackend.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	logger.Infof("[Optimizer] Response - status: %d, body: %s", resp.StatusCode, utils.TruncateString(bodyStr, 500))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("optimizer returned status %d: %s", resp.StatusCode, bodyStr)
	}

	// 根据后端类型解析响应
	if optimizerBackend.Type == "ollama" {
		var ollamaResp optimizeResponse
		if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
			return "", fmt.Errorf("decode response failed: %w", err)
		}
		return ollamaResp.Response, nil
	} else {
		// OpenAI 兼容格式
		var openAIResp openAIChatResponseForOptimize
		if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
			return "", fmt.Errorf("decode response failed: %w", err)
		}
		if len(openAIResp.Choices) > 0 {
			return openAIResp.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("no response content from optimizer")
	}
}

// buildAPIURL 构建 API URL
func (o *Optimizer) buildAPIURL(backend *backend.BackendConfig) string {
	baseURL := backend.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:21434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// 检查 baseURL 是否已经包含 /v1 或 /api 前缀
	hasV1Prefix := strings.Contains(baseURL, "/v1")
	hasAPIPrefix := strings.Contains(baseURL, "/api")

	// 根据后端类型选择 API 路径
	switch backend.Type {
	case "ollama":
		if hasAPIPrefix {
			return baseURL + "/generate"
		}
		return baseURL + "/api/generate"
	case "openai", "anthropic", "":
		// OpenAI 兼容格式
		if hasV1Prefix {
			return baseURL + "/chat/completions"
		}
		return baseURL + "/v1/chat/completions"
	default:
		// 默认使用 OpenAI 兼容格式
		if hasV1Prefix {
			return baseURL + "/chat/completions"
		}
		return baseURL + "/v1/chat/completions"
	}
}

// getOptimizerBackend 获取优化后端配置
func (o *Optimizer) getOptimizerBackend() *backend.BackendConfig {
	if o.backendMgr == nil {
		return nil
	}

	backends := o.backendMgr.List()
	for _, b := range backends {
		if b.ID == o.config.OptimizerBackend && b.Enabled {
			return b
		}
	}

	return nil
}

// getActualModelName 获取实际的模型名称
// 如果配置的模型在后端的模型映射中存在，则返回映射的 ActualModel
// 否则返回原始模型名称
func (o *Optimizer) getActualModelName(backend *backend.BackendConfig, requestedModel string) string {
	if requestedModel == "" || len(backend.SupportedModels) == 0 {
		// 没有指定模型或没有模型映射，使用后端的第一个模型
		if len(backend.SupportedModels) > 0 {
			return backend.SupportedModels[0].ActualModel
		}
		return ""
	}

	// 在模型映射中查找
	for _, mapping := range backend.SupportedModels {
		// 优先精确匹配 RequestedModel
		if mapping.RequestedModel == requestedModel && mapping.ActualModel != "" {
			logger.Infof("[Optimizer] Model mapping found: %s -> %s", requestedModel, mapping.ActualModel)
			return mapping.ActualModel
		}
	}

	// 如果没有找到映射，直接使用原始模型名称
	return requestedModel
}

// updateStats 更新优化统计
func (o *Optimizer) updateStats(result *OptimizeResult, durationMs int64) {
	var action OptimizeAction
	if result.Optimized {
		action = OptimizeActionOptimized
	} else {
		action = OptimizeActionBypass
	}
	o.stats.UpdateStats(result, durationMs, action)
}

// GetStats 获取优化统计
func (o *Optimizer) GetStats() *OptimizeStats {
	return o.stats
}
