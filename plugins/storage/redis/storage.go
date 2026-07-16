package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Storage Redis存储实现
type Storage struct {
	client    *redis.Client
	config    *Config
	mu        sync.RWMutex
	connected bool
}

// Config Redis配置
type Config struct {
	Addr         string `json:"addr"`
	Password     string `json:"password"`
	DB           int    `json:"db"`
	PoolSize     int    `json:"pool_size"`
	MinIdleConns int    `json:"min_idle_conns"`
	MaxRetries   int    `json:"max_retries"`
	DialTimeout  int    `json:"dial_timeout"`  // 秒
	ReadTimeout  int    `json:"read_timeout"`  // 秒
	WriteTimeout int    `json:"write_timeout"` // 秒
	PoolTimeout  int    `json:"pool_timeout"`  // 秒
}

// New 创建Redis存储
func New(config *Config) (*Storage, error) {
	if config.Addr == "" {
		config.Addr = "localhost:26379"
	}
	if config.PoolSize <= 0 {
		config.PoolSize = 10
	}
	if config.MinIdleConns <= 0 {
		config.MinIdleConns = 2
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.DialTimeout <= 0 {
		config.DialTimeout = 5
	}
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = 3
	}
	if config.WriteTimeout <= 0 {
		config.WriteTimeout = 3
	}
	if config.PoolTimeout <= 0 {
		config.PoolTimeout = 4
	}

	options := &redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  time.Duration(config.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		PoolTimeout:  time.Duration(config.PoolTimeout) * time.Second,
	}

	client := redis.NewClient(options)

	storage := &Storage{
		client:    client,
		config:    config,
		connected: false,
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	storage.connected = true
	logger.Info("Redis storage connected",
		zap.String("addr", config.Addr),
		zap.Int("db", config.DB),
		zap.Int("pool_size", config.PoolSize),
		zap.Int("min_idle_conns", config.MinIdleConns))

	return storage, nil
}

// KVStore 返回KV存储接口
func (s *Storage) KVStore() (storage.KVStore, error) {
	return s, nil
}

// VectorStore 返回向量存储接口
func (s *Storage) VectorStore() (storage.VectorStore, error) {
	return NewVectorStore(s.client), nil
}

// Type 返回存储类型
func (s *Storage) Type() storage.StorageType {
	return storage.StorageTypeRedis
}

// HealthCheck 健康检查
func (s *Storage) HealthCheck(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// DetailedHealthCheck 详细健康检查
func (s *Storage) DetailedHealthCheck(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		Healthy:   true,
		Connected: s.connected,
	}

	// 基础连接检查
	if err := s.client.Ping(ctx).Err(); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		return status, nil
	}

	// 检查连接池状态
	poolStats := s.client.PoolStats()
	status.PoolStats = PoolStats{
		Hits:       poolStats.Hits,
		Misses:     poolStats.Misses,
		Timeouts:   poolStats.Timeouts,
		TotalConns: poolStats.TotalConns,
		IdleConns:  poolStats.IdleConns,
		StaleConns: poolStats.StaleConns,
	}

	// 计算健康指标
	hitRate := float64(0)
	if poolStats.Hits+poolStats.Misses > 0 {
		hitRate = float64(poolStats.Hits) / float64(poolStats.Hits+poolStats.Misses) * 100
	}
	status.HitRate = hitRate

	// 检查连接池使用率
	utilization := float64(poolStats.TotalConns) / float64(s.config.PoolSize) * 100
	status.PoolUtilization = utilization

	// 如果连接池使用率过高,标记为不健康
	if utilization > 90 {
		status.Healthy = false
		status.Warning = "Pool utilization high"
	}

	// 检查超时次数
	if poolStats.Timeouts > 100 {
		status.Healthy = false
		status.Warning = "High connection timeout rate"
	}

	// 获取Redis信息
	info, err := s.client.Info(ctx).Result()
	if err != nil {
		status.Healthy = false
		status.Error = err.Error()
		return status, nil
	}
	status.ServerInfo = info

	return status, nil
}

// HealthStatus 健康状态
type HealthStatus struct {
	Healthy         bool      `json:"healthy"`
	Connected       bool      `json:"connected"`
	Error           string    `json:"error,omitempty"`
	Warning         string    `json:"warning,omitempty"`
	HitRate         float64   `json:"hit_rate_percent"`
	PoolUtilization float64   `json:"pool_utilization_percent"`
	PoolStats       PoolStats `json:"pool_stats"`
	ServerInfo      string    `json:"server_info,omitempty"`
}

