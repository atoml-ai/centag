package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/storage"
)

// FileKnowledgeStore 文件系统实现的知识库存储
// 支持的格式：txt、md、json
type FileKnowledgeStore struct {
	mu              sync.RWMutex
	knowledgeDir    string
	collections     map[string]bool
	defaultCollection string
}

// NewFileKnowledgeStore 创建文件知识库存储
func NewFileKnowledgeStore(knowledgeDir, defaultCollection string) (*FileKnowledgeStore, error) {
	if err := os.MkdirAll(knowledgeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create knowledge dir %s: %w", knowledgeDir, err)
	}

	store := &FileKnowledgeStore{
		knowledgeDir:      knowledgeDir,
		defaultCollection: defaultCollection,
		collections:       make(map[string]bool),
	}

	store.collections[defaultCollection] = true
	collectionsDir := filepath.Join(knowledgeDir, defaultCollection)
	_ = os.MkdirAll(collectionsDir, 0755)

	return store, nil
}

// HealthCheck 健康检查 — 验证知识库目录可访问
func (s *FileKnowledgeStore) HealthCheck(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := os.Stat(s.knowledgeDir); err != nil {
		return fmt.Errorf("knowledge directory not accessible: %w", err)
	}
	return nil
}

// Close 关闭连接
func (s *FileKnowledgeStore) Close() error {
	return nil
}

// GetStoreInfo 获取存储信息
func (s *FileKnowledgeStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "file",
	}
}

// GetDataType 获取数据类型
func (s *FileKnowledgeStore) GetDataType() storage.DataType {
	return storage.DataTypeKnowledge
}

// Store 存储知识文档到文件系统
// 根据文档内容自动检测格式（txt/md/json），以 ID 为文件名保存到指定集合目录
func (s *FileKnowledgeStore) Store(ctx context.Context, doc *storage.DataDocument) error {
	if doc.ID == "" {
		return fmt.Errorf("document ID is required")
	}

	collection := doc.Collection
	if collection == "" {
		collection = s.defaultCollection
	}

	ext := detectFormat(doc)
	subDir := filepath.Join(s.knowledgeDir, collection)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("failed to create collection dir %s: %w", subDir, err)
	}
	s.collections[collection] = true

	filePath := filepath.Join(subDir, doc.ID+ext)

	var fileContent []byte
	switch ext {
	case ".json":
		doc.UpdatedAt = time.Now()
		if doc.CreatedAt.IsZero() {
			doc.CreatedAt = doc.UpdatedAt
		}
		var err error
		fileContent, err = json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal document: %w", err)
		}
	default:
		fileContent = []byte(doc.Content)
	}

	if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
		return fmt.Errorf("failed to write document file: %w", err)
	}

	return nil
}

// Retrieve 检索知识文档
// 在指定集合（或默认集合）中搜索包含查询关键词的文档
func (s *FileKnowledgeStore) Retrieve(ctx context.Context, query string, topK int, filter map[string]interface{}) ([]*storage.DataResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var collection string
	if c, ok := filter["collection"].(string); ok {
		collection = c
	} else {
		collection = s.defaultCollection
	}

	subDir := filepath.Join(s.knowledgeDir, collection)
	entries, err := os.ReadDir(subDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read collection dir %s: %w", subDir, err)
	}

	queryLower := strings.ToLower(query)
	var results []*storage.DataResult

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := filepath.Ext(name)
		if !isSupportedFormat(ext) {
			continue
		}

		filePath := filepath.Join(subDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var doc *storage.DataDocument
		var content string

		switch ext {
		case ".json":
			doc = &storage.DataDocument{}
			if err := json.Unmarshal(data, doc); err != nil {
				continue
			}
			content = doc.Content
		default:
			docID := strings.TrimSuffix(name, ext)
			content = string(data)
			doc = &storage.DataDocument{
				ID:         docID,
				Content:    content,
				Collection: collection,
				DataType:   storage.DataTypeKnowledge,
			}
		}

		if !docMatchesFilter(doc, filter) {
			continue
		}

		score := computeRelevanceScore(content, queryLower)
		if score <= 0 {
			continue
		}

		results = append(results, &storage.DataResult{
			Document: doc,
			Score:    score,
		})
	}

	sortResultsByScore(results)

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}

	return results, nil
}

// DeleteKnowledge 删除知识文档
func (s *FileKnowledgeStore) DeleteKnowledge(ctx context.Context, docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for collection := range s.collections {
		subDir := filepath.Join(s.knowledgeDir, collection)
		for _, ext := range []string{".txt", ".md", ".json"} {
			filePath := filepath.Join(subDir, docID+ext)
			if _, err := os.Stat(filePath); err == nil {
				return os.Remove(filePath)
			}
		}
	}

	return fmt.Errorf("document %s not found in any collection", docID)
}

// ListCollections 列出所有知识库集合
func (s *FileKnowledgeStore) ListCollections(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.knowledgeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read knowledge dir: %w", err)
	}

	var collections []string
	for _, entry := range entries {
		if entry.IsDir() {
			collections = append(collections, entry.Name())
		}
	}

	for name := range s.collections {
		found := false
		for _, c := range collections {
			if c == name {
				found = true
				break
			}
		}
		if !found {
			collections = append(collections, name)
		}
	}

	return collections, nil
}

// DefaultCollection 返回默认集合名称
func (s *FileKnowledgeStore) DefaultCollection() string {
	return s.defaultCollection
}

func detectFormat(doc *storage.DataDocument) string {
	format := ""
	if doc.Metadata != nil {
		if f, ok := doc.Metadata["format"].(string); ok {
			format = strings.ToLower(strings.TrimSpace(f))
		}
	}

	switch format {
	case "json":
		return ".json"
	case "md", "markdown":
		return ".md"
	default:
		if isJSONContent(doc.Content) {
			return ".json"
		}
		return ".txt"
	}
}

func isJSONContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func isSupportedFormat(ext string) bool {
	switch ext {
	case ".txt", ".md", ".json":
		return true
	}
	return false
}

func docMatchesFilter(doc *storage.DataDocument, filter map[string]interface{}) bool {
	if len(filter) == 0 {
		return true
	}
	if doc.Metadata == nil {
		return false
	}
	for k, v := range filter {
		if k == "collection" {
			continue
		}
		mv, ok := doc.Metadata[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", mv) != fmt.Sprintf("%v", v) {
			return false
		}
	}
	return true
}

func computeRelevanceScore(content, queryLower string) float32 {
	contentLower := strings.ToLower(content)
	if contentLower == queryLower {
		return 1.0
	}
	count := strings.Count(contentLower, queryLower)
	if count == 0 {
		return 0
	}
	score := float32(count) / float32(len(contentLower)) * 100
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func sortResultsByScore(results []*storage.DataResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
