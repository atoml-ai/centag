package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
	"centag/core/pkg/utils"

	"go.uber.org/zap"
)

// Auditor 审核器
type Auditor struct {
	config     *AuditConfig
	backendMgr *backend.Manager
	client     *http.Client
	stats      *AuditStats
}

// auditRequest 审核请求结构
type auditRequest struct {
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Stream  bool   `json:"stream"`
	Format  string `json:"format,omitempty"`
	Options struct {
		Temperature float64 `json:"temperature,omitempty"`
		TopP        float64 `json:"top_p,omitempty"`
	} `json:"options,omitempty"`
}

// auditResponse 审核响应结构（Ollama 格式）
type auditResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

// openAIChatRequest OpenAI 格式的聊天请求
type openAIChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// openAIChatResponse OpenAI 格式的聊天响应
type openAIChatResponse struct {
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

// auditOutput 审核输出结构（从模型响应解析）
type auditOutput struct {
	Passed      bool     `json:"passed"`
	Score       float64  `json:"score"`
	Feedback    string   `json:"feedback"`
	Suggestions []string `json:"suggestions"`
	Issues      []string `json:"issues"`
}

// NewAuditor 创建审核器
func NewAuditor(config *AuditConfig, backendMgr *backend.Manager) *Auditor {
	if config == nil {
		config = DefaultAuditConfig()
	}
	if config.AuditTimeoutSec == 0 {
		config.AuditTimeoutSec = 30
	}

	return &Auditor{
		config:     config,
		backendMgr: backendMgr,
		client: &http.Client{
			Timeout: time.Duration(config.AuditTimeoutSec) * time.Second,
			Transport: &http.Transport{
				Proxy: func(r *http.Request) (*url.URL, error) {
					return nil, nil // 不使用任何代理，直连目标
				},
			},
		},
		stats: &AuditStats{},
	}
}

// Audit 执行审核
func (a *Auditor) Audit(
	ctx context.Context,
	originalQuestion string,
	originalAnswer string,
	executorModel string,
) (*AuditResult, error) {
	startTime := time.Now()

	logger.Info("Starting audit",
		zap.String("question", utils.TruncateString(originalQuestion, 50)),
		zap.String("executor_model", executorModel),
		zap.String("auditor_backend", a.config.AuditorBackendID),
		zap.String("auditor_model", a.config.AuditorModel))

	// 1. 构建审核 Prompt
	prompt := a.buildAuditPrompt(originalQuestion, originalAnswer)

	// 2. 调用审核模型
	auditResponse, err := a.callAuditorModel(ctx, prompt)
	if err != nil {
		logger.Warn("Audit call failed", zap.Error(err))
		return nil, fmt.Errorf("audit call failed: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	// 3. 解析审核结果
	result := a.parseAuditResult(auditResponse, originalAnswer)
	result.DurationMs = durationMs
	result.RawResponse = auditResponse

	// 4. 更新统计
	a.updateStats(result, durationMs)

	logger.Info("Audit completed",
		zap.Bool("passed", result.Passed),
		zap.Float64("score", result.Score),
		zap.Int64("duration_ms", durationMs))

	return result, nil
}

// buildAuditPrompt 构建审核 Prompt
func (a *Auditor) buildAuditPrompt(question, answer string) string {
	prompt := a.config.AuditPrompt
	if prompt == "" {
		prompt = DefaultAuditPrompt
	}

	// 替换模板变量
	prompt = strings.ReplaceAll(prompt, "{{.question}}", question)
	prompt = strings.ReplaceAll(prompt, "{{.answer}}", answer)
	prompt = strings.ReplaceAll(prompt, "{{.timestamp}}", time.Now().Format(time.RFC3339))

	return prompt
}

// callAuditorModel 调用审核模型
func (a *Auditor) callAuditorModel(ctx context.Context, prompt string) (string, error) {
	// 获取审核后端配置
	auditorBackend := a.getAuditorBackend()
	if auditorBackend == nil {
		return "", fmt.Errorf("auditor backend not found: %s", a.config.AuditorBackendID)
	}

	// 确定审核模型
	auditModel := a.config.AuditorModel
	if auditModel == "" {
		if len(auditorBackend.SupportedModels) > 0 {
			auditModel = auditorBackend.SupportedModels[0].ActualModel
		} else {
			auditModel = "gpt-4"
		}
	}

	// 获取后端 API 地址
	apiURL := a.buildAPIURL(auditorBackend)

	logger.Infof("[Auditor] Making request - backend: %s, type: %s, model: %s, url: %s",
		auditorBackend.Name, auditorBackend.Type, auditModel, apiURL)

	// 根据后端类型选择不同的请求格式
	var jsonData []byte
	var err error

	if auditorBackend.Type == "ollama" {
		// Ollama 格式
		reqBody := auditRequest{
			Model:  auditModel,
			Prompt: prompt,
			Stream: false,
			Format: "json",
		}
		reqBody.Options.Temperature = 0.1
		reqBody.Options.TopP = 0.9
		jsonData, err = json.Marshal(reqBody)
	} else {
		// OpenAI 兼容格式 - 使用 messages 数组
		reqBody := openAIChatRequest{
			Model: auditModel,
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "user", Content: prompt},
			},
			Temperature: 0.1,
		}
		jsonData, err = json.Marshal(reqBody)
	}

	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	logger.Infof("[Auditor] Request body: %s", string(jsonData))

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(auditorBackend.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体（无论成功还是失败都记录）
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	logger.Infof("[Auditor] Response - status: %d, body: %s", resp.StatusCode, utils.TruncateString(bodyStr, 500))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auditor returned status %d: %s", resp.StatusCode, bodyStr)
	}

	// 根据后端类型解析响应
	if auditorBackend.Type == "ollama" {
		var ollamaResp auditResponse
		if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
			return "", fmt.Errorf("decode response failed: %w", err)
		}
		return ollamaResp.Response, nil
	} else {
		// OpenAI 兼容格式
		var openAIResp openAIChatResponse
		if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
			return "", fmt.Errorf("decode response failed: %w", err)
		}
		if len(openAIResp.Choices) > 0 {
			return openAIResp.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("no response content from auditor")
	}
}

