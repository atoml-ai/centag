package hooks

import (
	"context"
	"log"
	"sync"
	"sync/atomic"

	"centag/core/pkg/types"
)

// hookFailureStats 进程内累计的钩子失败计数（fail-open 不中断请求，仅用于可观测性）。
var hookFailureStats struct {
	tokenUsedFailures    atomic.Int64
	billingUsageFailures atomic.Int64
	quotaExceedFailures  atomic.Int64
}

// HookFailureCounts 返回各钩子的累计失败次数：OnTokenUsed / OnUsage / OnQuotaExceeded。
func HookFailureCounts() (tokenUsed, billingUsage, quotaExceeded int64) {
	return hookFailureStats.tokenUsedFailures.Load(),
		hookFailureStats.billingUsageFailures.Load(),
		hookFailureStats.quotaExceedFailures.Load()
}

// DefaultHookManager is the fail-open HookManager implementation.
// Hook errors are logged and do not fail the trigger call (proxy path stays open).
type DefaultHookManager struct {
	mu sync.RWMutex

	storage []StorageHook
	billing []BillingHook
	token   []TokenHook
	logging []LoggingHook
}

// NewManager creates an empty DefaultHookManager.
func NewManager() *DefaultHookManager {
	return &DefaultHookManager{}
}

// RegisterStorageHook registers a storage hook (order preserved).
func (m *DefaultHookManager) RegisterStorageHook(hook StorageHook) {
	if m == nil || hook == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storage = append(m.storage, hook)
}

// RegisterBillingHook registers a billing hook (order preserved).
func (m *DefaultHookManager) RegisterBillingHook(hook BillingHook) {
	if m == nil || hook == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.billing = append(m.billing, hook)
}

// RegisterTokenHook registers a token metering hook (order preserved).
func (m *DefaultHookManager) RegisterTokenHook(hook TokenHook) {
	if m == nil || hook == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = append(m.token, hook)
}

// RegisterLoggingHook registers a logging hook (order preserved).
func (m *DefaultHookManager) RegisterLoggingHook(hook LoggingHook) {
	if m == nil || hook == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logging = append(m.logging, hook)
}

// TriggerRequestHooks runs StorageHook.OnRequest for all registered hooks.
func (m *DefaultHookManager) TriggerRequestHooks(ctx context.Context, req *types.UnifiedRequest) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	hooks := append([]StorageHook(nil), m.storage...)
	m.mu.RUnlock()
	for i, h := range hooks {
		if err := h.OnRequest(ctx, req); err != nil {
			log.Printf("[hooks] OnRequest hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// TriggerResponseHooks runs StorageHook.OnResponse for all registered hooks.
func (m *DefaultHookManager) TriggerResponseHooks(ctx context.Context, resp *types.UnifiedResponse) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	hooks := append([]StorageHook(nil), m.storage...)
	m.mu.RUnlock()
	for i, h := range hooks {
		if err := h.OnResponse(ctx, resp); err != nil {
			log.Printf("[hooks] OnResponse hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// TriggerCacheHitHooks runs StorageHook.OnCacheHit for all registered hooks.
func (m *DefaultHookManager) TriggerCacheHitHooks(ctx context.Context, key string, data []byte) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	hooks := append([]StorageHook(nil), m.storage...)
	m.mu.RUnlock()
	for i, h := range hooks {
		if err := h.OnCacheHit(ctx, key, data); err != nil {
			log.Printf("[hooks] OnCacheHit hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// TriggerTokenUsedHooks runs TokenHook.OnTokenUsed and BillingHook.OnUsage.
func (m *DefaultHookManager) TriggerTokenUsedHooks(ctx context.Context, usage *TokenUsage) error {
	if m == nil || usage == nil {
		return nil
	}
	m.mu.RLock()
	tokenHooks := append([]TokenHook(nil), m.token...)
	billingHooks := append([]BillingHook(nil), m.billing...)
	m.mu.RUnlock()
	for i, h := range tokenHooks {
		if err := h.OnTokenUsed(ctx, usage); err != nil {
			hookFailureStats.tokenUsedFailures.Add(1)
			log.Printf("[hooks] OnTokenUsed hook[%d] failed (fail-open): %v", i, err)
		}
	}
	for i, h := range billingHooks {
		if err := h.OnUsage(ctx, usage); err != nil {
			hookFailureStats.billingUsageFailures.Add(1)
			log.Printf("[hooks] OnUsage billing hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// TriggerQuotaExceededHooks runs BillingHook.OnQuotaExceeded for all billing hooks.
func (m *DefaultHookManager) TriggerQuotaExceededHooks(ctx context.Context, userID int64) error {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	billingHooks := append([]BillingHook(nil), m.billing...)
	m.mu.RUnlock()
	for i, h := range billingHooks {
		if err := h.OnQuotaExceeded(ctx, userID); err != nil {
			hookFailureStats.quotaExceedFailures.Add(1)
			log.Printf("[hooks] OnQuotaExceeded hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// TriggerLoggingHooks runs LoggingHook.OnRequestLog (helper beyond HookManager interface).
func (m *DefaultHookManager) TriggerLoggingHooks(ctx context.Context, logEntry *RequestLog) error {
	if m == nil || logEntry == nil {
		return nil
	}
	m.mu.RLock()
	hooks := append([]LoggingHook(nil), m.logging...)
	m.mu.RUnlock()
	for i, h := range hooks {
		if err := h.OnRequestLog(ctx, logEntry); err != nil {
			log.Printf("[hooks] OnRequestLog hook[%d] failed (fail-open): %v", i, err)
		}
	}
	return nil
}

// Counts returns registered hook counts (for tests/diagnostics).
func (m *DefaultHookManager) Counts() (storage, billing, token, logging int) {
	if m == nil {
		return 0, 0, 0, 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.storage), len(m.billing), len(m.token), len(m.logging)
}

// Ensure DefaultHookManager implements HookManager.
var _ HookManager = (*DefaultHookManager)(nil)
