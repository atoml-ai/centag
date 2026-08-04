package openai

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
	"centag/core/pkg/circuitbreaker"
	"centag/core/pkg/config"
	"centag/core/pkg/plugin"
)

// Backend OpenAI 后端插件
type Backend struct {
	name           string
	status         plugin.PluginStatus
	config         *plugin.BackendConfig
	backendManager *backend.Manager
	currentBackend *backend.BackendConfig
	client         *http.Client
	mu             sync.RWMutex
	accountSelector *backend.AccountPoolSelector
}

// NewBackend 创建 OpenAI 后端插件
func NewBackend() (plugin.Plugin, error) {
	// 对于流式请求，不应设置 HTTP client Timeout，否则会导致长响应被截断
	// 使用 0 表示不设置超时，依赖 context 来控制超时
	return &Backend{
		name:            "openai-backend",
		status:          plugin.StatusStopped,
		backendManager:  backend.GetManager(),
		accountSelector: backend.NewAccountPoolSelector(),
		config: &plugin.BackendConfig{
			BaseURL:    "https://api.openai.com/v1",
			Timeout:    60,
			MaxRetries: 3,
			RetryDelay: 1,
		},
		client: &http.Client{
			Timeout: 0, // 不设置超时，依赖 context 控制
		},
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
	// 加载后端配置
	if err := b.backendManager.Load(); err != nil {
		log.Printf("[OpenAI Backend] Warning: failed to load backend config: %v", err)
	}

	// 使用默认配置或传入的配置
	if cfg, ok := config.(*plugin.BackendConfig); ok {
		b.config = cfg
	}

	// 对于流式请求，不应设置 HTTP client Timeout，否则会导致长响应被截断
	// 使用 0 表示不设置超时，依赖 context 来控制超时
	b.client.Timeout = 0

	log.Printf("[OpenAI Backend] Plugin initialized with BaseURL: %s", b.config.BaseURL)
	return nil
}

// Start 启动插件
func (b *Backend) Start(ctx context.Context) error {
	b.status = plugin.StatusRunning
	log.Printf("[OpenAI Backend] Plugin started")
	return nil
}

// Stop 停止插件
func (b *Backend) Stop(ctx context.Context) error {
	b.status = plugin.StatusStopped
	log.Printf("[OpenAI Backend] Plugin stopped")
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

	// 如果当前有可用的后端配置,使用它
	if b.currentBackend != nil && b.currentBackend.Enabled {
		return b.currentBackend, nil
	}

	// 否则从管理器中选择
	selected, err := b.backendManager.SelectBackend("openai")
	if err != nil {
		// 如果没有配置,使用默认配置
		log.Printf("[OpenAI Backend] No configured backend, using default: %s", b.config.BaseURL)
		return &backend.BackendConfig{
			ID:      "default",
			Name:    "Default OpenAI",
			Type:    "openai",
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
	log.Printf("[OpenAI Backend] Switched to backend: %s (%s)", cfg.ID, cfg.Name)
	return nil
}

// getBackendConfigForReq 获取后端配置。
// 若 req.BackendID 不为空，则优先按 ID 直接获取指定后端；同时检查熔断器状态，
// 如果指定后端被熔断，则尝试 fallback 后端列表。
func (b *Backend) getBackendConfigForReq(req *plugin.ProxyRequest) (*backend.BackendConfig, error) {
	if req != nil && req.BackendID != "" {
		cfg, err := b.backendManager.Get(req.BackendID)
		if err == nil && cfg.Enabled {
			// 检查熔断器
			if !circuitbreaker.IsOpen(cfg.ID) {
				return cfg, nil
			}
			log.Printf("[OpenAI Backend] BackendID %s is circuit-open, trying fallbacks", req.BackendID)
			// 尝试 fallback 后端
			if fb, ok := b.findFallbackBackend(cfg, req); ok {
				return fb, nil
			}
		}
		if err != nil {
			log.Printf("[OpenAI Backend] BackendID %s not found (%v), falling back to SelectBackend", req.BackendID, err)
		} else {
			log.Printf("[OpenAI Backend] BackendID %s is disabled/circuit-open, falling back to SelectBackend", req.BackendID)
		}
	}
	return b.SelectBackend()
}

// findFallbackBackend 从 fallback 列表中查找可用后端
func (b *Backend) findFallbackBackend(primary *backend.BackendConfig, req *plugin.ProxyRequest) (*backend.BackendConfig, bool) {
	for _, fbID := range primary.FallbackBackends {
		if fbID == primary.ID {
			continue // 跳过自身
		}
		fb, err := b.backendManager.Get(fbID)
		if err != nil || !fb.Enabled {
			continue
		}
		if circuitbreaker.IsOpen(fb.ID) {
			continue
		}
		log.Printf("[OpenAI Backend] Using fallback backend: %s -> %s", primary.ID, fbID)
		return fb, true
	}
	return nil, false
}

// CallModel 调用模型 (非流式)
func (b *Backend) CallModel(ctx context.Context, req *plugin.ProxyRequest) (*plugin.ProxyResponse, error) {
	// 获取后端配置
	backendCfg, err := b.getBackendConfigForReq(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend config: %w", err)
	}

	// 构建 OpenAI 请求。优先透传协议层保留的原始 JSON，以保留 scripts/tools/tool_choice 等扩展字段。
	openaiReq := buildOpenAIRequestPayload(req, false)

	// 序列化请求
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")

	// 根据后端类型选择正确的端点
	var url string
	if backendCfg.Type == "ollama" {
		url = fmt.Sprintf("%s/chat", baseURL)
	} else {
		url = buildOpenAIChatURL(baseURL)
	}

	// 账户池：准备 session key
	sessionKey := ""
	if backend.HasAccountPool(backendCfg) {
		sessionKey = backend.ExtractSessionKey(ctx, reqBody, "")
	}

	// 账户池优先：限额/鉴权/5xx 等先换同后端其它 Key，再失败才返回给上层跨后端降级。
	maxAttempts := 1
	if backend.HasAccountPool(backendCfg) {
		enabled := 0
		for _, acc := range backendCfg.AccountPool.Accounts {
			if acc.Enabled {
				enabled++
			}
		}
		if enabled > 1 {
			maxAttempts = enabled
			if backendCfg.MaxRetries > 0 && backendCfg.MaxRetries < maxAttempts {
				maxAttempts = backendCfg.MaxRetries
			}
		}
	}

	var lastErr error
	tried := map[string]bool{}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 选择 API Key（账户池或单 Key）
		apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
		currentAccountID := ""
		if backend.HasAccountPool(backendCfg) {
			var result *backend.AccountPoolResult
			var selErr error
			for skip := 0; skip < maxAttempts; skip++ {
				result, selErr = b.accountSelector.SelectAccountForRequest(ctx, backendCfg.AccountPool, sessionKey)
				if selErr != nil {
					break
				}
				if result != nil && !tried[result.Account.ID] {
					break
				}
				if result != nil {
					b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, result.Account.ID)
				}
				result = nil
			}
			if selErr != nil || result == nil {
				if lastErr != nil {
					return nil, fmt.Errorf("account pool exhausted after %d attempts: %w", attempt, lastErr)
				}
				return nil, fmt.Errorf("account pool select: %w", selErr)
			}
			apiKey = backend.NormalizeOpenAICompatibleAPIKey(result.Key)
			currentAccountID = result.Account.ID
			tried[currentAccountID] = true
		}

		httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
		if reqErr != nil {
			return nil, fmt.Errorf("failed to create request: %w", reqErr)
		}

		// 设置请求头
		httpReq.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		// 发送请求
		resp, doErr := b.client.Do(httpReq)
		if doErr != nil {
			lastErr = fmt.Errorf("failed to send request: %w", doErr)
			if currentAccountID != "" && attempt < maxAttempts-1 {
				b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
				continue
			}
			continue
		}

		// 读取响应
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response: %w", readErr)
			continue
		}

		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			if shouldRotateOpenAIAccount(resp.StatusCode, string(respBody)) && backend.HasAccountPool(backendCfg) && currentAccountID != "" && attempt < maxAttempts-1 {
				log.Printf("[OpenAI Backend] rotate account %s status=%d (attempt %d/%d)", currentAccountID, resp.StatusCode, attempt+1, maxAttempts)
				b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
				lastErr = fmt.Errorf("API error (status %d) on account %s", resp.StatusCode, currentAccountID)
				continue
			}
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}

		// 解析响应
		var openaiResp ChatCompletionResponse
		if err := json.Unmarshal(respBody, &openaiResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// 转换为统一的 ProxyResponse
		content := ""
		reasoningContent := ""
		finishReason := "stop"
		var toolCalls []plugin.ToolCall

		if len(openaiResp.Choices) > 0 {
			originalContent := openaiResp.Choices[0].Message.Content.String()
			finishReason = openaiResp.Choices[0].FinishReason
			reasoningContent = openaiResp.Choices[0].Message.ReasoningContent

			// 优先读取标准 OpenAI 格式的 message.tool_calls（DeepSeek/OpenAI 等标准后端走此路径）
			if stdToolCalls := convertOpenAIToolCallsToPlugin(openaiResp.Choices[0].Message.ToolCalls); len(stdToolCalls) > 0 {
				toolCalls = stdToolCalls
				content = originalContent
				finishReason = "tool_calls"
				log.Printf("[OpenAI Backend] Read standard tool_calls from message: %d calls", len(toolCalls))
			} else if parsedToolCalls, cleanedContent := normalizeToolCalls(originalContent); len(parsedToolCalls) > 0 {
				// 规范化工具调用格式 (将各种非标准格式转换为OpenAI标准格式)
				toolCalls = parsedToolCalls
				content = cleanedContent
				finishReason = "tool_calls"
				log.Printf("[OpenAI Backend] Normalized tool calls to standard format: %d calls", len(toolCalls))
			} else {
				content = originalContent
			}
		}

		// 构建响应
		proxyResp := &plugin.ProxyResponse{
			Content:          content,
			ReasoningContent: reasoningContent,
			TokensUsed:       openaiResp.Usage.CompletionTokens,
			FinishReason:     finishReason,
			Model:            openaiResp.Model,
			ToolCalls:    toolCalls,
			Metadata: map[string]interface{}{
				"prompt_tokens": openaiResp.Usage.PromptTokens,
				"total_tokens":  openaiResp.Usage.TotalTokens,
				"backend_id":    backendCfg.ID,
				"backend_name":  backendCfg.Name,
			},
			RawBody: openaiResp,
		}

		// 如果有工具调用，更新 RawBody 中的响应以包含标准的 tool_calls 格式
		if len(toolCalls) > 0 && len(openaiResp.Choices) > 0 {
			openaiResp.Choices[0].Message.ToolCalls = convertToOpenAIToolCalls(toolCalls)
			openaiResp.Choices[0].FinishReason = "tool_calls"
			proxyResp.RawBody = openaiResp
		}

		return proxyResp, nil
	}

	return nil, fmt.Errorf("all %d attempts exhausted: %w", maxAttempts, lastErr)
}

