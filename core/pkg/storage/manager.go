package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// StoragePluginFactory 存储插件工厂函数类型
type StoragePluginFactory func(config map[string]interface{}) (StoragePlugin, error)

// PluginRegistry 插件注册器
var pluginRegistry = struct {
	sync.RWMutex
	factories map[StorageType]StoragePluginFactory
}{
	factories: make(map[StorageType]StoragePluginFactory),
}

// RegisterPlugin 注册存储插件工厂
func RegisterPlugin(storageType StorageType, factory StoragePluginFactory) {
	pluginRegistry.Lock()
	defer pluginRegistry.Unlock()
	pluginRegistry.factories[storageType] = factory
}

// ListRegisteredTypes returns storage plugin types registered via init().
func ListRegisteredTypes() []StorageType {
	pluginRegistry.RLock()
	defer pluginRegistry.RUnlock()
	types := make([]StorageType, 0, len(pluginRegistry.factories))
	for t := range pluginRegistry.factories {
		types = append(types, t)
	}
	return types
}

// GetPluginFactory 获取插件工厂
func getPluginFactory(storageType StorageType) (StoragePluginFactory, error) {
	pluginRegistry.RLock()
	defer pluginRegistry.RUnlock()

	factory, ok := pluginRegistry.factories[storageType]
	if !ok {
		return nil, fmt.Errorf("no plugin factory registered for type: %s", storageType)
	}
	return factory, nil
}

// KVStoreChangeCallback KV存储变更回调函数类型
type KVStoreChangeCallback func(kvStore KVStore)

// Manager 存储管理器
type Manager struct {
	mu           sync.RWMutex
	plugins      map[string]StoragePlugin  // name -> StoragePlugin
	kvStores     map[string]KVStore        // name -> KVStore
	vectorStores map[string]VectorStore    // name -> VectorStore
	dataStores   map[string]KnowledgeDataStore // name -> KnowledgeDataStore
	configPath   string
	storageCfg   *StorageConfig
	defaultKV    string                    // 默认KV存储名称
	kvStoreCb    KVStoreChangeCallback     // KV存储变更回调
}

// NewManager 创建存储管理器
func NewManager(configPath string) (*Manager, error) {
	return &Manager{
		plugins:      make(map[string]StoragePlugin),
		kvStores:     make(map[string]KVStore),
		vectorStores: make(map[string]VectorStore),
		dataStores:   make(map[string]KnowledgeDataStore),
		configPath:   configPath,
	}, nil
}

// LoadConfig 从全局运行时配置（由 database 加载）中读取存储配置。
func (m *Manager) LoadConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.storageCfg = &StorageConfig{}

	cfg := config.Get()
	if cfg != nil && len(cfg.Storages) > 0 {
		m.storageCfg.Storages = make([]StorageConfigItem, 0, len(cfg.Storages))
		for _, item := range cfg.Storages {
			m.storageCfg.Storages = append(m.storageCfg.Storages, StorageConfigItem{
				Name:        item.Name,
				Type:        StorageType(item.Type),
				Enabled:     item.Enabled,
				Config:      item.Config,
				Description: item.Description,
			})
		}
		m.storageCfg.DefaultKV = cfg.DefaultStorage

		for _, item := range m.storageCfg.Storages {
			if item.Enabled {
				if err := m.initStorage(&item); err != nil {
					logger.LogError("init storage on load", err, logger.GetField("name", item.Name))
				}
			}
		}

		if m.storageCfg.DefaultKV != "" {
			m.defaultKV = m.storageCfg.DefaultKV
			logger.Info("Default KV store loaded", logger.GetField("default_kv", m.defaultKV))
		}

		logger.Info("Storage config loaded", logger.GetField("storages", len(m.storageCfg.Storages)))
		return nil
	}

	// 配置为空，创建空占位结构
	m.storageCfg = &StorageConfig{
		Storages: []StorageConfigItem{},
	}
	logger.Info("No storage config found; starting with empty storage list")
	return nil
}

