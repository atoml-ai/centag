package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"centag/core/pkg/database"
	"centag/core/pkg/database/pgconn"
)

// plugin 实现 database.DatabasePlugin 接口
type plugin struct {
	db *sql.DB
}

// NewPlugin 创建 PostgreSQL 插件实例
func NewPlugin(config map[string]interface{}) (database.DatabasePlugin, error) {
	// 使用 pgconn.Manager 获取数据库连接
	manager := pgconn.NewManager()
	db, err := manager.GetSQLDB()
	if err != nil {
		// 检查是否是数据库不存在的错误
		if strings.Contains(err.Error(), "database") && strings.Contains(err.Error(), "does not exist") {
			fmt.Printf("database: database does not exist, attempting to create it\n")
			if createErr := createDatabase(); createErr != nil {
				return nil, fmt.Errorf("failed to create database: %w (original error: %v)", createErr, err)
			}
			// 重试连接
			manager = pgconn.NewManager()
			db, err = manager.GetSQLDB()
			if err != nil {
				return nil, err
			}
			fmt.Printf("database: successfully connected to newly created database\n")
		} else {
			return nil, err
		}
	}

	return &plugin{db: db}, nil
}

// createDatabase 尝试创建数据库
func createDatabase() error {
	// 使用 pgconn.Manager 获取配置
	manager := pgconn.NewManager()
	cfg := manager.GetConfig()

	if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
		return fmt.Errorf("PostgreSQL environment variables not fully configured (need PG_* or POSTGRES_*: host, user, db name)")
	}

	// 连接到默认的 postgres 数据库来创建新数据库
	defaultDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable connect_timeout=5",
		cfg.Host, cfg.Port, cfg.User, cfg.Password)

	db, err := sql.Open("pgx", defaultDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres default database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查数据库是否已存在
	var exists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		// 创建数据库
		// 注意：数据库名不能使用参数绑定，需要直接拼接
		createStmt := fmt.Sprintf("CREATE DATABASE \"%s\"", cfg.Database)
		_, err = db.ExecContext(ctx, createStmt)
		if err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
		fmt.Printf("database: created database %s\n", cfg.Database)
	} else {
		fmt.Printf("database: database %s already exists\n", cfg.Database)
	}

	return nil
}

func (p *plugin) Name() string {
	return "postgresql"
}

func (p *plugin) UserStore() database.UserStore {
	return &pgUserStore{db: p.db}
}

func (p *plugin) APIKeyStore() database.APIKeyStore {
	return &pgAPIKeyStore{db: p.db}
}

func (p *plugin) RefreshTokenStore() database.RefreshTokenStore {
	return &pgRefreshTokenStore{db: p.db}
}

func (p *plugin) SystemConfigStore() database.SystemConfigStore {
	return &pgSystemConfigStore{db: p.db}
}

func (p *plugin) UserConfigStore() database.UserConfigStore {
	return &pgUserConfigStore{db: p.db}
}

func (p *plugin) ClashRuleStore() database.ClashRuleStore {
	return &pgClashRuleStore{db: p.db}
}

// TenantStore returns the store for tenant management (multi-tenant).
func (p *plugin) TenantStore() database.TenantStore {
	return database.NewUnifiedTenantService(p.db, &database.PostgreSQLDialect{})
}

func (p *plugin) Migrate(ctx context.Context) error {
	_ = ctx
	migrator := database.NewMigrator(p.db, "postgresql")
	return migrator.Migrate()
}

func (p *plugin) HealthCheck(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *plugin) Close() error {
	return p.db.Close()
}

// GetDB returns the underlying sql.DB connection.
// This implements the optional DBProvider interface.
func (p *plugin) GetDB() *sql.DB {
	return p.db
}

// ── User Store ──────────────────────────────────────────────────────────────

type pgUserStore struct {
	db *sql.DB
}