// CallModelStream 流式调用模型
func (b *Backend) CallModelStream(ctx context.Context, req *plugin.ProxyRequest) (<-chan plugin.StreamChunk, error) {
	ch := make(chan plugin.StreamChunk, 10)

	go func() {
		defer close(ch)
		normalizer := &streamToolCallNormalizer{}

		// 获取后端配置（优先使用 req.BackendID 指定的后端）
		backendCfg, err := b.getBackendConfigForReq(req)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to get backend config: %w", err)}
			return
		}

		// 构建 OpenAI 请求。优先透传协议层保留的原始 JSON，以保留 scripts/tools/tool_choice 等扩展字段。
		openaiReq := buildOpenAIRequestPayload(req, true)
		// OpenAI Chat Completions：请求在流结束后返回 usage（需 stream_options；Ollama 等 /chat 端点勿带此字段）
		if backendCfg.Type != "ollama" {
			if _, exists := openaiReq["stream_options"]; !exists {
				openaiReq["stream_options"] = map[string]interface{}{
					"include_usage": true,
				}
			}
		}

		// 序列化请求
		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		// 创建 HTTP 请求
		baseURL := strings.TrimSuffix(backendCfg.BaseURL, "/")

		// 根据后端类型选择正确的端点
		var url string
		if backendCfg.Type == "ollama" {
			url = fmt.Sprintf("%s/chat", baseURL)
		} else {
			url = buildOpenAIChatURL(baseURL)
		}

		// 账户池：准备 session key；限额/鉴权/5xx 先换同后端其它 Key，再失败才交给上层跨后端降级。
		sessionKey := ""
		if backend.HasAccountPool(backendCfg) {
			sessionKey = backend.ExtractSessionKey(ctx, reqBody, "")
		}

		maxAttempts := 1
		if backend.HasAccountPool(backendCfg) {
			enabled := 0
			for _, acc := range backendCfg.AccountPool.Accounts {
				if acc.Enabled {
					enabled++
				}
			}
			if enabled > 1 {
				maxAttempts = enabled
				if backendCfg.MaxRetries > 0 && backendCfg.MaxRetries < maxAttempts {
					maxAttempts = backendCfg.MaxRetries
				}
			}
		}

		var resp *http.Response
		tried := map[string]bool{}
		var lastErr error
		for attempt := 0; attempt < maxAttempts; attempt++ {
			apiKey := backend.NormalizeOpenAICompatibleAPIKey(backendCfg.APIKey)
			currentAccountID := ""
			if backend.HasAccountPool(backendCfg) {
				var result *backend.AccountPoolResult
				var selErr error
				for skip := 0; skip < maxAttempts; skip++ {
					result, selErr = b.accountSelector.SelectAccountForRequest(ctx, backendCfg.AccountPool, sessionKey)
					if selErr != nil {
						break
					}
					if result != nil && !tried[result.Account.ID] {
						break
					}
					if result != nil {
						b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, result.Account.ID)
					}
					result = nil
				}
				if selErr != nil || result == nil {
					if lastErr != nil {
						ch <- plugin.StreamChunk{Error: fmt.Errorf("account pool exhausted after %d attempts: %w", attempt, lastErr)}
						return
					}
					ch <- plugin.StreamChunk{Error: fmt.Errorf("account pool select: %w", selErr)}
					return
				}
				apiKey = backend.NormalizeOpenAICompatibleAPIKey(result.Key)
				currentAccountID = result.Account.ID
				tried[currentAccountID] = true
			}

			httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
			if err != nil {
				ch <- plugin.StreamChunk{Error: fmt.Errorf("failed to create request: %w", err)}
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Accept", "text/event-stream")
			if apiKey != "" {
				httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			}

			var doErr error
			resp, doErr = b.client.Do(httpReq)
			if doErr != nil {
				lastErr = fmt.Errorf("failed to send request: %w", doErr)
				if currentAccountID != "" && attempt < maxAttempts-1 {
					b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
					continue
				}
				ch <- plugin.StreamChunk{Error: lastErr}
				return
			}

			if resp.StatusCode == http.StatusOK {
				break
			}

			statusCode := resp.StatusCode
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			resp = nil
			if shouldRotateOpenAIAccount(statusCode, string(body)) && currentAccountID != "" && attempt < maxAttempts-1 {
				log.Printf("[OpenAI Backend] rotate stream account %s status=%d (attempt %d/%d)", currentAccountID, statusCode, attempt+1, maxAttempts)
				b.accountSelector.DisableAccountTemporarily(backendCfg.AccountPool, currentAccountID)
				lastErr = fmt.Errorf("API error (status %d) on account %s", statusCode, currentAccountID)
				continue
			}
			ch <- plugin.StreamChunk{Error: fmt.Errorf("API error (status %d): %s", statusCode, string(body))}
			return
		}

		if resp == nil {
			if lastErr != nil {
				ch <- plugin.StreamChunk{Error: lastErr}
				return
			}
			ch <- plugin.StreamChunk{Error: fmt.Errorf("all accounts failed")}
			return
		}
		defer resp.Body.Close()

		// 按 SSE 事件边界（\n\n）读取，避免远程 API 返回多行 JSON 时被按单行拆分导致解析失败、输出被截断
		sseReader := newSSEEventReader(resp.Body)
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
			// 解析 data 行（SSE 允许多行 data:，用 \n 连接）；若多行则为多段 JSON，逐段解析避免整段丢弃
			dataStr := extractSSEData(event)
			if dataStr == "" {
				continue
			}
			// 支持一个 event 内多行 data（部分 API 用 \n 分隔多个 data: 行再跟 \n\n），逐行解析
			parts := splitSSEDataLines(dataStr)
			for _, data := range parts {
				data = strings.TrimSpace(data)
				if data == "" {
					continue
				}
				if data == "[DONE]" {
					if pending, ok := normalizer.flushPending(""); ok {
						ch <- pending
					}
					ch <- plugin.StreamChunk{Done: true}
					return
				}

				rawBytes := []byte(data)
				content, finishReason := parseStreamChunkData(rawBytes)
				usedFlex := false

				if content == "" && finishReason == "" {
					content, finishReason = parseStreamChunkDataFlex(rawBytes)
					usedFlex = true
				}

				if normalizedChunks, handled := normalizer.processChunk(rawBytes, content, finishReason, usedFlex); handled {
					for _, normalized := range normalizedChunks {
						ch <- normalized
						if normalized.Done {
							log.Printf("[OpenAI Backend] Stream finished with reason: %s", normalized.FinishReason)
							return
						}
					}
					continue
				}

			if usedFlex && content != "" {
				log.Printf("[OpenAI Backend] Stream chunk flex-extracted content len=%d", len(content))
			}
			// reasoning_content 等非标准字段产生的空 content 是正常行为，不打印日志

				ch <- plugin.StreamChunk{
					Content:      content,
					Done:         finishReason != "",
					FinishReason: finishReason,
					RawData:      rawBytes,
					// usedFlex && content != "" 说明内容来自非标准字段，需要重构 SSE
					ContentIsNonStandard: usedFlex && content != "",
				}
				if finishReason != "" {
					log.Printf("[OpenAI Backend] Stream finished with reason: %s", finishReason)
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
		httpReq.Header.Set("Authorization", "Bearer "+b.config.APIKey)
	}

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

type streamToolCallNormalizer struct {
	bufferedContent      strings.Builder
	bufferedRaw          []byte
	bufferedHasNonStd    bool
	pendingNormalization bool
}

func (n *streamToolCallNormalizer) processChunk(
	rawBytes []byte,
	content, finishReason string,
	usedFlex bool,
) ([]plugin.StreamChunk, bool) {
	hasNonStdContent := usedFlex && content != ""
	if n.pendingNormalization || shouldBufferToolCallChunk(content) {
		n.pendingNormalization = true
		n.bufferedContent.WriteString(content)
		n.bufferedRaw = rawBytes
		n.bufferedHasNonStd = n.bufferedHasNonStd || hasNonStdContent

		buffered := n.bufferedContent.String()
		if hasCompleteToolCallEnvelope(buffered) {
			parsedToolCalls, cleanedContent := normalizeToolCalls(buffered)
			if len(parsedToolCalls) > 0 {
				chunk := plugin.StreamChunk{
					Content:              cleanedContent,
					Done:                 finishReason != "",
					FinishReason:         finishReason,
					RawData:              rawBytes,
					ContentIsNonStandard: n.bufferedHasNonStd && cleanedContent != "",
				}

				// 已解析出工具调用后，finish_reason 统一标准化为 tool_calls，
				// 避免后端返回 stop 导致客户端不执行工具。
				if chunk.FinishReason == "" || chunk.FinishReason == "stop" {
					chunk.FinishReason = "tool_calls"
					chunk.Done = true
				}

				modifiedJSON, err := convertToolCallsInJSON(rawBytes, parsedToolCalls, cleanedContent)
				if err == nil {
					chunk.RawData = modifiedJSON
					log.Printf("[OpenAI Backend] Stream normalized buffered tool calls, modified raw JSON")
				} else {
					log.Printf("[OpenAI Backend] Failed to normalize buffered tool calls in JSON: %v", err)
				}

				n.reset()
				return []plugin.StreamChunk{chunk}, true
			}
		}

		if finishReason != "" || n.bufferedContent.Len() > 64*1024 {
			flushed, ok := n.flushPending(finishReason)
			if ok {
				return []plugin.StreamChunk{flushed}, true
			}
		}

		return nil, true
	}

	return nil, false
}

func (n *streamToolCallNormalizer) flushPending(finishReason string) (plugin.StreamChunk, bool) {
	if !n.pendingNormalization {
		return plugin.StreamChunk{}, false
	}

	content := n.bufferedContent.String()
	chunk := plugin.StreamChunk{
		Content:              content,
		Done:                 finishReason != "",
		FinishReason:         finishReason,
		RawData:              n.bufferedRaw,
		ContentIsNonStandard: n.bufferedHasNonStd && content != "",
	}
	n.reset()
	return chunk, true
}

func (n *streamToolCallNormalizer) reset() {
	n.bufferedContent.Reset()
	n.bufferedRaw = nil
	n.bufferedHasNonStd = false
	n.pendingNormalization = false
}

func shouldBufferToolCallChunk(content string) bool {
	if content == "" {
		return false
	}
	return strings.Contains(content, "<minimax:tool_call") ||
		strings.Contains(content, "<toolcall") ||
		strings.Contains(content, "<invoke name=")
}

func hasCompleteToolCallEnvelope(content string) bool {
	if content == "" {
		return false
	}
	if !strings.Contains(content, "</invoke>") {
		return false
	}

	hasMinimax := strings.Contains(content, "<minimax:tool_call") && strings.Contains(content, "</minimax:tool_call>")
	hasToolcall := strings.Contains(content, "<toolcall") && strings.Contains(content, "</toolcall>")
	return hasMinimax || hasToolcall
}

// GetAvailableModels 获取可用模型列表
func (b *Backend) GetAvailableModels() ([]plugin.ModelInfo, error) {
	return []plugin.ModelInfo{
		{
			ID:      "gpt-4",
			Name:    "GPT-4",
			Enabled: true,
		},
		{
			ID:      "gpt-4-turbo-preview",
			Name:    "GPT-4 Turbo Preview",
			Enabled: true,
		},
		{
			ID:      "qwen/qwen3-4b-fp8",
			Name:    "GPT-3.5 Turbo",
			Enabled: true,
		},
	}, nil
}

// convertToOpenAIMessages 转换为 OpenAI 消息格式
func convertToOpenAIMessages(messages []plugin.Message) []Message {
	result := make([]Message, len(messages))
	for i, msg := range messages {
		var toolCalls []ToolCall
		if len(msg.ToolCalls) > 0 {
			toolCalls = convertToOpenAIToolCalls(msg.ToolCalls)
		}
		result[i] = Message{
			Role: msg.Role,
			Content: MessageContent{
				Value: msg.Content, // plugin.Message.Content 是 string
			},
			ToolCalls:        toolCalls,
			ToolCallID:       msg.ToolCallID,
			ReasoningContent: msg.ReasoningContent,
		}
	}
	return result
}

func buildOpenAIRequestPayload(req *plugin.ProxyRequest, stream bool) map[string]interface{} {
	if req == nil {
		return map[string]interface{}{"stream": stream}
	}

	var payload map[string]interface{}
	if rawBody, ok := req.RawBody.(map[string]interface{}); ok && rawBody != nil {
		payload = cloneRawBodyMap(rawBody)
	} else {
		payload = buildLegacyOpenAIRequestPayload(req)
	}

	if req.Model != "" {
		payload["model"] = req.Model
	}
	payload["stream"] = stream

	if _, exists := payload["messages"]; !exists && len(req.Messages) > 0 {
		payload["messages"] = convertToOpenAIMessages(req.Messages)
	}

	return payload
}

func buildLegacyOpenAIRequestPayload(req *plugin.ProxyRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"messages": convertToOpenAIMessages(req.Messages),
		"model":    req.Model,
	}

	if req.Temperature != 0 {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens != 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.TopP != 0 {
		payload["top_p"] = req.TopP
	}
	if req.FrequencyPenalty != 0 {
		payload["frequency_penalty"] = req.FrequencyPenalty
	}
	if req.PresencePenalty != 0 {
		payload["presence_penalty"] = req.PresencePenalty
	}
	if len(req.Stop) > 0 {
		payload["stop"] = req.Stop
	}

	return payload
}

func cloneRawBodyMap(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}

	encoded, err := json.Marshal(raw)
	if err == nil {
		var cloned map[string]interface{}
		if err = json.Unmarshal(encoded, &cloned); err == nil {
			return cloned
		}
	}

	// 保底浅拷贝，避免直接修改上游结构。
	cloned := make(map[string]interface{}, len(raw))
	for k, v := range raw {
		cloned[k] = v
	}
	return cloned
}

// StreamChunk OpenAI 流式响应块
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content string `json:"content,omitempty"`
			Role    string `json:"role,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// ChatCompletionRequest OpenAI ChatCompletion 请求
type ChatCompletionRequest struct {
	Messages         []Message `json:"messages"`
	Model            string    `json:"model"`
	Stream           bool      `json:"stream,omitempty"`
	Temperature      float64   `json:"temperature,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
}

