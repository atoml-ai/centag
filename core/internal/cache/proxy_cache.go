package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	evalplugin "centag/core/internal/cache/evaluation/plugin"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/plugin"

	"go.uber.org/zap"
)

var (
	userLineRegex        = regexp.MustCompile("(?i)(?:^|\\n)user[\\x22\":\\s]+(.+?)(?:\\n\\n|\\n[A-Za-z]+:|\\n```|$)")
	codeFenceRegex       = regexp.MustCompile("(?s)```[\\s\\S]*?```")
	timestampPrefixRegex = regexp.MustCompile(`(?i)(?:^\[?)[^\]\n]*(?:\d{1,2}[\s/-]\d{1,2}[\s/-]\d{2,4}|\d{1,2}:\d{2})\s*(?:utc|gmt(?:[+-]\d{1,2})?(?:\:\d{2})?)[^\]\n]*(?:\])\s*`)
)

// ProxyCache 代理缓存包装器
type ProxyCache struct {
	manager  *Manager
	enabled  bool
	expander QueryExpander // 查询展开器
}

// QueryExpander 查询展开接口（避免循环导入）
type QueryExpander interface {
	Expand(ctx context.Context, current string, history []plugin.Message) (string, bool, error)
}

// NewProxyCache 创建代理缓存
func NewProxyCache(manager *Manager, enabled bool) *ProxyCache {
	return &ProxyCache{
		manager: manager,
		enabled: enabled,
	}
}

// SetExpander 设置查询展开器
func (p *ProxyCache) SetExpander(expander QueryExpander) {
	p.expander = expander
}

// NormalizeMessagesForKey 将 messages 归一化为与代理路径一致的结构（仅 role/content），
// 确保预测接口与 /v1/chat/completions 生成的缓存键一致。
func NormalizeMessagesForKey(messages []interface{}) []interface{} {
	if len(messages) == 0 {
		return messages
	}
	out := make([]interface{}, len(messages))
	for i, msg := range messages {
		role, content := "", ""
		if m, ok := msg.(map[string]interface{}); ok {
			if r, ok := m["role"].(string); ok {
				role = r
			}
			if c, ok := m["content"].(string); ok {
				content = c
			}
		}
		// 与 proxy.convertMessagesToInterface 一致：仅保留 role、content，key 顺序不影响 json.Marshal（会按字母序）
		out[i] = map[string]interface{}{
			"role":    role,
			"content": content,
		}
	}
	return out
}

