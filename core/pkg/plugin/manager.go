package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Manager 插件管理器
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin // name -> Plugin
	byType  map[PluginType][]string

	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager 创建插件管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		plugins: make(map[string]Plugin),
		byType:  make(map[PluginType][]string),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register 注册插件
func (m *Manager) Register(plugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()
	if _, exists := m.plugins[name]; exists {
		return &PluginError{
			PluginName: name,
			Operation:  "register",
			Err:        fmt.Errorf("plugin already registered"),
		}
	}

	m.plugins[name] = plugin

	// 按类型索引
	ptype := plugin.Type()
	m.byType[ptype] = append(m.byType[ptype], name)

	return nil
}

// Unregister 注销插件
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return &PluginError{
			PluginName: name,
			Operation:  "unregister",
			Err:        fmt.Errorf("plugin not found"),
		}
	}

	// 停止插件
	if err := plugin.Stop(m.ctx); err != nil {
		return &PluginError{
			PluginName: name,
			Operation:  "stop",
			Err:        err,
		}
	}

	// 从插件列表移除
	delete(m.plugins, name)

	// 从类型索引移除
	ptype := plugin.Type()
	for i, n := range m.byType[ptype] {
		if n == name {
			m.byType[ptype] = append(m.byType[ptype][:i], m.byType[ptype][i+1:]...)
			break
		}
	}

	return nil
}

// Get 获取插件
func (m *Manager) Get(name string) (Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return nil, &PluginError{
			PluginName: name,
			Operation:  "get",
			Err:        fmt.Errorf("plugin not found"),
		}
	}

	return plugin, nil
}

// GetByType 按类型获取插件
func (m *Manager) GetByType(ptype PluginType) []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var plugins []Plugin
	for _, name := range m.byType[ptype] {
		if plugin, ok := m.plugins[name]; ok {
			plugins = append(plugins, plugin)
		}
	}

	return plugins
}

// GetProtocol 获取协议插件
func (m *Manager) GetProtocol(name string) (ProtocolPlugin, error) {
	plugin, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	protocolPlugin, ok := plugin.(ProtocolPlugin)
	if !ok {
		return nil, &PluginError{
			PluginName: name,
			Operation:  "get protocol",
			Err:        fmt.Errorf("plugin is not a protocol plugin"),
		}
	}

	return protocolPlugin, nil
}

// GetBackend 获取后端插件
func (m *Manager) GetBackend(name string) (BackendPlugin, error) {
	plugin, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	backendPlugin, ok := plugin.(BackendPlugin)
	if !ok {
		return nil, &PluginError{
			PluginName: name,
			Operation:  "get backend",
			Err:        fmt.Errorf("plugin is not a backend plugin"),
		}
	}

	return backendPlugin, nil
}

// List 列出所有插件
func (m *Manager) List() []PluginMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var metadata []PluginMetadata
	for _, plugin := range m.plugins {
		metadata = append(metadata, PluginMetadata{
			Name:        plugin.Name(),
			Type:        plugin.Type(),
			Version:     plugin.Version(),
			Status:      plugin.Status(),
		})
	}

	return metadata
}

// ListByType 按类型列出插件
func (m *Manager) ListByType(ptype PluginType) []PluginMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var metadata []PluginMetadata
	for _, name := range m.byType[ptype] {
		if plugin, ok := m.plugins[name]; ok {
			metadata = append(metadata, PluginMetadata{
				Name:    plugin.Name(),
				Type:    plugin.Type(),
				Version: plugin.Version(),
				Status:  plugin.Status(),
			})
		}
	}

	return metadata
}

// Init 初始化所有插件
func (m *Manager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, plugin := range m.plugins {
		if err := plugin.Init(nil); err != nil {
			return &PluginError{
				PluginName: name,
				Operation:  "init",
				Err:        err,
			}
		}
	}

	return nil
}

// Start 启动所有插件
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, plugin := range m.plugins {
		if err := plugin.Start(m.ctx); err != nil {
			return &PluginError{
				PluginName: name,
				Operation:  "start",
				Err:        err,
			}
		}
	}

	return nil
}

// Stop 停止所有插件
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 取消上下文
	m.cancel()

	// 停止所有插件
	for name, plugin := range m.plugins {
		if err := plugin.Stop(m.ctx); err != nil {
			return &PluginError{
				PluginName: name,
				Operation:  "stop",
				Err:        err,
			}
		}
	}

	return nil
}

// Count 获取插件数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.plugins)
}
