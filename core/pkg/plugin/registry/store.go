package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Store 插件注册表存储接口
type Store interface {
	// 插件管理
	Register(ctx context.Context, plugin *PluginMetadata) error
	Get(ctx context.Context, id string) (*PluginMetadata, error)
	Update(ctx context.Context, plugin *PluginMetadata) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req *ListPluginsRequest) (*ListPluginsResponse, error)

	// 版本管理
	ListVersions(ctx context.Context, pluginID string) ([]string, error)
	GetVersion(ctx context.Context, pluginID string, version string) (*PluginMetadata, error)

	// 评分管理
	Rate(ctx context.Context, pluginID string, userID string, score int, comment string) error
	GetRating(ctx context.Context, pluginID string) (float64, int, error)

	// 统计
	IncrementDownloadCount(ctx context.Context, pluginID string) error
}

// MemoryStore 内存存储实现
type MemoryStore struct {
	mu       sync.RWMutex
	plugins  map[string]*PluginMetadata
	ratings  map[string][]*PluginRating
	versions map[string]map[string]*PluginMetadata // pluginID -> version -> metadata
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		plugins:  make(map[string]*PluginMetadata),
		ratings:  make(map[string][]*PluginRating),
		versions: make(map[string]map[string]*PluginMetadata),
	}
}

// Register 注册插件
func (s *MemoryStore) Register(ctx context.Context, plugin *PluginMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 生成唯一 ID
	if plugin.ID == "" {
		plugin.ID = fmt.Sprintf("%s@%s", plugin.Name, plugin.Version)
	}

	now := time.Now()
	if plugin.CreatedAt.IsZero() {
		plugin.CreatedAt = now
	}
	plugin.UpdatedAt = now

	s.plugins[plugin.ID] = plugin

	// 添加到版本映射
	if s.versions[plugin.Name] == nil {
		s.versions[plugin.Name] = make(map[string]*PluginMetadata)
	}
	s.versions[plugin.Name][plugin.Version] = plugin

	return nil
}

// Get 获取插件
func (s *MemoryStore) Get(ctx context.Context, id string) (*PluginMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, ok := s.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}

	return plugin, nil
}

// Update 更新插件
func (s *MemoryStore) Update(ctx context.Context, plugin *PluginMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plugins[plugin.ID]; !ok {
		return fmt.Errorf("plugin not found: %s", plugin.ID)
	}

	plugin.UpdatedAt = time.Now()
	s.plugins[plugin.ID] = plugin

	// 更新版本映射
	if s.versions[plugin.Name] == nil {
		s.versions[plugin.Name] = make(map[string]*PluginMetadata)
	}
	s.versions[plugin.Name][plugin.Version] = plugin

	return nil
}

// Delete 删除插件
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plugin, ok := s.plugins[id]
	if !ok {
		return fmt.Errorf("plugin not found: %s", id)
	}

	delete(s.plugins, id)

	// 从版本映射中删除
	if s.versions[plugin.Name] != nil {
		delete(s.versions[plugin.Name], plugin.Version)
	}

	return nil
}

