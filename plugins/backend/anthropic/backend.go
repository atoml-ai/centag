package anthropic

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

// Backend Anthropic 后端插件
type Backend struct {
	name            string
	status          plugin.PluginStatus
	config          *plugin.BackendConfig
	backendManager  *backend.Manager
	currentBackend  *backend.BackendConfig
	client          *http.Client
	mu              sync.RWMutex
	accountSelector *backend.AccountPoolSelector
}

// NewBackend 创建 Anthropic 后端插件
func NewBackend() (plugin.Plugin, error) {
	return &Backend{
		name:            "anthropic-backend",
		status:          plugin.StatusStopped,
		backendManager:  backend.GetManager(),
		accountSelector: backend.NewAccountPoolSelector(),
		config: &plugin.BackendConfig{
			BaseURL:    "https://api.anthropic.com/v1",
			Timeout:    60,
			MaxRetries: 3,
			RetryDelay: 1,
		},
		client: &http.Client{Timeout: 0},
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
	if err := b.backendManager.Load(); err != nil {
		log.Printf("[Anthropic Backend] Warning: failed to load backend config: %v", err)
	}

	if cfg, ok := config.(*plugin.BackendConfig); ok {
		b.config = cfg
	}

	b.client.Timeout = 0
	log.Printf("[Anthropic Backend] Plugin initialized with BaseURL: %s", b.config.BaseURL)
	return nil
}

// Start 启动插件
func (b *Backend) Start(ctx context.Context) error {
	b.status = plugin.StatusRunning
	log.Printf("[Anthropic Backend] Plugin started")
	return nil
}

// Stop 停止插件
func (b *Backend) Stop(ctx context.Context) error {
	b.status = plugin.StatusStopped
	log.Printf("[Anthropic Backend] Plugin stopped")
	return nil
}

// Status 返回插件状态
func (b *Backend) Status() plugin.PluginStatus {
	return b.status
}

// SelectBackend 选择后端(动态)
func (b *Backend) SelectBackend() (*backend.BackendConfig, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.currentBackend != nil && b.currentBackend.Enabled {
		return b.currentBackend, nil
	}

	selected, err := b.backendManager.SelectBackend("anthropic")
	if err != nil {
		log.Printf("[Anthropic Backend] No configured backend, using default: %s", b.config.BaseURL)
		return &backend.BackendConfig{
			ID:      "default",
			Name:    "Default Anthropic",
			Type:    "anthropic",
			BaseURL: b.config.BaseURL,
			APIKey:  b.config.APIKey,
			Timeout: b.config.Timeout,
		}, nil
	}

	b.currentBackend = selected
	return selected, nil
}

// SetBackend 手动设置后端
func (b *Backend) SetBackend(backendID string) error {
	cfg, err := b.backendManager.Get(backendID)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.currentBackend = cfg
	log.Printf("[Anthropic Backend] Switched to backend: %s (%s)", cfg.ID, cfg.Name)
	return nil
}

// getBackendConfigForReq 获取后端配置
func (b *Backend) getBackendConfigForReq(req *plugin.ProxyRequest) (*backend.BackendConfig, error) {
	if req != nil && req.BackendID != "" {
		cfg, err := b.backendManager.Get(req.BackendID)
		if err == nil && cfg.Enabled {
			return cfg, nil
		}
		if err != nil {
			log.Printf("[Anthropic Backend] BackendID %s not found (%v), falling back to SelectBackend", req.BackendID, err)
		} else {
			log.Printf("[Anthropic Backend] BackendID %s is disabled, falling back to SelectBackend", req.BackendID)
		}
	}
	return b.SelectBackend()
}

// CallModel 调用 Anthropic Messages API (非流式)
func (b *Backend) CallModel(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	backendCfg, err := b.getBackendConfigForReq(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend config: %w", err)
	}

	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"messages":   convertToAnthropicMessages(req.Messages),
		"stream":     false,
	}

	if req.TopP > 0 {
		anthropicReq["top_p"] = req.TopP
	}
	if req.Temperature > 0 {
		anthropicReq["temperature"] = req.Temperature
	}
	if req.System != "" {
		anthropicReq["system"] = req.System
	}

	reqBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")
	url := fmt.Sprintf("%s/messages", baseURL)

	// 账户池：准备 session key
	sessionKey := ""
	if backend.HasAccountPool(backendCfg) {
		sessionKey = backend.ExtractSessionKey(ctx, reqBody, "")
	}

	// 账户池 429 故障转移
	maxAttempts := 1
	if backend.HasAccountPool(backendCfg) && len(backendCfg.AccountPool.Accounts) > 1 {
		maxAttempts = backendCfg.MaxRetries
		if maxAttempts <= 0 {
			maxAttempts = 3
		}
		if maxAttempts > len(backendCfg.AccountPool.Accounts) {
			maxAttempts = len(backendCfg.AccountPool.Accounts)
		}
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 选择 API Key
		apiKey := backendCfg.APIKey
		currentAccountID := ""
		if backend.HasAccountPool(backendCfg) {
			result, selErr := b.accountSelector.SelectAccountForRequest(ctx, backendCfg.AccountPool, sessionKey)
			if selErr != nil {
				return nil, fmt.Errorf("account pool select: %w", selErr)
			}
			apiKey = result.Key
			currentAccountID = result.Account.ID
		}

		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if reqErr != nil {
			return nil, fmt.Errorf("failed to create request: %w", reqErr)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
		}
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		resp, doErr := b.client.Do(httpReq)
		if doErr != nil {
			lastErr = fmt.Errorf("failed to send request: %w", doErr)
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusTooManyRequests && backend.HasAccountPool(backendCfg) && attempt < maxAttempts-1 {
				log.Printf("[Anthropic Backend] 429 rate limit on account %s, rotating to next (attempt %d/%d)", currentAccountID, attempt+1, maxAttempts)
				b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
				lastErr = fmt.Errorf("API 429 on account %s", currentAccountID)
				continue
			}
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var anthropicResp MessagesResponse
		if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		content := ""
		for _, block := range anthropicResp.Content {
			if block.Type == "text" {
				content += block.Text
			}
		}

		finishReason := anthropicResp.StopReason
		if finishReason == "" {
			finishReason = "stop"
		}

		return &plugin.ProxyResponse{
			Content:      content,
			TokensUsed:   anthropicResp.Usage.OutputTokens,
			FinishReason: finishReason,
			Model:        anthropicResp.Model,
			Metadata: map[string]interface{}{
				"prompt_tokens": anthropicResp.Usage.InputTokens,
				"total_tokens":  anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
				"backend_id":    backendCfg.ID,
				"backend_name":  backendCfg.Name,
			},
			RawBody: anthropicResp,
		}, nil
	}

	return nil, fmt.Errorf("all %d attempts exhausted: %w", maxAttempts, lastErr)
}

// CallModelStream 流式调用 Anthropic Messages API
func (b *Backend) CallModelStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error) {
	ch := make(chan plugin.StreamChunk, 10)

	go func() {
		defer close(ch)

		backendCfg, err := b.getBackendConfigForReq(req)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to get backend config: %w", err)}
			return
		}

		anthropicReq := map[string]interface{}{
			"model":      req.Model,
			"max_tokens": req.MaxTokens,
			"messages":   convertToAnthropicMessages(req.Messages),
			"stream":     true,
		}

		if req.TopP > 0 {
			anthropicReq["top_p"] = req.TopP
		}
		if req.Temperature > 0 {
			anthropicReq["temperature"] = req.Temperature
		}
		if req.System != "" {
			anthropicReq["system"] = req.System
		}

		reqBody, err := json.Marshal(anthropicReq)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")
		url := fmt.Sprintf("%s/messages", baseURL)

		// 账户池：选择 API Key
		sessionKey := ""
		if backend.HasAccountPool(backendCfg) {
			sessionKey = backend.ExtractSessionKey(ctx, reqBody, "")
		}
		apiKey := backendCfg.APIKey
		currentAccountID := ""
		if backend.HasAccountPool(backendCfg) {
			result, selErr := b.accountSelector.SelectAccountForRequest(ctx, backendCfg.AccountPool, sessionKey)
			if selErr != nil {
				ch <- plugin.StreamChunk{Error: fmt.Errorf("account pool select: %w", selErr)}
				return
			}
			apiKey = result.Key
			currentAccountID = result.Account.ID
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if apiKey != "" {
			httpReq.Header.Set("x-api-key", apiKey)
		}
		httpReq.Header.Set("anthropic-version", "2023-06-01")

		resp, err := b.client.Do(httpReq)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusTooManyRequests && backend.HasAccountPool(backendCfg) {
				log.Printf("[Anthropic Backend] 429 rate limit on stream account %s, disabling", currentAccountID)
				b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
			}
			body, _ := io.ReadAll(resp.Body)
			ch <- plugin.StreamChunk{Error: fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))}
			return
		}

		sseReader := newSSEEventReader(resp.Body)
		var usageAcc anthropicStreamUsageAcc
		for {
			event, err := sseReader.ReadEvent()
			if err != nil {
				if err != io.EOF {
					ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to read stream: %w", err)}
				}
				return
			}
			if event == "" {
				continue
			}

			dataStr := extractSSEData(event)
			if dataStr == "" {
				continue
			}

			parts := splitSSEDataLines(dataStr)
			for _, data := range parts {
				data = strings.TrimSpace(data)
				if data == "" {
					continue
				}

				usageAcc.ingestDataLine(data)
				content, stopReason := parseAnthropicStreamChunk(data)
				sc := plugin.StreamChunk{
					Content:      content,
					Done:         stopReason != "",
					FinishReason: stopReason,
				}
				if stopReason != "" && usageAcc.prompt+usageAcc.completion > 0 {
					sc.UsagePromptTokens = usageAcc.prompt
					sc.UsageCompletionTokens = usageAcc.completion
				}
				ch <- sc

				if stopReason != "" {
					return
				}
			}
		}
	}()

	return ch, nil
}

