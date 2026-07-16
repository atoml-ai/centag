package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/utils"
)

// IntentClassifier 意图分类器
type IntentClassifier struct {
	config    IntentClassifierConfig
	client    *http.Client
	cache     map[string]*cacheEntry
	cacheMu   sync.RWMutex
	cacheSize int
}

type cacheEntry struct {
	result    *ClassificationResult
	expiresAt time.Time
}

// classificationRequest Ollama API 请求结构
type classificationRequest struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Stream   bool   `json:"stream"`
	Format   string `json:"format,omitempty"`
	Options  struct {
		Temperature float64 `json:"temperature,omitempty"`
		TopP        float64 `json:"top_p,omitempty"`
	} `json:"options,omitempty"`
}

// classificationResponse Ollama API 响应结构
type classificationResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

// classificationOutput 分类器输出结构（从小模型响应解析）
type classificationOutput struct {
	TaskType        string  `json:"task_type"`
	Confidence      float64 `json:"confidence"`
	Complexity      string  `json:"complexity"`
	Sensitivity     string  `json:"sensitivity"`
	Urgency         string  `json:"urgency"`
	EstimatedTokens int     `json:"estimated_tokens"`
	Reasoning       string  `json:"reasoning"`
}

// 分类提示词模板
const classifierPromptTemplate = `你是一个任务分类专家。请分析用户的问题，并给出以下维度的判断：

1. 任务类型 (task_type):
   - code_generation: 代码生成、修改、调试、技术问题
   - simple_chat: 简单问答、闲聊、日常对话
   - complex_reasoning: 复杂推理、数学问题、逻辑分析、科学问题
   - long_text: 长文档分析、总结、文章写作
   - embedding: 向量生成、语义搜索、相似度匹配
   - translation: 翻译任务、语言转换
   - creative: 创意写作、故事生成、诗歌创作
   - analysis: 数据分析、图表解读、商业分析

2. 复杂度 (complexity): low/medium/high
   - low: 简单问题，<1K tokens
   - medium: 中等难度，1K-10K tokens
   - high: 复杂问题，>10K tokens

3. 敏感度 (sensitivity): public/internal/confidential
   - public: 公开信息，无敏感内容
   - internal: 内部信息，一般敏感
   - confidential: 敏感数据，隐私信息，商业机密

4. 时效要求 (urgency): low/medium/high
   - low: 可接受延迟，批量任务
   - medium: 正常响应，一般对话
   - high: 实时响应，紧急任务

5. 预估 token 数 (estimated_tokens): 根据问题长度估算

用户问题：{{.question}}

请只返回 JSON 格式，不要其他内容。格式如下：
{
  "task_type": "任务类型",
  "confidence": 0.95,
  "complexity": "low/medium/high",
  "sensitivity": "public/internal/confidential",
  "urgency": "low/medium/high",
  "estimated_tokens": 100,
  "reasoning": "分类理由"
}`

// NewIntentClassifier 创建意图分类器
func NewIntentClassifier(config IntentClassifierConfig) *IntentClassifier {
	if config.LocalModel == "" {
		config.LocalModel = "qwen2.5:1.5b"
	}
	if config.OllamaAddr == "" {
		config.OllamaAddr = "http://localhost:21434"
	}
	if config.Timeout == 0 {
		config.Timeout = 10
	}

	return &IntentClassifier{
		config:    config,
		client:    &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		cache:     make(map[string]*cacheEntry, 100),
		cacheSize: 100,
	}
}

// Classify 对问题进行意图分类
func (c *IntentClassifier) Classify(question string) (*ClassificationResult, error) {
	if !c.config.Enabled {
		return c.getDefaultClassification(question), nil
	}

	// 检查缓存
	if c.config.CacheEnabled {
		if cached := c.getCached(question); cached != nil {
			logger.Debugf("[IntentClassifier] Cache hit for question: %s", utils.TruncateString(question, 50))
			return cached, nil
		}
	}

	// 调用小模型进行分类
	result, err := c.classifyWithModel(question)
	if err != nil {
		logger.Warnf("[IntentClassifier] Classification failed: %v", err)
		// 返回默认分类
		result = c.getDefaultClassification(question)
	}

	// 缓存结果
	if c.config.CacheEnabled && result != nil {
		c.setCached(question, result)
	}

	return result, nil
}

