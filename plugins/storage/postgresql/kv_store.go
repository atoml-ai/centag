package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"centag/core/pkg/storage"
)

// ErrKeyNotFound 键不存在错误
var ErrKeyNotFound = fmt.Errorf("key not found")

// KVStore KV 存储实现
type KVStore struct {
	pool  *pgxpool.Pool
	table string
}

// NewKVStore 创建 KV 存储
func NewKVStore(pool *pgxpool.Pool, table string) *KVStore {
	return &KVStore{
		pool:  pool,
		table: table,
	}
}

// Set 设置键值对
func (k *KVStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 将值序列化为 JSON 字符串
	var valueJSON string
	if valBytes, err := json.Marshal(value); err == nil {
		valueJSON = string(valBytes)
	} else {
		valueJSON = "null"
	}

	var expiresAt *time.Time
	if ttl > 0 {
		expiresAt = new(time.Time)
		*expiresAt = time.Now().Add(ttl)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (key, value, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key)
		DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at
	`, k.table)

	_, err := k.pool.Exec(ctx, query, key, valueJSON, expiresAt)
	return err
}

// Get 获取值
func (k *KVStore) Get(ctx context.Context, key string) (interface{}, error) {
	var value []byte
	var expiresAt *time.Time

	query := fmt.Sprintf(`
		SELECT value, expires_at
		FROM %s
		WHERE key = $1
	`, k.table)

	err := k.pool.QueryRow(ctx, query, key).Scan(&value, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}

	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return nil, ErrKeyNotFound
	}

	var result interface{}
	if err := json.Unmarshal(value, &result); err != nil {
		return string(value), nil
	}

	return result, nil
}

// GetBytes 获取字节值
func (k *KVStore) GetBytes(ctx context.Context, key string) ([]byte, error) {
	value, err := k.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if valBytes, ok := value.([]byte); ok {
		return valBytes, nil
	}

	if valStr, ok := value.(string); ok {
		return []byte(valStr), nil
	}

	return json.Marshal(value)
}

// GetString 获取字符串值
func (k *KVStore) GetString(ctx context.Context, key string) (string, error) {
	value, err := k.Get(ctx, key)
	if err != nil {
		return "", err
	}

	if valStr, ok := value.(string); ok {
		return valStr, nil
	}

	if valBytes, ok := value.([]byte); ok {
		return string(valBytes), nil
	}

	return fmt.Sprintf("%v", value), nil
}

// Delete 删除键
func (k *KVStore) Delete(ctx context.Context, key string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE key = $1
	`, k.table)

	result, err := k.pool.Exec(ctx, query, key)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// Exists 检查键是否存在
func (k *KVStore) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	var expiresAt *time.Time

	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s
			WHERE key = $1
		)
	`, k.table)

	err := k.pool.QueryRow(ctx, query, key).Scan(&exists)
	if err != nil || !exists {
		return false, err
	}

	query = fmt.Sprintf(`
		SELECT expires_at
		FROM %s
		WHERE key = $1
	`, k.table)

	err = k.pool.QueryRow(ctx, query, key).Scan(&expiresAt)
	if err != nil {
		return false, err
	}

	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}

// Expire 设置过期时间
func (k *KVStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	var value []byte
	query := fmt.Sprintf(`
		SELECT value
		FROM %s
		WHERE key = $1
	`, k.table)

	err := k.pool.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrKeyNotFound
		}
		return err
	}

	expiresAt := time.Now().Add(ttl)
	updateQuery := fmt.Sprintf(`
		UPDATE %s
		SET expires_at = $2
		WHERE key = $1
	`, k.table)

	result, err := k.pool.Exec(ctx, updateQuery, key, expiresAt)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// TTL 获取剩余过期时间
func (k *KVStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	var expiresAt *time.Time

	query := fmt.Sprintf(`
		SELECT expires_at
		FROM %s
		WHERE key = $1
	`, k.table)

	err := k.pool.QueryRow(ctx, query, key).Scan(&expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return -1, nil
		}
		return 0, err
	}

	if expiresAt == nil {
		return -1, nil
	}

	ttl := time.Until(*expiresAt)
	if ttl < 0 {
		return 0, nil
	}

	return ttl, nil
}

// SetBatch 批量设置
func (k *KVStore) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	batch := &pgx.Batch{}

	var expiresAt *time.Time
	if ttl > 0 {
		expiresAt = new(time.Time)
		*expiresAt = time.Now().Add(ttl)
	}

	for key, value := range items {
		var valueJSONB interface{}
		if valBytes, err := json.Marshal(value); err == nil {
			valueJSONB = valBytes
		} else {
			valueJSONB = nil
		}

		query := fmt.Sprintf(`
			INSERT INTO %s (key, value, expires_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (key)
			DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at
		`, k.table)

		batch.Queue(query, key, valueJSONB, expiresAt)
	}

	br := k.pool.SendBatch(ctx, batch)
	defer br.Close()

	return br.Close()
}

// GetBatch 批量获取
func (k *KVStore) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	now := time.Now()

	query := fmt.Sprintf(`
		SELECT key, value, expires_at
		FROM %s
		WHERE key = ANY($1)
	`, k.table)

	rows, err := k.pool.Query(ctx, query, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value []byte
		var expiresAt *time.Time

		if err := rows.Scan(&key, &value, &expiresAt); err != nil {
			continue
		}

		if expiresAt != nil && expiresAt.Before(now) {
			continue
		}

		var val interface{}
		if err := json.Unmarshal(value, &val); err != nil {
			result[key] = string(value)
		} else {
			result[key] = val
		}
	}

	return result, nil
}

// DeleteBatch 批量删除
func (k *KVStore) DeleteBatch(ctx context.Context, keys []string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE key = ANY($1)
	`, k.table)

	_, err := k.pool.Exec(ctx, query, keys)
	return err
}

