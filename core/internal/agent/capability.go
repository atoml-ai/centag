package agent

import (
	"context"

	"centag/core/pkg/storage"
)

// AgentCapability 定义 Agent 的存储能力接口
// 提供 KV、向量、知识库三种数据类型的统一操作入口
type AgentCapability interface {
	StoreKeyValue(ctx context.Context, key string, value []byte) error
	RetrieveKeyValue(ctx context.Context, key string) ([]byte, error)
	DeleteKeyValue(ctx context.Context, key string) error

	StoreDocument(ctx context.Context, doc *storage.DataDocument) error
	SearchDocuments(ctx context.Context, query []float32, topK int, threshold float32) ([]*storage.DataResult, error)
	DeleteDocument(ctx context.Context, docID string) error
	ListVectorCollections(ctx context.Context) ([]string, error)

	StoreKnowledge(ctx context.Context, doc *storage.DataDocument) error
	RetrieveKnowledge(ctx context.Context, query string, topK int, filter map[string]interface{}) ([]*storage.DataResult, error)
	DeleteKnowledge(ctx context.Context, docID string) error
	ListKnowledgeCollections(ctx context.Context) ([]string, error)
}
