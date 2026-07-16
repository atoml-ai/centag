package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
)

// AutoAssignAnalyzer 自动分配分析器
type AutoAssignAnalyzer struct {
	config     AutoAssignConfig
	backend    *backend.BackendConfig // 用于分析的后端配置
	httpClient *http.Client
}

// AutoAssignConfig 自动分配配置
type AutoAssignConfig struct {
	BackendID     string            `json:"backend_id"`      // 用于分析的后端 ID
	Model         string            `json:"model"`           // 用于分析的模型
	Prompt        string            `json:"prompt"`          // 分析提示词
	Timeout       int               `json:"timeout"`         // 超时时间（秒）
	TaskTypes     []TaskType        `json:"task_types"`      // 要分析的任务类型
}

// AutoAssignResult 自动分配结果
type AutoAssignResult struct {
	Success       bool                    `json:"success"`
	TaskAssignments []*TaskTypeAssignment `json:"task_assignments"` // 每个任务类型的分配结果
	AnalysisLog   string                  `json:"analysis_log"`     // 分析日志
	Error         string                  `json:"error,omitempty"`
}

// TaskTypeAssignment 任务类型分配
type TaskTypeAssignment struct {
	TaskType      TaskType `json:"task_type"`
	RecommendedBackendID string `json:"recommended_backend_id"`
	RecommendedModel string `json:"recommended_model"`
	Reason         string `json:"reason"`
	Confidence     float64 `json:"confidence"`
}

// 默认分析提示词
const DefaultAnalysisPrompt = `你是一个 AI 模型能力评估专家。请分析以下后端和模型配置，为每种任务类型推荐最合适的后端和模型。

## 任务类型说明
- code_generation: 代码生成、修改、调试、技术问题
- simple_chat: 简单问答、闲聊、日常对话
- complex_reasoning: 复杂推理、数学问题、逻辑分析、科学问题
- long_text: 长文档分析、总结、文章写作（>10K tokens）
- embedding: 向量生成、语义搜索、相似度匹配
- translation: 翻译任务、语言转换
- creative: 创意写作、故事生成、诗歌创作
- analysis: 数据分析、图表解读、商业分析

## 后端配置
{{.backends_info}}

## 分析要求
请为每种任务类型推荐最合适的后端和模型，考虑以下因素：
1. 模型能力匹配度（代码能力、推理能力、上下文长度等）
2. 成本效益（优先推荐性价比高的方案）
3. 响应速度（本地模型优先用于简单任务）
4. 隐私保护（敏感任务优先本地模型）

请只返回 JSON 格式，不要其他内容。格式如下：
{
  "task_assignments": [
    {
      "task_type": "任务类型",
      "recommended_backend_id": "后端 ID",
      "recommended_model": "推荐模型",
      "reason": "推荐理由（50 字以内）",
      "confidence": 0.95
    }
  ]
}`

// DefaultAutoAssignConfig 返回默认配置
func DefaultAutoAssignConfig() AutoAssignConfig {
	return AutoAssignConfig{
		BackendID: "ollama-local",
		Model:     "qwen2.5:1.5b",
		Prompt:    DefaultAnalysisPrompt,
		Timeout:   300, // 默认300秒超时（5分钟）
		TaskTypes: []TaskType{
			TaskCodeGeneration,
			TaskSimpleChat,
			TaskComplexReasoning,
			TaskLongText,
			TaskEmbedding,
			TaskTranslation,
			TaskCreative,
			TaskAnalysis,
		},
	}
}

// NewAutoAssignAnalyzer 创建自动分配分析器
func NewAutoAssignAnalyzer(config AutoAssignConfig) *AutoAssignAnalyzer {
	if config.Timeout == 0 {
		config.Timeout = 300
	}
	
	return &AutoAssignAnalyzer{
		config:     config,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
	}
}

// SetBackend 设置用于分析的后端配置
func (a *AutoAssignAnalyzer) SetBackend(cfg *backend.BackendConfig) {
	a.backend = cfg
}

