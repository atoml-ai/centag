package postgresql

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"centag/core/pkg/storage"
)

// vectorToString 将 []float32 转换为 pgvector 字符串格式 "[v1,v2,...]"
// pgx v5 不内置 pgvector 类型编码器，必须用字符串并显式 ::vector 转换
func vectorToString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'f', 8, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// parseVectorString 将 pgvector 字符串格式 "[v1,v2,...]" 解析为 []float32
func parseVectorString(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, fmt.Errorf("invalid vector format: %q", s)
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return []float32{}, nil
	}
	parts := strings.Split(s, ",")
	result := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("invalid float value %q: %w", p, err)
		}
		result[i] = float32(f)
	}
	return result, nil
}

// VectorStore 向量存储实现
type VectorStore struct {
	pool      *pgxpool.Pool
	table     string
	dimension int
}

// NewVectorStore 创建向量存储
func NewVectorStore(pool *pgxpool.Pool, table string, dimension int) *VectorStore {
	return &VectorStore{
		pool:      pool,
		table:     table,
		dimension: dimension,
	}
}

// Insert 插入向量
func (v *VectorStore) Insert(ctx context.Context, vectors []storage.Vector) error {
	if len(vectors) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, vec := range vectors {
		metadataJSON, _ := json.Marshal(vec.Metadata)

		// 必须用字符串 + ::vector 显式类型转换，pgx v5 不内置 pgvector OID 编码器
		vectorStr := vectorToString(vec.Vector)

		query := fmt.Sprintf(`
			INSERT INTO %s (id, vector, metadata)
			VALUES ($1, $2::vector, $3)
			ON CONFLICT (id)
			DO UPDATE SET vector = EXCLUDED.vector, metadata = EXCLUDED.metadata
		`, v.table)

		batch.Queue(query, vec.ID, vectorStr, metadataJSON)
	}

	br := v.pool.SendBatch(ctx, batch)
	defer br.Close()

	// 逐个读取每条结果，确保捕获插入错误
	for range vectors {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("vector batch insert failed: %w", err)
		}
	}

	return nil
}

// Search 搜索最相似的向量
func (v *VectorStore) Search(ctx context.Context, query []float32, topK int, filter map[string]interface{}) ([]storage.SearchResult, error) {
	var args []interface{}
	argPos := 1

	// 用字符串 + ::vector 显式转换，避免 pgx 无法编码 vector OID
	queryStr := vectorToString(query)

	// vector::text 以字符串返回向量，避免 pgx 解码 vector OID 失败
	baseQuery := fmt.Sprintf(`
		SELECT id, vector::text, metadata, 1 - (vector <=> $%d::vector) AS score
		FROM %s
	`, argPos, v.table)
	args = append(args, queryStr)
	argPos++

	if len(filter) > 0 {
		baseQuery += " WHERE metadata @> $" + fmt.Sprintf("%d", argPos) + "::jsonb"
		filterJSON, _ := json.Marshal(filter)
		args = append(args, filterJSON)
		argPos++
	}

	baseQuery += fmt.Sprintf(`
		ORDER BY score DESC
		LIMIT $%d
	`, argPos)
	args = append(args, topK)

	rows, err := v.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []storage.SearchResult
	for rows.Next() {
		var id string
		var vectorText string // 以文本接收，避免 pgx 解码 vector OID 问题
		var metadataJSON []byte
		var score float32

		if err := rows.Scan(&id, &vectorText, &metadataJSON, &score); err != nil {
			continue
		}

		// 将 pgvector 文本格式解析回 []float32
		vec, err := parseVectorString(vectorText)
		if err != nil {
			vec = nil // 解析失败时不影响其他字段
		}

		var metadata map[string]interface{}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &metadata)
		}

		results = append(results, storage.SearchResult{
			ID:       id,
			Vector:   vec,
			Score:    score,
			Metadata: metadata,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if len(results) == 0 {
		return []storage.SearchResult{}, nil
	}

	return results, nil
}

// SearchByText 基于 pg_trgm 三元组相似度进行文本检索，无需 Embedding API 调用。
// 实现了 storage.FullTextSearchStore 接口。
// 前提：pg_trgm 扩展已安装，且 metadata->>'request' 字段上建有 gin_trgm_ops 索引。
func (v *VectorStore) SearchByText(ctx context.Context, query string, topK int, minScore float32) ([]storage.SearchResult, error) {
	if query == "" {
		return []storage.SearchResult{}, nil
	}
	if minScore <= 0 {
		minScore = 0.3
	}
	if topK <= 0 {
		topK = 5
	}

	// similarity() 由 pg_trgm 提供，返回 0-1 的字符三元组相似度分数。
	// 注意：不在 SQL 层做 expires_at 过滤，与 Search() 行为一致，由调用方在 Go 层过滤。
	sql := fmt.Sprintf(`
		SELECT id, vector::text, metadata,
			similarity(COALESCE(metadata->>'request', ''), $1) AS score
		FROM %s
		WHERE COALESCE(metadata->>'request', '') <> ''
		  AND similarity(COALESCE(metadata->>'request', ''), $1) >= $2
		ORDER BY score DESC
		LIMIT $3
	`, v.table)

	rows, err := v.pool.Query(ctx, sql, query, minScore, topK)
	if err != nil {
		return nil, fmt.Errorf("trgm text search failed: %w", err)
	}
	defer rows.Close()

	var results []storage.SearchResult
	for rows.Next() {
		var id string
		var vectorText string
		var metadataJSON []byte
		var score float32

		if err := rows.Scan(&id, &vectorText, &metadataJSON, &score); err != nil {
			continue
		}

		vec, _ := parseVectorString(vectorText)

		var metadata map[string]interface{}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &metadata) //nolint:errcheck
		}

		results = append(results, storage.SearchResult{
			ID:       id,
			Vector:   vec,
			Score:    score,
			Metadata: metadata,
		})
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	if len(results) == 0 {
		return []storage.SearchResult{}, nil
	}

	return results, nil
}