// Set 设置键值对
func (s *Storage) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error

	// 序列化值
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// 设置值
	if ttl > 0 {
		err = s.client.Set(ctx, key, data, ttl).Err()
	} else {
		err = s.client.Set(ctx, key, data, 0).Err()
	}

	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

// Get 获取值
func (s *Storage) Get(ctx context.Context, key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		// 如果反序列化失败，返回原始字符串
		return string(data), nil
	}

	return value, nil
}

// GetBytes 获取字节值
func (s *Storage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	return data, nil
}

// GetString 获取字符串值
func (s *Storage) GetString(ctx context.Context, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return val, nil
}

// Delete 删除键
func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return nil
}

// Exists 检查键是否存在
func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return count > 0, nil
}

// Expire 设置过期时间
func (s *Storage) Expire(ctx context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set expire: %w", err)
	}

	return nil
}

// TTL 获取剩余过期时间
func (s *Storage) TTL(ctx context.Context, key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get ttl: %w", err)
	}

	return ttl, nil
}

// SetBatch 批量设置
func (s *Storage) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pipe := s.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			logger.Error("Failed to marshal value in batch",
				zap.String("key", key),
				zap.Error(err))
			continue
		}

		if ttl > 0 {
			pipe.Set(ctx, key, data, ttl)
		} else {
			pipe.Set(ctx, key, data, 0)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute batch set: %w", err)
	}

	return nil
}

// GetBatch 批量获取
func (s *Storage) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))

	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute batch get: %w", err)
	}

	result := make(map[string]interface{})
	for i, cmd := range cmds {
		val, err := cmd.Result()
		if err != nil && err != redis.Nil {
			logger.Error("Failed to get key in batch",
				zap.String("key", keys[i]),
				zap.Error(err))
			continue
		}

		if err == nil {
			var value interface{}
			if err := json.Unmarshal([]byte(val), &value); err == nil {
				result[keys[i]] = value
			} else {
				result[keys[i]] = val
			}
		}
	}

	return result, nil
}

// DeleteBatch 批量删除
func (s *Storage) DeleteBatch(ctx context.Context, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(keys) == 0 {
		return nil
	}

	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete batch keys: %w", err)
	}

	return nil
}

// Keys 获取匹配的键
func (s *Storage) Keys(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	var cursor uint64

	for {
		var batch []string
		var err error

		batch, cursor, err = s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys: %w", err)
		}

		keys = append(keys, batch...)

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// Count 获取键的数量
func (s *Storage) Count(ctx context.Context, pattern string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 对于Redis,使用DBSize获取当前数据库所有key的数量
	count, err := s.client.DBSize(ctx).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count keys: %w", err)
	}

	logger.Debug("Keys counted in Redis", logger.GetField("pattern", pattern), logger.GetField("count", count))
	return count, nil
}

// GetAll 获取所有键值对(包括已过期的)
func (s *Storage) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 获取所有匹配的键
	keys, err := s.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// 批量获取所有值
	values := make(map[string][]byte)
	for _, key := range keys {
		data, err := s.GetBytes(ctx, key)
		if err != nil {
			// 忽略vector类型等不匹配的数据类型(预期行为)
			logger.Debug("Failed to get key in GetAll (may be vector data)",
				logger.GetField("key", key),
				logger.GetField("error", err))
			continue
		}
		if data != nil {
			values[key] = data
		}
	}

	logger.Debug("All values retrieved from Redis",
		logger.GetField("pattern", pattern),
		logger.GetField("count", len(values)))

	return values, nil
}

// FlushDB 清空数据库
func (s *Storage) FlushDB(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.client.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("failed to flush db: %w", err)
	}

	return nil
}

// Close 关闭连接
func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		if err := s.client.Close(); err != nil {
			return err
		}
	}
	s.connected = false
	return nil
}

// GetDefaultConfig 获取默认配置
func (s *Storage) GetDefaultConfig() map[string]interface{} {
	return map[string]interface{}{
		"addr":            "localhost:26379",
		"password":         "",
		"db":              0,
		"pool_size":       10,
		"min_idle_conns":  2,
		"max_retries":     3,
		"dial_timeout":    5,
		"read_timeout":    3,
		"write_timeout":   3,
		"pool_timeout":    4,
	}
}


// GetStoreInfo 获取存储后端信息
func (s *Storage) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "redis",
	}
}

