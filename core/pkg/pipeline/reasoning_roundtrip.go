package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

const (
	reasoningRoundtripTTL      = 2 * time.Hour
	reasoningRoundtripMaxSess  = 4096
	reasoningRoundtripMaxTurns = 64
)

// reasoningRoundtripStore caches upstream reasoning_content per conversation so
// outbound requests can restore fields dropped by OpenAI-compatible clients.
//
// Narrow trigger (DeepSeek thinking + tools):
//   - Only activate after the session has observed non-empty reasoning_content
//     from upstream (or inbound history that already carries it).
//   - Only repair assistant messages that have tool_calls and are missing the field.
//   - Never invent reasoning text for backends/sessions that never used it.
type reasoningRoundtripStore struct {
	mu       sync.Mutex
	sessions map[string]*reasoningSession
	ttl      time.Duration
	maxSess  int
}

type reasoningSession struct {
	active    bool // saw non-empty reasoning_content
	byToolKey map[string]string
	turns     []reasoningTurn
	updatedAt time.Time
}

type reasoningTurn struct {
	toolKey   string
	reasoning string
}

var globalReasoningRoundtrip = newReasoningRoundtripStore(reasoningRoundtripTTL, reasoningRoundtripMaxSess)

func newReasoningRoundtripStore(ttl time.Duration, maxSess int) *reasoningRoundtripStore {
	if ttl <= 0 {
		ttl = reasoningRoundtripTTL
	}
	if maxSess <= 0 {
		maxSess = reasoningRoundtripMaxSess
	}
	return &reasoningRoundtripStore{
		sessions: make(map[string]*reasoningSession),
		ttl:      ttl,
		maxSess:  maxSess,
	}
}

// ReasoningContentRoundtripEnabled reports whether auto-repair is on (default true).
func ReasoningContentRoundtripEnabled() bool {
	cfg := config.Get()
	if cfg == nil {
		return true
	}
	return cfg.Proxy.ReasoningContentRoundtripEnabled()
}

// applyReasoningRoundtripOnRequest learns from inbound history then repairs gaps.
// Returns the (possibly rewritten) body and how many assistant messages were repaired.
func applyReasoningRoundtripOnRequest(meta map[string]interface{}, body []byte) ([]byte, int) {
	if !ReasoningContentRoundtripEnabled() || len(body) == 0 {
		return body, 0
	}
	sk := reasoningSessionKey(meta, body)
	if sk == "" {
		return body, 0
	}
	globalReasoningRoundtrip.learnFromRequest(sk, body)
	out, n := globalReasoningRoundtrip.repairRequest(sk, body)
	if n > 0 {
		if meta != nil {
			meta["reasoning_content_repaired"] = n
			meta["reasoning_session_key"] = sk
		}
		logger.Infof("reasoning_content roundtrip: repaired %d assistant message(s) session=%s", n, truncateForLog(sk, 64))
	}
	return out, n
}

// applyReasoningRoundtripOnResponse stores reasoning_content from a successful upstream reply.
func applyReasoningRoundtripOnResponse(meta map[string]interface{}, reqBody, respBody []byte, statusCode int) {
	if !ReasoningContentRoundtripEnabled() || statusCode >= 400 || len(respBody) == 0 {
		return
	}
	sk := ""
	if meta != nil {
		sk = strings.TrimSpace(stringMeta(meta, "reasoning_session_key"))
	}
	if sk == "" {
		sk = reasoningSessionKey(meta, reqBody)
	}
	if sk == "" {
		return
	}
	if meta != nil {
		meta["reasoning_session_key"] = sk
	}
	globalReasoningRoundtrip.rememberFromResponse(sk, respBody)
}

func reasoningSessionKey(meta map[string]interface{}, body []byte) string {
	prefix := ""
	if uid := strings.TrimSpace(stringMeta(meta, "user_id")); uid != "" {
		prefix = "u:" + uid + ":"
	}
	if sid := strings.TrimSpace(stringMeta(meta, "session_id")); sid != "" {
		return prefix + "sid:" + sid
	}
	fp := conversationFingerprint(body)
	if fp == "" {
		return ""
	}
	return prefix + "fp:" + fp
}

func conversationFingerprint(body []byte) string {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return ""
	}
	msgs, _ := raw["messages"].([]interface{})
	var b strings.Builder
	n := 0
	for _, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := mm["role"].(string)
		if role != "user" && role != "system" {
			continue
		}
		content := firstNonEmptyJSONString(mm["content"])
		if content == "" {
			continue
		}
		b.WriteString(role)
		b.WriteByte(':')
		b.WriteString(content)
		b.WriteByte('\n')
		n++
		if n >= 2 {
			break
		}
	}
	if b.Len() == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

func (s *reasoningRoundtripStore) learnFromRequest(sessionKey string, body []byte) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return
	}
	msgs, _ := raw["messages"].([]interface{})
	for _, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.TrimSpace(stringMeta(mm, "role")) != "assistant" {
			continue
		}
		rc, hasRC := mm["reasoning_content"]
		reasoning := firstNonEmptyJSONString(rc)
		if !hasRC && reasoning == "" {
			// also accept "reasoning"
			reasoning = firstNonEmptyJSONString(mm["reasoning"])
			if reasoning == "" {
				continue
			}
		}
		ids := toolCallIDsFromMessage(mm)
		s.rememberTurn(sessionKey, ids, reasoning, hasRC || reasoning != "")
	}
}

