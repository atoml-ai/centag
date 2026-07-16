package plugin

import (
	"context"
	"fmt"
)

// Plugin 插件基础接口
type Plugin interface {
	// Name 返回插件名称
	Name() string

	// Type 返回插件类型
	Type() PluginType

	// Version 返回插件版本
	Version() string

	// Init 初始化插件
	Init(config any) error

	// Start 启动插件
	Start(ctx context.Context) error

	// Stop 停止插件
	Stop(ctx context.Context) error

	// Status 返回插件状态
	Status() PluginStatus
}

// PluginType 插件类型
type PluginType string

const (
	// TypeProtocol 协议插件 - 处理客户端请求解析
	TypeProtocol PluginType = "protocol"

	// TypeBackend 后端插件 - 连接到大模型服务
	TypeBackend PluginType = "backend"

	// TypeStorage 存储插件 - KV/向量存储
	TypeStorage PluginType = "storage"

	// TypeProcessor 处理插件 - 请求处理逻辑
	TypeProcessor PluginType = "processor"

	// TypeDatabase 数据库插件 - 数据库驱动
	TypeDatabase PluginType = "database"

	// TypeCacheStrategy 缓存策略插件 - 缓存匹配策略（exact/semantic/hybrid）
	TypeCacheStrategy PluginType = "cache_strategy"
)

// PluginStatus 插件状态
type PluginStatus string

const (
	StatusStopped  PluginStatus = "stopped"
	StatusStarting PluginStatus = "starting"
	StatusRunning  PluginStatus = "running"
	StatusError    PluginStatus = "error"
)

// PluginMetadata 插件元数据
type PluginMetadata struct {
	Name        string       `json:"name"`
	Type        PluginType   `json:"type"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	Status      PluginStatus `json:"status"`
	Config      any          `json:"config,omitempty"`
}

// PluginError 插件错误
type PluginError struct {
	PluginName string
	Operation  string
	Err        error
}

func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s: %s failed: %v", e.PluginName, e.Operation, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}
