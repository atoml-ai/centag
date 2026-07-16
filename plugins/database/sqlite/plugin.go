package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"centag/core/pkg/database"
)

// plugin 实现 database.DatabasePlugin 接口
type plugin struct {
	db *sql.DB
}

// NewPlugin 创建 SQLite 插件实例
func NewPlugin(config map[string]interface{}) (database.DatabasePlugin, error) {
	db, err := createDBFromConfig(config)
	if err != nil {
		return nil, err
	}

	return &plugin{db: db}, nil
}

// createDBFromConfig 从配置或环境变量创建数据库连接
func createDBFromConfig(config map[string]interface{}) (*sql.DB, error) {
	// 优先从传入的 config 获取路径
	var path string
	if config != nil {
		if p, ok := config["path"].(string); ok && p != "" {
			path = p
		}
	}

	// 如果 config 中没有，使用项目标准环境变量 LLM_PROXY_DB_PATH
	if path == "" {
		path = os.Getenv("LLM_PROXY_DB_PATH")
	}

	if path == "" {
		return nil, fmt.Errorf("LLM_PROXY_DB_PATH environment variable is not set")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create SQLite directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL&_cache_size=-64000&_busy_timeout=5000", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite: %w", err)
	}

	return db, nil
}

func (p *plugin) Name() string {
	return "sqlite"
}

func (p *plugin) UserStore() database.UserStore {
	return &sqliteUserStore{db: p.db}
}

func (p *plugin) APIKeyStore() database.APIKeyStore {
	return &sqliteAPIKeyStore{db: p.db}
}

func (p *plugin) RefreshTokenStore() database.RefreshTokenStore {
	return &sqliteRefreshTokenStore{db: p.db}
}

func (p *plugin) SystemConfigStore() database.SystemConfigStore {
	return &sqliteSystemConfigStore{db: p.db}
}

func (p *plugin) UserConfigStore() database.UserConfigStore {
	return &sqliteUserConfigStore{db: p.db}
}

func (p *plugin) ClashRuleStore() database.ClashRuleStore {
	return &sqliteClashRuleStore{db: p.db}
}

// TenantStore returns the store for tenant management (multi-tenant).
func (p *plugin) TenantStore() database.TenantStore {
	return database.NewUnifiedTenantService(p.db, &database.SQLiteDialect{})
}

func (p *plugin) Migrate(ctx context.Context) error {
	_ = ctx
	migrator := database.NewMigrator(p.db, "sqlite")
	return migrator.Migrate()
}

func (p *plugin) HealthCheck(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *plugin) Close() error {
	return p.db.Close()
}

func (p *plugin) GetDB() *sql.DB {
	return p.db
}

// ── User Store ──────────────────────────────────────────────────────────────

type sqliteUserStore struct {
	db *sql.DB
}

func (s *sqliteUserStore) Create(ctx context.Context, user *database.User) error {
	query := `INSERT INTO users (username, password_hash, role, display_name, email, enabled) VALUES (?, ?, ?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		user.Username, user.Password, user.Role,
		user.DisplayName, user.Email, user.Enabled,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = id
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt
	return nil
}

func scanSQLiteUserTenantID(tenantID sql.NullString) *string {
	if tenantID.Valid && tenantID.String != "" {
		v := tenantID.String
		return &v
	}
	return nil
}

func (s *sqliteUserStore) GetByID(ctx context.Context, id int64) (*database.User, error) {
	query := `SELECT id, username, password_hash, role, display_name, email, enabled, tenant_id, created_at, updated_at FROM users WHERE id = ?`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var createdAtStr, updatedAtStr string
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Password, &role,
		&user.DisplayName, &user.Email, &user.Enabled, &tenantID, &createdAtStr, &updatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Role = database.UserRole(role)
	user.TenantID = scanSQLiteUserTenantID(tenantID)
	if createdAtStr != "" {
		user.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAtStr, time.Local)
	}
	if updatedAtStr != "" {
		user.UpdatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", updatedAtStr, time.Local)
	}
	return user, nil
}

func (s *sqliteUserStore) GetByUsername(ctx context.Context, username string) (*database.User, error) {
	query := `SELECT id, username, password_hash, role, display_name, email, enabled, tenant_id, created_at, updated_at FROM users WHERE username = ?`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var createdAtStr, updatedAtStr string
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Password, &role,
		&user.DisplayName, &user.Email, &user.Enabled, &tenantID, &createdAtStr, &updatedAtStr,
	)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.Role = database.UserRole(role)
	user.TenantID = scanSQLiteUserTenantID(tenantID)
	if createdAtStr != "" {
		user.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAtStr, time.Local)
	}
	if updatedAtStr != "" {
		user.UpdatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", updatedAtStr, time.Local)
	}
	return user, nil
}

func (s *sqliteUserStore) Update(ctx context.Context, user *database.User) error {
	query := `UPDATE users SET username = ?, password_hash = ?, role = ?, display_name = ?, email = ?, enabled = ?, tenant_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	var tenantID interface{}
	if user.TenantID != nil {
		tenantID = *user.TenantID
	}

	_, err := s.db.ExecContext(ctx, query,
		user.Username, user.Password, string(user.Role),
		user.DisplayName, user.Email, user.Enabled, tenantID,
		user.ID,
	)
	return err
}

