package proxy

import (
	"encoding/json"
	"centag/core/pkg/logger"
	"strings"

	"go.uber.org/zap"
)

// MetadataEnhancer 响应元数据增强器
// 在不破坏标准协议的前提下，为响应附加策略决策信息
type MetadataEnhancer struct {
	enabled bool
}

// NewMetadataEnhancer 创建元数据增强器
func NewMetadataEnhancer(enabled bool) *MetadataEnhancer {
	return &MetadataEnhancer{
		enabled: enabled,
	}
}

// StrategyMetadata 策略元数据
type StrategyMetadata struct {
	RequestID       string            `json:"proxyclaw_request_id,omitempty"`       // 请求ID
	CacheStatus     string            `json:"proxyclaw_cache,omitempty"`            // 缓存状态: hit, miss
	SelectedBackend string            `json:"proxyclaw_backend,omitempty"`         // 使用的后端
	BackendModel    string            `json:"proxyclaw_model,omitempty"`           // 使用的模型
	SelectionReason string            `json:"proxyclaw_reason,omitempty"`          // 选择原因
	LatencyMs       int               `json:"proxyclaw_latency,omitempty"`         // 总延迟(毫秒)
	FromCache       bool              `json:"proxyclaw_from_cache,omitempty"`      // 是否来自缓存
	CacheSimilarity float64           `json:"proxyclaw_cache_similarity,omitempty"` // 缓存相似度
	TokensUsed      int               `json:"proxyclaw_tokens,omitempty"`          // 使用token数
	DecisionPath    []DecisionStep    `json:"proxyclaw_path,omitempty"`           // 决策路径
	Metrics         map[string]interface{} `json:"proxyclaw_metrics,omitempty"`     // 额外指标
}

// DecisionStep 决策步骤
type DecisionStep struct {
	Step        int         `json:"step"`        // 步骤序号
	Action      string      `json:"action"`      // 动作名称
	Description string      `json:"description"` // 描述
	Data        interface{} `json:"data,omitempty"`      // 步骤数据
	Duration    int         `json:"duration,omitempty"`   // 耗时(毫秒)
}

// EnhanceResponse 增强响应，添加策略元数据
func (me *MetadataEnhancer) EnhanceResponse(
	responseBody []byte,
	metadata *StrategyMetadata,
) ([]byte, error) {
	if !me.enabled {
		return responseBody, nil
	}

	// 解析原始响应
	var originalResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &originalResponse); err != nil {
		logger.Error("解析原始响应失败", zap.Error(err))
		// 如果解析失败，返回原始响应
		return responseBody, nil
	}

	// 添加元数据字段
	metadataMap, err := me.metadataToMap(metadata)
	if err != nil {
		logger.Error("转换元数据失败", zap.Error(err))
		return responseBody, nil
	}

	for key, value := range metadataMap {
		originalResponse[key] = value
	}

	// 重新序列化
	enhancedResponse, err := json.Marshal(originalResponse)
	if err != nil {
		logger.Error("序列化增强响应失败", zap.Error(err))
		return responseBody, nil
	}

	logger.Debug("响应元数据增强成功",
		zap.String("request_id", func() string {
			if metadata != nil {
				return metadata.RequestID
			}
			return ""
		}()),
		zap.Int("added_fields", len(metadataMap)))

	return enhancedResponse, nil
}

// EnhanceStreamChunk 增强流式响应块
// 对于SSE流，在第一个chunk后发送一个特殊的元数据事件
func (me *MetadataEnhancer) EnhanceStreamChunk(
	isFirstChunk bool,
	metadata *StrategyMetadata,
) ([]byte, bool) {
	if !me.enabled || !isFirstChunk {
		return nil, false
	}

	// 构造元数据事件
	metadataEvent := map[string]interface{}{
		"id":      metadata.RequestID + "_metadata",
		"event":   "proxyclaw_metadata",
		"data":    metadata,
	}

	metadataJSON, err := json.Marshal(metadataEvent)
	if err != nil {
		logger.Error("构造流式元数据失败", zap.Error(err))
		return nil, false
	}

	// SSE格式: "data: <json>\n\n"
	sseData := "data: " + string(metadataJSON) + "\n\n"

	logger.Debug("流式元数据增强成功",
		zap.String("request_id", metadata.RequestID))

	return []byte(sseData), true
}

