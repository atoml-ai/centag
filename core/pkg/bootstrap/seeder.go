// Package bootstrap handles first-run initialisation of the database.
//
// When the system starts for the first time (i.e. the users table is empty),
// Seed() creates the default admin account and populates all system-level and
// user-level configuration with working defaults.  After the first run, Seed()
// is a no-op.
//
// 后端配置初始化优先级（按顺序）:
// 1. Profile/INITDATA_PATH 的 initial-backends.yaml|json（发行版/客户唯一种子）
// 2. 回退 ProjectRoot()/config/initdata（如自定义 --initdata zip）；无文件则空种子
//
// 环境变量占位符说明:
// - OLLAMA_HOST: 在 initial-backends 中用作占位符 {{OLLAMA_HOST|...}}，程序优先读取
// - LLM_PROXY_INIT_BACKEND_URL: 已合并到 OLLAMA_HOST，保留仅为向后兼容
// - 第三方后端 API Key：写在种子文件的 api_key 字段，或通过 WebUI 后端管理 / 下拉添加
//
// 其他可覆盖的环境变量:
//	LLM_PROXY_ADMIN_USERNAME      (default: admin)
//	LLM_PROXY_ADMIN_PASSWORD      (default: centag123)
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
// API Key secondary reveal (default on):
//
//	Startup calls auth.EnsureAPIKeyStorage — auto-generates a secret into system_config
//	when LLM_PROXY_API_KEY_STORAGE_SECRET is unset.
//	LLM_PROXY_API_KEY_REVEAL_ONCE=true   disables encryption / secondary reveal (create-time only)
//	LLM_PROXY_API_KEY_STORAGE_SECRET     optional override for AES-256 key material (SHA-256 digest)
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
	return "centag123"
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
	// 组模型（036）：不再为管理员预建租户；users.tenant_id 仅为过渡别名，
	// 管理员（全局访问）不再关联租户。
	logger.Infof("bootstrap: 已创建管理员用户 username=%q id=%d", adminUser.Username, adminUser.ID)

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
	if auth.APIKeyRevealOnce() {
		logger.Info("bootstrap: API Key 可在「个人设置」中管理；当前为仅创建时展示一次（LLM_PROXY_API_KEY_REVEAL_ONCE）")
	} else {
		logger.Info("bootstrap: API Key 可在「个人设置」中管理；完整密钥已加密落库，可在界面反复查看/复制")
	}
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
		// 未配置环境变量API key时，生成一个默认的API key
		return createDefaultAPIKey(ctx, db, user)
	}

	keyHash, keyPrefix := auth.APIKeyMetadataFromFullKey(raw)
	enc, err := auth.EncryptAPIKeyForStorage(raw)
	if err != nil {
		return fmt.Errorf("encrypt default admin api key: %w", err)
	}
	if enc == "" && auth.APIKeyRevealOnce() {
		logger.Warn("bootstrap: 已预置管理员 API Key（reveal-once 模式），界面无法二次查看完整密钥")
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

// createDefaultAPIKey creates a default API key for a user.
func createDefaultAPIKey(ctx context.Context, db *database.Manager, user *database.User) error {
	if user == nil || user.ID == 0 {
		return fmt.Errorf("invalid user")
	}

	// 确保API key存储已初始化
	if err := auth.EnsureAPIKeyStorage(ctx); err != nil {
		return fmt.Errorf("ensure api key storage: %w", err)
	}

	// 生成API key
	fullKey, keyHash, keyPrefix, err := auth.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("generate api key: %w", err)
	}

	// 加密存储
	enc, err := auth.EncryptAPIKeyForStorage(fullKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	key := &database.APIKey{
		UserID:       user.ID,
		TenantID:     user.TenantID,
		Name:         "default",
		KeyHash:      keyHash,
		KeyPrefix:    keyPrefix,
		KeySecretEnc: enc,
		Enabled:      true,
	}
	if err := db.APIKeyStore().Create(ctx, key); err != nil {
		return err
	}

	logger.Infof("bootstrap: 已为用户 %q 创建默认 API key", user.Username)
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

	// Personal/minimal版本使用普通用户角色，team版本使用管理员角色
	// 这样personal/minimal版本的用户行为与team版本普通用户一致
	role := database.RoleAdmin
	edition := strings.ToLower(strings.TrimSpace(os.Getenv("CENTAG_EDITION")))
	if edition == "personal" || edition == "minimal" {
		role = database.RoleNormal
	}

	user := &database.User{
		Username:                 username,
		Password:                 string(hash),
		Role:                     role,
		DisplayName:              "Administrator",
		Enabled:                  true,
		AllowedBackendIDs:        []string{},
		AllowedModelIDs:          []string{},
		AllowedPipelineIDs:       []string{},
		CanAddOwnBackends:        true,
		CanAddOwnPipelines:       true,
		CanChangeDefaultPipeline: true,
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
			ID:          "fixed-egress",
			Name:        "跳板模式",
			Description: "固定出站跳板：走系统默认后端/模型，不做跨后端模型匹配",
			Mode:        "fixed-egress",
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
