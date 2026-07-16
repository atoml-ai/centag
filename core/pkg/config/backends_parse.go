package config

import (
	"encoding/json"
	"strings"
)

// ParseAdminBackendsJSON 解析 system_config 中 admin_backends 的 JSON。
// 标准 json.Unmarshal 到 []BackendConfig 时只识别字段标签 "api_key"；若历史数据或外部工具
// 写入了 "APIKey" / "apiKey"，则 APIKey 会为空，导致内存 Manager 无密钥而数据库「看起来有密钥」。
// 本函数在反序列化后从原始对象的多个候选键补全 APIKey。
func ParseAdminBackendsJSON(raw string) ([]BackendConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var backends []BackendConfig
	if err := json.Unmarshal([]byte(raw), &backends); err != nil {
		return nil, err
	}
	mergeAlternateAPIKeysFromAdminBackendsRaw(raw, &backends)
	return backends, nil
}

func mergeAlternateAPIKeysFromAdminBackendsRaw(raw string, backends *[]BackendConfig) {
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return
	}
	for i := range *backends {
		if i >= len(arr) {
			break
		}
		if strings.TrimSpace((*backends)[i].APIKey) != "" {
			continue
		}
		m := arr[i]
		for _, key := range []string{"api_key", "APIKey", "apiKey"} {
			v, ok := m[key]
			if !ok {
				continue
			}
			s, ok := v.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				(*backends)[i].APIKey = s
				break
			}
		}
	}
}