// Analyze 执行自动分配分析
func (a *AutoAssignAnalyzer) Analyze(backends []*backend.BackendConfig) (*AutoAssignResult, error) {
	// 构建后端信息字符串
	backendsInfo := a.buildBackendsInfo(backends)
	
	// 构建提示词
	prompt := strings.ReplaceAll(a.config.Prompt, "{{.backends_info}}", backendsInfo)
	
	// 调用大模型进行分析
	analysisResult, err := a.callModel(prompt)
	if err != nil {
		return &AutoAssignResult{
			Success: false,
			Error:   fmt.Sprintf("模型调用失败：%v", err),
		}, err
	}
	
	// 解析结果
	result := a.parseAnalysisResult(analysisResult, backends)
	result.Success = true
	
	logger.Infof("[AutoAssign] Analysis completed: %d task types assigned", len(result.TaskAssignments))
	
	return result, nil
}

// buildBackendsInfo 构建后端信息字符串
func (a *AutoAssignAnalyzer) buildBackendsInfo(backends []*backend.BackendConfig) string {
	var sb strings.Builder
	
	sb.WriteString("### 可用后端列表\n\n")
	
	for _, b := range backends {
		if !b.Enabled {
			continue
		}
		
		sb.WriteString(fmt.Sprintf("#### 后端：%s (ID: %s)\n", b.Name, b.ID))
		sb.WriteString(fmt.Sprintf("- 类型：%s\n", b.Type))
		sb.WriteString(fmt.Sprintf("- 优先级：%d\n", b.Priority))
		sb.WriteString(fmt.Sprintf("- 权重：%d\n", b.Weight))
		
		if len(b.SupportedModels) > 0 {
			sb.WriteString("- 支持模型:\n")
			for _, m := range b.SupportedModels {
				sb.WriteString(fmt.Sprintf("  - %s → %s\n", m.RequestedModel, m.ActualModel))
			}
		}
		
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// callModel 调用大模型进行分析
func (a *AutoAssignAnalyzer) callModel(prompt string) (string, error) {
	// 检查是否设置了后端配置
	if a.backend == nil {
		return "", fmt.Errorf("backend config is nil")
	}

	baseURL := a.backend.BaseURL
	if baseURL == "" {
		return "", fmt.Errorf("backend base URL is empty")
	}

	var reqBody map[string]interface{}
	var url string

	// 根据后端类型构建请求
	if a.backend.Type == "ollama" {
		reqBody = map[string]interface{}{
			"model":  a.config.Model,
			"prompt": prompt,
			"stream": false,
			"format": "json",
			"options": map[string]interface{}{
				"temperature": 0.1,
				"top_p":       0.9,
			},
		}
		url = strings.TrimRight(baseURL, "/") + "/api/generate"
	} else {
		// OpenAI 兼容格式
		reqBody = map[string]interface{}{
			"model":    a.config.Model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"stream": false,
			"temperature": 0.1,
		}
		url = strings.TrimRight(baseURL, "/") + "/chat/completions"
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request failed: %w", err)
	}

	logger.Infof("[AutoAssign] Calling model: %s at %s (type: %s)", a.config.Model, url, a.backend.Type)

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	apiKey := backend.NormalizeOpenAICompatibleAPIKey(a.backend.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("model returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	if a.backend.Type == "ollama" {
		var ollamaResp struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			return "", fmt.Errorf("decode ollama response failed: %w", err)
		}
		return ollamaResp.Response, nil
	} else {
		// OpenAI 兼容格式响应
		var openaiResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
			return "", fmt.Errorf("decode openai response failed: %w", err)
		}
		if len(openaiResp.Choices) == 0 || openaiResp.Choices[0].Message.Content == "" {
			return "", fmt.Errorf("empty response from model")
		}
		return openaiResp.Choices[0].Message.Content, nil
	}
}

// parseAnalysisResult 解析分析结果
func (a *AutoAssignAnalyzer) parseAnalysisResult(response string, backends []*backend.BackendConfig) *AutoAssignResult {
	result := &AutoAssignResult{
		TaskAssignments: make([]*TaskTypeAssignment, 0),
		AnalysisLog:     response,
	}
	
	// 清理 JSON 响应
	cleaned := a.cleanJSONResponse(response)
	
	// 尝试解析 JSON
	var parsed struct {
		TaskAssignments []struct {
			TaskType         string  `json:"task_type"`
			RecommendedBackendID string  `json:"recommended_backend_id"`
			RecommendedModel string  `json:"recommended_model"`
			Reason           string  `json:"reason"`
			Confidence       float64 `json:"confidence"`
		} `json:"task_assignments"`
	}
	
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		// 安全日志输出（logger 可能未初始化）
		if logger.Sugar != nil {
			logger.Warnf("[AutoAssign] Failed to parse JSON: %v", err)
		}
		// 返回默认分配
		result.TaskAssignments = a.getDefaultAssignments(backends)
		result.Error = "JSON 解析失败，使用默认分配"
		return result
	}
	
	// 转换结果
	backendMap := make(map[string]*backend.BackendConfig)
	for _, b := range backends {
		backendMap[b.ID] = b
	}
	
	for _, assignment := range parsed.TaskAssignments {
		taskType := a.parseTaskType(assignment.TaskType)
		
		// 验证后端是否存在
		recommendedBackendID := assignment.RecommendedBackendID
		if _, ok := backendMap[recommendedBackendID]; !ok {
			// 安全日志输出（logger 可能未初始化）
			if logger.Sugar != nil {
				logger.Warnf("[AutoAssign] Unknown backend: %s, using first available", recommendedBackendID)
			}
			// 降级到第一个可用后端
			if len(backends) > 0 {
				recommendedBackendID = backends[0].ID
			}
		}
		
		// 规范化置信度到 [0, 1] 范围
	confidence := assignment.Confidence
	if confidence <= 0 {
		confidence = 0.5 // 零值处理
	} else if confidence > 1 {
		confidence = 1.0
	}
	
	result.TaskAssignments = append(result.TaskAssignments, &TaskTypeAssignment{
			TaskType:           taskType,
			RecommendedBackendID: recommendedBackendID,
			RecommendedModel:   assignment.RecommendedModel,
			Reason:             assignment.Reason,
			Confidence:         confidence,
		})
	}
	
	return result
}

