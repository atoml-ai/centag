package backend

import (
	"strings"

	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// MergeAPIKeysFromInitialFile 用 bootstrap.LoadInitialBackendsFromJSON()（initial-backends.json 中的 api_key 与 {{ENV|default}} 占位解析结果）
// 补全 DB 中 api_key 为空的条目。
// 返回新切片及是否发生过补全。
func MergeAPIKeysFromInitialFile(in []config.BackendConfig) ([]config.BackendConfig, bool) {
	fresh := bootstrap.LoadInitialBackendsFromJSON()
	if len(fresh) == 0 || len(in) == 0 {
		return in, false
	}
	byID := make(map[string]string)
	for _, b := range fresh {
		if strings.TrimSpace(b.APIKey) != "" {
			byID[b.ID] = b.APIKey
		}
	}
	out := make([]config.BackendConfig, len(in))
	changed := false
	for i := range in {
		out[i] = in[i]
		if strings.TrimSpace(out[i].APIKey) != "" {
			continue
		}
		if k, ok := byID[out[i].ID]; ok && k != "" {
			out[i].APIKey = k
			changed = true
			logger.Infof("补全后端 API Key（initial-backends.json）: id=%s", out[i].ID)
		}
	}
	if changed {
		logger.Info("已从 initial-backends.json 合并 API Key 到内存配置（即将持久化）")
	}
	return out, changed
}

// apiKeyFromInitialBackendsFile 从当前可解析到的 config/initdata/initial-backends.json 读取指定后端的 api_key（已规范化）。
func apiKeyFromInitialBackendsFile(backendID string) string {
	fresh := bootstrap.LoadInitialBackendsFromJSON()
	if len(fresh) == 0 {
		return ""
	}
	for _, b := range fresh {
		if b.ID != backendID {
			continue
		}
		return NormalizeOpenAICompatibleAPIKey(b.APIKey)
	}
	return ""
}

// ApplyAPIKeyFromInitialFileIfEmpty 当内存中某后端 api_key 为空时，用 initial-backends.json 中的非空 api_key 写入 Manager 并 Save 到数据库。
// 解决：仅 JSON 中配置了密钥、或启动时尚未合并进 DB 时，用户无需先点「保存」即可探测连接。
func (m *Manager) ApplyAPIKeyFromInitialFileIfEmpty(id string) error {
	k := apiKeyFromInitialBackendsFile(id)
	if k == "" {
		return nil
	}
	m.mu.Lock()
	b, ok := m.backends[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	if NormalizeOpenAICompatibleAPIKey(b.APIKey) != "" {
		m.mu.Unlock()
		return nil
	}
	b.APIKey = k
	m.mu.Unlock()
	if err := m.Save(); err != nil {
		return err
	}
	logger.Infof("已从 initial-backends.json 补全并持久化 API Key: id=%s", id)
	return nil
}

// MergeBackendsPreserveAPIKeys 统一配置保存时：incoming 中空 api_key 用 existing 同 id 的密钥填充，
// 避免前端因 omitempty 未带回密钥而清空数据库。
func MergeBackendsPreserveAPIKeys(existing []config.BackendConfig, incoming []config.BackendConfig) []config.BackendConfig {
	byID := make(map[string]config.BackendConfig, len(existing))
	for _, b := range existing {
		byID[b.ID] = b
	}
	out := make([]config.BackendConfig, len(incoming))
	for i, b := range incoming {
		out[i] = b
		if strings.TrimSpace(b.APIKey) != "" {
			continue
		}
		if old, ok := byID[b.ID]; ok && strings.TrimSpace(old.APIKey) != "" {
			out[i].APIKey = old.APIKey
		}
	}
	return out
}
