package backend

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// 客户端可见的配置类错误（无可用大模型后端 / 未配 Key）。
var (
	// ErrNoUsableBackend 表示当前没有可用于 LLM 调用的后端（未配置、未启用或缺少 Key）。
	ErrNoUsableBackend = errors.New("no usable llm backend")
	// ErrNoBackendAPIKey 表示后端已配置但缺少 API Key（非 Ollama）。
	ErrNoBackendAPIKey = errors.New("backend api key required")
)

// ClientHintNoBackendConfigured 返回给 OpenAI 兼容客户端的规范提示文案。
const ClientHintNoBackendConfigured = "尚未配置可用的大模型后端。请在 WebUI「后端管理」中添加至少一个后端（填写 Base URL），并为云端供应商保存有效的 API Key；本地 Ollama 可无需 Key。配置完成后重试请求。"

// ClientHintNoBackendAPIKey 后端存在但缺 Key 时的提示。
const ClientHintNoBackendAPIKey = "已配置的大模型后端缺少 API Key。请在 WebUI「后端管理」中为对应后端填写并保存有效的 API Key（令牌勿含 \"Bearer \" 前缀），然后重试。"

// ErrorCodeNoBackendConfigured OpenAI 风格 error.code。
const (
	ErrorCodeNoBackendConfigured = "no_backend_configured"
	ErrorCodeNoBackendAPIKey     = "no_backend_api_key"
)

// IsUsableLLMBackend 判断后端是否足以发起 LLM 调用。
// Ollama 只需启用且有 BaseURL；其余类型还需非空 API Key。
func IsUsableLLMBackend(cfg *BackendConfig) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Type), "ollama") {
		return true
	}
	return NormalizeOpenAICompatibleAPIKey(cfg.APIKey) != ""
}

// HasUsableLLMBackend 是否存在至少一个可用于 LLM 调用的后端。
func (m *Manager) HasUsableLLMBackend() bool {
	if m == nil {
		return false
	}
	for _, cfg := range m.GetEnabled() {
		if IsUsableLLMBackend(cfg) {
			return true
		}
	}
	return false
}

// HasEnabledBackends 是否存在任意启用后端（不论是否已填 Key）。
func (m *Manager) HasEnabledBackends() bool {
	if m == nil {
		return false
	}
	return len(m.GetEnabled()) > 0
}

// ClientError 描述应返回给客户端的规范化错误。
type ClientError struct {
	HTTPStatus int
	Code       string
	Message    string
	Err        error
}

func (e *ClientError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ClientError) Unwrap() error { return e.Err }

// NewNoUsableBackendError 构造「未配置可用后端」错误。
func NewNoUsableBackendError(cause error) error {
	wrapped := ErrNoUsableBackend
	if cause != nil {
		wrapped = fmt.Errorf("%w: %v", ErrNoUsableBackend, cause)
	}
	return &ClientError{
		HTTPStatus: http.StatusServiceUnavailable,
		Code:       ErrorCodeNoBackendConfigured,
		Message:    ClientHintNoBackendConfigured,
		Err:        wrapped,
	}
}

// NewNoBackendAPIKeyError 构造「缺少 API Key」错误。
func NewNoBackendAPIKeyError(backendID string) error {
	msg := ClientHintNoBackendAPIKey
	if backendID != "" {
		msg = fmt.Sprintf("%s（backend_id=%s）", ClientHintNoBackendAPIKey, backendID)
	}
	return &ClientError{
		HTTPStatus: http.StatusServiceUnavailable,
		Code:       ErrorCodeNoBackendAPIKey,
		Message:    msg,
		Err:        fmt.Errorf("%w: %s", ErrNoBackendAPIKey, backendID),
	}
}

// ClassifyClientError 将内部错误映射为面向客户端的配置类错误；无法识别则返回 nil。
func ClassifyClientError(err error) *ClientError {
	if err == nil {
		return nil
	}
	var ce *ClientError
	if errors.As(err, &ce) {
		return ce
	}
	if errors.Is(err, ErrNoBackendAPIKey) {
		return &ClientError{
			HTTPStatus: http.StatusServiceUnavailable,
			Code:       ErrorCodeNoBackendAPIKey,
			Message:    ClientHintNoBackendAPIKey,
			Err:        err,
		}
	}
	if errors.Is(err, ErrNoUsableBackend) {
		return &ClientError{
			HTTPStatus: http.StatusServiceUnavailable,
			Code:       ErrorCodeNoBackendConfigured,
			Message:    ClientHintNoBackendConfigured,
			Err:        err,
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no enabled backends"),
		strings.Contains(msg, "no enabled backend"),
		strings.Contains(msg, "no usable llm backend"),
		strings.Contains(msg, "backend manager not available"):
		return &ClientError{
			HTTPStatus: http.StatusServiceUnavailable,
			Code:       ErrorCodeNoBackendConfigured,
			Message:    ClientHintNoBackendConfigured,
			Err:        err,
		}
	case strings.Contains(msg, "not found") && strings.Contains(msg, "backend"):
		// 仅当全局确实无可用/无启用后端时，才映射为「未配置」；
		// 已有后端但模板引用错误 ID 时不吞掉根因，交给上层原样返回。
		if mgr := GetManager(); mgr != nil && (mgr.HasUsableLLMBackend() || mgr.HasEnabledBackends()) {
			return nil
		}
		return &ClientError{
			HTTPStatus: http.StatusServiceUnavailable,
			Code:       ErrorCodeNoBackendConfigured,
			Message:    ClientHintNoBackendConfigured,
			Err:        err,
		}
	case strings.Contains(msg, "api key") && (strings.Contains(msg, "未配置") || strings.Contains(msg, "required") || strings.Contains(msg, "missing") || strings.Contains(msg, "empty")):
		return &ClientError{
			HTTPStatus: http.StatusServiceUnavailable,
			Code:       ErrorCodeNoBackendAPIKey,
			Message:    ClientHintNoBackendAPIKey,
			Err:        err,
		}
	default:
		return nil
	}
}

// OpenAIErrorBody 构造 OpenAI 兼容错误 JSON 对象（供 gin.H 使用）。
func OpenAIErrorBody(code, message string) map[string]interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "proxyclaw_configuration_error",
			"code":    code,
		},
	}
}
