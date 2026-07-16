package file

import (
	"context"
	"fmt"
	"os"
	"sync"

	"centag/core/pkg/storage"
)

// Config 文件存储配置
type Config struct {
	// DataDir 数据存储根目录
	DataDir string `json:"data_dir"`
	// KVFile KV存储文件名（位于 DataDir 下）
	KVFile string `json:"kv_file"`
	// VectorDir 向量存储子目录名（位于 DataDir 下）
	VectorDir string `json:"vector_dir"`
	// DefaultCollection 默认集合名称
	DefaultCollection string `json:"default_collection"`
}

// Plugin 文件存储插件
type Plugin struct {
	config         *Config
	kvStore        *KVStore
	vectorStore    *VectorStore
	knowledgeStore *FileKnowledgeStore
	mu             sync.RWMutex
}

// NewPlugin 创建文件存储插件（工厂函数）
func NewPlugin(config map[string]interface{}) (storage.StoragePlugin, error) {
	cfg := parseConfig(config)

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", cfg.DataDir, err)
	}

	kvStore, err := NewKVStore(cfg.DataDir, cfg.KVFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create KVStore: %w", err)
	}

	vectorDir := cfg.DataDir
	if cfg.VectorDir != "" {
		vectorDir = cfg.DataDir + "/" + cfg.VectorDir
	}
	if err := os.MkdirAll(vectorDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vector directory %s: %w", vectorDir, err)
	}

	vectorStore, err := NewVectorStore(vectorDir, cfg.DefaultCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to create VectorStore: %w", err)
	}

	knowledgeDir := cfg.DataDir + "/knowledge"
	knowledgeStore, err := NewFileKnowledgeStore(knowledgeDir, cfg.DefaultCollection)
	if err != nil {
		return nil, fmt.Errorf("failed to create KnowledgeStore: %w", err)
	}

	return &Plugin{
		config:         cfg,
		kvStore:        kvStore,
		vectorStore:    vectorStore,
		knowledgeStore: knowledgeStore,
	}, nil
}

// parseConfig 从 map 解析配置，提供默认值
func parseConfig(config map[string]interface{}) *Config {
	cfg := &Config{
		DataDir:           "./data/file_storage",
		KVFile:            "kv_store.json",
		VectorDir:         "vectors",
		DefaultCollection: "default",
	}

	if config == nil {
		return cfg
	}

	if v, ok := config["data_dir"].(string); ok && v != "" {
		cfg.DataDir = v
	}
	if v, ok := config["kv_file"].(string); ok && v != "" {
		cfg.KVFile = v
	}
	if v, ok := config["vector_dir"].(string); ok && v != "" {
		cfg.VectorDir = v
	}
	if v, ok := config["default_collection"].(string); ok && v != "" {
		cfg.DefaultCollection = v
	}

	return cfg
}

// StorageType 返回存储类型
func (p *Plugin) StorageType() storage.StorageType {
	return storage.StorageTypeFile
}

// KVStore 返回 KV 存储接口
func (p *Plugin) KVStore() (storage.KVStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.kvStore == nil {
		return nil, fmt.Errorf("kv store not initialized")
	}
	return p.kvStore, nil
}

// VectorStore 返回向量存储接口
func (p *Plugin) VectorStore() (storage.VectorStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.vectorStore == nil {
		return nil, fmt.Errorf("vector store not initialized")
	}
	return p.vectorStore, nil
}

// KnowledgeStore 返回知识库存储接口
func (p *Plugin) KnowledgeStore() (storage.KnowledgeDataStore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.knowledgeStore == nil {
		return nil, fmt.Errorf("knowledge store not initialized")
	}
	return p.knowledgeStore, nil
}

// HealthCheck 健康检查 — 验证数据目录可访问
func (p *Plugin) HealthCheck(ctx context.Context) error {
	kvFile := p.config.DataDir + "/" + p.config.KVFile
	if _, err := os.Stat(kvFile); os.IsNotExist(err) {
		// 文件尚未创建，目录存在即可
		if _, err := os.Stat(p.config.DataDir); err != nil {
			return fmt.Errorf("data directory not accessible: %w", err)
		}
		return nil
	}
	return nil
}

// GetDefaultConfig 获取默认配置
func (p *Plugin) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"data_dir":           "./data/file_storage",
		"kv_file":            "kv_store.json",
		"vector_dir":         "vectors",
		"default_collection": "default",
	}
}

// init 注册文件存储插件工厂
func init() {
	storage.RegisterPlugin(storage.StorageTypeFile, NewPlugin)
}
