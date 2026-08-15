package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ManifestSource 是 skill manifest 的来源描述。
type ManifestSource struct {
	// Dir 是 manifest 目录（personal 文件版）。为空表示非文件源。
	Dir string
	// ManifestData 是内存中的 manifest 集合（team system_config / 测试注入）。
	// key 为 manifest 文件名（不含扩展名），value 为 YAML 内容。
	ManifestData map[string][]byte
	// Custom 标记该来源是否属于用户自定义（Custom=true 时注册结果 custom=true）。
	Custom bool
}

// SkillPluginRegistry skill 插件注册表。
// 启动时扫描内置/自定义 manifest 并注册 SkillPlugin，作为 ListSkills/CRUD/路由的数据源。
type SkillPluginRegistry struct {
	mu      sync.RWMutex
	plugins map[string]SkillPlugin
	order   []string // 保持注册顺序，保证列表稳定
}

// NewSkillPluginRegistry 创建空注册表。
func NewSkillPluginRegistry() *SkillPluginRegistry {
	return &SkillPluginRegistry{
		plugins: make(map[string]SkillPlugin),
	}
}

// Register 注册 skill 插件。重名返回错误。
func (r *SkillPluginRegistry) Register(p SkillPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.GetSkillDefinition().Name
	if name == "" {
		return fmt.Errorf("register skill plugin: empty skill name")
	}
	if _, ok := r.plugins[name]; ok {
		return fmt.Errorf("register skill plugin: duplicate skill %q", name)
	}
	r.plugins[name] = p
	r.order = append(r.order, name)
	return nil
}

// Delete 删除 skill 插件（内置保护由调用方判断）。
func (r *SkillPluginRegistry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.plugins[name]; !ok {
		return fmt.Errorf("delete skill plugin: skill %q not found", name)
	}
	delete(r.plugins, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	return nil
}

// Replace 覆盖已存在的 skill 插件（更新场景，保持注册顺序不变）。
// 插件不存在时返回错误。
func (r *SkillPluginRegistry) Replace(p SkillPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.GetSkillDefinition().Name
	if name == "" {
		return fmt.Errorf("replace skill plugin: empty skill name")
	}
	if _, ok := r.plugins[name]; !ok {
		return fmt.Errorf("replace skill plugin: skill %q not found", name)
	}
	r.plugins[name] = p
	return nil
}

// Get 获取 skill 插件。
func (r *SkillPluginRegistry) Get(name string) (SkillPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List 列出全部启用的 skill 插件（保持注册顺序）。
func (r *SkillPluginRegistry) List() []SkillPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []SkillPlugin
	for _, name := range r.order {
		if p, ok := r.plugins[name]; ok && p.Enabled() {
			result = append(result, p)
		}
	}
	return result
}

// ListAll 列出全部 skill 插件（含禁用，保持注册顺序）。
func (r *SkillPluginRegistry) ListAll() []SkillPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []SkillPlugin
	for _, name := range r.order {
		if p, ok := r.plugins[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// IsSkillAllowed 检查 skill 是否允许使用。
func (r *SkillPluginRegistry) IsSkillAllowed(name string, internalOnly bool) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	if !ok {
		return false
	}
	if internalOnly && !p.Internal() {
		return false
	}
	return p.Enabled()
}

// LoadFromSources 从多个 manifest 来源加载并注册。
// 单个 manifest 解析失败记录 error（source 级失败不中断整体加载，由调用方决定是否致命）。
func (r *SkillPluginRegistry) LoadFromSources(sources []ManifestSource) error {
	for _, src := range sources {
		if len(src.Dir) > 0 {
			if err := r.loadDir(src.Dir, src.Custom); err != nil {
				return fmt.Errorf("load skill manifests from %s: %w", src.Dir, err)
			}
		}
		for name, data := range src.ManifestData {
			// 跳过非 skill manifest（防御：源中混入 pipeline 模板等）。
			if !bytes.Contains(data, []byte("kind: agent.skill")) {
				continue
			}
			p, err := ParseSkillPluginManifest(data)
			if err != nil {
				return fmt.Errorf("load skill manifest %s: %w", name, err)
			}
			if src.Custom {
				// 自定义 skill 对普通用户可见：强制 internal=false
				if cp, ok := p.(*skillPlugin); ok {
					cp.internal = false
				}
				if err := r.registerCustom(p); err != nil {
					return fmt.Errorf("load skill manifest %s: %w", name, err)
				}
			} else if err := r.Register(p); err != nil {
				return fmt.Errorf("load skill manifest %s: %w", name, err)
			}
		}
	}
	return nil
}

// registerCustom 注册自定义 skill：内置同名冲突时以自定义覆盖（升级语义）。
// 自定义 skill 通过 skill name 去重，覆盖同名内置定义。
func (r *SkillPluginRegistry) registerCustom(p SkillPlugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.GetSkillDefinition().Name
	if name == "" {
		return fmt.Errorf("register custom skill: empty skill name")
	}
	if _, ok := r.plugins[name]; !ok {
		r.order = append(r.order, name)
	}
	r.plugins[name] = p
	return nil
}

// loadDir 扫描目录下 agent-skill-*.yaml 并注册。
// 目录中可能混放 pipeline 模板等其他 YAML，一律以 agent-skill- 前缀过滤。
func (r *SkillPluginRegistry) loadDir(dir string, custom bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		if !strings.HasPrefix(name, "agent-skill-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		// 跳过同目录下的 pipeline 模板（如 agent-skill-router.yaml，schema 非 skill manifest）。
		if !bytes.Contains(data, []byte("kind: agent.skill")) {
			continue
		}
		p, err := ParseSkillPluginManifest(data)
		if err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		if custom {
			// 自定义 skill 对普通用户可见：强制 internal=false
			if cp, ok := p.(*skillPlugin); ok {
				cp.internal = false
			}
			if err := r.registerCustom(p); err != nil {
				return fmt.Errorf("register %s: %w", name, err)
			}
		} else if err := r.Register(p); err != nil {
			return fmt.Errorf("register %s: %w", name, err)
		}
	}
	return nil
}

// Names 返回已注册 skill 名（保持注册顺序）。
func (r *SkillPluginRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.order))
	for _, n := range r.order {
		if _, ok := r.plugins[n]; ok {
			out = append(out, n)
		}
	}
	return out
}