// Stats 获取统计信息
func (s *Storage) Stats(ctx context.Context) (*Stats, error) {
	info, err := s.client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get redis info: %w", err)
	}

	stats := &Stats{
		Connected: s.connected,
		Addr:      s.config.Addr,
		DB:        s.config.DB,
		PoolSize:  s.config.PoolSize,
		MinIdle:   s.config.MinIdleConns,
		Info:      info,
	}

	// 获取连接池状态
	poolStats := s.client.PoolStats()
	stats.PoolStats = PoolStats{
		Hits:       poolStats.Hits,
		Misses:     poolStats.Misses,
		Timeouts:   poolStats.Timeouts,
		TotalConns: poolStats.TotalConns,
		IdleConns:  poolStats.IdleConns,
		StaleConns: poolStats.StaleConns,
	}

	// 解析一些关键指标
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_clients:") {
			stats.ConnectedClients, _ = strconv.Atoi(strings.TrimPrefix(line, "connected_clients:"))
		} else if strings.HasPrefix(line, "used_memory_human:") {
			stats.UsedMemory = strings.TrimPrefix(line, "used_memory_human:")
		} else if strings.HasPrefix(line, "total_commands_processed:") {
			stats.TotalCommands, _ = strconv.ParseInt(strings.TrimPrefix(line, "total_commands_processed:"), 10, 64)
		} else if strings.HasPrefix(line, "keyspace:") {
			stats.Keyspace = strings.TrimPrefix(line, "keyspace:")
		} else if strings.HasPrefix(line, "uptime_in_seconds:") {
			stats.Uptime, _ = strconv.ParseInt(strings.TrimPrefix(line, "uptime_in_seconds:"), 10, 64)
		} else if strings.HasPrefix(line, "redis_version:") {
			stats.Version = strings.TrimPrefix(line, "redis_version:")
		}
	}

	return stats, nil
}

// Stats 统计信息
type Stats struct {
	Connected        bool      `json:"connected"`
	Addr             string    `json:"addr"`
	DB               int       `json:"db"`
	PoolSize         int       `json:"pool_size"`
	MinIdle          int       `json:"min_idle"`
	Version          string    `json:"version"`
	Uptime           int64     `json:"uptime_seconds"`
	ConnectedClients int       `json:"connected_clients"`
	UsedMemory       string    `json:"used_memory"`
	TotalCommands    int64     `json:"total_commands"`
	Keyspace         string    `json:"keyspace"`
	PoolStats        PoolStats `json:"pool_stats"`
	Info             string    `json:"info,omitempty"`
}

// PoolStats 连接池统计
type PoolStats struct {
	Hits       uint32 `json:"hits"`
	Misses     uint32 `json:"misses"`
	Timeouts   uint32 `json:"timeouts"`
	TotalConns uint32 `json:"total_conns"`
	IdleConns  uint32 `json:"idle_conns"`
	StaleConns uint32 `json:"stale_conns"`
}

// GetPoolStatus 获取连接池状态
func (s *Storage) GetPoolStatus(ctx context.Context) (*PoolStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return nil, fmt.Errorf("redis not connected")
	}

	poolStats := s.client.PoolStats()

	hitRate := float64(0)
	if poolStats.Hits+poolStats.Misses > 0 {
		hitRate = float64(poolStats.Hits) / float64(poolStats.Hits+poolStats.Misses) * 100
	}

	status := &PoolStatus{
		PoolSize:           s.config.PoolSize,
		MinIdleConns:       s.config.MinIdleConns,
		CurrentConnections: poolStats.TotalConns,
		IdleConnections:    poolStats.IdleConns,
		Hits:               poolStats.Hits,
		Misses:             poolStats.Misses,
		Timeouts:           poolStats.Timeouts,
		StaleConns:         poolStats.StaleConns,
		HitRate:            hitRate,
		Utilization:        float64(poolStats.TotalConns) / float64(s.config.PoolSize) * 100,
	}

	return status, nil
}

// PoolStatus 连接池状态
type PoolStatus struct {
	PoolSize           int     `json:"pool_size"`
	MinIdleConns       int     `json:"min_idle_conns"`
	CurrentConnections uint32  `json:"current_connections"`
	IdleConnections    uint32  `json:"idle_connections"`
	Hits               uint32  `json:"hits"`
	Misses             uint32  `json:"misses"`
	Timeouts           uint32  `json:"timeouts"`
	StaleConns         uint32  `json:"stale_conns"`
	HitRate            float64 `json:"hit_rate_percent"`
	Utilization        float64 `json:"utilization_percent"`
}
