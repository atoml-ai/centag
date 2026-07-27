package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// readJSONMap 读取 JSON 对象；文件不存在或非对象时返回空 map。
func readJSONMap(path string) (map[string]interface{}, error) {
	path = expandPath(path)
	if !fileExists(path) {
		return map[string]interface{}{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 JSON 失败 %s: %w", path, err)
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败 %s: %w", path, err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, nil
	}
	return m, nil
}

// writeJSONMap 原子写入格式化 JSON（先备份）。
func writeJSONMap(path string, m map[string]interface{}) error {
	path = expandPath(path)
	if strings.Contains(path, "~") {
		return fmt.Errorf("无法解析 home 目录，请检查 HOME 环境变量: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", filepath.Dir(path), err)
	}
	if err := ensureCentagBackup(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败 %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入 JSON 失败 %s: %w", path, err)
	}
	return nil
}

// mergeClaudeSettingsEnv 合并 ~/.claude/settings.json 的 env 字段。
func mergeClaudeSettingsEnv(path string, env map[string]string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	envObj, _ := root["env"].(map[string]interface{})
	if envObj == nil {
		envObj = map[string]interface{}{}
	}
	for k, v := range env {
		envObj[k] = v
	}
	root["env"] = envObj
	return writeJSONMap(path, root)
}

// mergeGeminiAuthSettings 合并 Gemini settings.json 的 security.auth.selectedType。
func mergeGeminiAuthSettings(path string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	security, _ := root["security"].(map[string]interface{})
	if security == nil {
		security = map[string]interface{}{}
	}
	auth, _ := security["auth"].(map[string]interface{})
	if auth == nil {
		auth = map[string]interface{}{}
	}
	auth["selectedType"] = "gemini-api-key"
	security["auth"] = auth
	root["security"] = security
	return writeJSONMap(path, root)
}

// mergeOpenCodeProvider 合并 OpenCode provider.centag 与默认 model。
func mergeOpenCodeProvider(path, baseURL, apiKey, apiModel, modelRef string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	if _, ok := root["$schema"]; !ok {
		root["$schema"] = "https://opencode.ai/config.json"
	}
	providers, _ := root["provider"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
	}
	providers["centag"] = map[string]interface{}{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "Centag",
		"options": map[string]interface{}{
			"baseURL": baseURL,
			"apiKey":  apiKey,
		},
		"models": map[string]interface{}{
			apiModel: map[string]interface{}{
				"name": apiModel,
				"limit": map[string]interface{}{
					"context": 128000,
					"output":  16384,
				},
			},
		},
	}
	root["provider"] = providers
	root["model"] = modelRef
	return writeJSONMap(path, root)
}

// mergePiProvider 合并 Pi ~/.pi/agent/models.json 的 providers.centag。
func mergePiProvider(path, baseURL, apiKey, apiModel string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	providers, _ := root["providers"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
	}
	providers["centag"] = map[string]interface{}{
		"baseUrl": baseURL,
		"apiKey":  apiKey,
		"api":     "openai-completions",
		"models": []interface{}{
			map[string]interface{}{
				"id":            apiModel,
				"name":          apiModel,
				"reasoning":     false,
				"input":         []interface{}{"text"},
				"contextWindow": 128000,
				"maxTokens":     16384,
			},
		},
	}
	root["providers"] = providers
	return writeJSONMap(path, root)
}

// mergePiSettings 合并 Pi ~/.pi/agent/settings.json 的默认 provider/model。
func mergePiSettings(path, apiModel string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	root["defaultProvider"] = "centag"
	root["defaultModel"] = apiModel
	return writeJSONMap(path, root)
}

// mergeOpenClawProvider 合并 OpenClaw models.providers.centag 与默认 primary。
func mergeOpenClawProvider(path, baseURL, apiKey, apiModel, modelRef string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}

	agents, _ := root["agents"].(map[string]interface{})
	if agents == nil {
		agents = map[string]interface{}{}
	}
	defaults, _ := agents["defaults"].(map[string]interface{})
	if defaults == nil {
		defaults = map[string]interface{}{}
	}
	defaults["model"] = map[string]interface{}{
		"primary": modelRef,
	}
	agents["defaults"] = defaults
	root["agents"] = agents

	models, _ := root["models"].(map[string]interface{})
	if models == nil {
		models = map[string]interface{}{}
	}
	models["mode"] = "merge"
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
	}
	providers["centag"] = map[string]interface{}{
		"baseUrl": baseURL,
		"apiKey":  apiKey,
		"api":     "openai-completions",
		"models": []interface{}{
			map[string]interface{}{"id": apiModel, "name": apiModel},
		},
	}
	models["providers"] = providers
	root["models"] = models
	return writeJSONMap(path, root)
}

