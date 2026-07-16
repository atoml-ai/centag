package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"centag/core/internal/auth"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Minimal API key store (file-based). When at least one key is configured
// (file or CENTAG_PROXY_API_KEY env), /v1 requires a valid Bearer key or JWT.
// With zero keys, /v1 stays open for local/dev convenience.

type minimalAPIKeyRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Hash      string `json:"hash"`
	Key       string `json:"key,omitempty"` // full key; file mode 0600, admin UI may re-copy
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type minimalAPIKeyFile struct {
	Keys []minimalAPIKeyRecord `json:"keys"`
}

type minimalAPIKeyStore struct {
	dataDir string
	mu      sync.RWMutex
}

func newMinimalAPIKeyStore(dataDir string) *minimalAPIKeyStore {
	return &minimalAPIKeyStore{dataDir: dataDir}
}

func (s *minimalAPIKeyStore) path() string {
	return filepath.Join(s.dataDir, "api-keys.json")
}

func (s *minimalAPIKeyStore) load() (minimalAPIKeyFile, error) {
	var out minimalAPIKeyFile
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (s *minimalAPIKeyStore) save(file minimalAPIKeyFile) error {
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), append(data, '\n'), 0o600)
}

func (s *minimalAPIKeyStore) list() ([]minimalAPIKeyRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	file, err := s.load()
	if err != nil {
		return nil, err
	}
	return file.Keys, nil
}

func (s *minimalAPIKeyStore) create(name string) (fullKey string, rec minimalAPIKeyRecord, err error) {
	fullKey, hash, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", rec, err
	}
	if strings.TrimSpace(name) == "" {
		name = "default"
	}
	rec = minimalAPIKeyRecord{
		ID:        randomToken()[:16],
		Name:      strings.TrimSpace(name),
		Prefix:    prefix,
		Hash:      hash,
		Key:       fullKey,
		Enabled:   true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.load()
	if err != nil {
		return "", rec, err
	}
	file.Keys = append(file.Keys, rec)
	if err := s.save(file); err != nil {
		return "", rec, err
	}
	return fullKey, rec, nil
}

func (s *minimalAPIKeyStore) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := s.load()
	if err != nil {
		return err
	}
	next := make([]minimalAPIKeyRecord, 0, len(file.Keys))
	found := false
	for _, k := range file.Keys {
		if k.ID == id {
			found = true
			continue
		}
		next = append(next, k)
	}
	if !found {
		return os.ErrNotExist
	}
	file.Keys = next
	return s.save(file)
}

func (s *minimalAPIKeyStore) envKeyConfigured() bool {
	return strings.TrimSpace(os.Getenv("CENTAG_PROXY_API_KEY")) != ""
}

func (s *minimalAPIKeyStore) authRequired() bool {
	if s.envKeyConfigured() {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	file, err := s.load()
	if err != nil {
		logger.Warnf("[MinimalAPIKey] load keys: %v", err)
		return false
	}
	for _, k := range file.Keys {
		if k.Enabled && k.Hash != "" {
			return true
		}
	}
	return false
}

func (s *minimalAPIKeyStore) validateRawKey(token string) bool {
	if token == "" {
		return false
	}
	hash := auth.SHA256Hex(token)

	if env := strings.TrimSpace(os.Getenv("CENTAG_PROXY_API_KEY")); env != "" {
		if subtleConstantTimeEqual(hash, auth.SHA256Hex(env)) || token == env {
			return true
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	file, err := s.load()
	if err != nil {
		return false
	}
	for _, k := range file.Keys {
		if !k.Enabled || k.Hash == "" {
			continue
		}
		if subtleConstantTimeEqual(hash, k.Hash) {
			return true
		}
	}
	return false
}

func subtleConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// ProxyAuthOptionalMiddleware: open when no keys; otherwise require llmproxy_* or JWT.
func (h *MinimalAuthHandler) ProxyAuthOptionalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.apiKeys == nil || !h.apiKeys.authRequired() {
			c.Next()
			return
		}

		token := auth.ExtractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "需要认证：请在 Authorization 使用 Bearer（API Key 或管理 JWT），或查询参数 token=",
			})
			return
		}

		if h.apiKeys.validateRawKey(token) {
			c.Set(auth.CtxKeyUserID, minimalAdminUserID)
			c.Set(auth.CtxKeyUsername, minimalAdminUsername)
			c.Set(auth.CtxKeyRole, "admin")
			c.Next()
			return
		}

		// Also accept admin JWT (handy for WebUI probes)
		claims, err := auth.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "无效的 API key 或 token",
			})
			return
		}
		auth.SetUserContext(c, claims)
		c.Next()
	}
}

func (h *MinimalAuthHandler) registerAPIKeyRoutes(r gin.IRoutes) {
	r.GET("/api-keys", h.ListAPIKeys)
	r.GET("/api-keys/status", h.APIKeyStatus)
	r.POST("/api-keys", h.CreateAPIKey)
	r.DELETE("/api-keys/:id", h.DeleteAPIKey)
}

func (h *MinimalAuthHandler) APIKeyStatus(c *gin.Context) {
	required := h.apiKeys != nil && h.apiKeys.authRequired()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"auth_required": required,
			"env_key_set":   h.apiKeys != nil && h.apiKeys.envKeyConfigured(),
			"note":          "未配置任何密钥时 /v1 开放访问；创建密钥或设置 CENTAG_PROXY_API_KEY 后需 Bearer 鉴权",
		},
	})
}

func (h *MinimalAuthHandler) ListAPIKeys(c *gin.Context) {
	if h.apiKeys == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
		return
	}
	keys, err := h.apiKeys.list()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(keys))
	for _, k := range keys {
		out = append(out, gin.H{
			"id":         k.ID,
			"name":       k.Name,
			"prefix":     k.Prefix,
			"masked":     auth.MaskAPIKey(k.Prefix),
			"api_key":    k.Key, // full key for admin re-copy (file is 0600)
			"enabled":    k.Enabled,
			"created_at": k.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

func (h *MinimalAuthHandler) CreateAPIKey(c *gin.Context) {
	if h.apiKeys == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "api key store unavailable"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)
	fullKey, rec, err := h.apiKeys.create(req.Name)
	if err != nil {
		logger.Errorf("[MinimalAPIKey] create: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         rec.ID,
			"name":       rec.Name,
			"prefix":     rec.Prefix,
			"masked":     auth.MaskAPIKey(rec.Prefix),
			"api_key":    fullKey,
			"created_at": rec.CreatedAt,
		},
	})
}

func (h *MinimalAuthHandler) DeleteAPIKey(c *gin.Context) {
	if h.apiKeys == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "api key store unavailable"})
		return
	}
	id := c.Param("id")
	if err := h.apiKeys.delete(id); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "密钥不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