// ChatCompletionResponse OpenAI ChatCompletion 响应
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice 选择项
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 消息（支持多模态内容）
type Message struct {
	Role             string         `json:"role"`
	Content          MessageContent `json:"content"`
	ToolCalls        []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
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

// MarshalJSON 自定义JSON序列化
func (mc *MessageContent) MarshalJSON() ([]byte, error) {
	return json.Marshal(mc.Value)
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

// Usage 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// sseEventReader 按 SSE 事件边界（\n\n）读取，兼容多行 JSON 的 data 内容
type sseEventReader struct {
	reader *bufio.Reader
}

func newSSEEventReader(r io.Reader) *sseEventReader {
	return &sseEventReader{reader: bufio.NewReader(r)}
}

// ReadEvent 读取一个完整 SSE 事件（到 \n\n 为止），返回事件体原文（不含结尾 \n\n）
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
		// 若当前行仅为 \n，则与上一行组成 \n\n，事件结束
		if len(line) == 1 && line[0] == '\n' {
			return strings.TrimSpace(string(block)), nil
		}
	}
}

// extractSSEData 从 SSE 事件文本中提取 data 字段：支持多行 "data: xxx"，按规范用 \n 连接
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

// splitSSEDataLines 将一段 data 拆成多行处理：若整段可解析为 JSON 则返回单元素；否则按 \n 拆分（兼容同一 event 内多段 data）
func splitSSEDataLines(dataStr string) []string {
	dataStr = strings.TrimSpace(dataStr)
	if dataStr == "" {
		return nil
	}
	// 整段能解析则视为单条（含多行 pretty-print JSON）
	if json.Valid([]byte(dataStr)) {
		return []string{dataStr}
	}
	if !strings.Contains(dataStr, "\n") {
		return []string{dataStr}
	}
	// 多段 JSON（如 "json1\njson2"），按行拆开
	var out []string
	for _, line := range strings.Split(dataStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// parseStreamChunkData 用标准 StreamChunk 解析，返回 content 与 finish_reason
func parseStreamChunkData(data []byte) (content, finishReason string) {
	var chunk StreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return "", ""
	}
	if len(chunk.Choices) == 0 {
		return "", ""
	}
	return chunk.Choices[0].Delta.Content, chunk.Choices[0].FinishReason
}

// parseStreamChunkDataFlex 宽松解析：支持 delta.content 为 string 或 array；若无 delta 则尝试 message.content
func parseStreamChunkDataFlex(data []byte) (content, finishReason string) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", ""
	}
	choices, _ := raw["choices"].([]interface{})
	if len(choices) == 0 {
		return "", ""
	}
	choice, _ := choices[0].(map[string]interface{})
	if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
		finishReason = fr
	}
	// 优先 delta（流式常见）
	delta, _ := choice["delta"].(map[string]interface{})
	if delta != nil {
		content = extractStandardContentFromDelta(delta)
		if content != "" || finishReason != "" {
			return content, finishReason
		}
	}
	// 部分网关单 chunk 返回 message
	msg, _ := choice["message"].(map[string]interface{})
	if msg != nil {
		content = extractStandardContentFromDelta(msg)
	}
	return content, finishReason
}

