package pipeline

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"centag/core/pkg/database"
)

// PipelineStore 流水线持久化存储接口
type PipelineStore interface {
	Create(pipeline *AgentPatternPipeline) error
	CreateForTenant(tenantID string, pipeline *AgentPatternPipeline) error // 新增：租户隔离创建
	Get(id string) (*AgentPatternPipeline, error)
	GetByTenant(tenantID, id string) (*AgentPatternPipeline, error) // 新增：租户隔离获取
	GetByShortcutCode(code string) (*AgentPatternPipeline, error)
	Update(pipeline *AgentPatternPipeline) error
	Delete(id string) error
	DeleteForTenant(tenantID, id string) error
	List() ([]*AgentPatternPipeline, error)
	ListByTenant(tenantID string) ([]*AgentPatternPipeline, error) // 新增：租户隔离列表
	ListEnabled() ([]*AgentPatternPipeline, error)
	ListEnabledByTenant(tenantID string) ([]*AgentPatternPipeline, error) // 新增：租户隔离启用列表
	RecordExecution(log *ExecutionRecord) error
	GetExecutionHistory(pipelineID string, limit int) ([]*ExecutionRecord, error)
	GetExecution(id int64) (*ExecutionRecord, error)
}

// ExecutionRecord 执行历史记录
type ExecutionRecord struct {
	ID            int64     `json:"id"`
	PipelineID    string    `json:"pipeline_id"`
	InputContent  string    `json:"input_content"`
	OutputContent string    `json:"output_content"`
	Status        string    `json:"status"`
	DurationMs    int64     `json:"duration_ms"`
	TotalTokens   int       `json:"total_tokens"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	// 节点级审计摘要（JSON字符串）
	NodeAuditLog string `json:"node_audit_log,omitempty"`
}

// NodeAuditSummary 节点审计摘要
type NodeAuditSummary struct {
	NodeID         string `json:"node_id"`
	Implementation string `json:"implementation,omitempty"`
	Kind           string `json:"kind,omitempty"`
	Success        bool   `json:"success"`
	DurationMs     int64  `json:"duration_ms"`
	ErrorCode      string `json:"error_code,omitempty"`
}

// DBPipelineStore 基于数据库的流水线存储实现
type DBPipelineStore struct {
	db *sql.DB
	// pg 为 true 时使用 PostgreSQL 占位符 ($1…)；sqlite 驱动使用 ?
	pg bool
}

// NewDBPipelineStore 创建数据库流水线存储
func NewDBPipelineStore() (*DBPipelineStore, error) {
	db := database.Get().GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return &DBPipelineStore{
		db: db,
		pg: database.Get().DriverName() == "postgresql",
	}, nil
}

// Create 创建流水线
func (s *DBPipelineStore) Create(pipeline *AgentPatternPipeline) error {
	nodesJSON, err := json.Marshal(pipeline.Nodes)
	if err != nil {
		return fmt.Errorf("failed to marshal nodes: %w", err)
	}

	configJSON, err := json.Marshal(pipeline.GlobalConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal global config: %w", err)
	}

	var metadataJSON []byte
	if pipeline.Metadata != nil {
		metadataJSON, err = json.Marshal(pipeline.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	var query string
	var tenantID interface{}
	if pipeline.TenantID != "" {
		tenantID = pipeline.TenantID
	}

	if s.pg {
		query = `
		INSERT INTO pipelines (id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, enabled, updated_at, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			version = EXCLUDED.version,
			shortcut_code = EXCLUDED.shortcut_code,
			nodes_json = EXCLUDED.nodes_json,
			global_config_json = EXCLUDED.global_config_json,
			metadata_json = EXCLUDED.metadata_json,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at,
			tenant_id = EXCLUDED.tenant_id
	`
	} else {
		query = `
		INSERT INTO pipelines (id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, enabled, updated_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			version = excluded.version,
			shortcut_code = excluded.shortcut_code,
			nodes_json = excluded.nodes_json,
			global_config_json = excluded.global_config_json,
			metadata_json = excluded.metadata_json,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at,
			tenant_id = excluded.tenant_id
	`
	}

	// 空 shortcut_code 转为 NULL，避免 UNIQUE 索引冲突（SQLite UNIQUE 索引区分 "" 和 NULL）
	var shortcutCode interface{}
	if pipeline.ShortcutCode != "" {
		shortcutCode = pipeline.ShortcutCode
	}

	_, err = s.db.Exec(query,
		pipeline.ID, pipeline.Name, pipeline.Description, pipeline.Version, shortcutCode,
		nodesJSON, configJSON, metadataJSON, true, time.Now(), tenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to save pipeline: %w", err)
	}

	return nil
}

// Get 获取流水线
func (s *DBPipelineStore) Get(id string) (*AgentPatternPipeline, error) {
	var query string
	if s.pg {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = $1`
	} else {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = ?`
	}

	row := s.db.QueryRow(query, id)
	return s.scanPipeline(row)
}

// GetByShortcutCode 根据快捷码获取流水线
func (s *DBPipelineStore) GetByShortcutCode(code string) (*AgentPatternPipeline, error) {
	var query string
	if s.pg {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE shortcut_code = $1 AND enabled = true`
	} else {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE shortcut_code = ? AND enabled = 1`
	}

	row := s.db.QueryRow(query, code)
	return s.scanPipeline(row)
}

// Update 更新流水线
func (s *DBPipelineStore) Update(pipeline *AgentPatternPipeline) error {
	return s.Create(pipeline)
}

// Delete 删除流水线（管理员/全局：按 id 删除，不区分租户）
func (s *DBPipelineStore) Delete(id string) error {
	var err error
	if s.pg {
		_, err = s.db.Exec(`DELETE FROM pipelines WHERE id = $1`, id)
	} else {
		_, err = s.db.Exec(`DELETE FROM pipelines WHERE id = ?`, id)
	}
	if err != nil {
		return fmt.Errorf("failed to delete pipeline: %w", err)
	}
	return nil
}

// DeleteForTenant 删除租户私有流水线（不影响系统共享预设）
func (s *DBPipelineStore) DeleteForTenant(tenantID, id string) error {
	if tenantID == "" {
		return fmt.Errorf("tenant id is required")
	}
	var err error
	if s.pg {
		_, err = s.db.Exec(`DELETE FROM pipelines WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	} else {
		_, err = s.db.Exec(`DELETE FROM pipelines WHERE id = ? AND tenant_id = ?`, id, tenantID)
	}
	if err != nil {
		return fmt.Errorf("failed to delete tenant pipeline: %w", err)
	}
	return nil
}

// List 列出所有流水线
func (s *DBPipelineStore) List() ([]*AgentPatternPipeline, error) {
	return s.listByCondition("")
}

// ListEnabled 列出启用的流水线
func (s *DBPipelineStore) ListEnabled() ([]*AgentPatternPipeline, error) {
	if s.pg {
		return s.listByCondition("WHERE enabled = true")
	}
	return s.listByCondition("WHERE enabled = 1")
}

func (s *DBPipelineStore) listByCondition(condition string) ([]*AgentPatternPipeline, error) {
	query := fmt.Sprintf(`SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines %s ORDER BY created_at DESC`, condition)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}
	defer rows.Close()

	var pipelines []*AgentPatternPipeline
	for rows.Next() {
		p, err := s.scanPipeline(rows)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}

	return pipelines, rows.Err()
}

