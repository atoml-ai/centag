package rag_retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"centag/core/pkg/pipeline"
)

// RegisterRAGRetrievalPlugin 注册 RAG 检索插件
func RegisterRAGRetrievalPlugin(registry *pipeline.NodeRegistry) {
	registry.RegisterPlugin(&RAGRetrievalPlugin{})
}

// RAGRetrievalPlugin RAG 检索插件
type RAGRetrievalPlugin struct{}

func (p *RAGRetrievalPlugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "RAG Retrieval",
		Implementation: "example.rag-retrieval",
		Kind:           "retrieval.rag",
		Version:        "1.0.0",
		Description:    "从知识库中检索相关文档片段，用于增强生成",
		ConfigSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"top_k": map[string]interface{}{
					"type":        "integer",
					"description": "返回的最大文档数",
					"default":     5,
				},
				"threshold": map[string]interface{}{
					"type":        "number",
					"description": "相似度阈值",
					"default":     0.7,
				},
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "检索查询文本",
				},
			},
			"required": []string{"query"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"documents": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"content":  map[string]interface{}{"type": "string"},
							"score":    map[string]interface{}{"type": "number"},
							"source":   map[string]interface{}{"type": "string"},
						},
					},
				},
				"count": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		Permissions: []string{"storage.read"},
	}
}

func (p *RAGRetrievalPlugin) ValidateConfig(config pipeline.NodeConfig) error {
	return nil
}

func (p *RAGRetrievalPlugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}

	// 获取配置参数
	topK := 5
	threshold := 0.7
	
	if req.Config.CustomConfig != nil {
		if k, ok := req.Config.CustomConfig["top_k"].(float64); ok {
			topK = int(k)
		}
		if t, ok := req.Config.CustomConfig["threshold"].(float64); ok {
			threshold = t
		}
	}

	// 执行 RAG 检索
	query := req.Input.Content
	documents := p.retrieveDocuments(query, topK, threshold)

	// 构建结果
	result := map[string]interface{}{
		"documents": documents,
		"count":     len(documents),
		"query":     query,
	}

	resultJSON, _ := json.Marshal(result)

	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: string(resultJSON),
			Metadata: map[string]interface{}{
				"count":     len(documents),
				"threshold": threshold,
				"top_k":     topK,
			},
		},
	}, nil
}

// retrieveDocuments 模拟检索文档
func (p *RAGRetrievalPlugin) retrieveDocuments(query string, topK int, threshold float64) []map[string]interface{} {
	// 模拟文档库
	mockDocuments := []map[string]interface{}{
		{
			"content": "Go is a statically typed, compiled programming language designed at Google.",
			"score":   0.95,
			"source":  "go-docs",
		},
		{
			"content": "Go's concurrency model is based on goroutines and channels.",
			"score":   0.88,
			"source":  "go-concurrency",
		},
		{
			"content": "The Go standard library provides robust HTTP server support.",
			"score":   0.82,
			"source":  "go-net-http",
		},
		{
			"content": "Go modules provide dependency management for Go projects.",
			"score":   0.75,
			"source":  "go-modules",
		},
		{
			"content": "Testing is built into Go with the testing package.",
			"score":   0.71,
			"source":  "go-testing",
		},
	}

	// 根据查询关键词过滤和排序（简化实现）
	var results []map[string]interface{}
	queryLower := strings.ToLower(query)
	
	for _, doc := range mockDocuments {
		content := doc["content"].(string)
		score := doc["score"].(float64)
		
		// 如果查询词出现在文档中，增加相关性
		if strings.Contains(strings.ToLower(content), queryLower) {
			score += 0.1
		}
		
		// 只返回超过阈值的文档
		if score >= threshold {
			doc["score"] = score
			results = append(results, doc)
		}
	}

	// 限制返回数量
	if len(results) > topK {
		results = results[:topK]
	}

	return results
}
