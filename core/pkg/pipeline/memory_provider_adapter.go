package pipeline

import (
	"context"
	"fmt"

	"centag/core/pkg/agentmemory"
)

// MemoryServiceAdapter 将 agentmemory.Service 适配为 MemoryProvider
type MemoryServiceAdapter struct {
	service *agentmemory.Service
}

// NewMemoryServiceAdapter 创建记忆服务适配器
func NewMemoryServiceAdapter(service *agentmemory.Service) *MemoryServiceAdapter {
	return &MemoryServiceAdapter{service: service}
}

// GetMemory 根据命名空间获取记忆访问接口（适配为 pipeline.Memory 接口）
func (a *MemoryServiceAdapter) GetMemory(namespace string) (Memory, error) {
	if a.service == nil {
		return nil, fmt.Errorf("memory service not configured")
	}

	// 使用固定的用户ID（系统级），实际使用时应该从上下文获取
	// 这里为了简化，使用 "0" 作为系统用户ID
	return &memoryAdapter{
		service:   a.service,
		userID:    "0",
		namespace: namespace,
	}, nil
}

// memoryAdapter 将 agentmemory.Service 适配为 pipeline.Memory
type memoryAdapter struct {
	service   *agentmemory.Service
	userID    string
	namespace string
}

func (m *memoryAdapter) Read(ctx context.Context, key string) ([]byte, error) {
	// agentmemory 使用 path 而不是 key，这里做简单映射
	content, err := m.service.GetDoc(ctx, m.userID, m.namespace, key)
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func (m *memoryAdapter) Write(ctx context.Context, key string, value []byte) error {
	_, err := m.service.PutDoc(ctx, m.userID, m.namespace, key, string(value))
	return err
}

func (m *memoryAdapter) Search(ctx context.Context, query string, limit int) ([]MemoryResult, error) {
	// agentmemory.Service.Search 返回三个值：[]SearchResult, string（下一个游标）, error
	results, _, err := m.service.Search(ctx, m.userID, m.namespace, query, limit)
	if err != nil {
		return nil, err
	}

	// 转换结果格式
	memResults := make([]MemoryResult, 0, len(results))
	for _, r := range results {
		memResults = append(memResults, MemoryResult{
			Key:   r.Path,
			Score: r.Score,
			Data:  []byte(r.Content),
		})
	}
	return memResults, nil
}
