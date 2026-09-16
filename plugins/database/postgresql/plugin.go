package postgresql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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
		if strings.Contains(err.Error(), "does not exist") ||
			(strings.Contains(err.Error(), "SQLSTATE 3D000")) {
			fmt.Printf("database: 数据库不存在，尝试自动创建\n")
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
		} else if strings.Contains(err.Error(), "password authentication failed") ||
			strings.Contains(err.Error(), "SQLSTATE 28P01") {
			return nil, fmt.Errorf("PostgreSQL 认证失败（密码错误或角色未设置密码；请核对连接信息，fnOS 系统 PG 的 postgres 角色默认无密码）: %w", err)
		} else if strings.Contains(err.Error(), "no pg_hba.conf entry") ||
			strings.Contains(err.Error(), "SQLSTATE 28000") {
			return nil, fmt.Errorf("PostgreSQL 拒绝连接（pg_hba.conf 未放行该 user/数据库/来源；fnOS 系统 PG 默认仅放行业务库）: %w", err)
		} else {
			return nil, err
		}
	}

	return &plugin{db: db}, nil
}

// createDatabase 尝试创建数据库。
// 产品化要求：用户只需提供正确的连接信息即可开箱即用。目标库不存在时，
// 自动建库按以下候选链逐一尝试，直到任一管理连接成功：
//   1) TCP（用户提供的 host）
//   2) TCP 127.0.0.1 / ::1（host 写 localhost 时部分服务器 pg_hba 仅放行 IPv4 回环）
//   3) 本机 unix socket（探测 /var/run/postgresql 等目录下的 .s.PGSQL.<port>），
//      pg_hba 的 `local all all md5` 对密码账号普遍放行——覆盖 fnOS 等限制 TCP admin 库的环境
//
// 管理库依次尝试 postgres、template1（标准 PG 默认放行这两个库）。
// 所有路径失败时返回聚合错误，附带可操作的手工建库指引。
func createDatabase() error {
	manager := pgconn.NewManager()
	cfg := manager.GetConfig()

	if cfg.Host == "" || cfg.User == "" || cfg.Database == "" {
		return fmt.Errorf("PostgreSQL 连接信息不完整（需要 host、user、数据库名）")
	}

	// 组装候选管理连接（admin 数据库名 × 传输方式）
	adminDBs := []string{"postgres", "template1"}
	type candidate struct{ label, dsn string }
	var candidates []candidate

	// 1) 用户提供的 host（TCP）
	for _, adb := range adminDBs {
		c := *cfg
		c.Database = adb
		candidates = append(candidates, candidate{
			label: fmt.Sprintf("TCP(%s:%d/%s)", cfg.Host, cfg.Port, adb),
			dsn:   c.DSN(),
		})
	}
	// 2) localhost 时补 IPv4/IPv6 回环（pg_hba 可能只放行其一）
	if cfg.Host == "localhost" || cfg.Host == "" {
		for _, loop := range []string{"127.0.0.1", "::1"} {
			for _, adb := range adminDBs {
				c := *cfg
				c.Host = loop
				c.Database = adb
				candidates = append(candidates, candidate{
					label: fmt.Sprintf("TCP(%s:%d/%s)", loop, cfg.Port, adb),
					dsn:   c.DSN(),
				})
			}
		}
	}
	// 3) 本机 unix socket 目录（仅目标库在本机时有意义）
	if cfg.Host == "localhost" || cfg.Host == "" || cfg.Host == "127.0.0.1" || cfg.Host == "::1" {
		socketDirs := []string{"/var/run/postgresql", "/run/postgresql", "/tmp"}
		for _, dir := range socketDirs {
			sock := fmt.Sprintf("%s/.s.PGSQL.%d", dir, cfg.Port)
			if _, err := os.Stat(sock); err != nil {
				continue
			}
			for _, adb := range adminDBs {
				c := *cfg
				c.Host = dir
				c.Database = adb
				candidates = append(candidates, candidate{
					label: fmt.Sprintf("socket(%s/%s)", dir, adb),
					dsn:   c.DSN(),
				})
			}
			break // 同一端口只取第一个存在的 socket 目录
		}
	}

	var report strings.Builder
	for _, cand := range candidates {
		db, err := sql.Open("pgx", cand.dsn)
		if err != nil {
			report.WriteString(fmt.Sprintf("\n  [%s] open 失败: %v", cand.label, err))
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var exists bool
		err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.Database).Scan(&exists)
		cancel()
		if err != nil {
			db.Close()
			report.WriteString(fmt.Sprintf("\n  [%s] 连接/查询失败: %v", cand.label, err))
			continue
		}
		if !exists {
			// 数据库名不能用参数绑定，需拼接；插件名已在此前校验为合法标识符
			if _, err := db.Exec("CREATE DATABASE \"" + cfg.Database + "\""); err != nil {
				db.Close()
				report.WriteString(fmt.Sprintf("\n  [%s] CREATE DATABASE 失败: %v", cand.label, err))
				continue
			}
			fmt.Printf("database: 已创建数据库 %s\n", cfg.Database)
		} else {
			fmt.Printf("database: 数据库 %s 已存在\n", cfg.Database)
		}
		db.Close()
		return nil
	}

	return fmt.Errorf("无法自动创建数据库 %s（已尝试的管理连接:%s）\n%s", cfg.Database, report.String(),
		"  处理建议：\n"+
			"  a) 先在 PostgreSQL 服务器手工建库并以管理员执行 GRANT，或在服务器为该账号授予 CREATEDB 权限：\n"+
			fmt.Sprintf("       psql -U <管理员> -c \"CREATE DATABASE \\\"%s\\\";\"\n", cfg.Database)+
			"     2) 检查服务器 pg_hba.conf 是否放行本机连往所填 user/数据库（fnOS 系统 PG 默认不放行 admin 库）\n"+
			"     3) 确认密码正确（角色密码未设置时 TCP 登录会报密码错误）")
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
			default_pipeline_id,
			can_add_own_backends, can_add_own_pipelines, can_change_default_pipeline
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	return s.db.QueryRowContext(ctx, query,
		user.Username, user.Password, user.Role,
		user.DisplayName, user.Email, user.Enabled,
		user.DefaultPipelineID,
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
	COALESCE(default_pipeline_id, ''),
	COALESCE(can_add_own_backends, TRUE), COALESCE(can_add_own_pipelines, TRUE), COALESCE(can_change_default_pipeline, TRUE),
	created_at, updated_at`

func scanPGUserRow(user *database.User, role *string, tenantID *sql.NullString,
	canAddBackends, canAddPipelines, canChangeDefault *bool,
) []any {
	return []any{
		&user.ID, &user.Username, &user.Password, role,
		&user.DisplayName, &user.Email, &user.Enabled, tenantID,
		&user.DefaultPipelineID,
		canAddBackends, canAddPipelines, canChangeDefault,
		&user.CreatedAt, &user.UpdatedAt,
	}
}

func finishPGUser(user *database.User, role string, tenantID sql.NullString,
	canAddBackends, canAddPipelines, canChangeDefault bool,
) {
	user.Role = database.UserRole(role)
	user.TenantID = scanPGUserTenantID(tenantID)
	user.CanAddOwnBackends = canAddBackends
	user.CanAddOwnPipelines = canAddPipelines
	user.CanChangeDefaultPipeline = canChangeDefault
}

func (s *pgUserStore) GetByID(ctx context.Context, id int64) (*database.User, error) {
	query := `SELECT ` + pgUserSelectCols + ` FROM users WHERE id = $1`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var canAddBackends, canAddPipelines, canChangeDefault bool
	err := s.db.QueryRowContext(ctx, query, id).Scan(scanPGUserRow(user, &role, &tenantID,
		&canAddBackends, &canAddPipelines, &canChangeDefault)...)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	finishPGUser(user, role, tenantID,
		canAddBackends, canAddPipelines, canChangeDefault)
	return user, nil
}

func (s *pgUserStore) GetByUsername(ctx context.Context, username string) (*database.User, error) {
	query := `SELECT ` + pgUserSelectCols + ` FROM users WHERE username = $1`

	user := &database.User{}
	var role string
	var tenantID sql.NullString
	var canAddBackends, canAddPipelines, canChangeDefault bool
	err := s.db.QueryRowContext(ctx, query, username).Scan(scanPGUserRow(user, &role, &tenantID,
		&canAddBackends, &canAddPipelines, &canChangeDefault)...)
	if err == sql.ErrNoRows {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	finishPGUser(user, role, tenantID,
		canAddBackends, canAddPipelines, canChangeDefault)
	return user, nil
}

func (s *pgUserStore) Update(ctx context.Context, user *database.User) error {
	query := `
		UPDATE users SET username = $2, password_hash = $3, role = $4, display_name = $5, email = $6,
			enabled = $7, tenant_id = $8,
			default_pipeline_id = $9,
			can_add_own_backends = $10, can_add_own_pipelines = $11, can_change_default_pipeline = $12,
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
		user.DefaultPipelineID,
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
		var canAddBackends, canAddPipelines, canChangeDefault bool
		if err := rows.Scan(scanPGUserRow(user, &role, &tenantID,
			&canAddBackends, &canAddPipelines, &canChangeDefault)...); err != nil {
			return nil, err
		}
		finishPGUser(user, role, tenantID,
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Preserve usage history: release token_usage rows referencing the key,
	// otherwise the FK (NO ACTION) rejects deleting a key that has usage rows.
	if _, err := tx.ExecContext(ctx, `UPDATE token_usage SET api_key_id = NULL WHERE api_key_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM api_keys WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
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
	query := `SELECT value FROM system_config WHERE key = $1`

	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", database.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *pgSystemConfigStore) Set(ctx context.Context, key string, value string) error {
	query := `
		INSERT INTO system_config (key, value, value_type, scope)
		VALUES ($1, $2, 'string', 'core')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, key, value)
	return err
}

func (s *pgSystemConfigStore) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM system_config WHERE key = $1`
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

func (s *pgSystemConfigStore) List(ctx context.Context) (map[string]string, error) {
	query := `SELECT key, value FROM system_config ORDER BY key`

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

// ── User Config Store ─────────────────────────────────────────────────────

type pgUserConfigStore struct {
	db *sql.DB
}

func (s *pgUserConfigStore) Get(ctx context.Context, userID int64) (*database.UserConfig, error) {
	// UserConfig 存储在 system_config 中，key 为 user_{userID}_config
	key := fmt.Sprintf("user_%d_config", userID)
	query := `SELECT value FROM system_config WHERE key = $1`

	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if err == sql.ErrNoRows {
		// 返回空配置而不是错误
		return &database.UserConfig{UserID: userID}, nil
	}
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
	query := `
		INSERT INTO system_config (key, value, value_type, scope)
		VALUES ($1, $2, 'json', 'core')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query, key, string(data))
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