// extractStandardContentFromDelta 仅提取 OpenAI 标准主文本字段 content（string 或数组片段）。
// 不把 reasoning_content、thinking 等合并进主文本：否则代理会把推理与正文拼在一起，
// 与直连 API 时客户端只展示 delta.content、单独处理 reasoning 的行为不一致。
func extractStandardContentFromDelta(delta map[string]interface{}) string {
	if delta == nil {
		return ""
	}
	return extractDeltaContentValue(delta["content"])
}

// extractDeltaContentValue 从单个 delta 字段值中提取字符串：支持 string 或 []object 且取 .text
func extractDeltaContentValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch c := v.(type) {
	case string:
		return c
	case []interface{}:
		var s string
		for _, item := range c {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					s += t
				}
			}
		}
		return s
	}
	return ""
}

// normalizeToolCalls 规范化工具调用格式
// 检测并转换各种非标准的工具调用格式为OpenAI标准格式
// 目前支持的格式:
// - MiniMax: <minimax:tool_call><invoke name="xxx">...</invoke></minimax:tool_call>
func normalizeToolCalls(content string) ([]plugin.ToolCall, string) {
	// 尝试解析 MiniMax XML 格式
	if strings.Contains(content, "<minimax:tool_call>") || strings.Contains(content, "<toolcall>") {
		return parseXMLToolCalls(content)
	}

	// 未来可以添加其他格式的解析器
	// 例如: parseAnthropicToolCalls(), parseAzureToolCalls() 等

	return nil, content
}