func (s *sqliteUserStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *sqliteUserStore) List(ctx context.Context) ([]*database.User, error) {
	query := `SELECT id, username, password_hash, role, display_name, email, enabled, tenant_id, created_at, updated_at FROM users ORDER BY id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*database.User
	for rows.Next() {
		user := &database.User{}
		var role string
		var tenantID sql.NullString
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(
			&user.ID, &user.Username, &user.Password, &role,
			&user.DisplayName, &user.Email, &user.Enabled, &tenantID, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, err
		}
		user.Role = database.UserRole(role)
		user.TenantID = scanSQLiteUserTenantID(tenantID)
		if createdAtStr != "" {
			user.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAtStr, time.Local)
		}
		if updatedAtStr != "" {
			user.UpdatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", updatedAtStr, time.Local)
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (s *sqliteUserStore) Count(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// ── API Key Store ───────────────────────────────────────────────────────────

type sqliteAPIKeyStore struct {
	db *sql.DB
}

func (s *sqliteAPIKeyStore) Create(ctx context.Context, key *database.APIKey) error {
	query := `INSERT INTO api_keys (user_id, tenant_id, name, key_hash, key_prefix, key_secret_enc, expires_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var expiresAt *time.Time
	if key.ExpiresAt != nil {
		expiresAt = key.ExpiresAt
	}
	var tenantID interface{}
	if key.TenantID != nil {
		tenantID = *key.TenantID
	}

	result, err := s.db.ExecContext(ctx, query,
		key.UserID, tenantID, key.Name, key.KeyHash, key.KeyPrefix,
		key.KeySecretEnc, expiresAt, key.Enabled,
		key.BudgetUSD, key.UsedUSD, key.RateLimitRPM, key.RateLimitTPM, key.ModelWhitelist,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	key.ID = id
	key.CreatedAt = time.Now().UTC()
	return nil
}

func (s *sqliteAPIKeyStore) GetByHash(ctx context.Context, keyHash string) (*database.APIKey, error) {
	query := `SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at, tenant_id FROM api_keys WHERE key_hash = ? AND enabled = 1 AND (expires_at IS NULL OR expires_at > datetime('now'))`

	key := &database.APIKey{}
	var expiresAt, lastUsedAt, createdAt sql.NullString
	var secretEnc sql.NullString
	var tenantID sql.NullString
	err := s.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
		&secretEnc,
		&expiresAt, &lastUsedAt, &key.Enabled,
		&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
		&createdAt,
		&tenantID,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if tenantID.Valid {
		key.TenantID = &tenantID.String
	}

	if secretEnc.Valid {
		key.KeySecretEnc = secretEnc.String
	}
	if expiresAt.Valid && expiresAt.String != "" {
		t, _ := time.ParseInLocation("2006-01-02 15:04:05", expiresAt.String, time.Local)
		key.ExpiresAt = &t
	}
	if lastUsedAt.Valid && lastUsedAt.String != "" {
		t, _ := time.ParseInLocation("2006-01-02 15:04:05", lastUsedAt.String, time.Local)
		key.LastUsedAt = &t
	}
	if createdAt.Valid && createdAt.String != "" {
		key.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAt.String, time.Local)
	}

	return key, nil
}

func (s *sqliteAPIKeyStore) GetByID(ctx context.Context, id int64) (*database.APIKey, error) {
	query := `SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at, tenant_id FROM api_keys WHERE id = ?`

	key := &database.APIKey{}
	var expiresAt, lastUsedAt, createdAt sql.NullString
	var secretEnc sql.NullString
	var tenantID sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
		&secretEnc,
		&expiresAt, &lastUsedAt, &key.Enabled,
		&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
		&createdAt,
		&tenantID,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if tenantID.Valid {
		key.TenantID = &tenantID.String
	}

	if secretEnc.Valid {
		key.KeySecretEnc = secretEnc.String
	}
	if expiresAt.Valid && expiresAt.String != "" {
		t, _ := time.ParseInLocation("2006-01-02 15:04:05", expiresAt.String, time.Local)
		key.ExpiresAt = &t
	}
	if lastUsedAt.Valid && lastUsedAt.String != "" {
		t, _ := time.ParseInLocation("2006-01-02 15:04:05", lastUsedAt.String, time.Local)
		key.LastUsedAt = &t
	}
	if createdAt.Valid && createdAt.String != "" {
		key.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAt.String, time.Local)
	}

	return key, nil
}