func (s *DBPipelineStore) scanPipeline(scanner interface{ Scan(dest ...any) error }) (*AgentPatternPipeline, error) {
	var p AgentPatternPipeline
	var nodesJSON, configJSON, metadataJSON []byte
	var shortcutCode sql.NullString
	var tenantID sql.NullString

	err := scanner.Scan(&p.ID, &p.Name, &p.Description, &p.Version, &shortcutCode, &nodesJSON, &configJSON, &metadataJSON, &tenantID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pipeline not found")
		}
		return nil, fmt.Errorf("failed to scan pipeline: %w", err)
	}

	if shortcutCode.Valid {
		p.ShortcutCode = shortcutCode.String
	}
	if tenantID.Valid {
		p.TenantID = tenantID.String
	}

	if err := json.Unmarshal(nodesJSON, &p.Nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}
	// 归一化节点配置：将顶层 Backend/Model 归入 Config，统一出口
	for i := range p.Nodes {
		p.Nodes[i].Normalize()
	}
	if err := json.Unmarshal(configJSON, &p.GlobalConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal global config: %w", err)
	}
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &p.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &p, nil
}

// PluginRegistration 插件注册信息
type PluginRegistration struct {
	ID              int64     `json:"id"`
	Implementation  string    `json:"implementation"`
	Kind            string    `json:"kind"`
	Version         string    `json:"version"`
	DescriptorJSON  string    `json:"descriptor_json"`
	Source          string    `json:"source"` // "builtin", "remote", "local"
	Enabled         bool      `json:"enabled"`
	SignatureStatus string    `json:"signature_status"` // "verified", "invalid", "none"
	LastHealthCheck time.Time `json:"last_health_check"`
	HealthStatus    string    `json:"health_status"` // "healthy", "unhealthy", "unknown"
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PluginRegistryStore 插件注册表存储接口
type PluginRegistryStore interface {
	Register(plugin *PluginRegistration) error
	Get(implementation string) (*PluginRegistration, error)
	Update(plugin *PluginRegistration) error
	Delete(implementation string) error
	List() ([]*PluginRegistration, error)
	UpdateHealthCheck(implementation string, healthy bool, details string) error
}

// DBPluginRegistryStore 基于数据库的插件注册表实现
type DBPluginRegistryStore struct {
	db *sql.DB
	pg bool
}

// NewDBPluginRegistryStore 创建数据库插件注册表存储
func NewDBPluginRegistryStore() (*DBPluginRegistryStore, error) {
	db := database.Get().GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return &DBPluginRegistryStore{
		db: db,
		pg: database.Get().DriverName() == "postgresql",
	}, nil
}

// scanPluginRegistration 扫描插件注册信息
func scanPluginRegistration(scanner interface{ Scan(dest ...any) error }) (*PluginRegistration, error) {
	var pr PluginRegistration
	var descriptorJSON sql.NullString
	var signatureStatus sql.NullString
	var healthStatus sql.NullString
	var lastHealthCheckStr sql.NullString
	var createdAtStr sql.NullString
	var updatedAtStr sql.NullString
	err := scanner.Scan(
		&pr.ID, &pr.Implementation, &pr.Kind, &pr.Version,
		&descriptorJSON, &pr.Source, &pr.Enabled,
		&signatureStatus, &lastHealthCheckStr, &healthStatus,
		&createdAtStr, &updatedAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("plugin not found")
		}
		return nil, fmt.Errorf("failed to scan plugin registration: %w", err)
	}
	if descriptorJSON.Valid {
		pr.DescriptorJSON = descriptorJSON.String
	}
	if signatureStatus.Valid {
		pr.SignatureStatus = signatureStatus.String
	}
	if healthStatus.Valid {
		pr.HealthStatus = healthStatus.String
	}
	// 解析时间戳字符串（SQLite 和 PostgreSQL 都支持）
	if lastHealthCheckStr.Valid && lastHealthCheckStr.String != "" {
		if t, err := time.Parse(time.RFC3339, lastHealthCheckStr.String); err == nil {
			pr.LastHealthCheck = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", lastHealthCheckStr.String); err == nil {
			// SQLite CURRENT_TIMESTAMP 返回 UTC 时间
			pr.LastHealthCheck = t.UTC()
		}
	}
	if createdAtStr.Valid && createdAtStr.String != "" {
		if t, err := time.Parse(time.RFC3339, createdAtStr.String); err == nil {
			pr.CreatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr.String); err == nil {
			// SQLite CURRENT_TIMESTAMP 返回 UTC 时间
			pr.CreatedAt = t.UTC()
		}
	}
	if updatedAtStr.Valid && updatedAtStr.String != "" {
		if t, err := time.Parse(time.RFC3339, updatedAtStr.String); err == nil {
			pr.UpdatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr.String); err == nil {
			// SQLite CURRENT_TIMESTAMP 返回 UTC 时间
			pr.UpdatedAt = t.UTC()
		}
	}
	return &pr, nil
}