// Authenticate 认证
func (b *Backend) Authenticate(config any) error {
	cfg, ok := config.(*plugin.BackendConfig)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	b.config = cfg
	return nil
}

// HealthCheck 健康检查
func (b *Backend) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/models", b.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if b.config.APIKey != "" {
		httpReq.Header.Set("x-api-key", b.config.APIKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetAvailableModels 获取可用模型列表
func (b *Backend) GetAvailableModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{
			ID:      "claude-3-5-sonnet-20241022",
			Name:    "Claude 3.5 Sonnet",
			Enabled: true,
		},
		{
			ID:      "claude-3-opus-20240229",
			Name:    "Claude 3 Opus",
			Enabled: true,
		},
		{
			ID:      "claude-3-sonnet-20240229",
			Name:    "Claude 3 Sonnet",
			Enabled: true,
		},
		{
			ID:      "claude-3-haiku-20240307",
			Name:    "Claude 3 Haiku",
			Enabled: true,
		},
	}, nil
}

// convertToAnthropicMessages 转换为 Anthropic 消息格式
func convertToAnthropicMessages(messages []plugin.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = map[string]interface{}{
			"role":    msg.Role,
			"content": []ContentBlock{{Type: "text", Text: msg.Content}},
		}
	}
	return result
}

// MessagesResponse Anthropic Messages API 响应
type MessagesResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Usage 使用量
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// sseEventReader SSE 事件读取器
type sseEventReader struct {
	reader *bufio.Reader
}