// parseXMLToolCalls 解析XML样式的工具调用格式
// 支持格式: <minimax:tool_call><invoke name="xxx">...</invoke></minimax:tool_call>
func parseXMLToolCalls(content string) ([]plugin.ToolCall, string) {
	var toolCalls []plugin.ToolCall

	// 提取工具调用标签 (支持多种可能的标签名)
	startPatterns := []string{
		"<minimax:tool_call>",
		"<toolcall>",
	}

	var startTag, endTag string
	var startIdx int
	for _, pattern := range startPatterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			startTag = pattern
			startIdx = idx
			break
		}
	}

	if startTag == "" {
		return nil, content
	}

	// 确定结束标签
	endTag = strings.Replace(startTag, "<", "</", 1)
	if !strings.Contains(content, endTag) {
		// 如果没有结束标签,可能是因为标签格式不同,尝试常见变体
		endTagCandidates := []string{
			"</minimax:tool_call>",
			"</toolcall>",
		}
		for _, candidate := range endTagCandidates {
			if strings.Contains(content, candidate) {
				endTag = candidate
				break
			}
		}
	}

	// 提取工具调用部分
	toolCallContent := ""
	endIdx := strings.Index(content, endTag)
	if endIdx != -1 {
		toolCallContent = content[startIdx+len(startTag) : endIdx]
	} else {
		toolCallContent = content[startIdx+len(startTag):]
	}

	// 解析 invoke 标签 (支持单引号和双引号)
	invokePatterns := []struct {
		start string
		end   string
	}{
		{"<invoke name=\"", "\">"},
		{"<invoke name='", "'>"},
	}

	var funcName string
	var invokeCloseIdx int
	for _, pattern := range invokePatterns {
		if invokeIdx := strings.Index(toolCallContent, pattern.start); invokeIdx != -1 {
			nameStart := invokeIdx + len(pattern.start)
			if nameEnd := strings.Index(toolCallContent[nameStart:], pattern.end); nameEnd != -1 {
				funcName = toolCallContent[nameStart : nameStart+nameEnd]
				invokeCloseIdx = strings.Index(toolCallContent, "</invoke>")
				break
			}
		}
	}

	if funcName == "" {
		return nil, content
	}

	// 提取函数参数
	var funcArgs string
	if invokeCloseIdx != -1 {
		argsStart := strings.Index(toolCallContent, ">") + 1
		if argsStart < invokeCloseIdx {
			funcArgs = toolCallContent[argsStart:invokeCloseIdx]
		}
	}

	// 构建标准的 tool_call
	toolCalls = append(toolCalls, plugin.ToolCall{
		ID:   generateToolCallID(),
		Type: "function",
		Function: plugin.FunctionCall{
			Name:      funcName,
			Arguments: funcArgs,
		},
	})

	// 从原始内容中移除工具调用标签
	cleanedContent := content[:startIdx]
	if endIdx != -1 {
		cleanedContent += content[endIdx+len(endTag):]
	}
	cleanedContent = strings.TrimSpace(cleanedContent)

	log.Printf("[Tool Call Normalization] Parsed XML format: name=%s, args=%s", funcName, funcArgs)

	return toolCalls, cleanedContent
}

