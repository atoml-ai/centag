package memory_query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"centag/core/pkg/pipeline"
)

// RegisterMemoryQueryPlugin 注册记忆查询插件
func RegisterMemoryQueryPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&MemoryQueryPlugin{})
}

// MemoryQueryPlugin 记忆查询插件
type MemoryQueryPlugin struct{}

func (p *MemoryQueryPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "Memory Query",
		Implementation: "example.memory-query",
		Kind:           "memory.query",
		Version:        "1.0.0",
		Description:    "查询用户或会话的记忆数据，用于个性化响应",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scope": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"user", "session", "global"},
					"description": "查询范围",
					"default":     "user",
				},
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大记忆数",
					"default":     10,
				},
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "查询文本",
				},
			},
			"required": []string{"query"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"memories": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content": map[string]interface{}{"type": "string"},
							"score":   map[string]interface{}{"type": "number"},
							"time":    map[string]interface{}{"type": "string"},
						},
					},
				},
			},
		},
		Permissions: []string{"memory.read"},
	}
}

func (p *MemoryQueryPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	return nil
}

func (p *MemoryQueryPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	// 获取配置参数
	scope := "user"
	topK := 10
	
	if req.Config.CustomConfig != nil {
		if s, ok := req.Config.CustomConfig["scope"].(string); ok && s != "" {
			scope = s
		}
		if k, ok := req.Config.CustomConfig["top_k"].(float64); ok {
			topK = int(k)
		}
	}

	// 获取用户ID和会话ID（从 Context 中获取）
	userID := ""
	sessionID := ""
	if req.Context != nil {
		if uid, ok := req.Context["user_id"].(string); ok {
			userID = uid
		}
		if sid, ok := req.Context["session_id"].(string); ok {
			sessionID = sid
		}
	}

	// 执行记忆查询
	query := req.Input.Content
	memories := p.queryMemories(query, scope, userID, sessionID, topK)

	// 构建结果
	result := map[string]interface{}{
		"memories":   memories,
		"count":      len(memories),
		"query":      query,
		"scope":      scope,
		"user_id":    userID,
		"session_id": sessionID,
	}

	resultJSON, _ := json.Marshal(result)

	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: string(resultJSON),
			Metadata: map[string]interface{}{
				"count":      len(memories),
				"scope":      scope,
				"user_id":    userID,
				"session_id": sessionID,
			},
		},
	}, nil
}

// queryMemories 查询记忆
func (p *MemoryQueryPlugin) queryMemories(query, scope, userID, sessionID string, topK int) []map[string]interface{} {
	// 模拟记忆库
	mockMemories := []map[string]interface{}{
		{
			"content":   "用户偏好使用 Go 语言进行开发",
			"score":     0.95,
			"time":      "2026-05-01T10:00:00Z",
			"scope":     "user",
			"user_id":   "user123",
			"session_id": "session456",
		},
		{
			"content":   "用户询问过关于 LLM 代理的问题",
			"score":     0.88,
			"time":      "2026-05-02T14:30:00Z",
			"scope":     "user",
			"user_id":   "user123",
			"session_id": "session789",
		},
		{
			"content":   "当前会话讨论的是 Centag 项目",
			"score":     0.92,
			"time":      "2026-05-05T09:00:00Z",
			"scope":     "session",
			"user_id":   "user123",
			"session_id": "session456",
		},
		{
			"content":   "全局知识：Centag 是一个 LLM 反向代理",
			"score":     0.85,
			"time":      "2026-04-01T00:00:00Z",
			"scope":     "global",
			"user_id":   "",
			"session_id": "",
		},
		{
			"content":   "用户喜欢简洁的代码风格",
			"score":     0.78,
			"time":      "2026-04-15T16:00:00Z",
			"scope":     "user",
			"user_id":   "user123",
			"session_id": "session999",
		},
	}

	// 根据 scope 过滤记忆
	var filtered []map[string]interface{}
	queryLower := strings.ToLower(query)

	for _, memory := range mockMemories {
		memScope := memory["scope"].(string)
		memUserID := memory["user_id"].(string)
		memSessionID := memory["session_id"].(string)

		// 根据 scope 过滤
		include := false
		switch scope {
		case "user":
			// 用户范围：匹配 user_id
			if memScope == "user" && memUserID == userID {
				include = true
			}
		case "session":
			// 会话范围：匹配 session_id
			if memScope == "session" && memSessionID == sessionID {
				include = true
			}
		case "global":
			// 全局范围：只包含 global scope
			if memScope == "global" {
				include = true
			}
		}

		if include {
			// 根据查询相关性排序（简化实现）
			content := memory["content"].(string)
			score := memory["score"].(float64)

			// 如果查询词出现在记忆中，增加相关性
			if strings.Contains(strings.ToLower(content), queryLower) {
				score += 0.1
				memory["score"] = score
			}

			filtered = append(filtered, memory)
		}
	}

	// 按分数排序（降序）
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i]["score"].(float64) < filtered[j]["score"].(float64) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	// 限制返回数量
	if len(filtered) > topK {
		filtered = filtered[:topK]
	}

	return filtered
}