func newSSEEventReader(r io.Reader) *sseEventReader {
	return &sseEventReader{reader: bufio.NewReader(r)}
}

// ReadEvent 读取一个完整 SSE 事件
func (r *sseEventReader) ReadEvent() (string, error) {
	var block []byte
	for {
		line, err := r.reader.ReadBytes('\n')
		if len(line) > 0 {
			block = append(block, line...)
		}
		if err != nil {
			if err == io.EOF && len(block) > 0 {
				return strings.TrimSpace(string(block)), nil
			}
			return "", err
		}
		if len(line) == 1 && line[0] == '\n' {
			return strings.TrimSpace(string(block)), nil
		}
	}
}

// extractSSEData 从 SSE 事件文本中提取 data 字段
func extractSSEData(event string) string {
	var parts []string
	for _, line := range strings.Split(event, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			parts = append(parts, strings.TrimPrefix(after, " "))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// splitSSEDataLines 拆分 SSE 数据行
func splitSSEDataLines(dataStr string) []string {
	dataStr = strings.TrimSpace(dataStr)
	if dataStr == "" {
		return nil
	}
	if json.Valid([]byte(dataStr)) {
		return []string{dataStr}
	}
	if !strings.Contains(dataStr, "\n") {
		return []string{dataStr}
	}
	var out []string
	for _, line := range strings.Split(dataStr, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// anthropicStreamUsageAcc 从 Anthropic SSE JSON 行累积 input/output tokens（映射为 prompt/completion）。
type anthropicStreamUsageAcc struct {
	prompt     int
	completion int
}

func (a *anthropicStreamUsageAcc) ingestDataLine(data string) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return
	}
	typ, _ := m["type"].(string)
	switch typ {
	case "message_start":
		msg, ok := m["message"].(map[string]interface{})
		if !ok {
			return
		}
		u, ok := msg["usage"].(map[string]interface{})
		if !ok {
			return
		}
		a.prompt = jsonNumberToInt(u["input_tokens"])
		if c := jsonNumberToInt(u["output_tokens"]); c > a.completion {
			a.completion = c
		}
	case "message_delta":
		d, ok := m["delta"].(map[string]interface{})
		if !ok {
			return
		}
		u, ok := d["usage"].(map[string]interface{})
		if !ok {
			return
		}
		if c := jsonNumberToInt(u["output_tokens"]); c > a.completion {
			a.completion = c
		}
	}
}

func jsonNumberToInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// parseAnthropicStreamChunk 解析 Anthropic 流式数据块
func parseAnthropicStreamChunk(data string) (content, stopReason string) {
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", ""
	}

	if chunkType, ok := chunk["type"].(string); ok {
		if chunkType == "message_stop" {
			if sr, ok := chunk["stop_reason"].(string); ok {
				stopReason = sr
			} else {
				stopReason = "stop"
			}
			return "", stopReason
		}

		if chunkType == "content_block_delta" {
			if delta, ok := chunk["delta"].(map[string]interface{}); ok {
				if text, ok := delta["text"].(string); ok {
					return text, ""
				}
			}
		}
	}

	return "", ""
}
