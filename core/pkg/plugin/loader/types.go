package loader

import (
	"context"
	"time"

	"centag/core/pkg/pipeline"
)

// PluginPackage 插件包
type PluginPackage struct {
	Manifest    *PluginManifest   `json:"manifest"`
	Code        []byte            `json:"code"`
	Resources   map[string][]byte `json:"resources"`
	Checksum    string            `json:"checksum"`
	Signature   string            `json:"signature"`
}

// PluginManifest 插件清单
type PluginManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Kind        string            `json:"kind"` // generator, processor, reviewer, router
	
	// 入口点
	Entrypoint  string            `json:"entrypoint"`
	Runtime     string            `json:"runtime"` // go, python, wasm
	
	// 依赖
	Dependencies []Dependency     `json:"dependencies"`
	
	// 权限
	Permissions  []string         `json:"permissions"`
	
	// 资源限制
	Resources    ResourceLimits   `json:"resources"`
	
	// 配置
	Config       map[string]interface{} `json:"config"`
}

// Dependency 依赖项
type Dependency struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	MaxMemoryMB int     `json:"max_memory_mb"`
	MaxCPUPercent float64 `json:"max_cpu_percent"`
	TimeoutSec  int     `json:"timeout_sec"`
}

// PluginState 插件状态
type PluginState string

const (
	StateUnknown    PluginState = "unknown"
	StateLoading    PluginState = "loading"
	StateLoaded     PluginState = "loaded"
	StateValidating PluginState = "validating"
	StateValidated  PluginState = "validated"
	StateStarting   PluginState = "starting"
	StateRunning    PluginState = "running"
	StateStopping   PluginState = "stopping"
	StateStopped    PluginState = "stopped"
	StateUnloading  PluginState = "unloading"
	StateUnloaded   PluginState = "unloaded"
	StateError      PluginState = "error"
)

// IsValidTransition 检查状态转换是否有效
func IsValidTransition(from, to PluginState) bool {
	transitions := map[PluginState][]PluginState{
		StateUnknown:    {StateLoading},
		StateLoading:    {StateLoaded, StateError},
		StateLoaded:     {StateValidating, StateUnloading},
		StateValidating: {StateValidated, StateError},
		StateValidated:  {StateStarting, StateUnloading},
		StateStarting:   {StateRunning, StateError},
		StateRunning:    {StateStopping, StateUnloading},
		StateStopping:   {StateStopped, StateError},
		StateStopped:    {StateStarting, StateUnloading},
		StateUnloading:  {StateUnloaded, StateError},
		StateError:      {StateLoading, StateUnloading},
	}
	
	valid, ok := transitions[from]
	if !ok {
		return false
	}
	
	for _, s := range valid {
		if s == to {
			return true
		}
	}
	return false
}

// ManagedPlugin 被管理的插件
type ManagedPlugin struct {
	ID          string                 `json:"id"`
	Manifest    *PluginManifest        `json:"manifest"`
	State       PluginState            `json:"state"`
	Instance    pipeline.NodePlugin    `json:"-"`
	
	// 运行时信息
	StartTime   time.Time              `json:"start_time"`
	LastError   string                 `json:"last_error"`
	RestartCount int                   `json:"restart_count"`
	
	// 版本信息
	Version     string                 `json:"version"`
	PreviousVersion string             `json:"previous_version"`
	
	// 资源使用
	MemoryUsageMB int64              `json:"memory_usage_mb"`
	CPUUsagePercent float64           `json:"cpu_usage_percent"`
}

// LoadRequest 加载请求
type LoadRequest struct {
	Source      string            `json:"source"` // url, file, registry
	URL         string            `json:"url,omitempty"`
	FilePath    string            `json:"file_path,omitempty"`
	PluginID    string            `json:"plugin_id,omitempty"`
	Version     string            `json:"version,omitempty"`
	
	// 验证选项
	VerifyChecksum  bool          `json:"verify_checksum"`
	VerifySignature bool          `json:"verify_signature"`
	
	// 启动选项
	AutoStart     bool            `json:"auto_start"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

// UpdateRequest 更新请求
type UpdateRequest struct {
	PluginID        string            `json:"plugin_id"`
	NewVersion      string            `json:"new_version"`
	Strategy        UpdateStrategy    `json:"strategy"` // blue-green, canary, rolling
	CanaryPercent   int               `json:"canary_percent"` // 0-100
	RollbackOnError bool              `json:"rollback_on_error"`
}

// UpdateStrategy 更新策略
type UpdateStrategy string

const (
	StrategyBlueGreen UpdateStrategy = "blue-green"
	StrategyCanary    UpdateStrategy = "canary"
	StrategyRolling   UpdateStrategy = "rolling"
)

// UpdateStatus 更新状态
type UpdateStatus struct {
	UpdateID      string            `json:"update_id"`
	PluginID      string            `json:"plugin_id"`
	FromVersion   string            `json:"from_version"`
	ToVersion     string            `json:"to_version"`
	Strategy      UpdateStrategy    `json:"strategy"`
	State         string            `json:"state"` // pending, in_progress, completed, failed, rolled_back
	Progress      int               `json:"progress"` // 0-100
	StartTime     time.Time         `json:"start_time"`
	EndTime       *time.Time        `json:"end_time,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// LifecycleEvent 生命周期事件
type LifecycleEvent struct {
	PluginID    string            `json:"plugin_id"`
	FromState   PluginState       `json:"from_state"`
	ToState     PluginState       `json:"to_state"`
	Timestamp   time.Time         `json:"timestamp"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// LifecycleListener 生命周期监听器
type LifecycleListener interface {
	OnStateChanged(event *LifecycleEvent)
}

// Loader 插件加载器接口
type Loader interface {
	Load(ctx context.Context, req *LoadRequest) (*ManagedPlugin, error)
	Unload(ctx context.Context, pluginID string) error
	Get(pluginID string) (*ManagedPlugin, error)
	List() []*ManagedPlugin
	Start(pluginID string) error
	Stop(pluginID string) error
	
	// 更新
	Update(ctx context.Context, req *UpdateRequest) (*UpdateStatus, error)
	GetUpdateStatus(updateID string) (*UpdateStatus, error)
	Rollback(updateID string) error
	
	// 事件监听
	AddListener(listener LifecycleListener)
	RemoveListener(listener LifecycleListener)
}