// GetRequestKey 生成请求缓存键
// 只使用最后一条用户消息,因为代理服务应该是透明的,不应该关心历史对话
func (p *ProxyCache) GetRequestKey(model string, messages []interface{}, temperature float64, maxTokens int) (string, error) {
	// 只提取最后一条用户消息作为缓存键
	var lastUserMessage interface{}
	for i := len(messages) - 1; i >= 0; i-- {
		if msgMap, ok := messages[i].(map[string]interface{}); ok {
			if role, ok := msgMap["role"].(string); ok && role == "user" {
				lastUserMessage = msgMap
				break
			}
		}
	}

	// 如果没有用户消息,返回空字符串(不进行缓存)
	if lastUserMessage == nil {
		return "", nil
	}

	// 构建缓存键的内容 - 只包含最后一条用户消息
	keyData := map[string]interface{}{
		"model":       model,
		"message":     lastUserMessage,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	logger.Info("GetRequestKey called",
		zap.String("model", model),
		zap.Int("messages_count", len(messages)),
		zap.Float64("temperature", temperature),
		zap.Int("max_tokens", maxTokens))

	key, err := GenerateKey(keyData)
	if err != nil {
		return "", fmt.Errorf("failed to generate cache key: %w", err)
	}

	return key, nil
}

// GetRequestQuery 提取请求的查询文本(用于语义搜索)
// 如果配置了查询展开器，会将指代词展开为完整实体
func (p *ProxyCache) GetRequestQuery(messages []interface{}) string {
	if len(messages) == 0 {
		return ""
	}

	// 提取所有消息构建历史上下文
	history := make([]plugin.Message, 0, len(messages))
	var lastUserContent string

	for _, msg := range messages {
		if msgMap, ok := msg.(map[string]interface{}); ok {
			role, _ := msgMap["role"].(string)
			content, _ := msgMap["content"].(string)

			history = append(history, plugin.Message{
				Role:    role,
				Content: content,
			})

			if role == "user" {
				lastUserContent = content
			}
		}
	}

	// 如果没有用户消息，返回空
	if lastUserContent == "" {
		return ""
	}

	// 提取真正的问题内容（过滤元数据）
	cleanQuery := extractCleanQuestion(lastUserContent)

	// 如果有查询展开器，尝试展开查询
	if p.expander != nil && len(history) > 1 {
		expanded, isExpanded, err := p.expander.Expand(context.Background(), cleanQuery, history[:len(history)-1])
		if err == nil && isExpanded {
			logger.Debug("Query expanded for semantic search",
				zap.String("original", cleanQuery),
				zap.String("expanded", expanded))
			return expanded
		}
	}

	return cleanQuery
}

// extractCleanQuestion 从包含元数据的内容中提取真正的问题
// 过滤掉常见的元数据格式，如 Conversation info、Sender info 等
func extractCleanQuestion(content string) string {
	if content == "" {
		return ""
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimSpace(timestampPrefixRegex.ReplaceAllString(normalized, ""))

	// 额外处理：移除开头的数字 + GMT 时区格式（如 "19 GMT+8] "）
	// 这种格式在 openclaw 中很常见，但正则表达式难以完全匹配
	trimmed := strings.TrimSpace(normalized)
	if strings.Contains(trimmed, "GMT") && strings.Contains(trimmed, "]") {
		if idx := strings.Index(trimmed, "]"); idx > 0 {
			beforeColon := trimmed[:idx]
			if afterColon := strings.TrimSpace(trimmed[idx+1:]); afterColon != "" {
				// 检查 ] 前面是否包含时间和 GMT
				if strings.Contains(beforeColon, "GMT") {
					normalized = afterColon
				}
			}
		}
	}

	// 定义需要过滤的元数据模式
	patterns := []string{
		"Conversation info",
		"Sender",
		"message_id",
		"sender_id",
		"sender",
		"timestamp",
		"untrusted metadata",
		"```json",
		"```",
		"UTC]",
		"GMT]",
	}

	// 检查是否包含元数据标记
	hasMetadata := false
	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			hasMetadata = true
			break
		}
	}

	if !hasMetadata {
		return strings.TrimSpace(normalized)
	}

	// 先去掉代码块，避免 JSON 元数据中的冒号干扰后续解析。
	sanitized := codeFenceRegex.ReplaceAllString(normalized, "\n")

	// 使用正则表达式提取最后一个 "role: user" 后面的实际内容
	// 匹配模式: "user: 实际问题\n\n" 或 "user": "实际问题"
	matches := userLineRegex.FindAllStringSubmatch(sanitized, -1)
	if len(matches) > 0 {
		// 取最后一个匹配
		lastMatch := matches[len(matches)-1]
		if len(lastMatch) > 1 {
			question := strings.TrimSpace(lastMatch[1])
			// 移除可能的引号
			question = strings.Trim(question, `"`)
			if question != "" {
				return question
			}
		}
	}

	// 回退：尝试查找最后一个 "：" 或 ":" 后的内容
	lines := strings.Split(sanitized, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		line = strings.TrimSpace(timestampPrefixRegex.ReplaceAllString(line, ""))

		// 跳过空行和元数据行
		if line == "" || strings.HasPrefix(line, "```") ||
			strings.Contains(line, "Conversation info") ||
			strings.Contains(line, "Sender") ||
			strings.Contains(line, "metadata") ||
			strings.HasPrefix(line, "{") ||
			strings.HasPrefix(line, "}") ||
			strings.HasPrefix(line, `"`) {
			continue
		}
		// 找到实际的问题内容
		if idx := strings.LastIndex(line, ":"); idx > 0 && idx < len(line)-1 {
			question := strings.TrimSpace(line[idx+1:])
			if question != "" && !strings.HasPrefix(question, "{") {
				return question
			}
		}
		// 如果行中有问号，也可能是问题
		if strings.Contains(line, "?") || strings.Contains(line, "？") {
			return line
		}
		// 对中文短句（无问号）兜底返回，避免回退到整段元数据。
		if line != "" {
			return line
		}
	}

	return strings.TrimSpace(timestampPrefixRegex.ReplaceAllString(normalized, ""))
}