// ExtractMetadata 从响应中提取元数据
func (me *MetadataEnhancer) ExtractMetadata(responseBody []byte) (*StrategyMetadata, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	metadata := &StrategyMetadata{}
	found := false

	// 检查并提取所有以"proxyclaw_"开头的字段
	for key, value := range response {
		if strings.HasPrefix(key, "proxyclaw_") {
			found = true
			switch key {
			case "proxyclaw_request_id":
				if s, ok := value.(string); ok {
					metadata.RequestID = s
				}
			case "proxyclaw_cache":
				if s, ok := value.(string); ok {
					metadata.CacheStatus = s
				}
			case "proxyclaw_backend":
				if s, ok := value.(string); ok {
					metadata.SelectedBackend = s
				}
			case "proxyclaw_model":
				if s, ok := value.(string); ok {
					metadata.BackendModel = s
				}
			case "proxyclaw_reason":
				if s, ok := value.(string); ok {
					metadata.SelectionReason = s
				}
			case "proxyclaw_latency":
				if f, ok := value.(float64); ok {
					metadata.LatencyMs = int(f)
				}
			case "proxyclaw_from_cache":
				if b, ok := value.(bool); ok {
					metadata.FromCache = b
				}
			case "proxyclaw_cache_similarity":
				if f, ok := value.(float64); ok {
					metadata.CacheSimilarity = f
				}
			case "proxyclaw_tokens":
				if f, ok := value.(float64); ok {
					metadata.TokensUsed = int(f)
				}
			case "proxyclaw_path":
				// 解析决策路径
				if data, err := json.Marshal(value); err == nil {
					var steps []DecisionStep
					if err := json.Unmarshal(data, &steps); err == nil {
						metadata.DecisionPath = steps
					}
				}
			case "proxyclaw_metrics":
				if m, ok := value.(map[string]interface{}); ok {
					metadata.Metrics = m
				}
			}
		}
	}

	if !found {
		return nil, nil // 没有找到元数据
	}

	return metadata, nil
}

// metadataToMap 将元数据转换为map
func (me *MetadataEnhancer) metadataToMap(metadata *StrategyMetadata) (map[string]interface{}, error) {
	if metadata == nil {
		return make(map[string]interface{}), nil
	}

	// 检查是否是空元数据（所有字段都是默认值）
	if metadata.RequestID == "" &&
		metadata.CacheStatus == "" &&
		metadata.SelectedBackend == "" &&
		metadata.BackendModel == "" &&
		metadata.SelectionReason == "" &&
		metadata.LatencyMs == 0 &&
		!metadata.FromCache &&
		metadata.CacheSimilarity == 0 &&
		metadata.TokensUsed == 0 &&
		len(metadata.DecisionPath) == 0 &&
		len(metadata.Metrics) == 0 {
		return make(map[string]interface{}), nil
	}

	result := make(map[string]interface{})

	// 基本字段
	if metadata.RequestID != "" {
		result["proxyclaw_request_id"] = metadata.RequestID
	}
	if metadata.CacheStatus != "" {
		result["proxyclaw_cache"] = metadata.CacheStatus
	}
	if metadata.SelectedBackend != "" {
		result["proxyclaw_backend"] = metadata.SelectedBackend
	}
	if metadata.BackendModel != "" {
		result["proxyclaw_model"] = metadata.BackendModel
	}
	if metadata.SelectionReason != "" {
		result["proxyclaw_reason"] = metadata.SelectionReason
	}
	if metadata.LatencyMs > 0 {
		result["proxyclaw_latency"] = metadata.LatencyMs
	}
	result["proxyclaw_from_cache"] = metadata.FromCache
	if metadata.CacheSimilarity > 0 {
		result["proxyclaw_cache_similarity"] = metadata.CacheSimilarity
	}
	if metadata.TokensUsed > 0 {
		result["proxyclaw_tokens"] = metadata.TokensUsed
	}
	if len(metadata.DecisionPath) > 0 {
		result["proxyclaw_path"] = metadata.DecisionPath
	}
	if len(metadata.Metrics) > 0 {
		result["proxyclaw_metrics"] = metadata.Metrics
	}

	return result, nil
}

// BuildDecisionPath 构建决策路径
func BuildDecisionPath(steps []struct {
	Step        int
	Action      string
	Description string
	Data        interface{}
	Duration    int
}) []DecisionStep {
	path := make([]DecisionStep, len(steps))
	for i, step := range steps {
		path[i] = DecisionStep{
			Step:        step.Step,
			Action:      step.Action,
			Description: step.Description,
			Data:        step.Data,
			Duration:    step.Duration,
		}
	}
	return path
}

// CreateCacheHitMetadata 创建缓存命中的元数据
func CreateCacheHitMetadata(requestID, backend, model string, similarity float64, latency int) *StrategyMetadata {
	return &StrategyMetadata{
		RequestID:       requestID,
		CacheStatus:     "hit",
		SelectedBackend: backend,
		BackendModel:    model,
		SelectionReason: "缓存命中",
		LatencyMs:       latency,
		FromCache:       true,
		CacheSimilarity: similarity,
		DecisionPath: BuildDecisionPath([]struct {
			Step        int
			Action      string
			Description string
			Data        interface{}
			Duration    int
		}{
			{
				Step:        1,
				Action:      "cache_check",
				Description: "检查缓存",
				Data: map[string]interface{}{
					"similarity": similarity,
				},
				Duration: latency,
			},
		}),
	}
}

// CreateBackendSelectMetadata 创建后端选择的元数据
func CreateBackendSelectMetadata(requestID, backend, model, reason string, latency, tokens int, path []DecisionStep) *StrategyMetadata {
	return &StrategyMetadata{
		RequestID:       requestID,
		CacheStatus:     "miss",
		SelectedBackend: backend,
		BackendModel:    model,
		SelectionReason: reason,
		LatencyMs:       latency,
		FromCache:       false,
		TokensUsed:      tokens,
		DecisionPath:    path,
	}
}