// Delete 删除向量
func (v *VectorStore) Delete(ctx context.Context, ids []string) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE id = ANY($1)
	`, v.table)

	_, err := v.pool.Exec(ctx, query, ids)
	return err
}

// Get 获取向量
func (v *VectorStore) Get(ctx context.Context, ids []string) ([]storage.Vector, error) {
	query := fmt.Sprintf(`
		SELECT id, vector::text, metadata
		FROM %s
		WHERE id = ANY($1)
	`, v.table)

	rows, err := v.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vectors []storage.Vector
	for rows.Next() {
		var id string
		var vectorText string
		var metadataJSON []byte

		if err := rows.Scan(&id, &vectorText, &metadataJSON); err != nil {
			continue
		}

		vec, err := parseVectorString(vectorText)
		if err != nil {
			vec = nil
		}

		var metadata map[string]interface{}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &metadata)
		}

		vectors = append(vectors, storage.Vector{
			ID:       id,
			Vector:   vec,
			Metadata: metadata,
		})
	}

	if len(vectors) == 0 {
		return []storage.Vector{}, nil
	}

	return vectors, nil
}

// Update 更新向量
func (v *VectorStore) Update(ctx context.Context, vectors []storage.Vector) error {
	return v.Insert(ctx, vectors)
}

// CreateCollection 创建集合
func (v *VectorStore) CreateCollection(ctx context.Context, collection string, dimension int) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			vector vector(%d),
			metadata JSONB,
			created_at TIMESTAMP DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_%s_vector ON %s USING hnsw (vector vector_cosine_ops);
		CREATE INDEX IF NOT EXISTS idx_%s_metadata ON %s USING gin (metadata);
	`, collection, dimension, collection, collection, collection, collection)

	_, err := v.pool.Exec(ctx, query)
	return err
}

// DropCollection 删除集合
func (v *VectorStore) DropCollection(ctx context.Context, collection string) error {
	query := fmt.Sprintf(`DROP TABLE IF EXISTS %s CASCADE`, collection)
	_, err := v.pool.Exec(ctx, query)
	return err
}

// CollectionExists 检查集合是否存在
func (v *VectorStore) CollectionExists(ctx context.Context, collection string) (bool, error) {
	var exists bool

	query := `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
		)
	`

	err := v.pool.QueryRow(ctx, query, collection).Scan(&exists)
	return exists, err
}

// GetCollection 获取集合信息
func (v *VectorStore) GetCollection(ctx context.Context, collection string) (*storage.CollectionInfo, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
	`, collection)

	var count int64
	err := v.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &storage.CollectionInfo{
		Name:       collection,
		Dimension:  v.dimension,
		Count:      count,
		IndexType:  string(storage.IndexTypeHNSW),
		MetricType: string(storage.MetricTypeCosine),
	}, nil
}

// ListCollections 列出所有集合
func (v *VectorStore) ListCollections(ctx context.Context) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name LIKE 'vector_%'
		ORDER BY table_name
	`

	rows, err := v.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		collections = append(collections, name)
	}

	return collections, nil
}

// ListAll 列出集合中所有文档
func (v *VectorStore) ListAll(ctx context.Context, collection string, limit int, offset int) ([]storage.VectorEntry, int64, error) {
	var total int64

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, collection)
	if err := v.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	// limit=0 时仅返回总数，不查询数据行
	if limit == 0 {
		return []storage.VectorEntry{}, total, nil
	}

	query := fmt.Sprintf(`
		SELECT id, metadata, created_at
		FROM %s
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, collection)

	rows, err := v.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []storage.VectorEntry
	for rows.Next() {
		var id string
		var metadataJSON []byte
		var createdAt time.Time

		if err := rows.Scan(&id, &metadataJSON, &createdAt); err != nil {
			continue
		}

		var metadata map[string]interface{}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &metadata)
		}
		if metadata == nil {
			metadata = map[string]interface{}{}
		}

		entries = append(entries, storage.VectorEntry{
			ID:        id,
			Metadata:  metadata,
			CreatedAt: createdAt,
		})
	}

	return entries, total, nil
}

// GetStoreInfo 获取存储后端信息
func (v *VectorStore) GetStoreInfo() storage.StoreInfo {
	return storage.StoreInfo{
		Type: "postgresql",
	}
}

// GetDefaultCollection 获取默认集合名称
func (v *VectorStore) GetDefaultCollection() string {
	return v.table
}

// Close 关闭连接
func (v *VectorStore) Close() error {
	return nil
}