// classifyWithModel 使用小模型进行分类
func (c *IntentClassifier) classifyWithModel(question string) (*ClassificationResult, error) {
	// 构建提示词
	prompt := strings.ReplaceAll(classifierPromptTemplate, "{{.question}}", question)

	// 构建请求
	reqBody := classificationRequest{
		Model:  c.config.LocalModel,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}
	reqBody.Options.Temperature = 0.1 // 低温度，保证输出稳定
	reqBody.Options.TopP = 0.9

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	// 发送请求
	url := strings.TrimSuffix(c.config.OllamaAddr, "/") + "/api/generate"
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	// 解析响应
	var ollamaResp classificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %w", err)
	}

	// 解析分类结果
	result := c.parseClassificationResult(ollamaResp.Response, question)
	result.RawResponse = ollamaResp.Response

	// 安全日志输出（logger 可能未初始化）
	if logger.Sugar != nil {
		logger.Infof("[IntentClassifier] Classification result: task=%s, confidence=%.2f, complexity=%s",
			result.TaskType, result.Confidence, result.Complexity)
	}

	return result, nil
}

// parseClassificationResult 解析分类结果
func (c *IntentClassifier) parseClassificationResult(response, question string) *ClassificationResult {
	// 清理响应文本
	cleaned := c.cleanJSONResponse(response)

	// 尝试解析 JSON
	var output classificationOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		logger.Warnf("[IntentClassifier] Failed to parse JSON: %v, raw: %s", err, truncated(response, 200))
		// 使用正则提取关键字段
		return c.parseWithRegex(response, question)
	}

	// 验证并转换结果
	result := &ClassificationResult{
		TaskType:        c.parseTaskType(output.TaskType),
		Confidence:      c.normalizeConfidence(output.Confidence),
		Complexity:      c.parseComplexity(output.Complexity),
		Sensitivity:     c.parseSensitivity(output.Sensitivity),
		Urgency:         c.parseUrgency(output.Urgency),
		EstimatedTokens: c.estimateTokens(question, output.EstimatedTokens),
		Reasoning:       output.Reasoning,
	}

	return result
}

// parseWithRegex 使用正则解析（JSON 解析失败时的降级方案）
func (c *IntentClassifier) parseWithRegex(response, question string) *ClassificationResult {
	result := &ClassificationResult{
		TaskType:        TaskUnknown,
		Confidence:      0.5,
		Complexity:      ComplexityMedium,
		Sensitivity:     SensitivityPublic,
		Urgency:         UrgencyMedium,
		EstimatedTokens: len(question) / 4, // 粗略估算
		Reasoning:       "解析失败，使用默认分类",
	}

	// 尝试提取 task_type
	taskTypeRegex := regexp.MustCompile(`"task_type"\s*:\s*"([^"]+)"`)
	if matches := taskTypeRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.TaskType = c.parseTaskType(matches[1])
		result.Confidence = 0.7
	}

	// 尝试提取 confidence
	confRegex := regexp.MustCompile(`"confidence"\s*:\s*([0-9.]+)`)
	if matches := confRegex.FindStringSubmatch(response); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%f", &result.Confidence)
		result.Confidence = c.normalizeConfidence(result.Confidence)
	}

	// 尝试提取 complexity
	complexityRegex := regexp.MustCompile(`"complexity"\s*:\s*"([^"]+)"`)
	if matches := complexityRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Complexity = c.parseComplexity(matches[1])
	}

	// 尝试提取 sensitivity
	sensitivityRegex := regexp.MustCompile(`"sensitivity"\s*:\s*"([^"]+)"`)
	if matches := sensitivityRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Sensitivity = c.parseSensitivity(matches[1])
	}

	// 尝试提取 urgency
	urgencyRegex := regexp.MustCompile(`"urgency"\s*:\s*"([^"]+)"`)
	if matches := urgencyRegex.FindStringSubmatch(response); len(matches) > 1 {
		result.Urgency = c.parseUrgency(matches[1])
	}

	return result
}

// cleanJSONResponse 清理 JSON 响应（去除 markdown 标记等）
func (c *IntentClassifier) cleanJSONResponse(response string) string {
	// 去除 markdown 代码块标记
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// 提取第一个 { 到最后一个 } 之间的内容
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		return response[start : end+1]
	}

	return response
}