// SaveConfig 将当前存储配置持久化到数据库。
func (m *Manager) SaveConfig() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	globalCfg := config.Get()
	if globalCfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	storageConfigs := make([]config.StorageConfig, 0, len(m.storageCfg.Storages))
	for _, s := range m.storageCfg.Storages {
		storageConfigs = append(storageConfigs, config.StorageConfig{
			Name:        s.Name,
			Type:        string(s.Type),
			Enabled:     s.Enabled,
			Config:      s.Config,
			Description: s.Description,
		})
	}

	globalCfg.Storages = storageConfigs
	globalCfg.DefaultStorage = m.storageCfg.DefaultKV

	if err := config.SaveConfig(globalCfg); err != nil {
		logger.Error("Failed to save storage config to database",
			zap.Error(err),
			zap.Int("storage_count", len(storageConfigs)))
		return fmt.Errorf("failed to save storage config to database: %w", err)
	}

	logger.Info("Saved storage configs to database successfully",
		zap.Int("storage_count", len(storageConfigs)),
		zap.Strings("storage_names", func() []string {
			names := make([]string, len(storageConfigs))
			for i, s := range storageConfigs {
				names[i] = s.Name
			}
			return names
		}()))
	return nil
}

// AddStorage 添加存储配置
func (m *Manager) AddStorage(config *StorageConfigItem) error {
	m.mu.Lock()

	// 检查名称是否已存在
	for _, s := range m.storageCfg.Storages {
		if s.Name == config.Name {
			m.mu.Unlock()
			return fmt.Errorf("storage with name '%s' already exists", config.Name)
		}
	}

	// 添加到配置
	m.storageCfg.Storages = append(m.storageCfg.Storages, *config)

	// 初始化存储（如果启用）
	var initErr error
	if config.Enabled {
		m.mu.Unlock() // 临时释放锁以避免死锁
		initErr = m.initStorage(config)
		m.mu.Lock()
	}

	m.mu.Unlock()

	// 保存到数据库
	if err := m.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save storage config: %w", err)
	}

	logger.Info("Storage config saved to database")
	logger.Info("Storage added",
		logger.GetField("name", config.Name),
		logger.GetField("type", string(config.Type)))

	if initErr != nil {
		logger.Warn("Failed to init storage after add, storage saved but not active",
			logger.GetField("name", config.Name),
			zap.Error(initErr))
	}

	return nil
}

// UpdateStorage 更新存储配置
func (m *Manager) UpdateStorage(name string, config *StorageConfigItem) error {
	m.mu.Lock()

	// 查找并更新存储
	found := false
	var oldConfig *StorageConfigItem
	for i, s := range m.storageCfg.Storages {
		if s.Name == name {
			oldConfig = &s
			m.storageCfg.Storages[i] = *config
			found = true
			break
		}
	}

	if !found {
		m.mu.Unlock()
		return fmt.Errorf("storage '%s' not found", name)
	}

	// 如果禁用，先停止存储
	if oldConfig.Enabled && !config.Enabled {
		m.mu.Unlock()
		m.stopStorage(name)
		m.mu.Lock()
	}

	// 如果启用，重新初始化存储（无论之前是否启用，都重新初始化以应用新配置）
	var initErr error
	if config.Enabled {
		// 先停止旧的存储实例
		if oldConfig.Enabled {
			m.mu.Unlock()
			m.stopStorage(name)
			m.mu.Lock()
		}

		// 初始化新的存储实例
		m.mu.Unlock()
		initErr = m.initStorage(config)
		if initErr != nil {
			logger.Warn("Failed to init storage on update",
				logger.GetField("name", name),
				logger.GetField("error", initErr.Error()))
		}
		m.mu.Lock()
	}

	m.mu.Unlock()

	// 保存到数据库
	if err := m.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save storage config: %w", err)
	}

	logger.Info("Storage config saved to database")
	logger.Info("Storage updated", logger.GetField("name", name))

	// 如果初始化失败，返回警告但不阻止配置保存
	if initErr != nil {
		return fmt.Errorf("storage config saved but initialization failed: %w", initErr)
	}

	return nil
}

