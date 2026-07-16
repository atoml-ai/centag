// Package bootstrap handles first-run initialisation of the database.
//
// When the system starts for the first time (i.e. the users table is empty),
// Seed() creates the default admin account and populates all system-level and
// user-level configuration with working defaults.  After the first run, Seed()
// is a no-op.
//
// 后端配置初始化优先级（按顺序）:
// 1. config/initdata/initial-backends.json（主要配置源）
// 2. internal/config/defaults.go 硬编码回退（JSON加载失败时）
//
// 环境变量占位符说明:
// - OLLAMA_HOST: 在 initial-backends.json 中用作占位符 {{OLLAMA_HOST|...}}，程序优先读取
// - LLM_PROXY_INIT_BACKEND_URL: 已合并到 OLLAMA_HOST，保留仅为向后兼容
// - 第三方后端 API Key：写在 config/initdata/initial-backends.json 的 api_key 字段，或通过 WebUI 后端管理配置
//
// 其他可覆盖的环境变量:
//	LLM_PROXY_ADMIN_USERNAME      (default: admin)
//	LLM_PROXY_ADMIN_PASSWORD      (default: JEAofRz0WteQOsWI)
//	LLM_PROXY_DEFAULT_MODE             (default: transparent-proxy)
//	LLM_PROXY_DEFAULT_BACKEND_ID       (default: ollama-local)
//	LLM_PROXY_DEFAULT_MODEL            (default: qwen2.5:1.5b)
//	LLM_PROXY_DEFAULT_EMBEDDING_MODEL  (default: bge-m3:latest)
//
// Optional default admin API key (first-run only):
//
//	LLM_PROXY_DEFAULT_ADMIN_API_KEY           inline secret (trimmed)
//	LLM_PROXY_ADMIN_API_KEY                   generate-secrets.sh 写入；若未设 DEFAULT 则用此项首轮入库
//	LLM_PROXY_DEFAULT_ADMIN_API_KEY_FILE      path to file (first line / whole file trimmed); relative paths resolve from executable dir
//	LLM_PROXY_DEFAULT_ADMIN_API_KEY_NAME      display name for seeded key (default: Default (config)); deprecated alias LLM_PROXY_ADMIN_API_KEY_NAME still read if this is empty
//
// To allow viewing full keys again in the Web UI, set:
//
//	LLM_PROXY_API_KEY_STORAGE_SECRET   any non-empty string; used as AES-256 key material (SHA-256 digest)
package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"centag/core/internal/auth"
	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// AdminUsername 与首轮 seed 创建的管理员一致，运行期应用也应使用此值解析 admin 用户 ID。
func AdminUsername() string {
	if s := strings.TrimSpace(os.Getenv("LLM_PROXY_ADMIN_USERNAME")); s != "" {
		return s
	}
	return "admin"
}

func AdminPassword() string {
	if v := strings.TrimSpace(os.Getenv("LLM_PROXY_ADMIN_PASSWORD")); v != "" {
		return v
	}
	return "JEAofRz0WteQOsWI"
}

// Seed checks whether the database has been initialised and, if not, populates
// it with default data so the system is immediately usable after the first
// startup.
func Seed(ctx context.Context) error {
	db := database.Get()

	firstRun, err := db.IsFirstRun(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check first run: %w", err)
	}
	if !firstRun {
		n, cntErr := db.UserStore().Count(ctx)
		if cntErr != nil {
			logger.Warnf("bootstrap: 跳过首轮初始化，但统计用户数失败: %v", cntErr)
		} else {
			logger.Infof("bootstrap: 数据库已有用户（users 行数=%d），跳过首轮 seed；清空库后重启会再次执行初始化", n)
		}
		return nil
	}

	logger.Info("bootstrap: 检测到首轮启动（users 表为空），开始写入默认管理员与系统配置")

	// ── 1. Create admin user ─────────────────────────────────────────────────
	adminUser, err := createAdminUser(ctx, db)
	if err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}
	logger.Infof("bootstrap: 已创建管理员用户 username=%q id=%d", adminUser.Username, adminUser.ID)

	if _, err := database.ProvisionUserTenant(ctx, db, adminUser); err != nil {
		return fmt.Errorf("bootstrap: provision admin tenant: %w", err)
	}
	logger.Infof("bootstrap: 已为管理员创建租户 tenant_id=%q", *adminUser.TenantID)

	if err := SeedDefaultAdminAPIKeyFromConfig(ctx, db, adminUser); err != nil {
		return fmt.Errorf("bootstrap: seed default admin api key: %w", err)
	}

	// ── 2. Seed system-level config ──────────────────────────────────────────
	if err := seedSystemConfig(ctx, db); err != nil {
		return fmt.Errorf("bootstrap: seed system config: %w", err)
	}
	logger.Info("bootstrap: 系统级配置（proxy/cache/storages/admin_backends 等）已写入 system_config")

	// ── 3. Seed admin user config ────────────────────────────────────────────
	if err := seedAdminUserConfig(ctx, db, adminUser.ID); err != nil {
		return fmt.Errorf("bootstrap: seed admin user config: %w", err)
	}
	logger.Info("bootstrap: 管理员 user_configs 已写入")

	logger.Info("bootstrap: 首轮初始化完成")
	logger.Infof("bootstrap: Web 登录请使用用户名 %q 及环境变量 LLM_PROXY_ADMIN_PASSWORD（与 config/secrets/.env 一致；未设置时使用文档中的内置默认口令）", adminUser.Username)
	logger.Info("bootstrap: API Key 可在「个人设置」中管理；若已设置 LLM_PROXY_API_KEY_STORAGE_SECRET，可在界面反复查看完整密钥")
	return nil
}

