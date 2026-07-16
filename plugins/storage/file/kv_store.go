package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/storage"
)

// kvEntry 持久化的键值条目
type kvEntry struct {
	Value     []byte    `json:"value"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// kvData 持久化文件结构
type kvData struct {
	Entries map[string]*kvEntry `json:"entries"`
}

// KVStore 文件实现的键值存储
type KVStore struct {
	mu       sync.RWMutex
	filePath string
	data     *kvData
}

// NewKVStore 创建新的文件 KV 存储
func NewKVStore(dataDir, kvFile string) (*KVStore, error) {
	filePath := strings.TrimRight(dataDir, "/") + "/" + kvFile

	store := &KVStore{
		filePath: filePath,
		data: &kvData{
			Entries: make(map[string]*kvEntry),
		},
	}

	// 尝试从文件加载已有数据
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load kv store from %s: %w", filePath, err)
	}

	return store, nil
}

// load 从文件加载数据
func (s *KVStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var kv kvData
	if err := json.Unmarshal(data, &kv); err != nil {
		return fmt.Errorf("failed to unmarshal kv data: %w", err)
	}

	if kv.Entries != nil {
		s.data = &kv
	} else {
		s.data.Entries = make(map[string]*kvEntry)
	}

	return nil
}

// save 持久化数据到文件
func (s *KVStore) save() error {
	// 注意：调用方必须持有写锁
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal kv data: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write kv file: %w", err)
	}

	return nil
}

// isExpired 检查条目是否过期（调用方须持有读锁）
func (s *KVStore) isExpired(key string) bool {
	entry, ok := s.data.Entries[key]
	if !ok {
		return false
	}
	if entry.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(entry.ExpiresAt)
}

// Set 设置键值对
func (s *KVStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var valBytes []byte
	switch v := value.(type) {
	case []byte:
		valBytes = v
	case string:
		valBytes = []byte(v)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
		valBytes = jsonBytes
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry := &kvEntry{
		Value:     valBytes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if ttl > 0 {
		entry.ExpiresAt = now.Add(ttl)
	}

	// 保留 CreatedAt，如果键已存在
	if existing, ok := s.data.Entries[key]; ok {
		entry.CreatedAt = existing.CreatedAt
	}

	s.data.Entries[key] = entry
	return s.save()
}

// Get 获取值
func (s *KVStore) Get(ctx context.Context, key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data.Entries[key]
	if !ok || s.isExpired(key) {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	var v interface{}
	if err := json.Unmarshal(entry.Value, &v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return v, nil
}

// GetBytes 获取字节值
func (s *KVStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data.Entries[key]
	if !ok || s.isExpired(key) {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	result := make([]byte, len(entry.Value))
	copy(result, entry.Value)
	return result, nil
}

// GetString 获取字符串值
func (s *KVStore) GetString(ctx context.Context, key string) (string, error) {
	bytes, err := s.GetBytes(ctx, key)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Delete 删除键
func (s *KVStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data.Entries[key]; !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(s.data.Entries, key)
	return s.save()
}

// Exists 检查键是否存在（且未过期）
func (s *KVStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data.Entries[key]
	if !ok {
		return false, nil
	}
	if s.isExpired(key) {
		return false, nil
	}
	_ = entry
	return true, nil
}

// Expire 设置过期时间
func (s *KVStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data.Entries[key]
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}

	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(ttl)
	} else {
		entry.ExpiresAt = time.Time{}
	}

	entry.UpdatedAt = time.Now()
	return s.save()
}

// TTL 获取剩余过期时间
func (s *KVStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data.Entries[key]
	if !ok {
		return -2, nil // -2 表示键不存在
	}

	if entry.ExpiresAt.IsZero() {
		return -1, nil // -1 表示永不过期
	}

	remaining := time.Until(entry.ExpiresAt)
	if remaining <= 0 {
		return -2, nil // 已过期
	}

	return remaining, nil
}

// SetBatch 批量设置
func (s *KVStore) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, value := range items {
		var valBytes []byte
		switch v := value.(type) {
		case []byte:
			valBytes = v
		case string:
			valBytes = []byte(v)
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
			}
			valBytes = jsonBytes
		}

		entry := &kvEntry{
			Value:     valBytes,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if ttl > 0 {
			entry.ExpiresAt = now.Add(ttl)
		}

		if existing, ok := s.data.Entries[key]; ok {
			entry.CreatedAt = existing.CreatedAt
		}

		s.data.Entries[key] = entry
	}

	return s.save()
}

// GetBatch 批量获取
func (s *KVStore) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		entry, ok := s.data.Entries[key]
		if !ok || s.isExpired(key) {
			continue
		}

		var v interface{}
		if err := json.Unmarshal(entry.Value, &v); err != nil {
			continue
		}
		result[key] = v
	}

	return result, nil
}

// DeleteBatch 批量删除
func (s *KVStore) DeleteBatch(ctx context.Context, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range keys {
		delete(s.data.Entries, key)
	}

	return s.save()
}

// Keys 获取匹配的键
func (s *KVStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []string
	for key := range s.data.Entries {
		if s.isExpired(key) {
			continue
		}
		if matched, err := matchPattern(key, pattern); err != nil {
			return nil, err
		} else if matched {
			result = append(result, key)
		}
	}

	return result, nil
}

// Count 获取键的总数
func (s *KVStore) Count(ctx context.Context, pattern string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for key := range s.data.Entries {
		if s.isExpired(key) {
			continue
		}
		if matched, err := matchPattern(key, pattern); err != nil {
			return 0, err
		} else if matched {
			count++
		}
	}

	return count, nil
}

// GetAll 获取所有键值对
func (s *KVStore) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]byte)
	for key, entry := range s.data.Entries {
		if matched, err := matchPattern(key, pattern); err != nil {
			return nil, err
		} else if !matched {
			continue
		}

		valCopy := make([]byte, len(entry.Value))
		copy(valCopy, entry.Value)
		result[key] = valCopy
	}

	return result, nil
}

// FlushDB 清空数据库
func (s *KVStore) FlushDB(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Entries = make(map[string]*kvEntry)
	return s.save()
}

// Close 关闭连接（文件存储无需额外清理）
func (s *KVStore) Close() error {
	return nil
}

// GetStoreInfo 获取存储后端信息
func (s *KVStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "file",
	}
}

// matchPattern 简单的通配符匹配 (*)
func matchPattern(key, pattern string) (bool, error) {
	if pattern == "*" {
		return true, nil
	}

	// 支持前缀/后缀通配符: abc*, *abc, *abc*, a*b
	if !strings.ContainsRune(pattern, '*') {
		return key == pattern, nil
	}

	parts := strings.Split(pattern, "*")
	remaining := key

	for i, part := range parts {
		if part == "" {
			if i == 0 {
				continue // 开头的 *
			}
			if i == len(parts)-1 {
				return true, nil // 结尾的 *，已匹配
			}
			continue
		}

		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false, nil
		}

		if i == 0 && idx != 0 {
			return false, nil // 第一个非*部分必须从开头匹配
		}

		remaining = remaining[idx+len(part):]
	}

	// 如果最后一个部分不是空（即不以*结尾），remaining应该为空
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return remaining == "", nil
	}

	return true, nil
}