// cleanJSONResponse 清理 JSON 响应
func (a *AutoAssignAnalyzer) cleanJSONResponse(response string) string {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)
	
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		return response[start : end+1]
	}
	
	return response
}

// parseTaskType 解析任务类型
func (a *AutoAssignAnalyzer) parseTaskType(s string) TaskType {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "code_generation", "code", "coding":
		return TaskCodeGeneration
	case "simple_chat", "chat", "casual":
		return TaskSimpleChat
	case "complex_reasoning", "reasoning", "math":
		return TaskComplexReasoning
	case "long_text", "document", "summary":
		return TaskLongText
	case "embedding", "vector":
		return TaskEmbedding
	case "translation", "translate":
		return TaskTranslation
	case "creative", "story", "poem":
		return TaskCreative
	case "analysis", "data":
		return TaskAnalysis
	default:
		return TaskUnknown
	}
}

// getDefaultAssignments 返回默认分配（降级方案）
func (a *AutoAssignAnalyzer) getDefaultAssignments(backends []*backend.BackendConfig) []*TaskTypeAssignment {
	assignments := make([]*TaskTypeAssignment, 0)
	
	backendMap := make(map[string]*backend.BackendConfig)
	for _, b := range backends {
		if b.Enabled {
			backendMap[b.ID] = b
		}
	}
	
	// 默认分配逻辑
	defaults := []struct {
		taskType TaskType
		backendID string
		reason   string
	}{
		{TaskCodeGeneration, "bigmodel", "代码生成任务，使用智谱 GLM 模型"},
		{TaskSimpleChat, "ollama-local", "简单对话，使用本地模型节省成本"},
		{TaskComplexReasoning, "bigmodel", "复杂推理，使用高质量模型"},
		{TaskLongText, "ppinfra", "长文本处理，使用 Kimi 模型"},
		{TaskEmbedding, "ollama-local", "向量嵌入，使用本地模型"},
		{TaskTranslation, "ppinfra", "翻译任务，性价比高"},
		{TaskCreative, "bigmodel", "创意写作，使用智谱 GLM 模型"},
		{TaskAnalysis, "bigmodel", "数据分析，推理能力强"},
	}
	
	for _, d := range defaults {
		if _, ok := backendMap[d.backendID]; ok {
			assignments = append(assignments, &TaskTypeAssignment{
				TaskType:           d.taskType,
				RecommendedBackendID: d.backendID,
				RecommendedModel:   "",
				Reason:             d.reason,
				Confidence:         0.5,
			})
		}
	}
	
	return assignments
}
