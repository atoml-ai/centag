package chroma

import (
	"context"
	"fmt"
	"time"

	"centag/core/pkg/storage"

	"go.uber.org/zap"
)

// Plugin ChromaDB存储插件实现
type Plugin struct {
	storageInstance *Storage
	logger          *zap.Logger
}

// Name 插件名称
func (p *Plugin) Name() string {
	return "chromadb"
}

// Type 插件类型
func (p *Plugin) Type() string {
	return "storage"
}

// StorageType 返回存储类型
func (p *Plugin) StorageType() storage.StorageType {
	return storage.StorageTypeChroma
}

// Init 初始化插件
func (p *Plugin) Init(config map[string]interface{}, logger *zap.Logger) error {
	// 加载配置
	cfg, err := LoadConfig(config)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config failed: %w", err)
	}

	// 创建存储实例
	storageInstance, err := NewStorage(cfg, logger)
	if err != nil {
		return fmt.Errorf("create storage failed: %w", err)
	}

	p.storageInstance = storageInstance
	p.logger = logger

	return nil
}

// Start 启动插件
func (p *Plugin) Start(ctx context.Context) error {
	if err := p.storageInstance.Connect(ctx); err != nil {
		return fmt.Errorf("connect to ChromaDB failed: %w", err)
	}

	p.logger.Info("ChromaDB plugin started")
	return nil
}

// Stop 停止插件
func (p *Plugin) Stop(ctx context.Context) error {
	if err := p.storageInstance.Close(); err != nil {
		return fmt.Errorf("close ChromaDB failed: %w", err)
	}

	p.logger.Info("ChromaDB plugin stopped")
	return nil
}

// HealthCheck 健康检查
func (p *Plugin) HealthCheck(ctx context.Context) error {
	if p.storageInstance == nil {
		return fmt.Errorf("chroma storage not initialized")
	}
	return p.storageInstance.HealthCheck(ctx)
}

// Storage 获取存储实例
func (p *Plugin) Storage() *Storage {
	return p.storageInstance
}

// KVStore 返回KV存储接口（ChromaDB不支持KV存储）
func (p *Plugin) KVStore() (storage.KVStore, error) {
	return nil, fmt.Errorf("ChromaDB does not support KV storage")
}

// VectorStore 返回向量存储接口
func (p *Plugin) VectorStore() (storage.VectorStore, error) {
	return p.storageInstance, nil
}

// KnowledgeStore 获取知识库存储（ChromaDB 不支持知识库存储）
func (p *Plugin) KnowledgeStore() (storage.KnowledgeDataStore, error) {
	return nil, fmt.Errorf("ChromaDB does not support knowledge storage")
}

// NewPlugin 创建插件实例
func NewPlugin(cfg *Config) (storage.StoragePlugin, error) {
	return &Plugin{
		storageInstance: nil, // 会在 init 中设置
		logger:          zap.NewNop(),
	}, nil
}

// GetDefaultConfig 获取默认配置
func (p *Plugin) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"addr":       "localhost:28000",
		"collection": "llm_cache",
		"timeout":    30,
		"token":      "",
	}
}

// ConfigSchema 返回配置JSON Schema
func (p *Plugin) ConfigSchema() string {
	return `{
		"type": "object",
		"properties": {
			"addr": {
				"type": "string",
				"title": "服务器地址",
				"description": "ChromaDB服务器地址，例如 localhost:28000",
				"format": "hostname"
			},
			"collection": {
				"type": "string",
				"title": "集合名称",
				"description": "存储向量的集合名称",
				"default": "llm_cache"
			},
			"timeout": {
				"type": "number",
				"title": "超时时间（秒）",
				"description": "HTTP请求超时时间",
				"default": 30,
				"minimum": 1
			}
		},
		"required": ["addr"]
	}`
}

// init 在包初始化时注册插件工厂
func init() {
	storage.RegisterPlugin(storage.StorageTypeChroma, func(config map[string]interface{}) (storage.StoragePlugin, error) {
		chromaConfig := &Config{
			Addr:       getStr(config, "addr", "chromadb:8000"),
			Collection: getStr(config, "collection", "llm_cache"),
			Timeout:    time.Duration(getInt(config, "timeout", 30)) * time.Second,
			Token:      getStr(config, "token", ""),
		}

		// 创建插件实例
		plugin, err := NewPlugin(chromaConfig)
		if err != nil {
			return nil, err
		}

		// 初始化存储实例
		storageInstance, err := NewStorage(chromaConfig, zap.NewNop())
		if err != nil {
			return nil, fmt.Errorf("create storage failed: %w", err)
		}

		plugin.(*Plugin).storageInstance = storageInstance
		return plugin, nil
	})
}

func getStr(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getInt(m map[string]interface{}, key string, defaultValue int) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}
