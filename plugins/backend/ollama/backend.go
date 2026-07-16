package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"centag/core/pkg/backend"
	"centag/core/pkg/plugin"
)

// Backend Ollama后端插件
type Backend struct {
	name           string
	status         plugin.PluginStatus
	config         *Config
	backendManager *backend.Manager
	currentBackend *backend.BackendConfig
	client         *http.Client
	mu             sync.RWMutex
}

// ChatRequest Ollama聊天请求
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// Message 消息结构（支持多模态内容）
type Message struct {
	Role     string         `json:"role"`
	Content  MessageContent `json:"content"`
	Thinking string         `json:"thinking,omitempty"` // qwen3等模型的思考过程字段
}

// MessageContent 消息内容(支持字符串或数组)
type MessageContent struct {
	Type     string      `json:"type,omitempty"` // text, image_url等
	Text     string      `json:"text,omitempty"`
	ImageURL interface{} `json:"image_url,omitempty"`
	Value    interface{} `json:"-"` // 用于存储原始值
}

// UnmarshalJSON 自定义JSON解析
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		mc.Value = str
		return nil
	}

	// 尝试解析为数组
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		mc.Value = arr
		return nil
	}

	// 尝试解析为对象
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		mc.Value = obj
		return nil
	}

	return fmt.Errorf("invalid message content format")
}

// MarshalJSON 自定义JSON序列化 - Ollama API只支持字符串
func (mc MessageContent) MarshalJSON() ([]byte, error) {
	// Ollama API要求content为字符串，所以总是转换为字符串
	return json.Marshal(mc.String())
}

// String 返回内容的字符串表示
func (mc *MessageContent) String() string {
	if mc.Value == nil {
		return ""
	}

	// 如果是字符串
	if str, ok := mc.Value.(string); ok {
		return str
	}

	// 如果是数组,提取文本内容
	if arr, ok := mc.Value.([]map[string]interface{}); ok {
		result := ""
		for _, item := range arr {
			if typ, ok := item["type"].(string); ok && typ == "text" {
				if text, ok := item["text"].(string); ok {
					result += text
				}
			}
		}
		return result
	}

	// 如果是对象,尝试提取文本
	if obj, ok := mc.Value.(map[string]interface{}); ok {
		if typ, ok := obj["type"].(string); ok && typ == "text" {
			if text, ok := obj["text"].(string); ok {
				return text
			}
		}
	}

	return fmt.Sprintf("%v", mc.Value)
}

// ChatResponse Ollama聊天响应
type ChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

// NewBackend 创建Ollama后端插件
func NewBackend() (*Backend, error) {
	return &Backend{
		name:           "ollama-backend",
		config:         DefaultConfig(),
		backendManager: backend.GetManager(),
		client: &http.Client{
			Timeout: 0, // 不设置超时，依赖 context 控制
		},
		status: plugin.StatusStopped,
	}, nil
}

// Name 返回插件名称
func (b *Backend) Name() string {
	return b.name
}

// Type 返回插件类型
func (b *Backend) Type() plugin.PluginType {
	return plugin.TypeBackend
}

// Version 返回插件版本
func (b *Backend) Version() string {
	return "1.0.0"
}

// Init 初始化插件
func (b *Backend) Init(config any) error {
	// 使用默认配置或传入的配置
	if cfg, ok := config.(*Config); ok {
		b.config = cfg
	}

	// 对于流式请求，不应设置 HTTP client Timeout，否则会导致长响应被截断
	// 使用 0 表示不设置超时，依赖 context 来控制超时
	b.client.Timeout = 0

	log.Printf("[Ollama Backend] Plugin initialized with BaseURL: %s", b.config.BaseURL)
	return nil
}

// Start 启动插件
func (b *Backend) Start(ctx context.Context) error {
	b.status = plugin.StatusRunning
	log.Printf("[Ollama Backend] Plugin started")
	return nil
}

