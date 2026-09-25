package freellm

import (
	"fmt"

	"centag/core/pkg/backend"
)

// ToBackendConfig 将一个目录条目转换为 centag 的 BackendConfig。
// 转换时复用 centag 既有能力：
//   - AutoFetchModels=true：注册后由 centag 实时拉取真实模型列表并覆盖种子。
//   - ProbeModel：连通性/可用性探测模型。
//   - FallbackBackends / AccountPool：留空，由用户在后端管理页按需配置故障转移与多 key 轮转。
//   - Metadata：写入 source/free_tier/provider_id 等，供前端展示“免费”徽标与限额。
//
// apiKey 仅对 keyless 且要求非空 key 的 provider（DummyKey）生效；其余 keyless 传空即可。
func ToBackendConfig(entry ProviderCatalogEntry, apiKey, tenantID string) *backend.BackendConfig {
	probe := entry.ProbeModel
	if probe == "" && len(entry.KnownFreeModels) > 0 {
		probe = entry.KnownFreeModels[0]
	}

	// 解析有效 key：keyless 用占位 key（若有），keyed/oauth 必须用户提供。
	effectiveKey := apiKey
	if entry.AuthType == AuthKeyless && entry.DummyKey != "" && apiKey == "" {
		effectiveKey = entry.DummyKey
	}

	supported := make([]backend.ModelMapping, 0, len(entry.KnownFreeModels))
	for _, m := range entry.KnownFreeModels {
		supported = append(supported, backend.ModelMapping{
			RequestedModel:     m,
			ActualModel:        m,
			IsExact:            true,
			CompatibilityScore: 1.0,
		})
	}

	return &backend.BackendConfig{
		ID:              "freellm-" + entry.ID,
		Name:            entry.Name,
		Type:            entry.Type,
		BaseURL:         entry.BaseURL,
		APIKey:          effectiveKey,
		Enabled:         true,
		Timeout:         defaultTimeout,
		MaxRetries:      2,
		Description:     fmt.Sprintf("免费 LLM 后端（%s）：%s", entry.AuthType, entry.RateLimitNotes),
		AutoFetchModels: true,
		ProbeModel:      probe,
		SupportedModels: supported,
		Metadata: map[string]string{
			"source":      "freellm",
			"free_tier":   "true",
			"provider_id": entry.ID,
			"auth_type":   entry.AuthType,
			"doc_url":     entry.DocURL,
			"rate_limit":  entry.RateLimitNotes,
		},
		TenantID: tenantID,
	}
}

// uniqueBackendID 在管理器中找到不冲突的后端 ID。
func uniqueBackendID(manager *backend.Manager, base string) string {
	id := base
	for i := 1; ; i++ {
		if _, err := manager.Get(id); err != nil {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

// Register 将单个免费 provider 注册为 centag 后端。
// 对 keyless 条目无需 apiKey；对 keyed/oauth 条目，apiKey 为空则返回错误。
func Register(manager *backend.Manager, entry ProviderCatalogEntry, apiKey, tenantID string) (*backend.BackendConfig, error) {
	if entry.AuthType != AuthKeyless && apiKey == "" {
		return nil, fmt.Errorf("provider %q 需要 API Key（免费层通常无信用卡）", entry.ID)
	}

	cfg := ToBackendConfig(entry, apiKey, tenantID)
	cfg.ID = uniqueBackendID(manager, cfg.ID)

	if err := manager.Add(cfg); err != nil {
		return nil, fmt.Errorf("添加后端失败: %w", err)
	}
	if err := manager.Save(); err != nil {
		return nil, fmt.Errorf("保存后端失败: %w", err)
	}
	return cfg, nil
}

// ScanKeyless 一键注册全部 keyless（零配置）免费后端，返回注册成功的后端 ID 列表。
// keyless 后端无需任何密钥即可使用，是最贴合“自动获取免费大模型”目标的入口。
func ScanKeyless(manager *backend.Manager, tenantID string) ([]string, error) {
	keyless := ListCatalog(AuthKeyless)
	if len(keyless) == 0 {
		return nil, nil
	}

	registered := make([]string, 0, len(keyless))
	for _, entry := range keyless {
		cfg := ToBackendConfig(entry, "", tenantID)
		cfg.ID = uniqueBackendID(manager, cfg.ID)
		if err := manager.Add(cfg); err != nil {
			return registered, fmt.Errorf("添加 %s 失败: %w", entry.ID, err)
		}
		registered = append(registered, cfg.ID)
	}

	if err := manager.Save(); err != nil {
		return registered, fmt.Errorf("保存后端失败: %w", err)
	}
	return registered, nil
}