func (s *sqliteAPIKeyStore) ListByUserID(ctx context.Context, userID int64) ([]*database.APIKey, error) {
	query := `SELECT id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at, tenant_id FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*database.APIKey
	for rows.Next() {
		key := &database.APIKey{}
		var expiresAt, lastUsedAt, createdAt sql.NullString
		var secretEnc sql.NullString
		var tenantID sql.NullString
		if err := rows.Scan(
			&key.ID, &key.Name, &key.KeyPrefix,
			&secretEnc,
			&expiresAt, &lastUsedAt, &key.Enabled,
			&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
			&createdAt,
			&tenantID,
		); err != nil {
			return nil, err
		}

		key.UserID = userID
		if tenantID.Valid {
			key.TenantID = &tenantID.String
		}
		if secretEnc.Valid {
			key.KeySecretEnc = secretEnc.String
		}
		if expiresAt.Valid && expiresAt.String != "" {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05", expiresAt.String, time.Local)
			key.ExpiresAt = &t
		}
		if lastUsedAt.Valid && lastUsedAt.String != "" {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05", lastUsedAt.String, time.Local)
			key.LastUsedAt = &t
		}
		if createdAt.Valid && createdAt.String != "" {
			key.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAt.String, time.Local)
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

func (s *sqliteAPIKeyStore) Update(ctx context.Context, key *database.APIKey) error {
	query := `UPDATE api_keys SET name = ?, enabled = ?, expires_at = ?, tenant_id = ?, budget_usd = ?, rate_limit_rpm = ?, rate_limit_tpm = ?, model_whitelist = ? WHERE id = ?`

	var expiresAt *time.Time
	if key.ExpiresAt != nil {
		expiresAt = key.ExpiresAt
	}
	var tenantID interface{}
	if key.TenantID != nil {
		tenantID = *key.TenantID
	}

	_, err := s.db.ExecContext(ctx, query,
		key.Name, key.Enabled, expiresAt, tenantID,
		key.BudgetUSD, key.RateLimitRPM, key.RateLimitTPM, key.ModelWhitelist,
		key.ID,
	)
	return err
}

func (s *sqliteAPIKeyStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM api_keys WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *sqliteAPIKeyStore) UpdateLastUsed(ctx context.Context, id int64, t time.Time) error {
	query := `UPDATE api_keys SET last_used_at = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, t, id)
	return err
}

