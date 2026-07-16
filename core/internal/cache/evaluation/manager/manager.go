package manager

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"centag/core/internal/cache/evaluation/plugin"
)

// Manager 插件管理器
type Manager struct {
	plugins map[string]plugin.EvaluatorPlugin
	order   []string
	enabled map[string]bool
	stats   *Stats
	mu      sync.RWMutex
}

// Stats 统计信息
type Stats struct {
	TotalExecutions   int64            `json:"total_executions"`
	EnabledPlugins    int              `json:"enabled_plugins"`
	PluginExecTimes   map[string]int64 `json:"plugin_exec_times_ms"`
	LastExecutionTime time.Time        `json:"last_execution_time"`
}

// PipelineResult 流水线结果
type PipelineResult struct {
	Results     map[string]*plugin.EvalOutput `json:"results"`
	FinalOutput *plugin.EvalOutput            `json:"final_output"`
}

// NewManager 创建插件管理器
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]plugin.EvaluatorPlugin),
		order:   make([]string, 0),
		enabled: make(map[string]bool),
		stats: &Stats{
			PluginExecTimes: make(map[string]int64),
		},
	}
}

// Register 注册插件
func (m *Manager) Register(p plugin.EvaluatorPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	// 初始化插件
	if err := p.Init(); err != nil {
		return fmt.Errorf("failed to init plugin %s: %w", name, err)
	}

	m.plugins[name] = p
	m.order = append(m.order, name)
	m.enabled[name] = true

	return nil
}

// Unregister 注销插件
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// 关闭插件
	if err := p.Close(); err != nil {
		return fmt.Errorf("failed to close plugin %s: %w", name, err)
	}

	delete(m.plugins, name)
	delete(m.enabled, name)

	// 从顺序中移除
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}

	return nil
}

// Get 获取插件
func (m *Manager) Get(name string) (plugin.EvaluatorPlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	return p, nil
}

// ListPlugins 列出所有插件
func (m *Manager) ListPlugins() []plugin.EvaluatorPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]plugin.EvaluatorPlugin, 0, len(m.plugins))
	for _, name := range m.order {
		if p, exists := m.plugins[name]; exists {
			result = append(result, p)
		}
	}

	return result
}

// Enable 启用插件
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	m.enabled[name] = true
	return nil
}

// Disable 禁用插件
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[name]; !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	m.enabled[name] = false
	return nil
}

// IsEnabled 检查插件是否启用
func (m *Manager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.enabled[name]
}

// HasEnabledPlugins 检查是否有任何启用的插件
func (m *Manager) HasEnabledPlugins() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name := range m.enabled {
		if m.enabled[name] {
			return true
		}
	}
	return false
}

// UpdateOrder 更新插件执行顺序
func (m *Manager) UpdateOrder(order []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证所有插件都存在
	for _, name := range order {
		if _, exists := m.plugins[name]; !exists {
			return fmt.Errorf("plugin %s not found", name)
		}
	}

	m.order = order
	return nil
}

// SetConfig 设置插件配置
func (m *Manager) SetConfig(name string, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	return p.SetConfig(config)
}

// ExecutePipeline 执行评估流水线
func (m *Manager) ExecutePipeline(ctx context.Context, input *plugin.EvalInput) (*PipelineResult, error) {
	m.mu.RLock()
	plugins := make([]plugin.EvaluatorPlugin, 0)
	for _, name := range m.order {
		if m.enabled[name] {
			if p, exists := m.plugins[name]; exists {
				plugins = append(plugins, p)
			}
		}
	}
	m.mu.RUnlock()

	results := make(map[string]*plugin.EvalOutput)
	var finalOutput *plugin.EvalOutput

	start := time.Now()

	// 按顺序执行插件
	for _, p := range plugins {
		output, err := p.Evaluate(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("plugin %s evaluation failed: %w", p.Name(), err)
		}

		results[p.Name()] = output
		finalOutput = output

		// 更新执行时间统计
		m.mu.Lock()
		m.stats.PluginExecTimes[p.Name()] += output.ProcessTimeMs
		m.mu.Unlock()
	}

	// 更新统计
	m.mu.Lock()
	m.stats.TotalExecutions++
	m.stats.LastExecutionTime = time.Now()
	m.stats.EnabledPlugins = len(plugins)
	m.mu.Unlock()

	_ = time.Since(start)

	return &PipelineResult{
		Results:     results,
		FinalOutput: finalOutput,
	}, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	stats := &Stats{
		TotalExecutions:   m.stats.TotalExecutions,
		EnabledPlugins:    m.stats.EnabledPlugins,
		PluginExecTimes:   make(map[string]int64),
		LastExecutionTime: m.stats.LastExecutionTime,
	}

	for k, v := range m.stats.PluginExecTimes {
		stats.PluginExecTimes[k] = v
	}

	return stats
}

// ResetStats 重置统计信息
func (m *Manager) ResetStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = &Stats{
		PluginExecTimes: make(map[string]int64),
	}
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        plugin.PluginType `json:"type"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Order       int               `json:"order"`
}

// GetPluginInfos 获取插件信息列表
func (m *Manager) GetPluginInfos() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(m.plugins))
	for i, name := range m.order {
		if p, exists := m.plugins[name]; exists {
			infos = append(infos, PluginInfo{
				Name:        p.Name(),
				Version:     p.Version(),
				Type:        p.Type(),
				Description: p.Description(),
				Enabled:     m.enabled[name],
				Order:       i + 1,
			})
		}
	}

	return infos
}

// SortByOrder 按执行顺序排序插件名称
func (m *Manager) SortByOrder(names []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 创建名称到顺序的映射
	orderMap := make(map[string]int)
	for i, name := range m.order {
		orderMap[name] = i
	}

	// 排序
	sort.Slice(names, func(i, j int) bool {
		oi, ei := orderMap[names[i]]
		oj, ej := orderMap[names[j]]

		// 如果都不在顺序中，保持原序
		if !ei && !ej {
			return i < j
		}
		// 只有一个在顺序中，在顺序的排在前面
		if !ei {
			return false
		}
		if !ej {
			return true
		}
		// 都在顺序中，按顺序比较
		return oi < oj
	})

	return names
}

// IsEnabled 检查评估是否启用

// ExecuteForCache 执行评估流水线（为缓存评估适配）
func (m *Manager) ExecuteForCache(ctx context.Context, question, answer string, historyMessages []plugin.Message) (bool, float64, error) {
	input := &plugin.EvalInput{
		Question:         question,
		OriginalQuestion: question,
		Answer:           answer,
		HistoryMessages:  historyMessages,
		IsExpanded:       false,
	}

	result, err := m.ExecutePipeline(ctx, input)
	if err != nil {
		return false, 0, err
	}

	return result.FinalOutput.Passed, result.FinalOutput.Score, nil
}

// Execute 执行评估流水线（为缓存评估适配，直接返回shouldCache和score）
func (m *Manager) Execute(ctx context.Context, input *plugin.EvalInput) (*plugin.EvalOutput, error) {
	result, err := m.ExecutePipeline(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.FinalOutput, nil
}

