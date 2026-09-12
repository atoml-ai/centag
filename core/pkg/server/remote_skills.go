package server

import (
	"fmt"
	"os"
	"strings"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/logger"

	"gopkg.in/yaml.v3"
)

// RemoteSkillRow 远程 skill 行（飞书表格为权威来源）。
// 一行对应一个 skill：名称/描述/分类/工具/步骤/系统提示词。
type RemoteSkillRow struct {
	Name         string
	Description  string
	Category     string
	Tools        []string
	Steps        []string
	SystemPrompt string
	Version      string
	Edition      string // 空/all 表示全版本可用；personal/team 对应当前发行版
	Enabled      bool
}

// remoteSkillVersionMarker 远程同步 skill 的 manifest version 标记。
// 用于同步后清理「表中已删除但本地仍残留」的远程 skill manifest
// （同步以飞书表格为权威：表格删行 → 下轮同步删除本地对应 skill）。
const remoteSkillVersionMarker = "feishu"

// buildRemoteSkillPlugin 由远程 skill 行构建 SkillPlugin。
// 复用 custom skill 的 manifest 形状（implementation: custom.agent-skill-<name>，
// 准入链路与用户自建 skill 一致），仅 version 打 feishu 来源标记。
func buildRemoteSkillPlugin(row *RemoteSkillRow, name string) (skills.SkillPlugin, error) {
	if row == nil {
		return nil, fmt.Errorf("remote skill row is nil")
	}
	manifest := fmt.Sprintf(`api_version: %s
implementation: custom.agent-skill-%s
name: %s
kind: %s
version: %s
description: %s
skill:
  name: %s
  category: %s
  enabled: true
  internal: false
  tools:
%s
  steps:
%s
  system_prompt: |-
%s
`, skills.SkillPluginSchemaVersion, name, yamlScalar(name), skills.SkillPluginKind,
		yamlScalar(remoteSkillVersionMarker+"/"+firstNotEmpty(row.Version, "1.0.0")),
		yamlScalar(row.Description), name, yamlScalar(row.Category),
		yamlList(row.Tools), yamlList(row.Steps), yamlBlock(row.SystemPrompt))
	return skills.ParseSkillPluginManifest([]byte(manifest))
}

// firstNotEmpty 返回第一个非空字符串。
func firstNotEmpty(items ...string) string {
	for _, s := range items {
		if s != "" {
			return s
		}
	}
	return ""
}

// yamlVersionQuoter YAML 安全标量（复用 skill_crud 的安全转义策略）。版本留作扩展点。
type yamlVersionQuoter struct{}

func (yamlVersionQuoter) quote(s string) (string, error) {
	b, err := yaml.Marshal(s)
	if err != nil {
		return `""`, err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

// applyRemoteSkill 把一行远程 skill 以 custom skill 形式注册（同一登记链路）。
// 覆盖同名 skill（表格为权威来源）。
func (h *BuiltinAgentHandler) applyRemoteSkill(row *RemoteSkillRow) error {
	name := normalizeSkillName(row.Name)
	if name == "" {
		return fmt.Errorf("remote skill name is empty")
	}
	p, err := buildRemoteSkillPlugin(row, name)
	if err != nil {
		return err
	}
	if err := h.registerCustomSkill(p); err != nil {
		return err
	}
	// registerCustomSkill 用 MarshalSkillPluginManifest 序列化（会重置 version），
	// 这里重新落盘带来源标记的 manifest，保证后续清理判定可靠。
	return h.saveRemoteSkillManifest(p)
}

// saveRemoteSkillManifest 将 plugin 序列化为带 feishu 来源标记的 manifest 落盘。
func (h *BuiltinAgentHandler) saveRemoteSkillManifest(p skills.SkillPlugin) error {
	name := p.GetSkillDefinition().Name
	data, err := skills.MarshalSkillPluginManifest(p)
	if err != nil {
		return fmt.Errorf("marshal remote skill manifest: %w", err)
	}
	data = replaceManifestVersion(data, remoteSkillVersionMarker)
	return h.manifestStore.Save(name, data)
}

// replaceManifestVersion 将 manifest YAML 顶层 version: 值替换为来源标记。
// 仅替换 skill 顶层 version 行（首个行首锚定的 "version: " 行）；
// 必须按行首匹配，避免命中首行 "api_version: " 中的尾缀子串导致 schema 版本被写坏。
func replaceManifestVersion(data []byte, version string) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, "version: ") {
			continue
		}
		quoter := yamlVersionQuoter{}
		quoted, err := quoter.quote(version)
		if err != nil {
			return data
		}
		indent := line[:len(line)-len(trimmed)]
		lines[i] = indent + "version: " + quoted
		return []byte(strings.Join(lines, "\n"))
	}
	return data
}

// ApplyRemoteSkills 同步远程 skill 行到本地 skill 注册体系（飞书表格为权威来源）：
//   - 新行 / 变更行：以 custom skill 形式落盘并注册（幂等覆盖）；
//   - 上轮由远程同步、本轮表格已消失的 skill：删除本地 manifest 并从注册表移除。
//
// 返回 (应用数, 删除数)。
func (h *BuiltinAgentHandler) ApplyRemoteSkills(rows []RemoteSkillRow) (applied int, removed int) {
	if h == nil || h.manifestStore == nil || h.skillPluginRegistry == nil {
		logger.Warnf("[remote-skills] skill 插件未初始化，跳过远程 skill 同步")
		return 0, 0
	}
	edition := strings.ToLower(strings.TrimSpace(os.Getenv("CENTAG_EDITION")))
	incoming := make(map[string]bool, len(rows))

	for i := range rows {
		row := rows[i]
		name := normalizeSkillName(row.Name)
		if name == "" {
			continue
		}
		// 发行版过滤：edition 字段非空时需匹配 all / 当前发行版
		rowEdition := strings.ToLower(strings.TrimSpace(row.Edition))
		if rowEdition != "" && rowEdition != "all" && edition != "" && rowEdition != edition {
			continue
		}
		// 表格 enabled=false 的行跳过（删除由来源标记清理统一处理）
		if !row.Enabled {
			continue
		}
		incoming[name] = true
		if e := h.applyRemoteSkill(&row); e != nil {
			logger.Warnf("[remote-skills] 应用 skill %s 失败: %v", name, e)
			continue
		}
		applied++
	}

	// 清理：本地仍存在、带远程来源标记、但本轮表格已不存在的 skill
	if names, listErr := h.manifestStore.List(); listErr == nil {
		for _, name := range names {
			if incoming[name] {
				continue
			}
			p, ok := h.skillPluginRegistry.Get(name)
			if !ok || !strings.HasPrefix(p.Descriptor().Version, remoteSkillVersionMarker) {
				continue
			}
			if e := h.deleteCustomSkill(name); e != nil {
				logger.Warnf("[remote-skills] 清理残留 skill %s 失败: %v", name, e)
				continue
			}
			h.skillRegistry.RemoveSkill(name)
			removed++
		}
	}

	logger.Infof("[remote-skills] 同步完成: applied=%d removed=%d total_rows=%d", applied, removed, len(rows))
	return applied, removed
}