// DeleteStorage 删除存储配置
func (m *Manager) DeleteStorage(name string) error {
	m.mu.Lock()

	// 从配置中删除
	var storages []StorageConfigItem
	for _, s := range m.storageCfg.Storages {
		if s.Name != name {
			storages = append(storages, s)
		}
	}

	if len(storages) == len(m.storageCfg.Storages) {
		m.mu.Unlock()
		return fmt.Errorf("storage '%s' not found", name)
	}

	m.storageCfg.Storages = storages

	// 停止存储（需要先释放锁）
	m.mu.Unlock()
	m.stopStorage(name)

	// 保存到数据库
	if err := m.SaveConfig(); err != nil {
		return fmt.Errorf("failed to save storage config: %w", err)
	}

	logger.Info("Storage config saved to database")
	logger.Info("Storage deleted", logger.GetField("name", name))

	return nil
}

// GetStorage 获取存储
func (m *Manager) GetStorage(name string) (StoragePlugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	storage, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("storage '%s' not found", name)
	}

	return storage, nil
}

// GetKVStore 获取KV存储
func (m *Manager) GetKVStore(name string) (KVStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果没有指定名称，使用默认KV存储
	if name == "" {
		if m.defaultKV == "" {
			return nil, fmt.Errorf("no default kv store configured")
		}
		name = m.defaultKV
	}

	kvStore, exists := m.kvStores[name]
	if !exists {
		return nil, fmt.Errorf("kv store '%s' not found", name)
	}

	return kvStore, nil
}

// GetDefaultKVStore 获取默认KV存储
func (m *Manager) GetDefaultKVStore() (KVStore, error) {
	return m.GetKVStore("")
}

// GetVectorStore 获取向量存储。
// 当 name 为空时按优先级自动选取：postgresql → elasticsearch → chroma → 其他。
// PostgreSQL (pgvector) 优先：无需额外服务，利用已有的数据库连接。
func (m *Manager) GetVectorStore(name string) (VectorStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		// 按优先级选取向量存储
		preferredOrder := []StorageType{
			StorageTypePostgresql,
			StorageTypeElasticsearch,
			StorageTypeChroma,
		}
		for _, preferred := range preferredOrder {
			for storeName, store := range m.vectorStores {
				if storeType := m.getStoreType(storeName); storeType == string(preferred) {
					logger.Infof("Selected vector store: %s (type: %s)", storeName, preferred)
					return store, nil
				}
			}
		}
		// fallback: 返回任意可用的
		for _, store := range m.vectorStores {
			return store, nil
		}
		return nil, fmt.Errorf("no vector store available")
	}

	vectorStore, exists := m.vectorStores[name]
	if !exists {
		return nil, fmt.Errorf("vector store '%s' not found", name)
	}

	return vectorStore, nil
}

func (m *Manager) getStoreType(name string) string {
	if plugin, exists := m.plugins[name]; exists {
		return string(plugin.StorageType())
	}
	return ""
}

// SetDefaultKV 设置默认KV存储
func (m *Manager) SetDefaultKV(name string) error {
	m.mu.Lock()

	// 检查存储配置是否存在（允许设置未连接的存储为默认）
	found := false
	for _, s := range m.storageCfg.Storages {
		if s.Name == name {
			found = true
			break
		}
	}

	if !found {
		m.mu.Unlock()
		return fmt.Errorf("storage '%s' not found in configuration", name)
	}

	// 如果存储已连接，检查是否有 KV 能力
	if plugin, exists := m.plugins[name]; exists {
		if _, err := plugin.KVStore(); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("storage '%s' does not support KV operations", name)
		}
	}

	m.defaultKV = name

	// 更新配置文件中的默认KV
	if m.storageCfg != nil {
		m.storageCfg.DefaultKV = name
	}

	m.mu.Unlock()

	// 保存配置（需要在锁外调用）
	if err := m.SaveConfig(); err != nil {
		logger.Warn("Failed to save default kv to config file", zap.Error(err))
		// 即使保存失败，内存中的设置仍然生效
	}

	logger.Info("Default kv store set", zap.String("name", name))

	// 触发回调通知KV存储已变更
	if m.kvStoreCb != nil {
		if kvStore, err := m.GetKVStore(name); err == nil {
			m.kvStoreCb(kvStore)
		}
	}

	return nil
}

// GetDefaultKVName 获取默认KV存储名称
func (m *Manager) GetDefaultKVName() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultKV
}

// RegisterKVStoreChangeCallback 注册KV存储变更回调
// 当默认KV存储改变时，会调用此回调通知更新
func (m *Manager) RegisterKVStoreChangeCallback(callback KVStoreChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kvStoreCb = callback
}