// Stop 停止插件
func (b *Backend) Stop(ctx context.Context) error {
	b.status = plugin.StatusStopped
	log.Printf("[Ollama Backend] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (b *Backend) Status() plugin.PluginStatus {
	return b.status
}

// getBackendConfigForReq 获取后端配置。
// 若 req.BackendID 不为空，则优先按 ID 直接获取指定后端，确保"直接后端"和
// "智能调度"两种模式下插件都能使用 handler 层已经选定的后端。
func (b *Backend) getBackendConfigForReq(req *plugin.ProxyRequest) (*backend.BackendConfig, error) {
	if req != nil && req.BackendID != "" {
		cfg, err := b.backendManager.Get(req.BackendID)
		if err == nil && cfg.Enabled {
			return cfg, nil
		}
		if err != nil {
			log.Printf("[Ollama Backend] BackendID %s not found (%v), falling back to SelectBackend", req.BackendID, err)
		} else {
			log.Printf("[Ollama Backend] BackendID %s is disabled, falling back to SelectBackend", req.BackendID)
		}
	}
	return b.SelectBackend()
}

// SelectBackend 选择后端配置
func (b *Backend) SelectBackend() (*backend.BackendConfig, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.currentBackend != nil {
		return b.currentBackend, nil
	}

	// 从后端管理器获取Ollama类型的后端
	backends := b.backendManager.GetByType("ollama")
	if len(backends) == 0 {
		// 如果没有配置的Ollama后端，使用默认配置
		return &backend.BackendConfig{
			ID:      "ollama-default",
			Name:    "Ollama Local",
			Type:    "ollama",
			BaseURL: b.config.BaseURL,
			APIKey:  b.config.APIKey,
			Enabled: true,
			Weight:  1,
			Timeout: b.config.Timeout,
		}, nil
	}

	// 返回第一个启用的Ollama后端
	for _, backendCfg := range backends {
		if backendCfg.Enabled {
			return backendCfg, nil
		}
	}

	return nil, fmt.Errorf("no enabled ollama backend found")
}

// CallModel 调用模型 (非流式)
func (b *Backend) CallModel(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	// 获取后端配置（优先使用 req.BackendID 指定的后端）
	backendCfg, err := b.getBackendConfigForReq(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend config: %w", err)
	}

	// 构建Ollama请求
	ollamaReq := &ChatRequest{
		Model:    req.Model,
		Messages: convertToOllamaMessages(req.Messages),
		Stream:   false,
	}

	// 序列化请求
	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建HTTP请求
	baseURL := backend.NormalizeOllamaAPIBase(backendCfg.BaseURL)
	url := fmt.Sprintf("%s/api/chat", baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// 去除可能的空白字符
	respBody = bytes.TrimSpace(respBody)

	// 尝试解析为单个JSON响应
	var ollamaResp ChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err == nil {
		// 成功解析为单个响应
		return &plugin.ProxyResponse{
			Content:      ollamaResp.Message.Content.String(),
			TokensUsed:   0,
			FinishReason: "stop",
			Model:        ollamaResp.Model,
			Metadata: map[string]interface{}{
				"backend_type": "ollama",
				"created_at":   ollamaResp.CreatedAt,
			},
			RawBody: ollamaResp,
		}, nil
	}

	// 如果单个JSON解析失败,可能是流式响应,尝试合并
	lines := bytes.Split(respBody, []byte("\n"))
	var content strings.Builder
	var model string
	var finalDone bool

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var chunk ChatResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if model == "" {
			model = chunk.Model
		}

		// 提取内容 - 优先使用 content，如果没有则使用 Message.Thinking
		contentChunk := chunk.Message.Content.String()
		if contentChunk == "" && chunk.Message.Thinking != "" {
			contentChunk = chunk.Message.Thinking
		}
		content.WriteString(contentChunk)

		if chunk.Done {
			finalDone = true
		}
	}

	if finalDone || content.Len() > 0 {
		return &plugin.ProxyResponse{
			Content:      content.String(),
			TokensUsed:   0,
			FinishReason: "stop",
			Model:        model,
			Metadata: map[string]interface{}{
				"backend_type": "ollama",
				"combined":     true,
			},
		}, nil
	}

	return nil, fmt.Errorf("failed to parse response: %w", err)
}

// CallModelStream 流式调用模型
func (b *Backend) CallModelStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error) {
	ch := make(chan plugin.StreamChunk, 10)

	go func() {
		defer close(ch)

		// 获取后端配置（优先使用 req.BackendID 指定的后端）
		backendCfg, err := b.getBackendConfigForReq(req)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to get backend config: %w", err)}
			return
		}

		// 检查 BaseURL 是否配置
		baseURL := backend.NormalizeOllamaAPIBase(backendCfg.BaseURL)
		if baseURL == "" {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("ollama backend BaseURL is not configured. Please set the 'baseurl' field in the backend configuration")}
			return
		}

		// 构建Ollama请求
		ollamaReq := &ChatRequest{
			Model:    req.Model,
			Messages: convertToOllamaMessages(req.Messages),
			Stream:   true,
		}

		// 序列化请求
		reqBody, err := json.Marshal(ollamaReq)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		// 创建HTTP请求
		url := fmt.Sprintf("%s/api/chat", baseURL)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		// 设置请求头
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
		if apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// 发送请求
		resp, err := b.client.Do(httpReq)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			ch <- plugin.StreamChunk{Error: fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))}
			return
		}

		// 读取SSE流
		scanner := newLineScanner(resp.Body)
		lineCount := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineCount++
			log.Printf("[Ollama Backend] Stream line %d: %s", lineCount, line)

			// 去除空白
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Ollama流式响应格式
			var chunk ChatResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				log.Printf("[Ollama Backend] Failed to parse line %d: %v, content: %s", lineCount, err, line)
				continue
			}

		// 提取内容 - 优先使用 content，如果没有则使用 Message.Thinking
		content := chunk.Message.Content.String()
		if content == "" && chunk.Message.Thinking != "" {
			content = chunk.Message.Thinking
		}
		done := chunk.Done

			log.Printf("[Ollama Backend] Parsed chunk - content: %q, done: %v", content, done)

			ch <- plugin.StreamChunk{
				Content:      content,
				Done:         done,
				FinishReason: "stop",
			}

			if done {
				break
			}
		}

		log.Printf("[Ollama Backend] Stream read completed, total lines: %d", lineCount)

		if err := scanner.Err(); err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to read stream: %w", err)}
		}
	}()

	return ch, nil
}

