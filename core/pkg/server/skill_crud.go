package server

import (
	"fmt"
	"net/http"
	"strings"

	"centag/core/internal/agent/skills"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// skillForm 自定义 skill 表单（方案 §5.2）。
type skillForm struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tools       []string `json:"tools"`
	Steps       []string `json:"steps"`
	SystemPrompt string   `json:"system_prompt"`
}

// normalizeSkillName 规范 skill 注册名：小写、非法字符转 -、去 agent-skill- 前缀。
func normalizeSkillName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimPrefix(name, "agent-skill-")
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// buildCustomSkillPlugin 由表单构建 SkillPlugin。
// implementation = custom.agent-skill-<name>，pipeline id 由去前缀推导（agent-skill-<name>）。
// internal 参数保留原 skill 的内置属性（内置 skill 直接更新时保持 internal=true，仍受删除保护）。
func buildCustomSkillPlugin(f skillForm, internal bool) (skills.SkillPlugin, error) {
	name := normalizeSkillName(f.Name)
	if name == "" {
		return nil, fmt.Errorf("skill name is empty")
	}
	manifest := fmt.Sprintf(`api_version: centag.agent-skill/v1alpha1
implementation: custom.agent-skill-%s
name: %s
kind: agent.skill
version: 1.0.0
description: %s
skill:
  name: %s
  category: %s
  enabled: true
  internal: %t
  tools:
%s
  steps:
%s
  system_prompt: |-
%s
`, name, f.Name, f.Description, name, f.Category, internal, yamlList(f.Tools), yamlList(f.Steps), yamlBlock(f.SystemPrompt))
	return skills.ParseSkillPluginManifest([]byte(manifest))
}

// yamlList 生成 yaml 列表块。
func yamlList(items []string) string {
	if len(items) == 0 {
		return "    []"
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString("    - " + it + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// yamlBlock 生成缩进的块标量内容（每行前置 4 空格）。
func yamlBlock(content string) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// CreateSkill POST /api/v1/builtin-agent/skills 创建自定义 skill。
func (h *BuiltinAgentHandler) CreateSkill(c *gin.Context) {
	if h.manifestStore == nil || h.skillPluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "skill plugin not initialized"})
		return
	}
	var f skillForm
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := normalizeSkillName(f.Name)
	if _, ok := h.skillPluginRegistry.Get(name); ok {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("skill %q already exists", name)})
		return
	}
	p, err := buildCustomSkillPlugin(f, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.registerCustomSkill(p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Infof("[builtin-agent] custom skill created: %s", name)
	c.JSON(http.StatusCreated, gin.H{"skill": name, "pipeline_id": p.PipelineID()})
}

// UpdateSkill PUT /api/v1/builtin-agent/skills/:name 更新 skill。
// 内置 skill 允许直接更新（保留 internal=true，仍受删除保护，见 buildCustomSkillPlugin）。
func (h *BuiltinAgentHandler) UpdateSkill(c *gin.Context) {
	if h.manifestStore == nil || h.skillPluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "skill plugin not initialized"})
		return
	}
	name := c.Param("name")
	p, ok := h.skillPluginRegistry.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("skill %q not found", name)})
		return
	}
	var f skillForm
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	f.Name = name
	np, err := buildCustomSkillPlugin(f, p.Internal())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.registerCustomSkill(np); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Infof("[builtin-agent] custom skill updated: %s", name)
	c.JSON(http.StatusOK, gin.H{"skill": name, "pipeline_id": np.PipelineID()})
}

// DeleteSkill DELETE /api/v1/builtin-agent/skills/:name 删除自定义 skill（内置返回 403）。
func (h *BuiltinAgentHandler) DeleteSkill(c *gin.Context) {
	if h.manifestStore == nil || h.skillPluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "skill plugin not initialized"})
		return
	}
	name := c.Param("name")
	p, ok := h.skillPluginRegistry.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("skill %q not found", name)})
		return
	}
	if p.Internal() {
		c.JSON(http.StatusForbidden, gin.H{"error": "builtin skill cannot be deleted"})
		return
	}
	if err := h.deleteCustomSkill(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Infof("[builtin-agent] custom skill deleted: %s", name)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("skill %q deleted", name)})
}

// CloneSkill POST /api/v1/builtin-agent/skills/:name/clone 复制 skill 进入自定义态（internal: false）。
func (h *BuiltinAgentHandler) CloneSkill(c *gin.Context) {
	if h.manifestStore == nil || h.skillPluginRegistry == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "skill plugin not initialized"})
		return
	}
	src := c.Param("name")
	p, ok := h.skillPluginRegistry.Get(src)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("skill %q not found", src)})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		// 允许空 body（默认 <src>-copy）
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	target := strings.TrimSpace(req.Name)
	if target == "" {
		target = src + "-copy"
	}
	target = normalizeSkillName(target)
	if _, ok := h.skillPluginRegistry.Get(target); ok {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("skill %q already exists", target)})
		return
	}

	def := p.GetSkillDefinition()
	desc := p.Descriptor()
	f := skillForm{
		Name:         target,
		Description:  desc.Description,
		Category:     def.Category,
		Tools:        def.Tools,
		Steps:        def.Steps,
		SystemPrompt: def.SystemPrompt,
	}
	np, err := buildCustomSkillPlugin(f, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.registerCustomSkill(np); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	logger.Infof("[builtin-agent] skill %s cloned to %s", src, target)
	c.JSON(http.StatusCreated, gin.H{"skill": target, "pipeline_id": np.PipelineID()})
}

// registerCustomSkill 注册自定义 skill：写入 manifest 存储 → 注册到注册表 → 生成/注册 pipeline。
// 已存在同名 skill 时覆盖（更新/复制场景）。
func (h *BuiltinAgentHandler) registerCustomSkill(p skills.SkillPlugin) error {
	name := p.GetSkillDefinition().Name
	data, err := skills.MarshalSkillPluginManifest(p)
	if err != nil {
		return fmt.Errorf("marshal skill manifest: %w", err)
	}
	if err := h.manifestStore.Save(name, data); err != nil {
		return fmt.Errorf("save skill manifest: %w", err)
	}
	if _, exists := h.skillPluginRegistry.Get(name); exists {
		if err := h.skillPluginRegistry.Replace(p); err != nil {
			return fmt.Errorf("replace skill plugin: %w", err)
		}
	} else if err := h.skillPluginRegistry.Register(p); err != nil {
		return fmt.Errorf("register skill plugin: %w", err)
	}
	h.skillRegistry.RegisterSkill(skills.SkillFromPlugin(p))
	// 单一路由管线：skill 变更后重建 agent-skill-router（幂等覆盖，纳入新增/更新后的分支）
	if h.pipelineRegistry != nil {
		registerSkillRouter(h.skillPluginRegistry, h.pipelineRegistry, h.defaultBackend, h.defaultModel)
	}
	return nil
}

// deleteCustomSkill 删除自定义 skill：注册表 → manifest 存储 → 重建路由管线。
func (h *BuiltinAgentHandler) deleteCustomSkill(name string) error {
	if err := h.skillPluginRegistry.Delete(name); err != nil {
		return fmt.Errorf("delete skill plugin: %w", err)
	}
	if err := h.manifestStore.Delete(name); err != nil {
		return fmt.Errorf("delete skill manifest: %w", err)
	}
	// 删除后重建 agent-skill-router（移除对应分支；无剩余 skill 时注册表为空，router 不再注册）
	if h.pipelineRegistry != nil {
		registerSkillRouter(h.skillPluginRegistry, h.pipelineRegistry, h.defaultBackend, h.defaultModel)
	}
	return nil
}