// ListStorages 列出所有存储配置
func (m *Manager) ListStorages() []StorageConfigItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.storageCfg == nil {
		return []StorageConfigItem{}
	}

	return m.storageCfg.Storages
}

// ListActiveStorages 列出活跃的存储
func (m *Manager) ListActiveStorages() []StorageStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var statuses []StorageStatus

	for name, storagePlugin := range m.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		healthy := true
		err := storagePlugin.HealthCheck(ctx)
		if err != nil {
			healthy = false
		}
		cancel()

		statuses = append(statuses, StorageStatus{
			Name:    name,
			Type:    storagePlugin.StorageType(),
			Healthy: healthy,
			Error:   err,
		})
	}

	return statuses
}

// TestConnection 测试连接
func (m *Manager) TestConnection(config *StorageConfigItem) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 直接使用配置，不再应用环境变量覆盖
	// 环境变量仅在 GetDefaultConfig 中用于初始化默认配置
	plugin, err := m.createStoragePlugin(config)
	if err != nil {
		return err
	}

	if err := plugin.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}


// initStorage 初始化存储
func (m *Manager) initStorage(config *StorageConfigItem) error {
	// 直接使用数据库中的配置，不再应用环境变量覆盖
	// 环境变量仅在 GetDefaultConfig 中用于初始化默认配置
	plugin, err := m.createStoragePlugin(config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := plugin.HealthCheck(ctx); err != nil {
		return err
	}

	m.plugins[config.Name] = plugin

	// 获取KV存储
	if kvStore, err := plugin.KVStore(); err == nil {
		m.kvStores[config.Name] = kvStore
		// 如果是第一个启用的存储，设置为默认
		if m.defaultKV == "" {
			m.defaultKV = config.Name
		}
	}

	// 获取向量存储
	if vectorStore, err := plugin.VectorStore(); err == nil {
		m.vectorStores[config.Name] = vectorStore
	}

	// 获取知识库存储
	if knowledgeStore, err := plugin.KnowledgeStore(); err == nil {
		m.dataStores[config.Name] = knowledgeStore
	}

	return nil
}

// GetDataStore 获取知识库存储
func (m *Manager) GetDataStore(name string) (KnowledgeDataStore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		// 返回第一个可用的
		for _, ds := range m.dataStores {
			return ds, nil
		}
		return nil, fmt.Errorf("no data store available")
	}

	ds, exists := m.dataStores[name]
	if !exists {
		return nil, fmt.Errorf("data store '%s' not found", name)
	}
	return ds, nil
}

// createStoragePlugin 创建存储插件
func (m *Manager) createStoragePlugin(config *StorageConfigItem) (StoragePlugin, error) {
	factory, err := getPluginFactory(config.Type)
	if err != nil {
		return nil, err
	}
	return factory(config.Config)
}

// stopStorage 停止存储
func (m *Manager) stopStorage(name string) {
	// 关闭KV存储
	if kvStore, exists := m.kvStores[name]; exists {
		if err := kvStore.Close(); err != nil {
			logger.LogError("close kv storage", err, logger.GetField("name", name))
		}
	}

	// 关闭向量存储
	if vectorStore, exists := m.vectorStores[name]; exists {
		if err := vectorStore.Close(); err != nil {
			logger.LogError("close vector storage", err, logger.GetField("name", name))
		}
	}

	// 关闭知识库存储
	if dataStore, exists := m.dataStores[name]; exists {
		if err := dataStore.Close(); err != nil {
			logger.LogError("close data store", err, logger.GetField("name", name))
		}
	}

	delete(m.plugins, name)
	delete(m.kvStores, name)
	delete(m.vectorStores, name)
	delete(m.dataStores, name)

	if m.defaultKV == name {
		m.defaultKV = ""
	}
}

// ConnectStorage 连接存储（不修改配置）
func (m *Manager) ConnectStorage(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找存储配置
	var config *StorageConfigItem
	for _, s := range m.storageCfg.Storages {
		if s.Name == name {
			config = &s
			break
		}
	}

	if config == nil {
		return fmt.Errorf("storage '%s' not found", name)
	}

	if !config.Enabled {
		return fmt.Errorf("storage '%s' is disabled", name)
	}

	// 如果已经连接，直接返回
	if _, exists := m.plugins[name]; exists {
		return nil
	}

	// 初始化存储
	if err := m.initStorage(config); err != nil {
		return fmt.Errorf("failed to connect storage: %w", err)
	}

	logger.Info("Storage connected", logger.GetField("name", name))
	return nil
}

