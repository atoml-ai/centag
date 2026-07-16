package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockStoragePlugin is a test double for StoragePlugin
type mockStoragePlugin struct {
	storageType    StorageType
	healthCheckErr error
	kvStore        KVStore
	vectorStore    VectorStore
	knowledgeStore KnowledgeDataStore
	defaultConfig  map[string]interface{}
}

func (m *mockStoragePlugin) KVStore() (KVStore, error) {
	if m.kvStore == nil {
		return nil, assert.AnError
	}
	return m.kvStore, nil
}

func (m *mockStoragePlugin) VectorStore() (VectorStore, error) {
	if m.vectorStore == nil {
		return nil, assert.AnError
	}
	return m.vectorStore, nil
}

func (m *mockStoragePlugin) KnowledgeStore() (KnowledgeDataStore, error) {
	if m.knowledgeStore == nil {
		return nil, assert.AnError
	}
	return m.knowledgeStore, nil
}

func (m *mockStoragePlugin) StorageType() StorageType { return m.storageType }

func (m *mockStoragePlugin) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func (m *mockStoragePlugin) GetDefaultConfig() map[string]interface{} {
	if m.defaultConfig != nil {
		return m.defaultConfig
	}
	return map[string]interface{}{"host": "localhost", "port": 6379}
}

func TestNewManager(t *testing.T) {
	m, err := NewManager("/tmp/test-config.json")
	assert.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.plugins)
	assert.NotNil(t, m.kvStores)
	assert.NotNil(t, m.vectorStores)
	assert.NotNil(t, m.dataStores)
}

func TestRegisterPlugin(t *testing.T) {
	factory := func(config map[string]interface{}) (StoragePlugin, error) {
		return &mockStoragePlugin{storageType: StorageTypeRedis}, nil
	}

	RegisterPlugin(StorageTypeRedis, factory)

	// Verify registration by getting factory
	f, err := getPluginFactory(StorageTypeRedis)
	assert.NoError(t, err)
	assert.NotNil(t, f)
}

func TestGetPluginFactory_NotFound(t *testing.T) {
	_, err := getPluginFactory("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin factory registered")
}

func TestManager_ListStorages_Empty(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	storages := m.ListStorages()
	assert.Empty(t, storages)
}

func TestManager_GetKVStore_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetKVStore("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetKVStore_NoDefault(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetKVStore("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no default kv store")
}

func TestManager_GetVectorStore_NoAvailable(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetVectorStore("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no vector store available")
}

func TestManager_GetVectorStore_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetVectorStore("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetStorage_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetStorage("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_GetDataStore_NoAvailable(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetDataStore("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no data store available")
}

func TestManager_GetDataStore_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	_, err := m.GetDataStore("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_SetDefaultKV_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	// Need storageCfg initialized
	m.storageCfg = &StorageConfig{Storages: []StorageConfigItem{}}
	err := m.SetDefaultKV("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in configuration")
}

func TestManager_DefaultKVName_Empty(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	assert.Equal(t, "", m.GetDefaultKVName())
}

func TestManager_ListStorages_Nil(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	m.storageCfg = nil
	storages := m.ListStorages()
	assert.NotNil(t, storages)
	assert.Empty(t, storages)
}

func TestManager_Close(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	err := m.Close()
	assert.NoError(t, err)
}

func TestStorageType_File(t *testing.T) {
	assert.Equal(t, StorageType("file"), StorageTypeFile)
}

func TestPluginRegistry_Concurrent(t *testing.T) {
	// Test that concurrent RegisterPlugin calls don't race
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			factory := func(config map[string]interface{}) (StoragePlugin, error) {
				return &mockStoragePlugin{storageType: StorageType("test")}, nil
			}
			RegisterPlugin(StorageType("test"), factory)
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestManager_GlobalManager(t *testing.T) {
	// Reset global
	SetGlobalManager(nil)
	assert.Nil(t, GetGlobalManager())

	m, _ := NewManager("/tmp/test.json")
	SetGlobalManager(m)
	assert.Equal(t, m, GetGlobalManager())

	// Cleanup
	SetGlobalManager(nil)
}

func TestManager_ListActiveStorages_Empty(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	statuses := m.ListActiveStorages()
	assert.Empty(t, statuses)
}

func TestManager_RegisterKVStoreChangeCallback(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	called := false
	m.RegisterKVStoreChangeCallback(func(kvStore KVStore) {
		called = true
	})
	assert.NotNil(t, m.kvStoreCb)
	_ = called
}

func TestManager_ConnectStorage_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	m.storageCfg = &StorageConfig{Storages: []StorageConfigItem{}}
	err := m.ConnectStorage("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_DisconnectStorage_NotFound(t *testing.T) {
	m, _ := NewManager("/tmp/test.json")
	m.storageCfg = &StorageConfig{Storages: []StorageConfigItem{}}
	err := m.DisconnectStorage("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