// SeedDefaultAdminAPIKeyFromConfig creates one API key for the admin when
// LLM_PROXY_DEFAULT_ADMIN_API_KEY, LLM_PROXY_ADMIN_API_KEY, or LLM_PROXY_DEFAULT_ADMIN_API_KEY_FILE is set.
// Used by first-run Seed.
func SeedDefaultAdminAPIKeyFromConfig(ctx context.Context, db *database.Manager, user *database.User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("invalid user")
	}
	userID := user.ID
	raw := DefaultAdminAPIKeyString()
	if raw == "" {
		logger.Info("bootstrap: 未配置默认管理员 API Key（LLM_PROXY_DEFAULT_ADMIN_API_KEY / LLM_PROXY_ADMIN_API_KEY / _FILE），跳过预置")
		return nil
	}

	keyHash, keyPrefix := auth.APIKeyMetadataFromFullKey(raw)
	enc := ""
	if sk := auth.APIKeyStorageKey(); sk != nil {
		var err error
		enc, err = auth.EncryptAPIKeyPlaintext(raw, sk)
		if err != nil {
			return fmt.Errorf("encrypt default admin api key: %w", err)
		}
	} else {
		logger.Warn("bootstrap: 已预置管理员 API Key，但未设置 LLM_PROXY_API_KEY_STORAGE_SECRET，界面将无法解密查看完整密钥（请自行保存配置文件或环境变量中的密钥）")
	}

	name := strings.TrimSpace(os.Getenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY_NAME"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("LLM_PROXY_ADMIN_API_KEY_NAME"))
	}
	if name == "" {
		name = "Default (config)"
	}

	key := &database.APIKey{
		UserID:       userID,
		TenantID:     user.TenantID,
		Name:         name,
		KeyHash:      keyHash,
		KeyPrefix:    keyPrefix,
		KeySecretEnc: enc,
		Enabled:      true,
	}
	if err := db.APIKeyStore().Create(ctx, key); err != nil {
		return err
	}
	logger.Infof("bootstrap: 已从配置预置管理员 API Key（名称=%q）", name)
	return nil
}

func DefaultAdminAPIKeyString() string {
	if v := strings.TrimSpace(os.Getenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY")); v != "" {
		return v
	}
	// scripts/generate-secrets.sh 写入 LLM_PROXY_ADMIN_API_KEY；与 DEFAULT 二选一即可首轮入库
	if v := strings.TrimSpace(os.Getenv("LLM_PROXY_ADMIN_API_KEY")); v != "" {
		return v
	}
	p := strings.TrimSpace(os.Getenv("LLM_PROXY_DEFAULT_ADMIN_API_KEY_FILE"))
	if p == "" {
		return ""
	}
	p = resolvePathRelativeToExecutable(p)
	b, err := os.ReadFile(p)
	if err != nil {
		logger.Warnf("bootstrap: 读取 LLM_PROXY_DEFAULT_ADMIN_API_KEY_FILE %q 失败: %v", p, err)
		return ""
	}
	return strings.TrimSpace(string(b))
}

func resolvePathRelativeToExecutable(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	execPath, err := os.Executable()
	if err != nil {
		return path
	}
	return filepath.Join(filepath.Dir(execPath), path)
}

// ── admin user ───────────────────────────────────────────────────────────────