func (s *sqliteAPIKeyStore) UpdateUsedUSD(ctx context.Context, id int64, usedUSD float64) error {
	query := `UPDATE api_keys SET used_usd = used_usd + ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, usedUSD, id)
	return err
}

func (s *sqliteAPIKeyStore) ListAll(ctx context.Context, offset, limit int) ([]*database.APIKey, int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at FROM api_keys ORDER BY id LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var keys []*database.APIKey
	for rows.Next() {
		key := &database.APIKey{}
		var expiresAt, lastUsedAt, createdAt sql.NullString
		var secretEnc sql.NullString
		if err := rows.Scan(
			&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
			&secretEnc,
			&expiresAt, &lastUsedAt, &key.Enabled,
			&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
			&createdAt,
		); err != nil {
			return nil, 0, err
		}

		if secretEnc.Valid {
			key.KeySecretEnc = secretEnc.String
		}
		if expiresAt.Valid && expiresAt.String != "" {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05", expiresAt.String, time.Local)
			key.ExpiresAt = &t
		}
		if lastUsedAt.Valid && lastUsedAt.String != "" {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05", lastUsedAt.String, time.Local)
			key.LastUsedAt = &t
		}
		if createdAt.Valid && createdAt.String != "" {
			key.CreatedAt, _ = time.ParseInLocation("2006-01-02 15:04:05", createdAt.String, time.Local)
		}

		keys = append(keys, key)
	}

	return keys, total, rows.Err()
}

// ── Refresh Token Store ───────────────────────────────────────────────────

type sqliteRefreshTokenStore struct {
	db *sql.DB
}

func (s *sqliteRefreshTokenStore) Create(ctx context.Context, token *database.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES (?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		token.UserID, token.TokenHash, token.ExpiresAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	token.ID = id
	token.CreatedAt = time.Now().UTC()
	return nil
}

func (s *sqliteRefreshTokenStore) GetByHash(ctx context.Context, hash string) (*database.RefreshToken, error) {
	query := `SELECT id, user_id, expires_at, created_at, revoked FROM refresh_tokens WHERE token_hash = ? AND revoked = 0 AND expires_at > datetime('now')`

	token := &database.RefreshToken{}
	var expiresAt, createdAt sql.NullString
	err := s.db.QueryRowContext(ctx, query, hash).Scan(
		&token.ID, &token.UserID, &expiresAt, &createdAt, &token.Revoked,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if expiresAt.Valid {
		if t, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
			token.ExpiresAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", expiresAt.String); err == nil {
			token.ExpiresAt = t
		}
	}
	if createdAt.Valid {
		if t, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
			token.CreatedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
			token.CreatedAt = t
		}
	}
	return token, err
}

func (s *sqliteRefreshTokenStore) Revoke(ctx context.Context, hash string) error {
	query := `UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?`
	_, err := s.db.ExecContext(ctx, query, hash)
	return err
}

func (s *sqliteRefreshTokenStore) RevokeAllForUser(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

func (s *sqliteRefreshTokenStore) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < datetime('now')`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// ── System Config Store ───────────────────────────────────────────────────

type sqliteSystemConfigStore struct {
	db *sql.DB
}

func (s *sqliteSystemConfigStore) Get(ctx context.Context, key string) (string, error) {
	query := `SELECT config_value FROM system_config WHERE config_key = ?`

	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", database.ErrNotFound
	}
	return value, err
}

func (s *sqliteSystemConfigStore) Set(ctx context.Context, key string, value string) error {
	jsonValue, err := ensureJSONString(value)
	if err != nil {
		return err
	}

	query := `INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)`

	_, err = s.db.ExecContext(ctx, query, key, jsonValue)
	return err
}