// Authenticate 认证
func (b *Backend) Authenticate(config any) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	b.config = cfg
	return nil
}

// HealthCheck 健康检查
func (b *Backend) HealthCheck(ctx context.Context) error {
	backendCfg, err := b.SelectBackend()
	if err != nil {
		return err
	}
	
	url := fmt.Sprintf("%s/api/tags", backend.NormalizeOllamaAPIBase(backendCfg.BaseURL))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}

// GetAvailableModels 获取可用模型列表
func (b *Backend) GetAvailableModels() ([]plugin.ModelInfo, error) {
	backendCfg, err := b.SelectBackend()
	if err != nil {
		return nil, err
	}
	
	url := fmt.Sprintf("%s/api/tags", backend.NormalizeOllamaAPIBase(backendCfg.BaseURL))

	resp, err := b.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]plugin.ModelInfo, len(result.Models))
	for i, model := range result.Models {
		models[i] = plugin.ModelInfo{
			ID:   model.Name,
			Name: model.Name,
		}
	}

	return models, nil
}

// 辅助函数：转换消息格式
func convertToOllamaMessages(messages []plugin.Message) []Message {
	ollamaMessages := make([]Message, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = Message{
			Role: msg.Role,
			Content: MessageContent{
				Value: msg.Content, // plugin.Message.Content 是 string
			},
		}
	}
	return ollamaMessages
}

// newLineScanner 创建行扫描器
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	return scanner
}