func (s *reasoningRoundtripStore) rememberFromResponse(sessionKey string, respBody []byte) {
	extracted := extractChatCompletionResult(respBody)
	if strings.TrimSpace(extracted.Reasoning) == "" && len(extracted.ToolCalls) == 0 {
		return
	}
	ids := make([]string, 0, len(extracted.ToolCalls))
	for _, tc := range extracted.ToolCalls {
		if id := strings.TrimSpace(tc.ID); id != "" {
			ids = append(ids, id)
		}
	}
	s.rememberTurn(sessionKey, ids, extracted.Reasoning, strings.TrimSpace(extracted.Reasoning) != "")
}

func (s *reasoningRoundtripStore) rememberTurn(sessionKey string, toolIDs []string, reasoning string, markActive bool) {
	if strings.TrimSpace(sessionKey) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked()
	sess := s.sessions[sessionKey]
	if sess == nil {
		sess = &reasoningSession{byToolKey: make(map[string]string)}
		s.sessions[sessionKey] = sess
	}
	if markActive && strings.TrimSpace(reasoning) != "" {
		sess.active = true
	}
	key := toolCallKey(toolIDs)
	if key != "" && strings.TrimSpace(reasoning) != "" {
		sess.byToolKey[key] = reasoning
	}
	if key != "" || strings.TrimSpace(reasoning) != "" {
		sess.turns = append(sess.turns, reasoningTurn{toolKey: key, reasoning: reasoning})
		if len(sess.turns) > reasoningRoundtripMaxTurns {
			sess.turns = sess.turns[len(sess.turns)-reasoningRoundtripMaxTurns:]
		}
	}
	sess.updatedAt = time.Now()
}

func (s *reasoningRoundtripStore) repairRequest(sessionKey string, body []byte) ([]byte, int) {
	s.mu.Lock()
	sess := s.sessions[sessionKey]
	if sess != nil {
		sess.updatedAt = time.Now()
	}
	active := sess != nil && sess.active
	byTool := map[string]string{}
	turns := []reasoningTurn{}
	if sess != nil {
		for k, v := range sess.byToolKey {
			byTool[k] = v
		}
		turns = append(turns, sess.turns...)
	}
	s.mu.Unlock()

	if !active && len(byTool) == 0 {
		return body, 0
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, 0
	}
	msgs, ok := raw["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return body, 0
	}

	// Ordinal fallback: assistant+tool_calls turns in request ↔ cached turns with tool keys.
	var toolTurns []reasoningTurn
	for _, t := range turns {
		if t.toolKey != "" && strings.TrimSpace(t.reasoning) != "" {
			toolTurns = append(toolTurns, t)
		}
	}
	toolTurnIdx := 0

	repaired := 0
	for i, m := range msgs {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if strings.TrimSpace(stringMeta(mm, "role")) != "assistant" {
			continue
		}
		ids := toolCallIDsFromMessage(mm)
		if len(ids) == 0 {
			continue
		}
		ord := toolTurnIdx
		toolTurnIdx++
		if _, has := mm["reasoning_content"]; has {
			continue
		}
		key := toolCallKey(ids)
		reasoning := ""
		if key != "" {
			reasoning = byTool[key]
		}
		if reasoning == "" && ord < len(toolTurns) {
			reasoning = toolTurns[ord].reasoning
		}
		if reasoning == "" && !active {
			continue
		}
		// Cache hit restores original text; active+miss pads "" so DeepSeek
		// sees a consistent field on tool-call assistant turns.
		mm["reasoning_content"] = reasoning
		msgs[i] = mm
		repaired++
	}
	if repaired == 0 {
		return body, 0
	}
	raw["messages"] = msgs
	out, err := json.Marshal(raw)
	if err != nil {
		return body, 0
	}
	return out, repaired
}

func (s *reasoningRoundtripStore) evictLocked() {
	now := time.Now()
	for k, sess := range s.sessions {
		if now.Sub(sess.updatedAt) > s.ttl {
			delete(s.sessions, k)
		}
	}
	for len(s.sessions) >= s.maxSess {
		var oldestKey string
		var oldest time.Time
		first := true
		for k, sess := range s.sessions {
			if first || sess.updatedAt.Before(oldest) {
				oldestKey = k
				oldest = sess.updatedAt
				first = false
			}
		}
		if oldestKey == "" {
			break
		}
		delete(s.sessions, oldestKey)
	}
}

func toolCallIDsFromMessage(mm map[string]interface{}) []string {
	tcs := parseChatToolCalls(mm["tool_calls"])
	ids := make([]string, 0, len(tcs))
	for _, tc := range tcs {
		if id := strings.TrimSpace(tc.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func toolCallKey(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	cp := append([]string(nil), ids...)
	sort.Strings(cp)
	return strings.Join(cp, "|")
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// resetReasoningRoundtripStoreForTest clears the global store (tests only).
func resetReasoningRoundtripStoreForTest() {
	globalReasoningRoundtrip = newReasoningRoundtripStore(reasoningRoundtripTTL, reasoningRoundtripMaxSess)
}
