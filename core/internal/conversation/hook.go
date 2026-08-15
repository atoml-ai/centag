package conversation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"centag/core/pkg/types"
)

const (
	metaSessionID   = "session_id"
	metaUserID      = "user_id"
	metaTenantID    = "tenant_id"
	metaCategory    = "category"
	metaPipelineID  = "pipeline_id"
	metaProxyMode   = "proxy_mode"
	metaRequestID   = "request_id"
	metaUserContent = "user_content"
	metaStatusCode  = "status_code"
	metaBackend     = "backend"
	metaLatencyMs   = "latency_ms"
	metaInputTok    = "input_tokens"
	metaOutputTok   = "output_tokens"

	// dedupWindow 同一 session 内相同 user_content 的整轮去重时间窗口。
	// 仅用于"客户端短期重试"：同内容且同回复（assistant 相同）在窗口内视为重放，整轮跳过。
	// 同内容但回复不同（如 agent 多轮推理的每轮追问同一任务）不属于重试：
	// user 消息只记录一次，assistant 每轮均保留，用于展示各轮次差异。
	dedupWindow = 30 * time.Second
)

// LoggingHook records conversations via StorageHook interface (fail-open async writes).
type LoggingHook struct {
	store Store

	mu      sync.Mutex
	pending map[string]pendingTurn // key: session_id|request_id
	recent  map[string]recentUser  // sessionID -> last recorded user turn for dedup
}

type pendingTurn struct {
	sessionID   string
	userID      int64
	tenantID    string
	category    string
	pipelineID  string
	proxyMode   string
	requestID   string
	userContent string
	isAuxiliary bool // skip recording (client-internal title gen / summarisation etc.)
}

type recentUser struct {
	content   string
	at        time.Time
	assistant string // 上一次记录的 assistant 回复（用于识别"同内容同回复"的重试）
}

// NewLoggingHook creates a conversation logging hook.
func NewLoggingHook(store Store) *LoggingHook {
	return &LoggingHook{
		store:   store,
		pending: make(map[string]pendingTurn),
		recent:  make(map[string]recentUser),
	}
}

func (h *LoggingHook) pendingKey(sessionID, requestID string) string {
	if requestID != "" {
		return sessionID + "|" + requestID
	}
	return sessionID
}

// OnRequest ensures session and buffers the user turn.
func (h *LoggingHook) OnRequest(ctx context.Context, req *types.UnifiedRequest) error {
	if h == nil || h.store == nil || req == nil {
		return nil
	}
	meta := req.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
		req.Metadata = meta
	}
	sessionID := metaString(meta, metaSessionID)
	userContent := metaString(meta, metaUserContent)
	if userContent == "" {
		userContent = lastUserContent(req.Messages)
	}
	category := metaString(meta, metaCategory)
	if category == "" {
		category = "general"
	}
	userID := metaInt64(meta, metaUserID)
	tenantID := metaString(meta, metaTenantID)
	pipelineID := metaString(meta, metaPipelineID)
	proxyMode := metaString(meta, metaProxyMode)
	requestID := metaString(meta, metaRequestID)

	sess, err := h.store.EnsureSession(ctx, &Session{
		ID:         sessionID,
		UserID:     userID,
		TenantID:   tenantID,
		Category:   category,
		PipelineID: pipelineID,
		ProxyMode:  proxyMode,
	})
	if err != nil {
		return fmt.Errorf("ensure session: %w", err)
	}
	meta[metaSessionID] = sess.ID

	h.mu.Lock()
	h.pending[h.pendingKey(sess.ID, requestID)] = pendingTurn{
		sessionID:   sess.ID,
		userID:      userID,
		tenantID:    tenantID,
		category:    category,
		pipelineID:  pipelineID,
		proxyMode:   proxyMode,
		requestID:   requestID,
		userContent: userContent,
		isAuxiliary: metaString(meta, "is_auxiliary") == "true",
	}
	h.mu.Unlock()
	return nil
}

// OnResponse appends user + assistant messages asynchronously.
// Skips client-internal auxiliary requests (title generation, summarisation, etc.).
// 去重策略（同一 session 内）：
//   - 相同 user_content + 相同 assistant 且在 dedupWindow 内 → 视为客户端短期重试，整轮跳过；
//   - 相同 user_content 但 assistant 不同（agent 多轮推理）→ user 只记一次，assistant 每轮保留；
//   - 不同 user_content → 完整记录。
func (h *LoggingHook) OnResponse(ctx context.Context, resp *types.UnifiedResponse) error {
	if h == nil || h.store == nil || resp == nil {
		return nil
	}
	meta := resp.Metadata
	if meta == nil {
		return nil
	}
	sessionID := metaString(meta, metaSessionID)
	requestID := metaString(meta, metaRequestID)
	key := h.pendingKey(sessionID, requestID)

	h.mu.Lock()
	pending, ok := h.pending[key]
	if ok {
		delete(h.pending, key)
	}
	h.mu.Unlock()
	if !ok {
		if sessionID == "" {
			return nil
		}
		pending = pendingTurn{sessionID: sessionID, requestID: requestID}
	}

	if pending.isAuxiliary {
		return nil
	}

	assistant := NormalizeAssistantContent(resp.Content)
	model := resp.Model
	backend := metaString(meta, metaBackend)
	pipelineID := metaString(meta, metaPipelineID)
	if pipelineID == "" {
		pipelineID = pending.pipelineID
	}
	status := metaInt(meta, metaStatusCode)
	if status == 0 {
		status = 200
	}
	latency := metaInt64(meta, metaLatencyMs)
	inTok := metaInt(meta, metaInputTok)
	outTok := metaInt(meta, metaOutputTok)
	if outTok == 0 && resp.TokensUsed > 0 {
		outTok = resp.TokensUsed
	}

	go func() {
		bg := context.Background()
		now := time.Now().UTC()

		if pending.userContent != "" {
			writeUser := true
			h.mu.Lock()
			if r, ok := h.recent[pending.sessionID]; ok {
				if pending.userContent == r.content {
					writeUser = false
					// 同内容 + 同回复且在窗口内 → 客户端短期重试，整轮跳过。
					if now.Sub(r.at) < dedupWindow && assistant != "" && assistant == r.assistant {
						h.mu.Unlock()
						return
					}
				}
			}
			h.recent[pending.sessionID] = recentUser{content: pending.userContent, at: now, assistant: assistant}
			h.mu.Unlock()

			if writeUser {
				_ = h.store.AppendMessage(bg, &Message{
					SessionID:  pending.sessionID,
					Role:       "user",
					Content:    pending.userContent,
					RequestID:  pending.requestID,
					PipelineID: pipelineID,
					CreatedAt:  now,
				})
			}
		}
		if assistant != "" {
			_ = h.store.AppendMessage(bg, &Message{
				SessionID:    pending.sessionID,
				Role:         "assistant",
				Content:      assistant,
				RequestID:    pending.requestID,
				Model:        model,
				Backend:      backend,
				PipelineID:   pipelineID,
				InputTokens:  inTok,
				OutputTokens: outTok,
				LatencyMs:    latency,
				StatusCode:   status,
				CreatedAt:    now.Add(time.Millisecond),
			})
		}
	}()
	return nil
}

// OnCacheHit is a no-op for conversation logging.
func (h *LoggingHook) OnCacheHit(ctx context.Context, key string, data []byte) error {
	return nil
}

func lastUserContent(msgs []types.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && msgs[i].Content != "" {
			return msgs[i].Content
		}
	}
	return ""
}

func metaString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func metaInt64(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func metaInt(m map[string]interface{}, key string) int {
	return int(metaInt64(m, key))
}
