package backend

import (
	"context"
	"strings"

	"centag/core/pkg/config"
	"centag/core/pkg/database"
	"centag/core/pkg/logger"
)

// RepairAPIKeyFromDBIfEmpty 当内存中某后端 api_key 为空时，从 system_config.admin_backends 再读一遍并写回 Manager。
// 用于修复：仅 config 与 DB 已持久化密钥，但某条路径未同步到 manager 时，测试连接仍拿到空密钥的问题。
func (m *Manager) RepairAPIKeyFromDBIfEmpty(ctx context.Context, id string) {
	if id == "" || !database.IsInitialized() {
		return
	}
	raw, err := database.Get().SystemConfigStore().Get(ctx, config.KeyBackends)
	if err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	list, err := config.ParseAdminBackendsJSON(raw)
	if err != nil || len(list) == 0 {
		return
	}
	var dbKey string
	for i := range list {
		if list[i].ID == id {
			dbKey = NormalizeOpenAICompatibleAPIKey(list[i].APIKey)
			break
		}
	}
	if dbKey == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.backends[id]
	if !ok {
		return
	}
	if NormalizeOpenAICompatibleAPIKey(b.APIKey) != "" {
		return
	}
	b.APIKey = dbKey
	logger.Infof("已从数据库补全后端 API Key 到内存（探测前）: id=%s", id)
}