func (s *sqliteSystemConfigStore) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM system_config WHERE config_key = ?`
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

func (s *sqliteSystemConfigStore) List(ctx context.Context) (map[string]string, error) {
	query := `SELECT config_key, config_value FROM system_config ORDER BY config_key`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}

	return result, rows.Err()
}

func ensureJSONString(value string) (string, error) {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(value), &js); err == nil {
		return value, nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ── User Config Store ───────────────────────────────────────────────────

type sqliteUserConfigStore struct {
	db *sql.DB
}

func (s *sqliteUserConfigStore) Get(ctx context.Context, userID int64) (*database.UserConfig, error) {
	query := `SELECT id, user_id, backends, proxy_settings, cache_settings, embedding, qa_split, preset_modes, scheduling, cache_control, auth_settings, created_at, updated_at FROM user_config WHERE user_id = ?`

	cfg := &database.UserConfig{}
	var createdAt, updatedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&cfg.ID, &cfg.UserID, &cfg.Backends, &cfg.ProxySettings,
		&cfg.CacheSettings, &cfg.Embedding, &cfg.QASplit, &cfg.PresetModes,
		&cfg.Scheduling, &cfg.CacheControl, &cfg.AuthSettings,
		&createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return database.DefaultUserConfig(userID), nil
	}
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		cfg.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		cfg.UpdatedAt = updatedAt.Time
	}
	return cfg, nil
}

func (s *sqliteUserConfigStore) Upsert(ctx context.Context, cfg *database.UserConfig) error {
	query := `INSERT INTO user_config (user_id, backends, proxy_settings, cache_settings, embedding, qa_split, preset_modes, scheduling, cache_control, auth_settings) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET backends = excluded.backends, proxy_settings = excluded.proxy_settings, cache_settings = excluded.cache_settings, embedding = excluded.embedding, qa_split = excluded.qa_split, preset_modes = excluded.preset_modes, scheduling = excluded.scheduling, cache_control = excluded.cache_control, auth_settings = excluded.auth_settings, updated_at = CURRENT_TIMESTAMP`

	_, err := s.db.ExecContext(ctx, query,
		cfg.UserID, cfg.Backends, cfg.ProxySettings, cfg.CacheSettings,
		cfg.Embedding, cfg.QASplit, cfg.PresetModes, cfg.Scheduling,
		cfg.CacheControl, cfg.AuthSettings,
	)
	return err
}

// ── Clash Rule Store ───────────────────────────────────────────────────

type sqliteClashRuleStore struct {
	db *sql.DB
}

func (s *sqliteClashRuleStore) ListByUserID(ctx context.Context, userID int64) ([]*database.ClashRule, error) {
	query := `SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at FROM clash_rules WHERE user_id = ? ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*database.ClashRule
	for rows.Next() {
		rule := &database.ClashRule{}
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
			&rule.SubscribeToken, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			rule.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			rule.UpdatedAt = updatedAt.Time
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

func (s *sqliteClashRuleStore) GetByID(ctx context.Context, id int64) (*database.ClashRule, error) {
	query := `SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at FROM clash_rules WHERE id = ?`

	rule := &database.ClashRule{}
	var createdAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
		&rule.SubscribeToken, &createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if createdAt.Valid {
		rule.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		rule.UpdatedAt = updatedAt.Time
	}
	return rule, err
}

func (s *sqliteClashRuleStore) GetByToken(ctx context.Context, token string) (*database.ClashRule, error) {
	query := `SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at FROM clash_rules WHERE subscribe_token = ?`

	rule := &database.ClashRule{}
	var createdAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, query, token).Scan(
		&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
		&rule.SubscribeToken, &createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if createdAt.Valid {
		rule.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		rule.UpdatedAt = updatedAt.Time
	}
	return rule, err
}

func (s *sqliteClashRuleStore) Create(ctx context.Context, rule *database.ClashRule) error {
	query := `INSERT INTO clash_rules (user_id, name, rule_content, subscribe_token) VALUES (?, ?, ?, ?)`

	result, err := s.db.ExecContext(ctx, query,
		rule.UserID, rule.Name, rule.RuleContent, rule.SubscribeToken,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	rule.ID = id
	rule.CreatedAt = time.Now().UTC()
	rule.UpdatedAt = rule.CreatedAt
	return nil
}

func (s *sqliteClashRuleStore) Update(ctx context.Context, rule *database.ClashRule) error {
	query := `UPDATE clash_rules SET name = ?, rule_content = ?, subscribe_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query,
		rule.Name, rule.RuleContent, rule.SubscribeToken, rule.ID,
	)
	return err
}

func (s *sqliteClashRuleStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM clash_rules WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func init() {
	database.RegisterPlugin("sqlite", NewPlugin)
}

// ListByTenantID returns all API keys belonging to a specific tenant.
func (s *sqliteAPIKeyStore) ListByTenantID(ctx context.Context, tenantID string) ([]*database.APIKey, error) {
	query := `SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at FROM api_keys WHERE tenant_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*database.APIKey
	for rows.Next() {
		key := &database.APIKey{}
		var expiresAt, lastUsedAt sql.NullTime
		var secretEnc sql.NullString
		err := rows.Scan(
			&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
			&secretEnc,
			&expiresAt, &lastUsedAt, &key.Enabled,
			&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
			&key.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if secretEnc.Valid {
			key.KeySecretEnc = secretEnc.String
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = &lastUsedAt.Time
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