// Register 注册插件
func (s *DBPluginRegistryStore) Register(plugin *PluginRegistration) error {
	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}
	if plugin.Implementation == "" {
		return fmt.Errorf("implementation is required")
	}

	var query string
	var err error

	if s.pg {
		query = `
		INSERT INTO plugin_registry (implementation, kind, version, descriptor_json, source, enabled, signature_status, health_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (implementation) DO UPDATE SET
			kind = EXCLUDED.kind,
			version = EXCLUDED.version,
			descriptor_json = EXCLUDED.descriptor_json,
			source = EXCLUDED.source,
			updated_at = CURRENT_TIMESTAMP
		`
		_, err = s.db.Exec(query,
			plugin.Implementation, plugin.Kind, plugin.Version,
			plugin.DescriptorJSON, plugin.Source, plugin.Enabled,
			plugin.SignatureStatus, plugin.HealthStatus)
	} else {
		query = `
		INSERT INTO plugin_registry (implementation, kind, version, descriptor_json, source, enabled, signature_status, health_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(implementation) DO UPDATE SET
			kind = excluded.kind,
			version = excluded.version,
			descriptor_json = excluded.descriptor_json,
			source = excluded.source,
			updated_at = CURRENT_TIMESTAMP
		`
		_, err = s.db.Exec(query,
			plugin.Implementation, plugin.Kind, plugin.Version,
			plugin.DescriptorJSON, plugin.Source, plugin.Enabled,
			plugin.SignatureStatus, plugin.HealthStatus)
	}

	if err != nil {
		return fmt.Errorf("failed to register plugin: %w", err)
	}
	return nil
}