func createAdminUser(ctx context.Context, db *database.Manager) (*database.User, error) {
	username := AdminUsername()
	password := AdminPassword()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &database.User{
		Username:    username,
		Password:    string(hash),
		Role:        database.RoleAdmin,
		DisplayName: "Administrator",
		Enabled:     true,
	}
	if err := db.UserStore().Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// ── system config ─────────────────────────────────────────────────────────────

// backendsForSeed 避免 json.Marshal(nil) 写入 null，导致 LoadFromDB 反序列化后 backends 为空切片语义异常。
func backendsForSeed() []config.BackendConfig {
	b := LoadInitialBackendsFromJSON()
	if b == nil {
		return []config.BackendConfig{}
	}
	return b
}

func seedSystemConfig(ctx context.Context, db *database.Manager) error {
	entries := map[string]interface{}{
		config.KeyProxyConfig:       config.DefaultProxyConfig(),
		config.KeyCacheConfig:       config.DefaultCacheConfig(),
		config.KeyRedisConfig:       config.DefaultRedisConfig(),
		config.KeyVectorConfig:      config.DefaultVectorConfig(),
		config.KeyEmbeddingConfig:   config.GetDefaultEmbeddingConfig(),
		config.KeyQASplitConfig:     config.GetDefaultQASplitConfig(),
		config.KeyPluginsConfig:     config.DefaultPluginsConfig(),
		config.KeySystemProxyConfig: config.GetDefaultSystemProxyConfig(),
		config.KeyHostProxyConfig:   config.GetDefaultHostProxyConfig(),
		config.KeyModelMatching:     config.DefaultModelMatchingConfig(),
		config.KeyCacheControl:      config.DefaultCacheControlConfig(),
		config.KeyStorages:          config.DefaultStorages(),
		// 默认存储由用户在 UI 中配置，首次启动时不设置默认存储
		config.KeyDefaultStorage: "",
		config.KeyBackends:          backendsForSeed(),
	}

	scs := db.SystemConfigStore()
	for key, val := range entries {
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", key, err)
		}
		if err := scs.Set(ctx, key, string(data)); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

// ── admin user config ─────────────────────────────────────────────────────────

func seedAdminUserConfig(ctx context.Context, db *database.Manager, adminID int64) error {
	// Fetch backend list that was just written to system_config so the admin's
	// user_config.backends column references the same data.
	backendsJSON, _ := db.SystemConfigStore().Get(ctx, config.KeyBackends)
	if backendsJSON == "" {
		backendsJSON = "[]"
	}

	embeddingJSON, _ := marshalJSON(config.GetDefaultEmbeddingConfig())
	qaSplitJSON, _ := marshalJSON(config.GetDefaultQASplitConfig())
	proxyJSON, _ := marshalJSON(config.DefaultProxyConfig())
	cacheJSON, _ := marshalJSON(config.DefaultCacheConfig())
	cacheControlJSON, _ := marshalJSON(config.DefaultCacheControlConfig())
	schedulingJSON, _ := marshalJSON(config.DefaultModelMatchingConfig())
	presetJSON := buildDefaultPresetModes()

	cfg := &database.UserConfig{
		UserID:        adminID,
		Backends:      backendsJSON,
		ProxySettings: proxyJSON,
		CacheSettings: cacheJSON,
		Embedding:     embeddingJSON,
		QASplit:       qaSplitJSON,
		PresetModes:   presetJSON,
		Scheduling:    schedulingJSON,
		CacheControl:  cacheControlJSON,
		AuthSettings:  `{"require_api_key":false,"allow_no_auth":true}`,
	}

	return db.UserConfigStore().Upsert(ctx, cfg)
}

// buildDefaultPresetModes returns a JSON array of the three built-in preset
// modes so the admin has something to start with.
func buildDefaultPresetModes() string {
	type PresetMode struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Mode        string `json:"mode"`
	}
	modes := []PresetMode{
		{
			ID:          "smart-scheduling",
			Name:        "智能调度",
			Description: "根据模型匹配自动选择最合适的后端",
			Mode:        "smart-scheduling",
		},
		{
			ID:          "direct-backend",
			Name:        "直连后端",
			Description: "直接连接到指定的默认后端",
			Mode:        "direct-backend",
		},
		{
			ID:          "transparent-proxy",
			Name:        "透明模式",
			Description: "直连已配置后端，不注入网关 system prompt",
			Mode:        "transparent-proxy",
		},
		{
			ID:          "raw-forward",
			Name:        "原始HTTP转发",
			Description: "高级：HTTP 透传到 X-Target-URL / hostproxy",
			Mode:        "raw-forward",
		},
	}
	data, _ := json.Marshal(modes)
	return string(data)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func marshalJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}", err
	}
	return string(data), nil
}
