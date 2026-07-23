package backend

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AccountPoolSelector 账户池选择器
type AccountPoolSelector struct {
	mu             sync.RWMutex
	sessions       map[string]string    // session_key -> account_id (sticky_session 用)
	counters       map[string]int       // account_id -> request_count (round_robin/least_usage 用)
	disabledUntil  map[string]time.Time // account_id -> 禁用截止时间（429 故障转移用）
}

// NewAccountPoolSelector 创建账户池选择器
func NewAccountPoolSelector() *AccountPoolSelector {
	return &AccountPoolSelector{
		sessions:      make(map[string]string),
		counters:      make(map[string]int),
		disabledUntil: make(map[string]time.Time),
	}
}

// AccountPoolResult 账户选择结果
type AccountPoolResult struct {
	Account BackendAccount
	Key     string // 选择的 API Key
}

// SelectAccountForRequest 为请求选择账户（统一入口，流/非流共用）
// sessionKey 用于 sticky_session 策略，可从 context 或请求头提取
func (s *AccountPoolSelector) SelectAccountForRequest(
	ctx context.Context,
	pool *AccountPoolConfig,
	sessionKey string,
) (*AccountPoolResult, error) {
	if pool == nil || len(pool.Accounts) == 0 {
		return nil, fmt.Errorf("account pool is empty")
	}

	// 过滤健康且未被临时禁用的账户
	now := time.Now()
	healthyAccounts := make([]BackendAccount, 0, len(pool.Accounts))
	for _, acc := range pool.Accounts {
		if !acc.Enabled {
			continue
		}
		// 检查是否被临时禁用（429 故障转移）
		s.mu.RLock()
		disabled, exists := s.disabledUntil[acc.ID]
		s.mu.RUnlock()
		if exists && now.Before(disabled) {
			continue
		}
		// 禁用过期，清除记录
		if exists && !now.Before(disabled) {
			s.mu.Lock()
			delete(s.disabledUntil, acc.ID)
			s.mu.Unlock()
		}
		healthyAccounts = append(healthyAccounts, acc)
	}

	if len(healthyAccounts) == 0 {
		return nil, fmt.Errorf("no healthy accounts available")
	}

	// 根据策略选择
	var selected BackendAccount
	var err error

	switch strings.ToLower(pool.Strategy) {
	case "sticky_session":
		selected, err = s.selectStickySession(healthyAccounts, sessionKey)
	case "least_usage":
		selected = s.selectLeastUsage(healthyAccounts)
	case "round_robin", "":
		selected = s.selectRoundRobin(healthyAccounts)
	default:
		selected = s.selectRoundRobin(healthyAccounts)
	}

	if err != nil {
		return nil, err
	}

	// 更新计数
	s.mu.Lock()
	s.counters[selected.ID]++
	s.mu.Unlock()

	return &AccountPoolResult{
		Account: selected,
		Key:     selected.APIKey,
	}, nil
}

// selectRoundRobin 轮询选择
func (s *AccountPoolSelector) selectRoundRobin(accounts []BackendAccount) BackendAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(accounts) == 0 {
		return BackendAccount{}
	}

	// 找到请求次数最少的账户
	minCount := -1
	selected := accounts[0]

	for _, acc := range accounts {
		count := s.counters[acc.ID]
		if minCount == -1 || count < minCount {
			minCount = count
			selected = acc
		}
	}

	return selected
}

// selectLeastUsage 最少使用选择
func (s *AccountPoolSelector) selectLeastUsage(accounts []BackendAccount) BackendAccount {
	// least_usage 与 round_robin 在简单计数模式下行为相同
	// 区别在于 least_usage 可以考虑窗口内请求量
	return s.selectRoundRobin(accounts)
}

