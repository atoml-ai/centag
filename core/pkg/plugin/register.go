// Package plugin 提供插件注册和发现机制。
//
// 插件通过 init() 调用 Register* 函数注册工厂函数，
// 框架在启动时通过 Get* 函数获取已注册的插件并初始化。
//
// 使用示例：
//
//	// 插件方（plugins/backend-openai/openai.go）
//	func init() {
//	    plugin.RegisterBackend("openai", NewOpenAIBackend)
//	}
//
//	// 框架方（core/internal/server/server.go）
//	factory, ok := plugin.GetBackend("openai")
//	if ok {
//	    backend, _ := factory(config)
//	}
package plugin

import (
	"fmt"
	"sync"
)

// PluginType、TypeProtocol 等类型常量定义在 interface.go 中。
// 这里只定义工厂函数类型和注册表。

// ProtocolFactory 协议插件工厂函数类型。
type ProtocolFactory func(config map[string]interface{}) (interface{}, error)

// BackendFactory 后端插件工厂函数类型。
type BackendFactory func(config map[string]interface{}) (interface{}, error)

// StorageFactory 存储插件工厂函数类型。
type StorageFactory func(config map[string]interface{}) (interface{}, error)

// BusinessPluginFactory 业务插件工厂函数类型。
// 业务插件的注册签名与标准插件不同，需要接收 nodeRegistry 和 bizRegistry。
type BusinessPluginFactory func(nodeRegistry interface{}, bizRegistry interface{}) error

// 全局注册表（并发安全）
var (
	protocolRegistry  = &sync.Map{}
	backendRegistry   = &sync.Map{}
	storageRegistry   = &sync.Map{}
	businessRegistry  = &sync.Map{}
)

// RegisterProtocol 注册协议插件工厂函数。
func RegisterProtocol(name string, factory ProtocolFactory) {
	if _, loaded := protocolRegistry.LoadOrStore(name, factory); loaded {
		panic(fmt.Sprintf("protocol plugin %q already registered", name))
	}
}

// GetProtocol 获取已注册的协议插件工厂函数。
func GetProtocol(name string) (ProtocolFactory, bool) {
	f, ok := protocolRegistry.Load(name)
	if !ok {
		return nil, false
	}
	return f.(ProtocolFactory), true
}

// ListProtocols 列出所有已注册的协议插件名称。
func ListProtocols() []string {
	return listKeys(protocolRegistry)
}

// RegisterBackend 注册后端插件工厂函数。
func RegisterBackend(name string, factory BackendFactory) {
	if _, loaded := backendRegistry.LoadOrStore(name, factory); loaded {
		panic(fmt.Sprintf("backend plugin %q already registered", name))
	}
}

// GetBackend 获取已注册的后端插件工厂函数。
func GetBackend(name string) (BackendFactory, bool) {
	f, ok := backendRegistry.Load(name)
	if !ok {
		return nil, false
	}
	return f.(BackendFactory), true
}

// ListBackends 列出所有已注册的后端插件名称。
func ListBackends() []string {
	return listKeys(backendRegistry)
}

// RegisterStorage 注册存储插件工厂函数。
func RegisterStorage(name string, factory StorageFactory) {
	if _, loaded := storageRegistry.LoadOrStore(name, factory); loaded {
		panic(fmt.Sprintf("storage plugin %q already registered", name))
	}
}

// GetStorage 获取已注册的存储插件工厂函数。
func GetStorage(name string) (StorageFactory, bool) {
	f, ok := storageRegistry.Load(name)
	if !ok {
		return nil, false
	}
	return f.(StorageFactory), true
}

// ListStorages 列出所有已注册的存储插件名称。
func ListStorages() []string {
	return listKeys(storageRegistry)
}

// RegisterBusinessPlugin 注册业务插件工厂函数。
func RegisterBusinessPlugin(name string, factory BusinessPluginFactory) {
	if _, loaded := businessRegistry.LoadOrStore(name, factory); loaded {
		panic(fmt.Sprintf("business plugin %q already registered", name))
	}
}

// GetBusinessPlugin 获取已注册的业务插件工厂函数。
func GetBusinessPlugin(name string) (BusinessPluginFactory, bool) {
	f, ok := businessRegistry.Load(name)
	if !ok {
		return nil, false
	}
	return f.(BusinessPluginFactory), true
}

// ListBusinessPlugins 列出所有已注册的业务插件名称。
func ListBusinessPlugins() []string {
	return listKeys(businessRegistry)
}

// listKeys 从 sync.Map 中提取所有 key。
func listKeys(m *sync.Map) []string {
	var keys []string
	m.Range(func(key, _ interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}
