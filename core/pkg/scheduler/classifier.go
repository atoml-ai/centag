package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/logger"
	"centag/core/pkg/utils"
)

// IntentClassifier 意图分类器
type IntentClassifier struct {
	config     IntentClassifierConfig
	backendMgr *backend.Manager
	client     *http.Client
	cache      map[string]*cacheEntry
	cacheMu    sync.RWMutex
	cacheSize  int
}

type cacheEntry struct {
	result    *ClassificationResult
	expiresAt time.Time
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
func NewIntentClassifier(config IntentClassifierConfig, backendMgr *backend.Manager) *IntentClassifier {
	if config.Timeout == 0 {
		config.Timeout = 10
	}

	return &IntentClassifier{
		config:     config,
		backendMgr: backendMgr,
		client:     &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		cache:      make(map[string]*cacheEntry, 100),
		cacheSize:  100,
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

// classifyWithModel 使用后端模型进行分类
func (c *IntentClassifier) classifyWithModel(question string) (*ClassificationResult, error) {
	// 使用自定义提示词或默认提示词
	prompt := c.config.ClassifyPrompt
	if prompt == "" {
		prompt = classifierPromptTemplate
	}
	prompt = strings.ReplaceAll(prompt, "{{.question}}", question)

	// 必须配置 BackendID 才能调用 LLM
	if c.config.BackendID == "" || c.backendMgr == nil {
		return nil, fmt.Errorf("classifier backend not configured")
	}

	// 尝试主后端
	result, err := c.callBackend(prompt, c.config.BackendID, c.config.Model)
	if err == nil {
		return result, nil
	}
	logger.Warnf("[IntentClassifier] Primary backend failed: %v", err)

	// 尝试备用后端
	if c.config.FallbackBackendID != "" {
		result, err = c.callBackend(prompt, c.config.FallbackBackendID, c.config.FallbackModel)
		if err == nil {
			return result, nil
		}
		logger.Warnf("[IntentClassifier] Fallback backend failed: %v", err)
	}

	return nil, fmt.Errorf("all classifier backends failed")
}

// callBackend 通过 backendMgr 调用指定后端进行分类
func (c *IntentClassifier) callBackend(prompt, backendID, model string) (*ClassificationResult, error) {
	// 1. 获取后端配置
	backendCfg, err := c.backendMgr.Get(backendID)
	if err != nil {
		return nil, fmt.Errorf("backend %q not found: %w", backendID, err)
	}
	if !backendCfg.Enabled {
		return nil, fmt.Errorf("backend %q is disabled", backendID)
	}

	// 2. 确定模型
	if model == "" {
		model = backend.PreferredDefaultModel(backendCfg)
	}
	if model == "" {
		return nil, fmt.Errorf("no model specified for backend %q", backendID)
	}

	// 3. 构建请求 URL
	apiURL := c.buildAPIURL(backendCfg)

	logger.Infof("[IntentClassifier] calling backend: backend_id=%s, model=%s, url=%s",
		backendID, model, apiURL)

	// 4. 构建请求体（根据后端类型选择格式）
	var jsonData []byte
	if backendCfg.Type == "ollama" {
		reqBody := map[string]interface{}{
			"model":  model,
			"prompt": prompt,
			"stream": false,
			"format": "json",
			"options": map[string]interface{}{
				"temperature": 0.1,
				"top_p":       0.9,
			},
		}
		jsonData, err = json.Marshal(reqBody)
	} else {
		reqBody := map[string]interface{}{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.1,
		}
		jsonData, err = json.Marshal(reqBody)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	// 5. 发送请求
	req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend %q returned status %d: %s", backendID, resp.StatusCode, string(bodyBytes))
	}

	// 6. 解析响应
	rawResponse, err := c.parseResponse(backendCfg.Type, bodyBytes)
	if err != nil {
		return nil, err
	}

	result := c.parseClassificationResult(rawResponse, "")
	result.RawResponse = rawResponse

	if logger.Sugar != nil {
		logger.Infof("[IntentClassifier] Classification result: task=%s, confidence=%.2f, complexity=%s",
			result.TaskType, result.Confidence, result.Complexity)
	}

	return result, nil
}

// buildAPIURL 根据后端配置构建 API URL
func (c *IntentClassifier) buildAPIURL(b *backend.BackendConfig) string {
	baseURL := b.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	hasAPIPrefix := strings.Contains(baseURL, "/api")

	switch b.Type {
	case "ollama":
		if hasAPIPrefix {
			return baseURL + "/generate"
		}
		return baseURL + "/api/generate"
	default:
		// 与 plugins/backend/openai 的 buildOpenAIChatURL 对齐：仅当 baseURL 以 /vN 结尾时才直接拼接，
		// 避免 bigmodel 等 /v4 网关被错误拼成 /v4/v1/chat/completions。
		if hasAPIVersionPrefix(baseURL) {
			return baseURL + "/chat/completions"
		}
		return baseURL + "/v1/chat/completions"
	}
}

// hasAPIVersionPrefix 检查 baseURL 是否以 /vN（如 /v1、/v4）结尾。
func hasAPIVersionPrefix(baseURL string) bool {
	trimmed := strings.TrimSuffix(baseURL, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		seg := trimmed[idx+1:]
		if len(seg) > 1 && seg[0] == 'v' {
			for _, ch := range seg[1:] {
				if ch < '0' || ch > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}

// parseResponse 根据后端类型解析响应
func (c *IntentClassifier) parseResponse(backendType string, bodyBytes []byte) (string, error) {
	if backendType == "ollama" {
		var resp struct {
			Response string `json:"response"`
		}
		if err := json.Unmarshal(bodyBytes, &resp); err != nil {
			return "", fmt.Errorf("decode ollama response failed: %w", err)
		}
		return resp.Response, nil
	}

	// OpenAI 兼容格式
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return "", fmt.Errorf("decode openai response failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return resp.Choices[0].Message.Content, nil
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

	// 关键词检测
	q := strings.ToLower(question)
	var taskType TaskType
	var confidence float64

	switch {
	case containsAny(q, "代码", "code", "编程", "实现", "编写", "function", "bug", "debug", "refactor", "go ", "python", "java"):
		taskType = TaskCodeGeneration
		confidence = 0.8
	case containsAny(q, "翻译", "translate", "译成", "译为"):
		taskType = TaskTranslation
		confidence = 0.8
	case containsAny(q, "总结", "summary", "概括", "摘要", "summarize") || length > 500:
		taskType = TaskLongText
		confidence = 0.8
	case containsAny(q, "分析", "analyze", "数据", "data", "图表", "chart", "统计"):
		taskType = TaskAnalysis
		confidence = 0.8
	case containsAny(q, "推理", "reasoning", "数学", "math", "逻辑", "logic", "证明", "prove", "为什么", "why"):
		taskType = TaskComplexReasoning
		confidence = 0.7
	case containsAny(q, "故事", "story", "诗", "poem", "创意", "creative", "写作", "writing", "小说"):
		taskType = TaskCreative
		confidence = 0.7
	default:
		taskType = TaskSimpleChat
		confidence = 0.6
	}

	return &ClassificationResult{
		TaskType:        taskType,
		Confidence:      confidence,
		Complexity:      complexity,
		Sensitivity:     SensitivityPublic,
		Urgency:         UrgencyMedium,
		EstimatedTokens: length / 3,
		Reasoning:       "默认分类（分类模型不可用）",
	}
}

// containsAny 检查字符串是否包含任一子串
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
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