// parseTaskType 解析任务类型
func (c *IntentClassifier) parseTaskType(s string) TaskType {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "code_generation", "code", "coding", "programming":
		return TaskCodeGeneration
	case "simple_chat", "chat", "casual", "greeting":
		return TaskSimpleChat
	case "complex_reasoning", "reasoning", "math", "logic", "science":
		return TaskComplexReasoning
	case "long_text", "document", "summary", "article":
		return TaskLongText
	case "embedding", "vector", "semantic":
		return TaskEmbedding
	case "translation", "translate", "language":
		return TaskTranslation
	case "creative", "story", "poem", "writing":
		return TaskCreative
	case "analysis", "data", "chart", "business":
		return TaskAnalysis
	default:
		return TaskUnknown
	}
}

// parseComplexity 解析复杂度
func (c *IntentClassifier) parseComplexity(s string) ComplexityLevel {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "low", "simple", "easy":
		return ComplexityLow
	case "high", "complex", "difficult":
		return ComplexityHigh
	default:
		return ComplexityMedium
	}
}

// parseSensitivity 解析敏感度
func (c *IntentClassifier) parseSensitivity(s string) SensitivityLevel {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "public", "open", "general":
		return SensitivityPublic
	case "confidential", "private", "secret", "sensitive":
		return SensitivityConfidential
	default:
		return SensitivityInternal
	}
}

// parseUrgency 解析时效
func (c *IntentClassifier) parseUrgency(s string) UrgencyLevel {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "low", "normal", "casual":
		return UrgencyLow
	case "high", "urgent", "emergency", "asap":
		return UrgencyHigh
	default:
		return UrgencyMedium
	}
}

// normalizeConfidence 规范化置信度
func (c *IntentClassifier) normalizeConfidence(conf float64) float64 {
	if conf <= 0 {
		return 0.5
	}
	if conf > 1 {
		return 1.0
	}
	return conf
}

// estimateTokens 估算 token 数
func (c *IntentClassifier) estimateTokens(question string, provided int) int {
	if provided > 0 {
		return provided
	}
	// 中文约 1.5 字符/token，英文约 4 字符/token
	return len(question) / 3
}

// getDefaultClassification 获取默认分类（降级方案）
func (c *IntentClassifier) getDefaultClassification(question string) *ClassificationResult {
	length := len(question)
	var complexity ComplexityLevel
	if length < 50 {
		complexity = ComplexityLow
	} else if length < 500 {
		complexity = ComplexityMedium
	} else {
		complexity = ComplexityHigh
	}

	// 简单的关键词检测
	q := strings.ToLower(question)
	var taskType TaskType
	switch {
	case strings.Contains(q, "代码") || strings.Contains(q, "code") || strings.Contains(q, "编程") ||
		strings.Contains(q, "实现") || strings.Contains(q, "编写") || strings.Contains(q, "go "):
		taskType = TaskCodeGeneration
	case strings.Contains(q, "翻译") || strings.Contains(q, "translate"):
		taskType = TaskTranslation
	case strings.Contains(q, "总结") || strings.Contains(q, "summary") || length > 500:
		taskType = TaskLongText
	case strings.Contains(q, "分析") || strings.Contains(q, "analyze") || strings.Contains(q, "数据"):
		taskType = TaskAnalysis
	default:
		taskType = TaskSimpleChat
	}

	return &ClassificationResult{
		TaskType:        taskType,
		Confidence:      0.5,
		Complexity:      complexity,
		Sensitivity:     SensitivityPublic,
		Urgency:         UrgencyMedium,
		EstimatedTokens: length / 3,
		Reasoning:       "默认分类（小模型不可用）",
	}
}

// getCached 从缓存获取结果
func (c *IntentClassifier) getCached(question string) *ClassificationResult {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	entry, ok := c.cache[question]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil // 已过期
	}

	return entry.result
}

// setCached 设置缓存
func (c *IntentClassifier) setCached(question string, result *ClassificationResult) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	// 清理过期缓存
	now := time.Now()
	for k, v := range c.cache {
		if now.After(v.expiresAt) {
			delete(c.cache, k)
		}
	}

	// 限制缓存大小
	if len(c.cache) >= c.cacheSize {
		// 删除最旧的一半
		count := 0
		for k := range c.cache {
			delete(c.cache, k)
			count++
			if count >= c.cacheSize/2 {
				break
			}
		}
	}

	c.cache[question] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(time.Duration(c.config.CacheTTL) * time.Second),
	}
}

// Close 清理资源
func (c *IntentClassifier) Close() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache = nil
}

// truncated 截断字符串（简化版）
func truncated(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// GetDefaultClassification 公开方法：获取默认分类（用于测试）
func (c *IntentClassifier) GetDefaultClassification(question string) *ClassificationResult {
	return c.getDefaultClassification(question)
}