// globToLike converts a glob-style pattern (using * as wildcard) to a SQL
// LIKE pattern (using % as wildcard).  This lets callers use the Redis-style
// "*" convention that the cache layer passes in.
func globToLike(pattern string) string {
	// Replace glob wildcard with SQL LIKE wildcard.
	// A literal '%' in the original pattern is escaped first.
	result := ""
	for _, ch := range pattern {
		switch ch {
		case '%':
			result += "\\%"
		case '_':
			result += "\\_"
		case '*':
			result += "%"
		case '?':
			result += "_"
		default:
			result += string(ch)
		}
	}
	return result
}

// Keys 获取匹配的键
func (k *KVStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	likePattern := globToLike(pattern)
	query := fmt.Sprintf(`
		SELECT key
		FROM %s
		WHERE key LIKE $1
	`, k.table)

	rows, err := k.pool.Query(ctx, query, likePattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// Count 获取键的总数
func (k *KVStore) Count(ctx context.Context, pattern string) (int64, error) {
	now := time.Now()

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s
		WHERE key LIKE $1
		AND (expires_at IS NULL OR expires_at > $2)
	`, k.table)

	var count int64
	err := k.pool.QueryRow(ctx, query, globToLike(pattern), now).Scan(&count)
	return count, err
}

// GetAll 获取所有键值对
func (k *KVStore) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	query := fmt.Sprintf(`
		SELECT key, value
		FROM %s
		WHERE key LIKE $1
	`, k.table)

	rows, err := k.pool.Query(ctx, query, globToLike(pattern))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]byte)
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		result[key] = value
	}

	return result, nil
}

// FlushDB 清空数据库
func (k *KVStore) FlushDB(ctx context.Context) error {
	query := fmt.Sprintf(`TRUNCATE TABLE %s`, k.table)
	_, err := k.pool.Exec(ctx, query)
	return err
}

// Close 关闭连接
func (k *KVStore) Close() error {
	return nil
}

// GetStoreInfo 获取存储后端信息
func (k *KVStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "postgresql",
	}
}