// Get 获取插件注册信息
func (s *DBPluginRegistryStore) Get(implementation string) (*PluginRegistration, error) {
	if implementation == "" {
		return nil, fmt.Errorf("implementation is required")
	}

	var query string
	if s.pg {
		query = `SELECT id, implementation, kind, version, descriptor_json, source, enabled, signature_status, last_health_check, health_status, created_at, updated_at
		FROM plugin_registry WHERE implementation = $1`
	} else {
		query = `SELECT id, implementation, kind, version, descriptor_json, source, enabled, signature_status, last_health_check, health_status, created_at, updated_at
		FROM plugin_registry WHERE implementation = ?`
	}

	row := s.db.QueryRow(query, implementation)
	return scanPluginRegistration(row)
}

// Update 更新插件注册信息
func (s *DBPluginRegistryStore) Update(plugin *PluginRegistration) error {
	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}
	if plugin.Implementation == "" {
		return fmt.Errorf("implementation is required")
	}

	var query string
	var err error

	if s.pg {
		query = `
		UPDATE plugin_registry SET
			kind = $1,
			version = $2,
			descriptor_json = $3,
			source = $4,
			enabled = $5,
			signature_status = $6,
			health_status = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE implementation = $8`
		_, err = s.db.Exec(query,
			plugin.Kind, plugin.Version, plugin.DescriptorJSON,
			plugin.Source, plugin.Enabled, plugin.SignatureStatus,
			plugin.HealthStatus, plugin.Implementation)
	} else {
		query = `
		UPDATE plugin_registry SET
			kind = ?,
			version = ?,
			descriptor_json = ?,
			source = ?,
			enabled = ?,
			signature_status = ?,
			health_status = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE implementation = ?`
		_, err = s.db.Exec(query,
			plugin.Kind, plugin.Version, plugin.DescriptorJSON,
			plugin.Source, plugin.Enabled, plugin.SignatureStatus,
			plugin.HealthStatus, plugin.Implementation)
	}

	if err != nil {
		return fmt.Errorf("failed to update plugin: %w", err)
	}
	return nil
}