// List 列出插件
func (s *MemoryStore) List(ctx context.Context, req *ListPluginsRequest) (*ListPluginsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 过滤和搜索
	var filtered []*PluginMetadata
	for _, plugin := range s.plugins {
		// 分类过滤
		if req.Category != "" && plugin.Category != req.Category {
			continue
		}

		// 标签过滤
		if len(req.Tags) > 0 {
			hasTag := false
			for _, tag := range req.Tags {
				for _, pluginTag := range plugin.Tags {
					if tag == pluginTag {
						hasTag = true
						break
					}
				}
				if hasTag {
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// 作者过滤
		if req.Author != "" && plugin.Author != req.Author {
			continue
		}

		// 搜索
		if req.Search != "" {
			searchLower := strings.ToLower(req.Search)
			nameMatch := strings.Contains(strings.ToLower(plugin.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(plugin.Description), searchLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		filtered = append(filtered, plugin)
	}

	// 排序
	sortPlugins(filtered, req.SortBy, req.SortOrder)

	// 分页
	total := len(filtered)
	totalPages := (total + req.PageSize - 1) / req.PageSize

	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	result := make([]PluginMetadata, len(filtered[start:end]))
	for i, p := range filtered[start:end] {
		result[i] = *p
	}

	return &ListPluginsResponse{
		Plugins:    result,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ListVersions 列出插件版本
func (s *MemoryStore) ListVersions(ctx context.Context, pluginID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, ok := s.plugins[pluginID]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	versions := make([]string, 0)
	if s.versions[plugin.Name] != nil {
		for version := range s.versions[plugin.Name] {
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// GetVersion 获取特定版本
func (s *MemoryStore) GetVersion(ctx context.Context, pluginID string, version string) (*PluginMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugin, ok := s.plugins[pluginID]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	if s.versions[plugin.Name] == nil {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	metadata, ok := s.versions[plugin.Name][version]
	if !ok {
		return nil, fmt.Errorf("version not found: %s@%s", plugin.Name, version)
	}

	return metadata, nil
}

// Rate 评分插件
func (s *MemoryStore) Rate(ctx context.Context, pluginID string, userID string, score int, comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.plugins[pluginID]; !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	rating := &PluginRating{
		ID:        fmt.Sprintf("%s:%s", pluginID, userID),
		PluginID:  pluginID,
		UserID:    userID,
		Score:     score,
		Comment:   comment,
		CreatedAt: time.Now(),
	}

	s.ratings[pluginID] = append(s.ratings[pluginID], rating)

	// 更新平均评分
	s.updateRating(pluginID)

	return nil
}

// GetRating 获取评分
func (s *MemoryStore) GetRating(ctx context.Context, pluginID string) (float64, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.plugins[pluginID]; !ok {
		return 0, 0, fmt.Errorf("plugin not found: %s", pluginID)
	}

	plugin := s.plugins[pluginID]
	return plugin.Rating, plugin.RatingCount, nil
}

// IncrementDownloadCount 增加下载计数
func (s *MemoryStore) IncrementDownloadCount(ctx context.Context, pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plugin, ok := s.plugins[pluginID]
	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	plugin.DownloadCount++

	return nil
}

// updateRating 更新评分
func (s *MemoryStore) updateRating(pluginID string) {
	ratings := s.ratings[pluginID]
	if len(ratings) == 0 {
		return
	}

	total := 0
	for _, r := range ratings {
		total += r.Score
	}

	plugin := s.plugins[pluginID]
	plugin.Rating = float64(total) / float64(len(ratings))
	plugin.RatingCount = len(ratings)
}

// sortPlugins 排序插件
func sortPlugins(plugins []*PluginMetadata, sortBy, sortOrder string) {
	switch sortBy {
	case "name":
		for i := 0; i < len(plugins)-1; i++ {
			for j := i + 1; j < len(plugins); j++ {
				if (sortOrder == "desc" && plugins[i].Name < plugins[j].Name) ||
					(sortOrder != "desc" && plugins[i].Name > plugins[j].Name) {
					plugins[i], plugins[j] = plugins[j], plugins[i]
				}
			}
		}
	case "download_count":
		for i := 0; i < len(plugins)-1; i++ {
			for j := i + 1; j < len(plugins); j++ {
				if (sortOrder == "desc" && plugins[i].DownloadCount < plugins[j].DownloadCount) ||
					(sortOrder != "desc" && plugins[i].DownloadCount > plugins[j].DownloadCount) {
					plugins[i], plugins[j] = plugins[j], plugins[i]
				}
			}
		}
	case "rating":
		for i := 0; i < len(plugins)-1; i++ {
			for j := i + 1; j < len(plugins); j++ {
				if (sortOrder == "desc" && plugins[i].Rating < plugins[j].Rating) ||
					(sortOrder != "desc" && plugins[i].Rating > plugins[j].Rating) {
					plugins[i], plugins[j] = plugins[j], plugins[i]
				}
			}
		}
	case "created_at":
		for i := 0; i < len(plugins)-1; i++ {
			for j := i + 1; j < len(plugins); j++ {
				if (sortOrder == "desc" && plugins[i].CreatedAt.Before(plugins[j].CreatedAt)) ||
					(sortOrder != "desc" && plugins[i].CreatedAt.After(plugins[j].CreatedAt)) {
					plugins[i], plugins[j] = plugins[j], plugins[i]
				}
			}
		}
	default:
		// 默认按名称排序
		for i := 0; i < len(plugins)-1; i++ {
			for j := i + 1; j < len(plugins); j++ {
				if plugins[i].Name > plugins[j].Name {
					plugins[i], plugins[j] = plugins[j], plugins[i]
				}
			}
		}
	}
}

// DBStore 数据库存储实现
type DBStore struct {
	db *sql.DB
}

const (
	pluginMarketRegistryTable = "plugin_market_registry"
	pluginMarketRatingsTable  = "plugin_market_ratings"
)

// NewDBStore 创建数据库存储
func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{db: db}
}

// Register 注册插件
func (s *DBStore) Register(ctx context.Context, plugin *PluginMetadata) error {
	if plugin.ID == "" {
		plugin.ID = fmt.Sprintf("%s@%s", plugin.Name, plugin.Version)
	}

	now := time.Now()
	if plugin.CreatedAt.IsZero() {
		plugin.CreatedAt = now
	}
	plugin.UpdatedAt = now

	tags, _ := json.Marshal(plugin.Tags)
	permissions, _ := json.Marshal(plugin.Permissions)
	dependencies, _ := json.Marshal(plugin.Dependencies)

	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, name, version, description, author, email, url,
			category, tags, permissions, dependencies,
			download_url, checksum, signature, size,
			download_count, rating, rating_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (id) DO UPDATE SET
			version = EXCLUDED.version,
			description = EXCLUDED.description,
			updated_at = EXCLUDED.updated_at
	`, pluginMarketRegistryTable)

	_, err := s.db.ExecContext(ctx, query,
		plugin.ID, plugin.Name, plugin.Version, plugin.Description,
		plugin.Author, plugin.Email, plugin.URL, plugin.Category,
		tags, permissions, dependencies,
		plugin.DownloadURL, plugin.Checksum, plugin.Signature, plugin.Size,
		plugin.DownloadCount, plugin.Rating, plugin.RatingCount,
		plugin.CreatedAt, plugin.UpdatedAt,
	)

	return err
}

// Get 获取插件
func (s *DBStore) Get(ctx context.Context, id string) (*PluginMetadata, error) {
	query := fmt.Sprintf(`
		SELECT id, name, version, description, author, email, url,
			category, tags, permissions, dependencies,
			download_url, checksum, signature, size,
			download_count, rating, rating_count, created_at, updated_at
		FROM %s WHERE id = $1
	`, pluginMarketRegistryTable)

	plugin := &PluginMetadata{}
	var tags, permissions, dependencies []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&plugin.ID, &plugin.Name, &plugin.Version, &plugin.Description,
		&plugin.Author, &plugin.Email, &plugin.URL, &plugin.Category,
		&tags, &permissions, &dependencies,
		&plugin.DownloadURL, &plugin.Checksum, &plugin.Signature, &plugin.Size,
		&plugin.DownloadCount, &plugin.Rating, &plugin.RatingCount,
		&plugin.CreatedAt, &plugin.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(tags, &plugin.Tags)
	json.Unmarshal(permissions, &plugin.Permissions)
	json.Unmarshal(dependencies, &plugin.Dependencies)

	return plugin, nil
}

// Update 更新插件
func (s *DBStore) Update(ctx context.Context, plugin *PluginMetadata) error {
	plugin.UpdatedAt = time.Now()
	return s.Register(ctx, plugin)
}

// Delete 删除插件
func (s *DBStore) Delete(ctx context.Context, id string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, pluginMarketRegistryTable)
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// List 列出插件
func (s *DBStore) List(ctx context.Context, req *ListPluginsRequest) (*ListPluginsResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 构建查询
	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if req.Category != "" {
		where += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, req.Category)
		argIdx++
	}

	if req.Author != "" {
		where += fmt.Sprintf(" AND author = $%d", argIdx)
		args = append(args, req.Author)
		argIdx++
	}

	if req.Search != "" {
		where += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+req.Search+"%")
		argIdx++
	}

	// 排序
	orderBy := "name"
	switch req.SortBy {
	case "download_count":
		orderBy = "download_count"
	case "rating":
		orderBy = "rating"
	case "created_at":
		orderBy = "created_at"
	}

	order := "ASC"
	if req.SortOrder == "desc" {
		order = "DESC"
	}

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s ", pluginMarketRegistryTable) + where
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// 查询分页数据
	query := fmt.Sprintf(`
		SELECT id, name, version, description, author, email, url,
			category, tags, permissions, dependencies,
			download_url, checksum, signature, size,
			download_count, rating, rating_count, created_at, updated_at
		FROM %s
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, pluginMarketRegistryTable, where, orderBy, order, argIdx, argIdx+1)

	args = append(args, req.PageSize, (req.Page-1)*req.PageSize)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plugins := make([]PluginMetadata, 0)
	for rows.Next() {
		plugin := PluginMetadata{}
		var tags, permissions, dependencies []byte

		err := rows.Scan(
			&plugin.ID, &plugin.Name, &plugin.Version, &plugin.Description,
			&plugin.Author, &plugin.Email, &plugin.URL, &plugin.Category,
			&tags, &permissions, &dependencies,
			&plugin.DownloadURL, &plugin.Checksum, &plugin.Signature, &plugin.Size,
			&plugin.DownloadCount, &plugin.Rating, &plugin.RatingCount,
			&plugin.CreatedAt, &plugin.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(tags, &plugin.Tags)
		json.Unmarshal(permissions, &plugin.Permissions)
		json.Unmarshal(dependencies, &plugin.Dependencies)

		plugins = append(plugins, plugin)
	}

	totalPages := (total + req.PageSize - 1) / req.PageSize

	return &ListPluginsResponse{
		Plugins:    plugins,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ListVersions 列出版本
func (s *DBStore) ListVersions(ctx context.Context, pluginID string) ([]string, error) {
	query := fmt.Sprintf(`SELECT version FROM %s WHERE name = (SELECT name FROM %s WHERE id = $1)`, pluginMarketRegistryTable, pluginMarketRegistryTable)
	rows, err := s.db.QueryContext(ctx, query, pluginID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make([]string, 0)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, nil
}

// GetVersion 获取特定版本
func (s *DBStore) GetVersion(ctx context.Context, pluginID string, version string) (*PluginMetadata, error) {
	query := fmt.Sprintf(`
		SELECT id, name, version, description, author, email, url,
			category, tags, permissions, dependencies,
			download_url, checksum, signature, size,
			download_count, rating, rating_count, created_at, updated_at
		FROM %s
		WHERE name = (SELECT name FROM %s WHERE id = $1) AND version = $2
	`, pluginMarketRegistryTable, pluginMarketRegistryTable)

	plugin := &PluginMetadata{}
	var tags, permissions, dependencies []byte

	err := s.db.QueryRowContext(ctx, query, pluginID, version).Scan(
		&plugin.ID, &plugin.Name, &plugin.Version, &plugin.Description,
		&plugin.Author, &plugin.Email, &plugin.URL, &plugin.Category,
		&tags, &permissions, &dependencies,
		&plugin.DownloadURL, &plugin.Checksum, &plugin.Signature, &plugin.Size,
		&plugin.DownloadCount, &plugin.Rating, &plugin.RatingCount,
		&plugin.CreatedAt, &plugin.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("version not found: %s@%s", pluginID, version)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(tags, &plugin.Tags)
	json.Unmarshal(permissions, &plugin.Permissions)
	json.Unmarshal(dependencies, &plugin.Dependencies)

	return plugin, nil
}

// Rate 评分插件
func (s *DBStore) Rate(ctx context.Context, pluginID string, userID string, score int, comment string) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (id, plugin_id, user_id, score, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (plugin_id, user_id) DO UPDATE SET
			score = EXCLUDED.score,
			comment = EXCLUDED.comment,
			created_at = EXCLUDED.created_at
	`, pluginMarketRatingsTable)

	id := fmt.Sprintf("%s:%s", pluginID, userID)
	_, err := s.db.ExecContext(ctx, query, id, pluginID, userID, score, comment, time.Now())
	if err != nil {
		return err
	}

	// 更新平均评分
	return s.updateRating(ctx, pluginID)
}

// GetRating 获取评分
func (s *DBStore) GetRating(ctx context.Context, pluginID string) (float64, int, error) {
	query := fmt.Sprintf(`SELECT rating, rating_count FROM %s WHERE id = $1`, pluginMarketRegistryTable)
	var rating float64
	var count int
	err := s.db.QueryRowContext(ctx, query, pluginID).Scan(&rating, &count)
	if err == sql.ErrNoRows {
		return 0, 0, fmt.Errorf("plugin not found: %s", pluginID)
	}
	return rating, count, err
}

// IncrementDownloadCount 增加下载计数
func (s *DBStore) IncrementDownloadCount(ctx context.Context, pluginID string) error {
	query := fmt.Sprintf(`UPDATE %s SET download_count = download_count + 1 WHERE id = $1`, pluginMarketRegistryTable)
	_, err := s.db.ExecContext(ctx, query, pluginID)
	return err
}

// updateRating 更新评分
func (s *DBStore) updateRating(ctx context.Context, pluginID string) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET rating = (SELECT AVG(score) FROM %s WHERE plugin_id = $1),
		    rating_count = (SELECT COUNT(*) FROM %s WHERE plugin_id = $1)
		WHERE id = $1
	`, pluginMarketRegistryTable, pluginMarketRatingsTable, pluginMarketRatingsTable)
	_, err := s.db.ExecContext(ctx, query, pluginID)
	return err
}