// buildAPIURL 构建 API URL
func (a *Auditor) buildAPIURL(backend *backend.BackendConfig) string {
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

// getAuditorBackend 获取审核后端配置
func (a *Auditor) getAuditorBackend() *backend.BackendConfig {
	if a.backendMgr == nil {
		return nil
	}

	backends := a.backendMgr.List()
	for _, b := range backends {
		if b.ID == a.config.AuditorBackendID && b.Enabled {
			return b
		}
	}

	return nil
}

// parseAuditResult 解析审核结果
func (a *Auditor) parseAuditResult(response, originalAnswer string) *AuditResult {
	// 清理响应文本
	cleaned := a.cleanJSONResponse(response)

	// 尝试解析 JSON
	var output auditOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		logger.Warn("Failed to parse audit JSON",
			zap.Error(err),
			zap.String("raw_response", utils.TruncateString(response, 200)))
		// 使用正则提取关键字段
		return a.parseWithRegex(response, originalAnswer)
	}

	// 验证并转换结果
	result := &AuditResult{
		Passed:      output.Passed,
		Score:       a.normalizeScore(output.Score),
		Feedback:    output.Feedback,
		Suggestions: output.Suggestions,
		Issues:      output.Issues,
	}

	// 如果评分 >= 0.8 但未标记通过，自动修正
	if result.Score >= 0.8 && !result.Passed {
		result.Passed = true
	}

	return result
}

// parseWithRegex 使用正则解析（JSON 解析失败时的降级方案）
func (a *Auditor) parseWithRegex(response, originalAnswer string) *AuditResult {
	result := &AuditResult{
		Passed:      false,
		Score:       0.5,
		Feedback:    "解析失败，使用默认审核结果",
		Suggestions: []string{"建议人工复核"},
		Issues:      []string{"JSON 解析失败"},
	}

	// 尝试提取 passed 字段
	passedRegex := regexp.MustCompile(`"passed"\s*:\s*(true|false)`)
	if matches := passedRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Passed = matches[1] == "true"
	}

	// 尝试提取 score 字段
	scoreRegex := regexp.MustCompile(`"score"\s*:\s*([\d.]+)`)
	if matches := scoreRegex.FindStringSubmatch(response); len(matches) > 1 {
		if score := a.parseScore(matches[1]); score >= 0 {
			result.Score = score
		}
	}

	// 尝试提取 feedback 字段
	feedbackRegex := regexp.MustCompile(`"feedback"\s*:\s*"([^"]*)"`)
	if matches := feedbackRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Feedback = matches[1]
	}

	return result
}

// cleanJSONResponse 清理 JSON 响应
func (a *Auditor) cleanJSONResponse(response string) string {
	// 移除 Markdown 代码块标记
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// 提取第一个 { 到最后一个 } 之间的内容
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	return response
}

// parseScore 解析评分字符串
func (a *Auditor) parseScore(scoreStr string) float64 {
	var score float64
	_, err := fmt.Sscanf(scoreStr, "%f", &score)
	if err != nil {
		return -1
	}
	return score
}

// normalizeScore 标准化评分
func (a *Auditor) normalizeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

// updateStats 更新审核统计
func (a *Auditor) updateStats(result *AuditResult, durationMs int64) {
	var action AuditAction
	if result.Passed {
		action = AuditActionPass
	} else {
		action = AuditActionRetry
	}
	a.stats.UpdateStats(result, durationMs, action)
}

// GetStats 获取审核统计
func (a *Auditor) GetStats() *AuditStats {
	return a.stats
}