// Delete 删除插件注册信息
func (s *DBPluginRegistryStore) Delete(implementation string) error {
	if implementation == "" {
		return fmt.Errorf("implementation is required")
	}

	var err error
	if s.pg {
		_, err = s.db.Exec(`DELETE FROM plugin_registry WHERE implementation = $1`, implementation)
	} else {
		_, err = s.db.Exec(`DELETE FROM plugin_registry WHERE implementation = ?`, implementation)
	}

	if err != nil {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}
	return nil
}

// List 列出所有插件注册信息
func (s *DBPluginRegistryStore) List() ([]*PluginRegistration, error) {
	return s.listByCondition("")
}

// ListEnabled 列出启用的插件
func (s *DBPluginRegistryStore) ListEnabled() ([]*PluginRegistration, error) {
	if s.pg {
		return s.listByCondition("WHERE enabled = true")
	}
	return s.listByCondition("WHERE enabled = 1")
}

func (s *DBPluginRegistryStore) listByCondition(condition string) ([]*PluginRegistration, error) {
	query := fmt.Sprintf(`
		SELECT id, implementation, kind, version, descriptor_json, source, enabled,
			signature_status, last_health_check, health_status, created_at, updated_at
		FROM plugin_registry %s
		ORDER BY created_at DESC`, condition)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}
	defer rows.Close()

	var plugins []*PluginRegistration
	for rows.Next() {
		pr, err := scanPluginRegistration(rows)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, pr)
	}

	return plugins, rows.Err()
}

// UpdateHealthCheck 更新健康检查状态
func (s *DBPluginRegistryStore) UpdateHealthCheck(implementation string, healthy bool, details string) error {
	if implementation == "" {
		return fmt.Errorf("implementation is required")
	}

	var healthStatus string
	if healthy {
		healthStatus = "healthy"
	} else {
		healthStatus = "unhealthy"
	}

	var query string
	var err error

	if s.pg {
		query = `
		UPDATE plugin_registry SET
			health_status = $1,
			last_health_check = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE implementation = $2`
		_, err = s.db.Exec(query, healthStatus, implementation)
	} else {
		query = `
		UPDATE plugin_registry SET
			health_status = ?,
			last_health_check = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE implementation = ?`
		_, err = s.db.Exec(query, healthStatus, implementation)
	}

	if err != nil {
		return fmt.Errorf("failed to update health check: %w", err)
	}
	return nil
}

// CreateForTenant 创建租户专属流水线
func (s *DBPipelineStore) CreateForTenant(tenantID string, pipeline *AgentPatternPipeline) error {
	pipeline.TenantID = tenantID
	return s.Create(pipeline)
}

// GetByTenant 按租户获取流水线（租户专属优先，否则系统共享）
func (s *DBPipelineStore) GetByTenant(tenantID, id string) (*AgentPatternPipeline, error) {
	if tenantID != "" {
		var query string
		if s.pg {
			query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = $1 AND tenant_id = $2`
		} else {
			query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = ? AND tenant_id = ?`
		}
		row := s.db.QueryRow(query, id, tenantID)
		p, err := s.scanPipeline(row)
		if err == nil {
			return p, nil
		}
	}
	return s.getSystemPipeline(id)
}

func (s *DBPipelineStore) getSystemPipeline(id string) (*AgentPatternPipeline, error) {
	var query string
	if s.pg {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = $1 AND (tenant_id IS NULL OR tenant_id = '')`
	} else {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE id = ? AND (tenant_id IS NULL OR tenant_id = '')`
	}
	row := s.db.QueryRow(query, id)
	return s.scanPipeline(row)
}