// mergeCodeBuddyModel 合并 ~/.codebuddy/models.json 中 id 匹配的模型（SmartMerge）。
// url 必须为完整 /chat/completions 路径；同 id 覆盖，不同 id 追加。
func mergeCodeBuddyModel(path, modelID, displayName, apiKey, chatURL string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	entry := map[string]interface{}{
		"id":               modelID,
		"name":             displayName,
		"vendor":           "Centag",
		"apiKey":           apiKey,
		"url":              chatURL,
		"maxInputTokens":   128000,
		"maxOutputTokens":  16384,
		"supportsToolCall": true,
		"supportsImages":   false,
	}
	rawModels, _ := root["models"].([]interface{})
	replaced := false
	for i, item := range rawModels {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == modelID {
			rawModels[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		rawModels = append(rawModels, entry)
	}
	root["models"] = rawModels

	// 确保 availableModels 包含该 id（若列表已存在则追加）
	if avail, ok := root["availableModels"].([]interface{}); ok {
		found := false
		for _, a := range avail {
			if s, ok := a.(string); ok && s == modelID {
				found = true
				break
			}
		}
		if !found {
			root["availableModels"] = append(avail, modelID)
		}
	}
	return writeJSONMap(path, root)
}

// mergeTraeCustomModel 合并 settings.json 的 trae.customModels（社区/UI 持久化键）。
// baseURL 使用 …/v1（关闭「完整 URL」时由 Trae 自动补全 /chat/completions）。
func mergeTraeCustomModel(path, modelID, displayName, apiKey, baseURL string) error {
	root, err := readJSONMap(path)
	if err != nil {
		return err
	}
	entry := map[string]interface{}{
		"id":          modelID,
		"name":        displayName,
		"displayName": displayName,
		"modelId":     modelID,
		"provider":    "openai",
		"apiProtocol": "openai",
		"baseUrl":     baseURL,
		"url":         baseURL,
		"apiKey":      apiKey,
		"useFullUrl":  false,
	}
	raw, _ := root["trae.customModels"].([]interface{})
	replaced := false
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			id, _ = m["modelId"].(string)
		}
		if id == modelID {
			raw[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		raw = append(raw, entry)
	}
	root["trae.customModels"] = raw
	return writeJSONMap(path, root)
}

// mergeHermesProvider 合并 Hermes custom_providers 中 name=centag，并更新 model 默认。
func mergeHermesProvider(path, baseURL, apiKey, model string) error {
	path = expandPath(path)
	var root map[string]interface{}
	if fileExists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取 YAML 失败 %s: %w", path, err)
		}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("解析 YAML 失败 %s: %w", path, err)
		}
	}
	if root == nil {
		root = map[string]interface{}{}
	}

	root["model"] = map[string]interface{}{
		"default":  model,
		"provider": "centag",
		"base_url": baseURL,
	}

	centagProvider := map[string]interface{}{
		"name":     "centag",
		"base_url": baseURL,
		"api_key":  apiKey,
		"api_mode": "chat_completions",
		"model":    model,
	}

	rawList, _ := root["custom_providers"].([]interface{})
	replaced := false
	for i, item := range rawList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		if strings.EqualFold(name, "centag") {
			rawList[i] = centagProvider
			replaced = true
			break
		}
	}
	if !replaced {
		rawList = append(rawList, centagProvider)
	}
	root["custom_providers"] = rawList

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", filepath.Dir(path), err)
	}
	if err := ensureCentagBackup(path); err != nil {
		return err
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("序列化 YAML 失败 %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入 YAML 失败 %s: %w", path, err)
	}
	return nil
}