// TryGet 尝试从缓存获取响应(精确匹配)
// 注意: 此方法只负责获取缓存数据,不记录命中/未命中统计
// 调用方根据返回结果决定是否记录统计
func (p *ProxyCache) TryGet(ctx context.Context, key string) (string, bool, error) {
	if !p.enabled {
		return "", false, nil
	}

	// 直接从精确匹配缓存获取,不记录统计
	entry, err := p.manager.GetExactCache().Get(ctx, key)
	if err != nil {
		logger.Error("Failed to get cache", zap.Error(err))
		return "", false, nil
	}

	if entry != nil {
		logger.Debug("Exact cache hit", zap.String("key", key))
		return entry.Response, true, nil
	}

	logger.Debug("Exact cache miss", zap.String("key", key))
	return "", false, nil
}

// TryGetEntry 尝试从缓存获取完整条目(精确匹配)
// 返回完整的CacheEntry,包含流式数据等信息
func (p *ProxyCache) TryGetEntry(ctx context.Context, key string) (*CacheEntry, bool, error) {
	if !p.enabled {
		return nil, false, nil
	}

	// 直接从精确匹配缓存获取
	entry, err := p.manager.GetExactCache().Get(ctx, key)
	if err != nil {
		logger.Error("Failed to get cache entry", zap.Error(err))
		return nil, false, nil
	}

	if entry != nil {
		logger.Debug("Exact cache entry hit", zap.String("key", key))
		return entry, true, nil
	}

	logger.Debug("Exact cache entry miss", zap.String("key", key))
	return nil, false, nil
}

// SemanticResult 语义搜索结果
type SemanticResult struct {
	Response   string
	Similarity float32
	CacheKey   string
}

// TryGetSemantic 尝试从语义缓存获取响应
func (p *ProxyCache) TryGetSemantic(ctx context.Context, query string, threshold float32, topK int) (*SemanticResult, bool, error) {
	if !p.enabled {
		return nil, false, nil
	}

	// 使用语义搜索
	entries, err := p.manager.SearchByQuery(ctx, query, threshold, topK)
	if err != nil {
		logger.Error("Failed to search semantic cache", zap.Error(err))
		return nil, false, nil
	}

	if len(entries) > 0 {
		// 返回相似度最高的缓存
		bestEntry := entries[0]
		similarity := extractSimilarity(bestEntry.Metadata)

		// ⚠️ 重要: 必须检查相似度是否达到阈值
		// SearchByQuery返回所有结果(包括低于阈值的),用于前端显示
		if similarity < threshold {
			logger.Info("Semantic cache miss (similarity below threshold)",
				zap.String("query", query),
				zap.String("cache_key", bestEntry.Key),
				zap.Float32("similarity", similarity),
				zap.Float32("threshold", threshold))
			return nil, false, nil
		}

		logger.Info("Semantic cache hit",
			zap.String("query", query),
			zap.String("cache_key", bestEntry.Key),
			zap.Float32("similarity", similarity),
			zap.Float32("threshold", threshold))

		// 如果Response为空,尝试从StreamData中合并
		response := bestEntry.Response
		if response == "" && len(bestEntry.StreamData) > 0 {
			var fullContent strings.Builder
			for _, chunk := range bestEntry.StreamData {
				fullContent.WriteString(chunk.Content)
			}
			response = fullContent.String()
			logger.Debug("Reconstructed response from stream data",
				zap.String("key", bestEntry.Key),
				zap.Int("reconstructed_length", len(response)))
		}

		result := &SemanticResult{
			Response:   response,
			Similarity: similarity,
			CacheKey:   bestEntry.Key,
		}

		return result, true, nil
	}

	return nil, false, nil
}

// extractSimilarity 从元数据中提取相似度分数
func extractSimilarity(metadata map[string]interface{}) float32 {
	if metadata == nil {
		return 0
	}
	if score, ok := metadata["similarity_score"].(float32); ok {
		return score
	}
	if score, ok := metadata["similarity_score"].(float64); ok {
		return float32(score)
	}
	return 0
}