// DisconnectStorage 断开存储连接（不修改配置）
func (m *Manager) DisconnectStorage(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找存储配置
	var config *StorageConfigItem
	for _, s := range m.storageCfg.Storages {
		if s.Name == name {
			config = &s
			break
		}
	}

	if config == nil {
		return fmt.Errorf("storage '%s' not found", name)
	}

	// 停止存储
	m.stopStorage(name)

	logger.Info("Storage disconnected", logger.GetField("name", name))
	return nil
}

// Close 关闭所有存储
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, kvStore := range m.kvStores {
		if err := kvStore.Close(); err != nil {
			logger.LogError("close kv storage", err, logger.GetField("name", name))
		}
	}

	for name, vectorStore := range m.vectorStores {
		if err := vectorStore.Close(); err != nil {
			logger.LogError("close vector storage", err, logger.GetField("name", name))
		}
	}

	for name, dataStore := range m.dataStores {
		if err := dataStore.Close(); err != nil {
			logger.LogError("close data store", err, logger.GetField("name", name))
		}
	}

	m.plugins = make(map[string]StoragePlugin)
	m.kvStores = make(map[string]KVStore)
	m.vectorStores = make(map[string]VectorStore)
	m.dataStores = make(map[string]KnowledgeDataStore)

	return nil
}

// GetDefaultConfig 获取指定存储类型的默认配置
func (m *Manager) GetDefaultConfig(storageType string) (map[string]interface{}, error) {
	// 创建一个临时插件实例来获取默认配置
	// 注意：这不会建立实际的连接
	factory, err := getPluginFactory(StorageType(storageType))
	if err != nil {
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	// 创建一个临时插件（空配置）
	// 插件应该在初始化时提供合理的默认配置
	plugin, err := factory(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin instance: %w", err)
	}

	// 获取插件提供的默认配置
	defaultConfig := plugin.GetDefaultConfig()

	// 关闭临时插件
	if closeable, ok := plugin.(interface{ Close() error }); ok {
		closeable.Close()
	}

	return defaultConfig, nil
}

// StorageStatus 存储状态
type StorageStatus struct {
	Name    string      `json:"name"`
	Type    StorageType `json:"type"`
	Healthy bool        `json:"healthy"`
	Error   error       `json:"error,omitempty"`
}

// ========== 全局存储管理器支持 ==========

var globalStorageManager *Manager

// SetGlobalManager 设置全局存储管理器（由初始化代码调用）
func SetGlobalManager(m *Manager) {
	globalStorageManager = m
}

// GetGlobalManager 获取全局存储管理器
func GetGlobalManager() *Manager {
	return globalStorageManager
}

// ──────────── DataStore 管理 API ────────────

// ListDataStores 列出所有数据存储配置
func (m *Manager) ListDataStores() []config.DataStoreConfig {
	cfg := config.Get()
	if cfg == nil {
		return nil
	}
	return cfg.DataStores
}

// GetDataStoreConfig 获取单个数据存储配置
func (m *Manager) GetDataStoreConfig(name string) (*config.DataStoreConfig, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("global config not initialized")
	}
	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name == name {
			return &cfg.DataStores[i], nil
		}
	}
	return nil, fmt.Errorf("data store '%s' not found", name)
}

// AddDataStore 添加数据存储配置
func (m *Manager) AddDataStore(ds *config.DataStoreConfig) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name == ds.Name {
			return fmt.Errorf("data store '%s' already exists", ds.Name)
		}
	}

	cfg.DataStores = append(cfg.DataStores, *ds)

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save data store config: %w", err)
	}

	logger.Info("Data store added", logger.GetField("name", ds.Name), logger.GetField("type", ds.Type))
	return nil
}

// UpdateDataStore 更新数据存储配置
func (m *Manager) UpdateDataStore(oldName string, ds *config.DataStoreConfig) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	found := false
	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name == oldName {
			cfg.DataStores[i] = *ds
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("data store '%s' not found", oldName)
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save data store config: %w", err)
	}

	logger.Info("Data store updated", logger.GetField("name", oldName))
	return nil
}

