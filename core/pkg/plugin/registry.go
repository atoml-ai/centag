package plugin

import (
	"fmt"
	"sync"
)

// PluginFactory 插件工厂函数
type PluginFactory func() (Plugin, error)

// Registry 插件注册表
type Registry struct {
	mu       sync.RWMutex
	factory  map[string]PluginFactory // name -> factory
	metadata map[string]PluginMetadata
}

var (
	globalRegistry *Registry
	once           sync.Once
)

// GetRegistry 获取全局插件注册表
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			factory:  make(map[string]PluginFactory),
			metadata: make(map[string]PluginMetadata),
		}
	})
	return globalRegistry
}

// Register 注册插件工厂
func (r *Registry) Register(name string, metadata PluginMetadata, factory PluginFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factory[name]; exists {
		return fmt.Errorf("plugin factory %s already registered", name)
	}

	r.factory[name] = factory
	r.metadata[name] = metadata

	return nil
}

// Unregister 注销插件工厂
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.factory, name)
	delete(r.metadata, name)
}

// Create 创建插件实例
func (r *Registry) Create(name string) (Plugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factory[name]
	if !exists {
		return nil, fmt.Errorf("plugin factory %s not found", name)
	}

	return factory()
}

// List 列出所有已注册的插件
func (r *Registry) List() []PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metadata []PluginMetadata
	for _, m := range r.metadata {
		metadata = append(metadata, m)
	}

	return metadata
}

// ListByType 按类型列出插件
func (r *Registry) ListByType(ptype PluginType) []PluginMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metadata []PluginMetadata
	for _, m := range r.metadata {
		if m.Type == ptype {
			metadata = append(metadata, m)
		}
	}

	return metadata
}