// ShouldCache 评估是否应该缓存此响应
func (p *ProxyCache) ShouldCache(ctx context.Context, question, answer string, historyMessages []plugin.Message) bool {
	if p.manager == nil {
		return true // 默认允许缓存
	}

	// 将 plugin.Message 转换为 evalplugin.Message
	evalMessages := make([]evalplugin.Message, len(historyMessages))
	for i, msg := range historyMessages {
		evalMessages[i] = evalplugin.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	result, err := p.manager.EvaluateCacheEntry(ctx, question, answer, evalMessages)
	if err != nil {
		logger.Warnf("Evaluation failed, allowing cache: %v", err)
		return true
	}

	return result.ShouldCache
}

// SetResponse 缓存响应
func (p *ProxyCache) SetResponse(ctx context.Context, key string, response string, metadata map[string]interface{}, ttl time.Duration) error {
	if !p.enabled {
		return nil
	}

	// 从metadata中提取原始请求文本(用于语义匹配的embedding)
	requestText := ""
	if reqText, ok := metadata["request_text"].(string); ok {
		requestText = reqText
	} else {
		// 如果没有request_text，则使用metadata的字符串表示
		requestText = fmt.Sprintf("%v", metadata)
	}

	entry := &CacheEntry{
		Key:       key,
		Request:   requestText, // 使用原始请求文本而不是metadata的字符串
		Response:  response,
		Metadata:  metadata,
		Timestamp: time.Now(),
		IsStream:  false, // 默认为非流式
	}

	if err := p.manager.Set(ctx, key, entry, ttl); err != nil {
		logger.Error("Failed to set cache", zap.Error(err))
		return err
	}

	logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// SetStreamResponse 缓存流式响应
// streamChunks: 流式分块数据
// fullResponse: 合并后的完整响应
func (p *ProxyCache) SetStreamResponse(ctx context.Context, key string, streamChunks []StreamChunk, fullResponse string, metadata map[string]interface{}, ttl time.Duration) error {
	if !p.enabled {
		return nil
	}

	// 从metadata中提取原始请求文本
	requestText := ""
	if reqText, ok := metadata["request_text"].(string); ok {
		requestText = reqText
	} else {
		requestText = fmt.Sprintf("%v", metadata)
	}

	entry := &CacheEntry{
		Key:        key,
		Request:    requestText,
		Response:   fullResponse, // 缓存合并后的完整响应
		Metadata:   metadata,
		Timestamp:  time.Now(),
		IsStream:   true,
		StreamData: streamChunks, // 缓存流式分块数据
	}

	if err := p.manager.Set(ctx, key, entry, ttl); err != nil {
		logger.Error("Failed to set stream cache", zap.Error(err))
		return err
	}

	logger.Info("Stream cache set", zap.String("key", key), zap.Int("chunk_count", len(streamChunks)), zap.Duration("ttl", ttl))
	return nil
}

// Invalidate 使缓存失效
func (p *ProxyCache) Invalidate(ctx context.Context, key string) error {
	if !p.enabled {
		return nil
	}

	return p.manager.Delete(ctx, key)
}

// ClearAll 清空所有缓存
func (p *ProxyCache) ClearAll(ctx context.Context) error {
	if !p.enabled {
		return nil
	}

	return p.manager.Clear(ctx)
}

// GetStats 获取缓存统计
func (p *ProxyCache) GetStats() *CacheStats {
	return p.manager.Stats()
}

// GetManager 获取缓存管理器
func (p *ProxyCache) GetManager() *Manager {
	return p.manager
}

// IsEnabled 检查缓存是否启用
func (p *ProxyCache) IsEnabled() bool {
	return p.enabled
}

// SetEnabled 设置缓存启用状态
func (p *ProxyCache) SetEnabled(enabled bool) {
	p.enabled = enabled
	logger.Info("Cache enabled status changed", zap.Bool("enabled", enabled))
}

// SetSaveOnlyMode 设置仅保存模式
func (p *ProxyCache) SetSaveOnlyMode(enabled bool) {
	if p.manager != nil {
		p.manager.SetSaveOnlyMode(enabled)
	}
}

// ShouldSplitQA 检查是否应该进行问答拆分
// saveOnlyMode: 是否是仅保存模式，仅保存模式下不进行问答拆分
func (p *ProxyCache) ShouldSplitQA(saveOnlyMode bool) bool {
	// 仅保存模式下不进行问答拆分
	if saveOnlyMode {
		return false
	}
	return p.manager.ShouldSplitQA()
}

// InsertSemanticCacheEntry 直接将条目写入语义缓存（不经过 manager.Set 的 skip 检测）。
// 用于 QA split 未实际拆分（原子问题）时，补写被跳过的主请求语义缓存。
// 与「仅保存」「精确策略」「关闭自动向量化」解耦：这些场景下不得走向量化。
func (p *ProxyCache) InsertSemanticCacheEntry(ctx context.Context, key, requestText, response string, metadata map[string]interface{}, ttl time.Duration) error {
	if !p.enabled {
		return nil
	}
	if p.manager != nil && p.manager.GetSaveOnlyMode() {
		logger.Debug("InsertSemanticCacheEntry skipped: save-only mode")
		return nil
	}
	if cfg := config.Get(); cfg != nil {
		if cfg.Cache.Strategy == "exact" {
			logger.Debug("InsertSemanticCacheEntry skipped: exact-only cache strategy")
			return nil
		}
		if !cfg.Cache.Semantic.EnableAutoEmbedding {
			logger.Debug("InsertSemanticCacheEntry skipped: semantic.enable_auto_embedding is false")
			return nil
		}
	}
	if p.manager == nil {
		return nil
	}
	semanticCache := p.manager.GetSemanticCache()
	if semanticCache == nil {
		return nil
	}
	entry := &CacheEntry{
		Key:       key,
		Request:   requestText,
		Response:  response,
		Metadata:  metadata,
		Timestamp: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}
	return semanticCache.InsertWithEmbedding(ctx, entry, nil)
}

// SetSaveOnlyResponse 仅保存模式：保存问答数据用于浏览，不用于缓存命中
// 只写入精确缓存，不进行向量化，不参与缓存命中流程
func (p *ProxyCache) SetSaveOnlyResponse(ctx context.Context, key, requestText, response string, metadata map[string]interface{}, ttl time.Duration) error {
	if !p.enabled {
		return nil
	}

	// 确保标记为仅保存模式
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["save_only"] = true
	metadata["request_text"] = requestText

	entry := &CacheEntry{
		Key:       key,
		Request:   requestText,
		Response:  response,
		Metadata:  metadata,
		Timestamp: time.Now(),
		IsStream:  false,
	}

	// 只写入精确缓存（用于浏览），不写入语义缓存
	if err := p.manager.GetExactCache().Set(ctx, key, entry, ttl); err != nil {
		logger.Error("Failed to save-only cache (exact)", zap.Error(err))
		return err
	}

	logger.Info("Save-only cache written (for browsing only)",
		zap.String("key", key),
		zap.String("request_text", requestText))
	return nil
}

// GetQASplitter 获取问答拆分器
func (p *ProxyCache) GetQASplitter() interface{} {
	return p.manager.GetQASplitter()
}

// ParseRequestMetadata 解析请求元数据
func ParseRequestMetadata(req map[string]interface{}) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	// 提取模型
	if model, ok := req["model"]; ok {
		metadata["model"] = model
	}

	// 提取温度
	if temp, ok := req["temperature"]; ok {
		metadata["temperature"] = temp
	}

	// 提取最大token数
	if maxTokens, ok := req["max_tokens"]; ok {
		metadata["max_tokens"] = maxTokens
	}

	// 提取消息数
	if messages, ok := req["messages"]; ok {
		if msgArray, ok := messages.([]interface{}); ok {
			metadata["message_count"] = len(msgArray)
		}
	}

	return metadata, nil
}

// ParseResponseMetadata 解析响应元数据
func ParseResponseMetadata(resp string) (map[string]interface{}, error) {
	metadata := make(map[string]interface{})

	// 尝试解析OpenAI响应格式
	var openaiResp map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &openaiResp); err == nil {
		// 提取token使用信息
		if usage, ok := openaiResp["usage"].(map[string]interface{}); ok {
			metadata["prompt_tokens"] = usage["prompt_tokens"]
			metadata["completion_tokens"] = usage["completion_tokens"]
			metadata["total_tokens"] = usage["total_tokens"]
		}

		// 提取模型
		if model, ok := openaiResp["model"]; ok {
			metadata["model"] = model
		}
	}

	return metadata, nil
}