// ListByTenant 按租户列出流水线（系统共享 + 本租户私有）
func (s *DBPipelineStore) ListByTenant(tenantID string) ([]*AgentPatternPipeline, error) {
	if tenantID == "" {
		return s.listByCondition("WHERE tenant_id IS NULL OR tenant_id = ''")
	}
	var query string
	if s.pg {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = $1 ORDER BY created_at DESC`
	} else {
		query = `SELECT id, name, description, version, shortcut_code, nodes_json, global_config_json, metadata_json, tenant_id FROM pipelines WHERE tenant_id IS NULL OR tenant_id = '' OR tenant_id = ? ORDER BY created_at DESC`
	}
	rows, err := s.db.Query(query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tenant pipelines: %w", err)
	}
	defer rows.Close()

	var pipelines []*AgentPatternPipeline
	for rows.Next() {
		p, err := s.scanPipeline(rows)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, rows.Err()
}

// ListEnabledByTenant 按租户列出启用的流水线（租户隔离）
func (s *DBPipelineStore) ListEnabledByTenant(tenantID string) ([]*AgentPatternPipeline, error) {
	// 预留接口：当前返回全局启用列表
	// 实际实现需添加 tenant_id 过滤
	return s.ListEnabled()
}

// RecordExecution 记录执行历史
func (s *DBPipelineStore) RecordExecution(log *ExecutionRecord) error {
	var query string
	if s.pg {
		query = `
		INSERT INTO pipeline_executions (pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, node_audit_log)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`
	} else {
		query = `
		INSERT INTO pipeline_executions (pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, node_audit_log)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
	}

	_, err := s.db.Exec(query,
		log.PipelineID, log.InputContent, log.OutputContent,
		log.Status, log.DurationMs, log.TotalTokens, log.ErrorMessage, log.NodeAuditLog,
	)
	if err != nil {
		return fmt.Errorf("failed to record execution: %w", err)
	}

	return nil
}

func scanExecutionRecord(scanner interface{ Scan(dest ...any) error }) (*ExecutionRecord, error) {
	var r ExecutionRecord
	var errMsg sql.NullString
	var auditLog sql.NullString
	var createdAtStr sql.NullString
	err := scanner.Scan(&r.ID, &r.PipelineID, &r.InputContent, &r.OutputContent,
		&r.Status, &r.DurationMs, &r.TotalTokens, &errMsg, &createdAtStr, &auditLog)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("execution record not found")
		}
		return nil, fmt.Errorf("failed to scan execution record: %w", err)
	}
	if errMsg.Valid {
		r.ErrorMessage = errMsg.String
	}
	if auditLog.Valid {
		r.NodeAuditLog = auditLog.String
	}
	// 解析时间戳字符串（SQLite 和 PostgreSQL 都支持）
	if createdAtStr.Valid && createdAtStr.String != "" {
		if t, err := time.Parse(time.RFC3339, createdAtStr.String); err == nil {
			r.CreatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr.String); err == nil {
			// SQLite CURRENT_TIMESTAMP 返回 UTC 时间
			r.CreatedAt = t.UTC()
		}
	}
	return &r, nil
}

// GetExecutionHistory 获取执行历史
func (s *DBPipelineStore) GetExecutionHistory(pipelineID string, limit int) ([]*ExecutionRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var query string
	if s.pg {
		query = `
		SELECT id, pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, created_at, node_audit_log
		FROM pipeline_executions
		WHERE pipeline_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	} else {
		query = `
		SELECT id, pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, created_at, node_audit_log
		FROM pipeline_executions
		WHERE pipeline_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`
	}

	rows, err := s.db.Query(query, pipelineID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution history: %w", err)
	}
	defer rows.Close()

	var records []*ExecutionRecord
	for rows.Next() {
		r, err := scanExecutionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	return records, rows.Err()
}

// GetExecution 按 ID 获取单条执行记录
func (s *DBPipelineStore) GetExecution(id int64) (*ExecutionRecord, error) {
	var query string
	if s.pg {
		query = `
		SELECT id, pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, created_at, node_audit_log
		FROM pipeline_executions
		WHERE id = $1
	`
	} else {
		query = `
		SELECT id, pipeline_id, input_content, output_content, status, duration_ms, total_tokens, error_message, created_at, node_audit_log
		FROM pipeline_executions
		WHERE id = ?
	`
	}

	row := s.db.QueryRow(query, id)
	return scanExecutionRecord(row)
}
