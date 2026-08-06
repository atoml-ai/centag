// Package rag_retrieval registers the business.rag_retrieval pipeline plugin (S3 usage B).
// Production default: empty retrieval (no silent mock). Set allow_mock=true in node config
// or CENTAG_RAG_ALLOW_MOCK=1 only for local demos.
package rag_retrieval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"centag/core/pkg/pipeline"
	"centag/core/pkg/plugin"
)

func init() {
	plugin.RegisterBusinessPlugin("rag_retrieval", Register)
}

// Register wires the plugin into NodeRegistry + BusinessPluginRegistry.
func Register(nodeRegistry interface{}, bizRegistry interface{}) error {
	nr, ok := nodeRegistry.(*pipeline.NodeRegistry)
	if !ok || nr == nil {
		return fmt.Errorf("rag_retrieval: invalid nodeRegistry")
	}
	br, ok := bizRegistry.(*pipeline.BusinessPluginRegistry)
	if !ok || br == nil {
		return fmt.Errorf("rag_retrieval: invalid bizRegistry")
	}
	p := &Plugin{}
	return br.Register(p, nr.RegisterPlugin)
}

// Plugin implements pipeline.BusinessPlugin for RAG retrieval.
type Plugin struct{}

func (p *Plugin) Descriptor() pipeline.NodePluginDescriptor {
	return pipeline.NodePluginDescriptor{
		Name:           "RAG Retrieval",
		Implementation: "business.rag_retrieval",
		Kind:           "retrieval.rag",
		Version:        "1.0.0",
		Description:    "Retrieve knowledge snippets for prompt augmentation (fail-closed without corpus)",
		Permissions:    []string{"storage.read"},
	}
}

func (p *Plugin) GetBusinessType() string { return "rag_retrieval" }

func (p *Plugin) GetDependencies() []string { return []string{"storage.read"} }

func (p *Plugin) GetBusinessMetadata() pipeline.BusinessPluginMetadata {
	return pipeline.BusinessPluginMetadata{
		BusinessType: "rag_retrieval",
		Category:     "content",
		InputFormat:  "text",
		OutputFormat: "json",
		RequiresLLM:  false,
	}
}

func (p *Plugin) ValidateConfig(config pipeline.NodeConfig) error { return nil }

func (p *Plugin) Execute(ctx context.Context, req *pipeline.NodeExecutionRequest) (*pipeline.NodeExecutionResponse, error) {
	if req == nil || req.Input == nil {
		return nil, fmt.Errorf("invalid input")
	}
	topK := 5
	threshold := 0.7
	allowMock := false
	if req.Config.CustomConfig != nil {
		if k, ok := req.Config.CustomConfig["top_k"].(float64); ok {
			topK = int(k)
		}
		if t, ok := req.Config.CustomConfig["threshold"].(float64); ok {
			threshold = t
		}
		if m, ok := req.Config.CustomConfig["allow_mock"].(bool); ok {
			allowMock = m
		}
	}
	if os.Getenv("CENTAG_RAG_ALLOW_MOCK") == "1" {
		allowMock = true
	}

	query := req.Input.Content
	var documents []map[string]interface{}
	if allowMock {
		documents = mockRetrieve(query, topK, threshold)
	}
	// Without mock / real KnowledgeStore: return empty docs (miss path continues to generator)

	result := map[string]interface{}{
		"documents": documents,
		"count":     len(documents),
		"query":     query,
		"mock":      allowMock && len(documents) > 0,
	}
	raw, _ := json.Marshal(result)
	return &pipeline.NodeExecutionResponse{
		Output: &pipeline.NodeOutput{
			Content: string(raw),
			Metadata: map[string]interface{}{
				"count":     len(documents),
				"threshold": threshold,
				"top_k":     topK,
				"mock":      allowMock && len(documents) > 0,
			},
		},
	}, nil
}

func mockRetrieve(query string, topK int, threshold float64) []map[string]interface{} {
	mockDocuments := []map[string]interface{}{
		{"content": "Go is a statically typed, compiled programming language designed at Google.", "score": 0.95, "source": "go-docs"},
		{"content": "Go's concurrency model is based on goroutines and channels.", "score": 0.88, "source": "go-concurrency"},
	}
	var results []map[string]interface{}
	q := strings.ToLower(query)
	for _, doc := range mockDocuments {
		score := doc["score"].(float64)
		if strings.Contains(strings.ToLower(doc["content"].(string)), q) {
			score += 0.1
		}
		if score >= threshold {
			cp := map[string]interface{}{"content": doc["content"], "score": score, "source": doc["source"]}
			results = append(results, cp)
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results
}