// DeleteDataStore 删除数据存储配置
func (m *Manager) DeleteDataStore(name string) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	found := false
	newList := make([]config.DataStoreConfig, 0, len(cfg.DataStores))
	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name != name {
			newList = append(newList, cfg.DataStores[i])
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("data store '%s' not found", name)
	}

	cfg.DataStores = newList

	defaultDataStores := make([]string, 0, len(cfg.DefaultDataStores))
	for _, n := range cfg.DefaultDataStores {
		if n != name {
			defaultDataStores = append(defaultDataStores, n)
		}
	}
	cfg.DefaultDataStores = defaultDataStores

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save data store config: %w", err)
	}

	logger.Info("Data store deleted", logger.GetField("name", name))
	return nil
}

// ToggleDataStore 切换数据存储启用状态
func (m *Manager) ToggleDataStore(name string, enabled bool) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	found := false
	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name == name {
			cfg.DataStores[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("data store '%s' not found", name)
	}

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save data store config: %w", err)
	}

	logger.Info("Data store toggled", logger.GetField("name", name), logger.GetField("enabled", enabled))
	return nil
}

// SetDefaultDataStore 设置默认数据存储
func (m *Manager) SetDefaultDataStore(name string) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	found := false
	for i := range cfg.DataStores {
		if cfg.DataStores[i].Name == name {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("data store '%s' not found", name)
	}

	for _, n := range cfg.DefaultDataStores {
		if n == name {
			return nil
		}
	}
	cfg.DefaultDataStores = append(cfg.DefaultDataStores, name)

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save default data store config: %w", err)
	}

	logger.Info("Default data store set", logger.GetField("name", name))
	return nil
}

// RemoveDefaultDataStore 取消默认数据存储
func (m *Manager) RemoveDefaultDataStore(name string) error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("global config not initialized")
	}

	newDefaults := make([]string, 0, len(cfg.DefaultDataStores))
	for _, n := range cfg.DefaultDataStores {
		if n != name {
			newDefaults = append(newDefaults, n)
		}
	}
	cfg.DefaultDataStores = newDefaults

	if err := config.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save default data store config: %w", err)
	}

	logger.Info("Default data store removed", logger.GetField("name", name))
	return nil
}

// GetDefaultDataStoreNames 获取默认数据存储名称列表
func (m *Manager) GetDefaultDataStoreNames() []string {
	cfg := config.Get()
	if cfg == nil {
		return nil
	}
	return cfg.DefaultDataStores
}

// TestDataStoreConnection 测试数据存储的连接（通过底层存储）
func (m *Manager) TestDataStoreConnection(ds *config.DataStoreConfig) error {
	if ds.StorageName == "" {
		return fmt.Errorf("data store '%s' has no storage_name configured", ds.Name)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[ds.StorageName]
	if !exists {
		return fmt.Errorf("underlying storage '%s' is not connected", ds.StorageName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := plugin.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed for storage '%s': %w", ds.StorageName, err)
	}

	return nil
}

// ListDataStoreStatuses 列出活跃的数据存储状态
func (m *Manager) ListDataStoreStatuses() []map[string]interface{} {
	cfg := config.Get()
	if cfg == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	isDefault := func(name string) bool {
		for _, n := range cfg.DefaultDataStores {
			if n == name {
				return true
			}
		}
		return false
	}

	result := make([]map[string]interface{}, 0, len(cfg.DataStores))
	for _, ds := range cfg.DataStores {
		item := map[string]interface{}{
			"name":         ds.Name,
			"type":         ds.Type,
			"storage_name": ds.StorageName,
			"enabled":      ds.Enabled,
			"description":  ds.Description,
			"config":       ds.Config,
			"is_default":   isDefault(ds.Name),
			"healthy":      false,
		}

		if ds.Enabled && ds.StorageName != "" {
			if plugin, exists := m.plugins[ds.StorageName]; exists {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := plugin.HealthCheck(ctx); err == nil {
					item["healthy"] = true
				} else {
					item["error"] = err.Error()
				}
				cancel()
			}
		}

		result = append(result, item)
	}

	return result
}
