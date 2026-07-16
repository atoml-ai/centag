package elasticsearch

import (
	"context"
	"fmt"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"
)

const (
	// StorageTypeElasticsearch Elasticsearch 存储类型
	StorageTypeElasticsearch storage.StorageType = "elasticsearch"
)

func init() {
	// 注册插件工厂
	storage.RegisterPlugin(StorageTypeElasticsearch, NewPlugin)
}

// Plugin Elasticsearch 存储插件
type Plugin struct {
	client        *Client
	kvStore       *KVStore
	vectorStore   *VectorStore
	config        *Config
	exactIndex    string
	semanticIndex string
}

// Config Elasticsearch 配置
type Config struct {
	Addresses       []string `json:"addresses"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	APIKey          string   `json:"api_key"`
	CloudID         string   `json:"cloud_id"`
	ExactIndex      string   `json:"exact_index"`      // 精确匹配索引名称
	SemanticIndex   string   `json:"semantic_index"`   // 语义匹配索引名称
	VectorDimension int      `json:"vector_dimension"`  // 向量维度
	EnableTLS       bool     `json:"enable_tls"`
	EnableCompress  bool     `json:"enable_compress"`
	RequestTimeout  int      `json:"request_timeout"`  // 请求超时（秒）
}

// NewPlugin 创建 Elasticsearch 插件工厂函数
func NewPlugin(config map[string]interface{}) (storage.StoragePlugin, error) {
	cfg, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 创建 ES 客户端
	client, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// 设置默认值
	if cfg.ExactIndex == "" {
		cfg.ExactIndex = "cache_exact_index"
	}
	if cfg.SemanticIndex == "" {
		cfg.SemanticIndex = "cache_semantic_index"
	}
	if cfg.VectorDimension == 0 {
		cfg.VectorDimension = 1024 // 默认维度，与bge-m3模型一致
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 // 默认 30 秒
	}

	plugin := &Plugin{
		client:        client,
		config:        cfg,
		exactIndex:    cfg.ExactIndex,
		semanticIndex: cfg.SemanticIndex,
	}

	// 初始化索引
	if err := plugin.initIndices(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize indices: %w", err)
	}

	fmt.Printf("Elasticsearch plugin initialized: exact=%s, semantic=%s, dimension=%d\n",
		cfg.ExactIndex, cfg.SemanticIndex, cfg.VectorDimension)

	return plugin, nil
}

// parseConfig 解析配置
func parseConfig(config map[string]interface{}) (*Config, error) {
	cfg := &Config{}

	if addresses, ok := config["addresses"].([]interface{}); ok {
		for _, addr := range addresses {
			cfg.Addresses = append(cfg.Addresses, addr.(string))
		}
	}

	if username, ok := config["username"].(string); ok {
		cfg.Username = username
	}

	if password, ok := config["password"].(string); ok {
		cfg.Password = password
	}

	if apiKey, ok := config["api_key"].(string); ok {
		cfg.APIKey = apiKey
	}

	if cloudID, ok := config["cloud_id"].(string); ok {
		cfg.CloudID = cloudID
	}

	if exactIndex, ok := config["exact_index"].(string); ok {
		cfg.ExactIndex = exactIndex
	}

	if semanticIndex, ok := config["semantic_index"].(string); ok {
		cfg.SemanticIndex = semanticIndex
	}

	if dimension, ok := config["vector_dimension"].(float64); ok {
		cfg.VectorDimension = int(dimension)
	}

	if enableTLS, ok := config["enable_tls"].(bool); ok {
		cfg.EnableTLS = enableTLS
	}

	if enableCompress, ok := config["enable_compress"].(bool); ok {
		cfg.EnableCompress = enableCompress
	}

	if timeout, ok := config["request_timeout"].(float64); ok {
		cfg.RequestTimeout = int(timeout)
	}

	// 验证配置
	if len(cfg.Addresses) == 0 && cfg.CloudID == "" {
		return nil, fmt.Errorf("either 'addresses' or 'cloud_id' must be configured")
	}

	if len(cfg.Addresses) > 0 && cfg.Username == "" && cfg.Password == "" && cfg.APIKey == "" {
		// 本地开发可以没有认证
		logger.Warn("No authentication configured for Elasticsearch, connecting without credentials")
	}

	return cfg, nil
}

// KVStore 返回 KV 存储实现
func (p *Plugin) KVStore() (storage.KVStore, error) {
	if p.kvStore == nil {
		p.kvStore = NewKVStore(p.client, p.exactIndex)
	}
	return p.kvStore, nil
}

// VectorStore 返回向量存储实现
func (p *Plugin) VectorStore() (storage.VectorStore, error) {
	if p.vectorStore == nil {
		p.vectorStore = NewVectorStore(p.client, p.semanticIndex, p.config.VectorDimension)
	}
	return p.vectorStore, nil
}

// KnowledgeStore 获取知识库存储（Elasticsearch 不支持知识库存储）
func (p *Plugin) KnowledgeStore() (storage.KnowledgeDataStore, error) {
	return nil, fmt.Errorf("Elasticsearch does not support knowledge storage")
}

// StorageType 返回存储类型
func (p *Plugin) StorageType() storage.StorageType {
	return StorageTypeElasticsearch
}

// HealthCheck 健康检查
func (p *Plugin) HealthCheck(ctx context.Context) error {
	// 设置超时
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	// 检查集群健康状态
	info, err := p.client.GetClusterHealth(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster health: %w", err)
	}

	// 检查状态（yellow 或 green 都可以）
	if info.Status == "red" {
		return fmt.Errorf("cluster status is red")
	}

	// 检查索引是否存在
	exactExists, err := p.client.IndexExists(ctx, p.exactIndex)
	if err != nil {
		return fmt.Errorf("failed to check exact index: %w", err)
	}

	semanticExists, err := p.client.IndexExists(ctx, p.semanticIndex)
	if err != nil {
		return fmt.Errorf("failed to check semantic index: %w", err)
	}

	if !exactExists || !semanticExists {
		return fmt.Errorf("one or more indices do not exist")
	}

	return nil
}

// GetDefaultConfig 获取默认配置
func (p *Plugin) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"addresses":         []string{"http://localhost:29200"},
		"username":          "elastic",
		"password":          "",
		"api_key":           "",
		"exact_index":       "cache_exact_index",
		"semantic_index":     "cache_semantic_index",
		"vector_dimension":  1024,
		"request_timeout":   30,
		"enable_tls":        false,
	}
}

// initIndices 初始化索引
func (p *Plugin) initIndices(ctx context.Context) error {
	// 创建精确匹配索引
	if err := p.createExactIndex(ctx); err != nil {
		return fmt.Errorf("failed to create exact index: %w", err)
	}

	// 创建语义匹配索引
	if err := p.createSemanticIndex(ctx); err != nil {
		return fmt.Errorf("failed to create semantic index: %w", err)
	}

	return nil
}

// createExactIndex 创建精确匹配索引
func (p *Plugin) createExactIndex(ctx context.Context) error {
	// 检查索引是否已存在
	exists, err := p.client.IndexExists(ctx, p.exactIndex)
	if err != nil {
		return err
	}

	if exists {
		logger.Info("Exact index already exists", logger.GetField("index", p.exactIndex))
		return nil
	}

	// 创建索引
	err = p.client.CreateIndex(ctx, p.exactIndex, map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type": "keyword",
				},
				"model": map[string]interface{}{
					"type": "keyword",
				},
				"request": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type":         "keyword",
							"ignore_above": 256,
						},
					},
				},
				"response": map[string]interface{}{
					"type":  "text",
					"index": false,
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"dynamic": true,
				},
				"tokens_used": map[string]interface{}{
					"type": "integer",
				},
				"timestamp": map[string]interface{}{
					"type": "date",
				},
				"ttl": map[string]interface{}{
					"type": "date",
				},
				"access_count": map[string]interface{}{
					"type": "integer",
				},
				"last_accessed": map[string]interface{}{
					"type": "date",
				},
			},
		},
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
			"refresh_interval":   "1s",
		},
	})

	if err != nil {
		return err
	}

	logger.Info("Exact index created successfully", logger.GetField("index", p.exactIndex))
	return nil
}

// createSemanticIndex 创建语义匹配索引
func (p *Plugin) createSemanticIndex(ctx context.Context) error {
	// 检查索引是否已存在
	exists, err := p.client.IndexExists(ctx, p.semanticIndex)
	if err != nil {
		return err
	}

	if exists {
		logger.Info("Semantic index already exists", logger.GetField("index", p.semanticIndex))
		return nil
	}

	// 创建索引
	err = p.client.CreateIndex(ctx, p.semanticIndex, map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type": "keyword",
				},
				"model": map[string]interface{}{
					"type": "keyword",
				},
				"request": map[string]interface{}{
					"type": "text",
					"fields": map[string]interface{}{
						"keyword": map[string]interface{}{
							"type":         "keyword",
							"ignore_above": 256,
						},
					},
				},
				"response": map[string]interface{}{
					"type":  "text",
					"index": false,
				},
				"embedding": map[string]interface{}{
					"type":       "dense_vector",
					"dims":       p.config.VectorDimension,
					"index":      true,
					"similarity": "cosine",
					"index_options": map[string]interface{}{
						"type":             "hnsw",
						"m":                16,
						"ef_construction":  200,
					},
				},
				"metadata": map[string]interface{}{
					"type":    "object",
					"dynamic": true,
				},
				"tokens_used": map[string]interface{}{
					"type": "integer",
				},
				"timestamp": map[string]interface{}{
					"type": "date",
				},
				"ttl": map[string]interface{}{
					"type": "date",
				},
				"access_count": map[string]interface{}{
					"type": "integer",
				},
				"last_accessed": map[string]interface{}{
					"type": "date",
				},
				"embedding_dimension": map[string]interface{}{
					"type": "integer",
				},
			},
		},
		"settings": map[string]interface{}{
			"number_of_shards":   3,
			"number_of_replicas": 1,
			"refresh_interval":   "1s",
		},
	})

	if err != nil {
		return err
	}

	logger.Info("Semantic index created successfully",
		logger.GetField("index", p.semanticIndex),
		logger.GetField("dimension", p.config.VectorDimension))
	return nil
}