// generateToolCallID 生成工具调用 ID
func generateToolCallID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 29)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return "call_" + string(b)
}

// convertToOpenAIToolCalls 将内部 ToolCall 转换为 OpenAI 格式
func convertToOpenAIToolCalls(toolCalls []plugin.ToolCall) []ToolCall {
	if toolCalls == nil {
		return nil
	}

	result := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		result[i] = ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// convertOpenAIToolCallsToPlugin 将 OpenAI 协议的 ToolCall 转换为 plugin.ToolCall。
// 用于从后端响应的 message.tool_calls 字段读取标准 function calling 结果。
func convertOpenAIToolCallsToPlugin(toolCalls []ToolCall) []plugin.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	result := make([]plugin.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		tcType := tc.Type
		if tcType == "" {
			tcType = "function"
		}
		result[i] = plugin.ToolCall{
			ID:   tc.ID,
			Type: tcType,
			Function: plugin.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return result
}

// convertToolCallsInJSON 在原始 JSON 数据中转换工具调用格式
func convertToolCallsInJSON(rawJSON []byte, toolCalls []plugin.ToolCall, cleanedContent string) ([]byte, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// 获取 choices[0].delta
	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no delta in choice")
	}

	// 更新 content
	delta["content"] = cleanedContent

	// 添加 tool_calls
	if len(toolCalls) > 0 {
		openAIToolCalls := make([]map[string]interface{}, len(toolCalls))
		for i, tc := range toolCalls {
			openAIToolCalls[i] = map[string]interface{}{
				"id":   tc.ID,
				"type": tc.Type,
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			}
		}
		delta["tool_calls"] = openAIToolCalls
	}

	// 序列化回 JSON
	modifiedJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified JSON: %w", err)
	}

	return modifiedJSON, nil
}