func (s *pgUserStore) Create(ctx context.Context, user *database.User) error {
	query := `
		INSERT INTO users (
			username, password_hash, role, display_name, email, enabled,
			default_pipeline_id, daily_token_limit, monthly_token_limit,
			allowed_backend_ids, allowed_model_ids, allowed_pipeline_ids,
			can_add_own_backends, can_add_own_pipelines, can_change_default_pipeline
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`

	return s.db.QueryRowContext(ctx, query,
		user.Username, user.Password, user.Role,
		user.DisplayName, user.Email, user.Enabled,
		user.DefaultPipelineID, user.DailyTokenLimit, user.MonthlyTokenLimit,
		encodeUserIDs(user.AllowedBackendIDs), encodeUserIDs(user.AllowedModelIDs), encodeUserIDs(user.AllowedPipelineIDs),
		user.CanAddOwnBackends, user.CanAddOwnPipelines, user.CanChangeDefaultPipeline,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func scanPGUserTenantID(tenantID sql.NullString) *string {
	if tenantID.Valid && tenantID.String != "" {
		v := tenantID.String
		return &v
	}
	return nil
}

const pgUserSelectCols = `id, username, password_hash, role, display_name, email, enabled, tenant_id,
	COALESCE(default_pipeline_id, ''), COALESCE(daily_token_limit, 0), COALESCE(monthly_token_limit, 0),
	COALESCE(daily_token_used, 0), COALESCE(monthly_token_used, 0),
	COALESCE(allowed_backend_ids, '[]'), COALESCE(allowed_model_ids, '[]'), COALESCE(allowed_pipeline_ids, '[]'),
	COALESCE(can_add_own_backends, TRUE), COALESCE(can_add_own_pipelines, TRUE), COALESCE(can_change_default_pipeline, TRUE),
	created_at, updated_at`

func scanPGUserRow(user *database.User, role *string, tenantID *sql.NullString,
	backendsJSON, modelsJSON, pipelinesJSON *string,
	canAddBackends, canAddPipelines, canChangeDefault *bool,
) []any {
	return []any{
		&user.ID, &user.Username, &user.Password, role,
		&user.DisplayName, &user.Email, &user.Enabled, tenantID,
		&user.DefaultPipelineID, &user.DailyTokenLimit, &user.MonthlyTokenLimit,
		&user.DailyTokenUsed, &user.MonthlyTokenUsed,
		backendsJSON, modelsJSON, pipelinesJSON,
		canAddBackends, canAddPipelines, canChangeDefault,
		&user.CreatedAt, &user.UpdatedAt,
	}
}

func finishPGUser(user *database.User, role string, tenantID sql.NullString,
	backendsJSON, modelsJSON, pipelinesJSON string,
	canAddBackends, canAddPipelines, canChangeDefault bool,
) {
	user.Role = database.UserRole(role)
	user.TenantID = scanPGUserTenantID(tenantID)
	user.AllowedBackendIDs = decodeUserIDs(backendsJSON)
	user.AllowedModelIDs = decodeUserIDs(modelsJSON)
	user.AllowedPipelineIDs = decodeUserIDs(pipelinesJSON)
	user.CanAddOwnBackends = canAddBackends
	user.CanAddOwnPipelines = canAddPipelines
	user.CanChangeDefaultPipeline = canChangeDefault
}

func (s *pgUserStore) GetByID(ctx context.Context, id int64) (*database.User, error) {
	query := `SELECT ` + pgUserSelectCols + ` FROM users WHERE id = $1`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var backendsJSON, modelsJSON, pipelinesJSON string
	var canAddBackends, canAddPipelines, canChangeDefault bool
	err := s.db.QueryRowContext(ctx, query, id).Scan(scanPGUserRow(user, &role, &tenantID,
		&backendsJSON, &modelsJSON, &pipelinesJSON,
		&canAddBackends, &canAddPipelines, &canChangeDefault)...)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	finishPGUser(user, role, tenantID, backendsJSON, modelsJSON, pipelinesJSON,
		canAddBackends, canAddPipelines, canChangeDefault)
	return user, nil
}

func (s *pgUserStore) GetByUsername(ctx context.Context, username string) (*database.User, error) {
	query := `SELECT ` + pgUserSelectCols + ` FROM users WHERE username = $1`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var backendsJSON, modelsJSON, pipelinesJSON string
	var canAddBackends, canAddPipelines, canChangeDefault bool
	err := s.db.QueryRowContext(ctx, query, username).Scan(scanPGUserRow(user, &role, &tenantID,
		&backendsJSON, &modelsJSON, &pipelinesJSON,
		&canAddBackends, &canAddPipelines, &canChangeDefault)...)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	finishPGUser(user, role, tenantID, backendsJSON, modelsJSON, pipelinesJSON,
		canAddBackends, canAddPipelines, canChangeDefault)
	return user, nil
}

func (s *pgUserStore) Update(ctx context.Context, user *database.User) error {
	query := `
		UPDATE users SET username = $2, password_hash = $3, role = $4, display_name = $5, email = $6,
			enabled = $7, tenant_id = $8,
			default_pipeline_id = $9, daily_token_limit = $10, monthly_token_limit = $11,
			allowed_backend_ids = $12, allowed_model_ids = $13, allowed_pipeline_ids = $14,
			can_add_own_backends = $15, can_add_own_pipelines = $16, can_change_default_pipeline = $17,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	var tenantID sql.NullString
	if user.TenantID != nil {
		tenantID = sql.NullString{String: *user.TenantID, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, query,
		user.ID, user.Username, user.Password, string(user.Role),
		user.DisplayName, user.Email, user.Enabled, tenantID,
		user.DefaultPipelineID, user.DailyTokenLimit, user.MonthlyTokenLimit,
		encodeUserIDs(user.AllowedBackendIDs), encodeUserIDs(user.AllowedModelIDs), encodeUserIDs(user.AllowedPipelineIDs),
		user.CanAddOwnBackends, user.CanAddOwnPipelines, user.CanChangeDefaultPipeline,
	)
	return err
}

func (s *pgUserStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *pgUserStore) List(ctx context.Context) ([]*database.User, error) {
	query := `SELECT ` + pgUserSelectCols + ` FROM users ORDER BY id`

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
		var backendsJSON, modelsJSON, pipelinesJSON string
		var canAddBackends, canAddPipelines, canChangeDefault bool
		if err := rows.Scan(scanPGUserRow(user, &role, &tenantID,
			&backendsJSON, &modelsJSON, &pipelinesJSON,
			&canAddBackends, &canAddPipelines, &canChangeDefault)...); err != nil {
			return nil, err
		}
		finishPGUser(user, role, tenantID, backendsJSON, modelsJSON, pipelinesJSON,
			canAddBackends, canAddPipelines, canChangeDefault)
		users = append(users, user)
	}

	return users, rows.Err()
}

func encodeUserIDs(ids []string) string {
	if ids == nil {
		ids = []string{}
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeUserIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (s *pgUserStore) Count(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// ── API Key Store ───────────────────────────────────────────────────────────

type pgAPIKeyStore struct {
	db *sql.DB
}

func (s *pgAPIKeyStore) Create(ctx context.Context, key *database.APIKey) error {
	query := `
		INSERT INTO api_keys (user_id, tenant_id, name, key_hash, key_prefix, key_secret_enc, expires_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`

	var expiresAt sql.NullTime
	if key.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *key.ExpiresAt, Valid: true}
	}
	var secretEnc sql.NullString
	if key.KeySecretEnc != "" {
		secretEnc = sql.NullString{String: key.KeySecretEnc, Valid: true}
	}
	var tenantID sql.NullString
	if key.TenantID != nil {
		tenantID = sql.NullString{String: *key.TenantID, Valid: true}
	}

	return s.db.QueryRowContext(ctx, query,
		key.UserID, tenantID, key.Name, key.KeyHash, key.KeyPrefix,
		secretEnc, expiresAt, key.Enabled,
		key.BudgetUSD, key.UsedUSD, key.RateLimitRPM, key.RateLimitTPM, key.ModelWhitelist,
	).Scan(&key.ID, &key.CreatedAt)
}

func (s *pgAPIKeyStore) GetByHash(ctx context.Context, keyHash string) (*database.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at
		FROM api_keys
		WHERE key_hash = $1 AND enabled = true
		  AND (expires_at IS NULL OR expires_at > NOW())
	`

	key := &database.APIKey{}
	var expiresAt, lastUsedAt sql.NullTime
	var secretEnc sql.NullString
	err := s.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
		&secretEnc,
		&expiresAt, &lastUsedAt, &key.Enabled,
		&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
		&key.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
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

	return key, nil
}

func (s *pgAPIKeyStore) GetByID(ctx context.Context, id int64) (*database.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at
		FROM api_keys WHERE id = $1
	`

	key := &database.APIKey{}
	var expiresAt, lastUsedAt sql.NullTime
	var secretEnc sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
		&secretEnc,
		&expiresAt, &lastUsedAt, &key.Enabled,
		&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
		&key.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
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

	return key, nil
}

func (s *pgAPIKeyStore) ListByUserID(ctx context.Context, userID int64) ([]*database.APIKey, error) {
	query := `
		SELECT id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at
		FROM api_keys WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*database.APIKey
	for rows.Next() {
		key := &database.APIKey{}
		var expiresAt, lastUsedAt sql.NullTime
		var secretEnc sql.NullString
		if err := rows.Scan(
			&key.ID, &key.Name, &key.KeyPrefix,
			&secretEnc,
			&expiresAt, &lastUsedAt, &key.Enabled,
			&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
			&key.CreatedAt,
		); err != nil {
			return nil, err
		}

		key.UserID = userID
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

func (s *pgAPIKeyStore) Update(ctx context.Context, key *database.APIKey) error {
	query := `
		UPDATE api_keys SET
			name = $2, enabled = $3, expires_at = $4,
			budget_usd = $5, rate_limit_rpm = $6, rate_limit_tpm = $7, model_whitelist = $8
		WHERE id = $1
	`

	var expiresAt sql.NullTime
	if key.ExpiresAt != nil {
		expiresAt = sql.NullTime{Time: *key.ExpiresAt, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, query,
		key.ID, key.Name, key.Enabled, expiresAt,
		key.BudgetUSD, key.RateLimitRPM, key.RateLimitTPM, key.ModelWhitelist,
	)
	return err
}

func (s *pgAPIKeyStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *pgAPIKeyStore) UpdateLastUsed(ctx context.Context, id int64, t time.Time) error {
	query := `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, t)
	return err
}

func (s *pgAPIKeyStore) UpdateUsedUSD(ctx context.Context, id int64, usedUSD float64) error {
	query := `UPDATE api_keys SET used_usd = used_usd + $2 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, usedUSD)
	return err
}

func (s *pgAPIKeyStore) ListAll(ctx context.Context, offset, limit int) ([]*database.APIKey, int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at
		FROM api_keys
		ORDER BY id
		OFFSET $1 LIMIT $2
	`
	rows, err := s.db.QueryContext(ctx, query, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var keys []*database.APIKey
	for rows.Next() {
		key := &database.APIKey{}
		var expiresAt, lastUsedAt sql.NullTime
		var secretEnc sql.NullString
		if err := rows.Scan(
			&key.ID, &key.UserID, &key.Name, &key.KeyPrefix,
			&secretEnc,
			&expiresAt, &lastUsedAt, &key.Enabled,
			&key.BudgetUSD, &key.UsedUSD, &key.RateLimitRPM, &key.RateLimitTPM, &key.ModelWhitelist,
			&key.CreatedAt,
		); err != nil {
			return nil, 0, err
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

	return keys, total, rows.Err()
}

// ── Refresh Token Store ───────────────────────────────────────────────────

type pgRefreshTokenStore struct {
	db *sql.DB
}

func (s *pgRefreshTokenStore) Create(ctx context.Context, token *database.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`

	return s.db.QueryRowContext(ctx, query,
		token.UserID, token.TokenHash, token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)
}

func (s *pgRefreshTokenStore) GetByHash(ctx context.Context, hash string) (*database.RefreshToken, error) {
	query := `
		SELECT id, user_id, expires_at, created_at, revoked
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked = false AND expires_at > NOW()
	`

	token := &database.RefreshToken{}
	err := s.db.QueryRowContext(ctx, query, hash).Scan(
		&token.ID, &token.UserID, &token.ExpiresAt, &token.CreatedAt, &token.Revoked,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	return token, err
}

func (s *pgRefreshTokenStore) Revoke(ctx context.Context, hash string) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`
	_, err := s.db.ExecContext(ctx, query, hash)
	return err
}

func (s *pgRefreshTokenStore) RevokeAllForUser(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}

func (s *pgRefreshTokenStore) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW()`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

// ── system_config：列为 JSONB，须存合法 JSON；SQLite 侧为 TEXT 原样字符串。──

// ensureJSONBConfigValue 将待写入值转为可被 ::jsonb 解析的文本（已是 JSON 则保持；否则按 JSON 字符串转义）。
func ensureJSONBConfigValue(value string) (string, error) {
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

// decodeJSONBConfigBytes 将 JSONB 读出的字节还原为与 SQLite TEXT 一致的「配置字符串」（JSON 标量字符串则解包一层引号）。
func decodeJSONBConfigBytes(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	return string(raw), nil
}

// ── System Config Store ───────────────────────────────────────────────────

type pgSystemConfigStore struct {
	db *sql.DB
}

func (s *pgSystemConfigStore) Get(ctx context.Context, key string) (string, error) {
	query := `SELECT config_value FROM system_config WHERE config_key = $1`

	var raw []byte
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return "", database.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return decodeJSONBConfigBytes(raw)
}

func (s *pgSystemConfigStore) Set(ctx context.Context, key string, value string) error {
	jv, err := ensureJSONBConfigValue(value)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO system_config (config_key, config_value)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (config_key) DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query, key, jv)
	return err
}

func (s *pgSystemConfigStore) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM system_config WHERE config_key = $1`
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

func (s *pgSystemConfigStore) List(ctx context.Context) (map[string]string, error) {
	query := `SELECT config_key, config_value FROM system_config ORDER BY config_key`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		v, err := decodeJSONBConfigBytes(raw)
		if err != nil {
			return nil, err
		}
		result[key] = v
	}

	return result, rows.Err()
}

// ── User Config Store ─────────────────────────────────────────────────────

type pgUserConfigStore struct {
	db *sql.DB
}

func (s *pgUserConfigStore) Get(ctx context.Context, userID int64) (*database.UserConfig, error) {
	// UserConfig 存储在 system_config 中，key 为 user_{userID}_config
	key := fmt.Sprintf("user_%d_config", userID)
	query := `SELECT config_value FROM system_config WHERE config_key = $1`

	var raw []byte
	err := s.db.QueryRowContext(ctx, query, key).Scan(&raw)
	if err == sql.ErrNoRows {
		// 返回空配置而不是错误
		return &database.UserConfig{UserID: userID}, nil
	}
	if err != nil {
		return nil, err
	}

	value, err := decodeJSONBConfigBytes(raw)
	if err != nil {
		return nil, err
	}

	cfg := &database.UserConfig{UserID: userID}
	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s *pgUserConfigStore) Upsert(ctx context.Context, cfg *database.UserConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("user_%d_config", cfg.UserID)
	jv, err := ensureJSONBConfigValue(string(data))
	if err != nil {
		return err
	}
	query := `
		INSERT INTO system_config (config_key, config_value)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (config_key) DO UPDATE SET config_value = EXCLUDED.config_value, updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query, key, jv)
	return err
}

// ── Clash Rule Store ─────────────────────────────────────────────────────

type pgClashRuleStore struct {
	db *sql.DB
}

func (s *pgClashRuleStore) ListByUserID(ctx context.Context, userID int64) ([]*database.ClashRule, error) {
	query := `
		SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at
		FROM clash_rules
		WHERE user_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*database.ClashRule
	for rows.Next() {
		rule := &database.ClashRule{}
		if err := rows.Scan(
			&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
			&rule.SubscribeToken, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

func (s *pgClashRuleStore) GetByID(ctx context.Context, id int64) (*database.ClashRule, error) {
	query := `
		SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at
		FROM clash_rules WHERE id = $1
	`

	rule := &database.ClashRule{}
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
		&rule.SubscribeToken, &rule.CreatedAt, &rule.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	return rule, err
}

func (s *pgClashRuleStore) GetByToken(ctx context.Context, token string) (*database.ClashRule, error) {
	query := `
		SELECT id, user_id, name, rule_content, subscribe_token, created_at, updated_at
		FROM clash_rules WHERE subscribe_token = $1
	`

	rule := &database.ClashRule{}
	err := s.db.QueryRowContext(ctx, query, token).Scan(
		&rule.ID, &rule.UserID, &rule.Name, &rule.RuleContent,
		&rule.SubscribeToken, &rule.CreatedAt, &rule.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	return rule, err
}

func (s *pgClashRuleStore) Create(ctx context.Context, rule *database.ClashRule) error {
	query := `
		INSERT INTO clash_rules (user_id, name, rule_content, subscribe_token)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	return s.db.QueryRowContext(ctx, query,
		rule.UserID, rule.Name, rule.RuleContent, rule.SubscribeToken,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
}

func (s *pgClashRuleStore) Update(ctx context.Context, rule *database.ClashRule) error {
	query := `
		UPDATE clash_rules
		SET name = $2, rule_content = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, rule.ID, rule.Name, rule.RuleContent)
	return err
}

func (s *pgClashRuleStore) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM clash_rules WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// init 注册插件
func init() {
	database.RegisterPlugin("postgresql", func(config map[string]interface{}) (database.DatabasePlugin, error) {
		return NewPlugin(config)
	})
}

// ListByTenantID returns all API keys belonging to a specific tenant.
func (s *pgAPIKeyStore) ListByTenantID(ctx context.Context, tenantID string) ([]*database.APIKey, error) {
	query := `
		SELECT id, user_id, name, key_prefix, key_secret_enc, expires_at, last_used_at, enabled, budget_usd, used_usd, rate_limit_rpm, rate_limit_tpm, model_whitelist, created_at
		FROM api_keys
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
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