// selectStickySession 会话亲和选择
func (s *AccountPoolSelector) selectStickySession(accounts []BackendAccount, sessionKey string) (BackendAccount, error) {
	if sessionKey == "" {
		// 无 session key，退化为 least_usage
		return s.selectLeastUsage(accounts), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有绑定
	if boundID, exists := s.sessions[sessionKey]; exists {
		// 检查绑定的账户是否仍然健康
		for _, acc := range accounts {
			if acc.ID == boundID {
				return acc, nil
			}
		}
		// 绑定的账户不健康，清除绑定
		delete(s.sessions, sessionKey)
	}

	// 新绑定：选择请求次数最少的健康账户
	minCount := -1
	var selected BackendAccount

	for _, acc := range accounts {
		count := s.counters[acc.ID]
		if minCount == -1 || count < minCount {
			minCount = count
			selected = acc
		}
	}

	// 绑定 session
	s.sessions[sessionKey] = selected.ID

	return selected, nil
}

// ExtractSessionKey 从请求上下文或 body 提取 session key
// 优先级：Header X-Session-ID > OpenAI user > Anthropic metadata.user_id
func ExtractSessionKey(ctx context.Context, body []byte, headerSessionID string) string {
	// 1. Header X-Session-ID
	if headerSessionID != "" {
		return headerSessionID
	}

	// 2. OpenAI user 字段
	if body != nil {
		var req struct {
			User     string `json:"user"`
			Metadata struct {
				UserID string `json:"user_id"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(body, &req); err == nil {
			if req.User != "" {
				return "openai:" + req.User
			}
			if req.Metadata.UserID != "" {
				return "anthropic:" + req.Metadata.UserID
			}
		}
	}

	// 3. 无 key
	return ""
}

// HashSessionKey 对 session key 进行哈希（用于 stable 绑定）
func HashSessionKey(key string) string {
	if key == "" {
		return ""
	}
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:8]) // 取前 8 字节
}

// ValidateAccountPool 验证账户池配置
func ValidateAccountPool(pool *AccountPoolConfig) error {
	if pool == nil {
		return nil
	}

	// 验证策略
	switch strings.ToLower(pool.Strategy) {
	case "round_robin", "least_usage", "sticky_session", "":
		// 有效策略
	default:
		return fmt.Errorf("invalid strategy: %s", pool.Strategy)
	}

	// 验证账户
	if len(pool.Accounts) == 0 {
		return fmt.Errorf("account pool has no accounts")
	}

	ids := make(map[string]bool)
	for _, acc := range pool.Accounts {
		if acc.ID == "" {
			return fmt.Errorf("account id is required")
		}
		if ids[acc.ID] {
			return fmt.Errorf("duplicate account id: %s", acc.ID)
		}
		ids[acc.ID] = true

		if acc.APIKey == "" {
			return fmt.Errorf("account %s: api_key is required", acc.ID)
		}

		if acc.Weight < 0 {
			return fmt.Errorf("account %s: weight must be non-negative", acc.ID)
		}
	}

	return nil
}

// NormalizeAccountPool 规范化账户池配置
func NormalizeAccountPool(pool *AccountPoolConfig) {
	if pool == nil {
		return
	}

	// 默认策略
	if pool.Strategy == "" {
		pool.Strategy = "round_robin"
	}

	// 规范化账户
	for i := range pool.Accounts {
		if pool.Accounts[i].Weight <= 0 {
			pool.Accounts[i].Weight = 1
		}
		if pool.Accounts[i].CreatedAt == "" {
			pool.Accounts[i].CreatedAt = time.Now().Format(time.RFC3339)
		}
	}
}

// GetAccountByID 通过 ID 获取账户
func GetAccountByID(pool *AccountPoolConfig, accountID string) (*BackendAccount, error) {
	if pool == nil {
		return nil, fmt.Errorf("account pool is nil")
	}

	for i := range pool.Accounts {
		if pool.Accounts[i].ID == accountID {
			return &pool.Accounts[i], nil
		}
	}

	return nil, fmt.Errorf("account not found: %s", accountID)
}

// AddAccount 添加账户到池
func AddAccount(pool *AccountPoolConfig, account BackendAccount) error {
	if pool == nil {
		return fmt.Errorf("account pool is nil")
	}

	// 检查 ID 唯一性
	for _, acc := range pool.Accounts {
		if acc.ID == account.ID {
			return fmt.Errorf("duplicate account id: %s", account.ID)
		}
	}

	// 设置默认值
	if account.Weight <= 0 {
		account.Weight = 1
	}
	if account.CreatedAt == "" {
		account.CreatedAt = time.Now().Format(time.RFC3339)
	}

	pool.Accounts = append(pool.Accounts, account)
	return nil
}

// UpdateAccount 更新池中的账户
func UpdateAccount(pool *AccountPoolConfig, account BackendAccount) error {
	if pool == nil {
		return fmt.Errorf("account pool is nil")
	}

	for i := range pool.Accounts {
		if pool.Accounts[i].ID == account.ID {
			// 保留创建时间
			if account.CreatedAt == "" {
				account.CreatedAt = pool.Accounts[i].CreatedAt
			}
			pool.Accounts[i] = account
			return nil
		}
	}

	return fmt.Errorf("account not found: %s", account.ID)
}

// RemoveAccount 从池中删除账户
func RemoveAccount(pool *AccountPoolConfig, accountID string) error {
	if pool == nil {
		return fmt.Errorf("account pool is nil")
	}

	for i := range pool.Accounts {
		if pool.Accounts[i].ID == accountID {
			pool.Accounts = append(pool.Accounts[:i], pool.Accounts[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("account not found: %s", accountID)
}

// HasAccountPool 检查后端是否配置了账户池
func HasAccountPool(cfg *BackendConfig) bool {
	return cfg != nil && cfg.AccountPool != nil && len(cfg.AccountPool.Accounts) > 0
}

// GetEffectiveAPIKey 获取有效的 API Key（账户池优先，否则使用单 Key）
func GetEffectiveAPIKey(cfg *BackendConfig) string {
	if HasAccountPool(cfg) {
		// 返回第一个健康账户的 Key（用于简单场景）
		for _, acc := range cfg.AccountPool.Accounts {
			if acc.Enabled {
				return acc.APIKey
			}
		}
	}
	return cfg.APIKey
}

// DisableAccountTemporarily 临时禁用账户（429 故障转移），默认禁用 30 秒
func (s *AccountPoolSelector) DisableAccountTemporarily(pool *AccountPoolConfig, accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disabledUntil[accountID] = time.Now().Add(30 * time.Second)
}

// IsAccountDisabled 检查账户是否被临时禁用
func (s *AccountPoolSelector) IsAccountDisabled(accountID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	disabled, exists := s.disabledUntil[accountID]
	if !exists {
		return false
	}
	return time.Now().Before(disabled)
}