// shouldRotateOpenAIAccount：同后端多 key 时，限额/鉴权/上游错误优先换本后端其它 key。
func shouldRotateOpenAIAccount(statusCode int, body string) bool {
	if statusCode == http.StatusBadRequest {
		return config.IsBillingOrQuotaFailure(statusCode, body)
	}
	if statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusPaymentRequired ||
		statusCode == http.StatusForbidden ||
		statusCode >= 500 {
		return true
	}
	return config.IsBillingOrQuotaFailure(statusCode, body)
}

// buildOpenAIChatURL 构建 OpenAI 兼容的聊天 API URL
// 标准 OpenAI API 路径为 /v1/chat/completions，但部分后端配置的 base_url 已包含 /v1 前缀
// （如 "https://api.ppio.com/openai/v1"），此时只需追加 /chat/completions。
// 对于未包含版本前缀的（如 "https://integrate.api.nvidia.com"），则追加 /v1/chat/completions。
func buildOpenAIChatURL(baseURL string) string {
	if hasAPIVersionPrefix(baseURL) {
		return baseURL + "/chat/completions"
	}
	return baseURL + "/v1/chat/completions"
}

// hasAPIVersionPrefix 检查 baseURL 是否已包含 API 版本前缀（/v1、/v2、/v4 等）
func hasAPIVersionPrefix(baseURL string) bool {
	trimmed := strings.TrimSuffix(baseURL, "/")
	// 查找最后一个 / 之后的路径段
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		seg := trimmed[idx+1:]
		if len(seg) > 1 && seg[0] == 'v' {
			for _, c := range seg[1:] {
				if c < '0' || c > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
}
