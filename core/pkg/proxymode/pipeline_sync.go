package proxymode

import (
	"strings"
	"time"

	"centag/core/pkg/pipeline"
)

// ModeFromPipeline 将流水线定义转换为 ModeManager 可注册的模式配置。
// 仅当流水线配置了合法的 # 前缀快捷码时返回 ok=true。
func ModeFromPipeline(p *pipeline.AgentPatternPipeline) (ModeConfig, bool) {
	if p == nil {
		return ModeConfig{}, false
	}

	code := strings.TrimSpace(p.ShortcutCode)
	if code == "" || !strings.HasPrefix(code, "#") {
		return ModeConfig{}, false
	}
	if err := validateModeKey(code); err != nil {
		return ModeConfig{}, false
	}

	modeType := resolvePipelineModeType(p)
	cfg := map[string]interface{}{
		"pipeline_id": p.ID,
	}
	if p.Metadata != nil {
		if apm, ok := p.Metadata["aligned_proxy_mode"].(string); ok && apm != "" {
			cfg["aligned_proxy_mode"] = apm
		}
	}

	return ModeConfig{
		Key:         code,
		Name:        p.Name,
		Type:        modeType,
		Description: p.Description,
		Enabled:     true,
		Config:      cfg,
	}, true
}

func resolvePipelineModeType(p *pipeline.AgentPatternPipeline) string {
	if p.Metadata != nil {
		if apm, ok := p.Metadata["aligned_proxy_mode"].(string); ok && apm != "" {
			if em, err := FromString(apm); err == nil {
				return em.GetType()
			}
			return apm
		}
	}
	if em, err := FromString(p.ID); err == nil {
		return em.GetType()
	}
	return "pipeline"
}

// UpsertPipelineMode 注册或更新由流水线同步而来的模式。
// 已存在的内置受保护模式（#d、#s 等）仅更新展示信息与 pipeline_id，不会被标记为可删除。
func (m *ModeManager) UpsertPipelineMode(mode ModeConfig) error {
	if err := mode.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if existing, exists := m.modes[mode.Key]; exists {
		existing.Name = mode.Name
		existing.Type = mode.Type
		existing.Description = mode.Description
		existing.Enabled = mode.Enabled
		existing.Config = mode.Config
		existing.UpdatedAt = now
		if !m.protected[mode.Key] {
			m.pipelineDerived[mode.Key] = true
		}
		return nil
	}

	modeCopy := mode
	modeCopy.CreatedAt = now
	modeCopy.UpdatedAt = now
	m.modes[mode.Key] = &modeCopy
	m.pipelineDerived[mode.Key] = true
	return nil
}

// SyncFromPipelines 将已加载流水线（含快捷码）全量同步到 ModeManager。
// 策略管理列表中的流水线与 ModeManager 保持同源：启动加载、API 增删改后均应调用。
func (m *ModeManager) SyncFromPipelines(pipelines []*pipeline.AgentPatternPipeline) int {
	active := make(map[string]bool)
	synced := 0

	for _, p := range pipelines {
		mode, ok := ModeFromPipeline(p)
		if !ok {
			continue
		}
		if err := m.UpsertPipelineMode(mode); err != nil {
			continue
		}
		active[mode.Key] = true
		synced++
	}

	m.removeStalePipelineModes(active)
	return synced
}

func (m *ModeManager) removeStalePipelineModes(active map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key := range m.pipelineDerived {
		if active[key] || m.protected[key] {
			continue
		}
		delete(m.modes, key)
		delete(m.pipelineDerived, key)
	}
}

// RemovePipelineShortcut 流水线删除或快捷码清空时，移除非受保护的同步模式。
func (m *ModeManager) RemovePipelineShortcut(shortcutCode string) {
	code := strings.TrimSpace(shortcutCode)
	if code == "" || !strings.HasPrefix(code, "#") {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.protected[code] {
		return
	}
	delete(m.modes, code)
	delete(m.pipelineDerived, code)
}

// PipelineIDForShortcut 返回已同步模式中绑定的流水线 ID（供中间件设置 X-Pipeline-ID）。
func (m *ModeManager) PipelineIDForShortcut(shortcutCode string) string {
	mode, exists := m.GetMode(shortcutCode)
	if !exists || mode.Config == nil {
		return ""
	}
	pid, _ := mode.Config["pipeline_id"].(string)
	return strings.TrimSpace(pid)
}