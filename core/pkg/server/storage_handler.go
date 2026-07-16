package server

import (
	"fmt"
	"net/http"

	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"github.com/gin-gonic/gin"
)

// StorageHandler 存储配置处理器
type StorageHandler struct {
	manager *storage.Manager
}

// NewStorageHandler 创建存储配置处理器
func NewStorageHandler(storageManager *storage.Manager) *StorageHandler {
	return &StorageHandler{
		manager: storageManager,
	}
}

// ListStorages 列出所有存储配置
func (h *StorageHandler) ListStorages(c *gin.Context) {
	storages := h.manager.ListStorages()
	activeStatuses := h.manager.ListActiveStorages()

	// 获取默认KV存储名称
	defaultKVStorage := h.manager.GetDefaultKVName()

	// 合并配置和状态
	result := make([]map[string]interface{}, 0)

	for _, s := range storages {
		item := map[string]interface{}{
			"name":        s.Name,
			"type":        s.Type,
			"enabled":     s.Enabled,
			"description": s.Description,
			"config":      s.Config,
			"is_default":  (s.Name == defaultKVStorage),
		}

		// 查找状态
		statusFound := false
		for _, status := range activeStatuses {
			if status.Name == s.Name {
				item["healthy"] = status.Healthy
				if status.Error != nil {
					item["error"] = status.Error.Error()
				}
				statusFound = true
				break
			}
		}

		// 如果存储未激活但已启用，尝试健康检查以获取状态
		if !statusFound && s.Enabled {
			if err := h.manager.TestConnection(&s); err != nil {
				item["healthy"] = false
				item["error"] = err.Error()
			} else {
				item["healthy"] = true
			}
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"storages":   result,
		"default_kv": defaultKVStorage,
	})
}

// GetStorage 获取存储配置
func (h *StorageHandler) GetStorage(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		RespondBadRequest(c, "name parameter is required")
		return
	}

	storages := h.manager.ListStorages()
	for _, s := range storages {
		if s.Name == name {
			c.JSON(http.StatusOK, s)
			return
		}
	}

	RespondNotFound(c, "storage not found")
}

// AddStorage 添加存储配置
func (h *StorageHandler) AddStorage(c *gin.Context) {
	var config storage.StorageConfigItem
	if !BindJSON(c, &config) {
		return
	}

	if err := h.manager.AddStorage(&config); err != nil {
		logger.LogError("add storage", err, logger.GetField("name", config.Name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage added successfully")
}

// UpdateStorage 更新存储配置
func (h *StorageHandler) UpdateStorage(c *gin.Context) {
	var config storage.StorageConfigItem
	if !BindJSON(c, &config) {
		return
	}

	// 获取旧名称(从query参数或请求体中)
	oldName := c.Query("name")
	if oldName == "" {
		oldName = config.Name
	}

	if err := h.manager.UpdateStorage(oldName, &config); err != nil {
		// 检查是否只是初始化失败的警告
		errMsg := err.Error()
		if len(errMsg) > 30 && errMsg[:30] == "storage config saved but ini" {
			// 配置已保存但初始化失败，返回警告但不算错误
			logger.Warn("storage updated but initialization failed",
				logger.GetField("name", oldName),
				logger.GetField("error", errMsg))
			RespondSuccessWithMessage(c, "storage config saved but initialization failed: "+errMsg)
			return
		}

		logger.LogError("update storage", err, logger.GetField("name", oldName))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage updated successfully")
}

// DeleteStorage 删除存储配置
func (h *StorageHandler) DeleteStorage(c *gin.Context) {
	// 支持 query 参数
	name := c.Query("name")

	// 如果 query 参数不存在,尝试从 body 获取
	if name == "" {
		var req struct {
			Name string `json:"name"`
		}
		if !BindJSON(c, &req) {
			return
		}
		name = req.Name
	}

	if name == "" {
		RespondBadRequest(c, "name parameter is required")
		return
	}

	if err := h.manager.DeleteStorage(name); err != nil {
		logger.LogError("delete storage", err, logger.GetField("name", name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage deleted successfully")
}

// ToggleStorage 切换存储启用状态
func (h *StorageHandler) ToggleStorage(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if !BindJSON(c, &req) {
		return
	}

	storages := h.manager.ListStorages()
	var found *storage.StorageConfigItem
	for _, s := range storages {
		if s.Name == req.Name {
			s.Enabled = req.Enabled
			found = &s
			break
		}
	}

	if found == nil {
		RespondNotFound(c, "storage not found")
		return
	}

	if err := h.manager.UpdateStorage(req.Name, found); err != nil {
		logger.LogError("toggle storage", err, logger.GetField("name", req.Name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage toggled successfully")
}

// TestConnection 测试存储连接
func (h *StorageHandler) TestConnection(c *gin.Context) {
	var config storage.StorageConfigItem
	if !BindJSON(c, &config) {
		return
	}

	if err := h.manager.TestConnection(&config); err != nil {
		logger.LogError("storage connection test", err,
			logger.GetField("name", config.Name),
			logger.GetField("type", string(config.Type)))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "connection test successful")
}

// GetStorageStatus 获取存储状态
func (h *StorageHandler) GetStorageStatus(c *gin.Context) {
	statuses := h.manager.ListActiveStorages()
	c.JSON(http.StatusOK, gin.H{"statuses": statuses})
}

// ConnectStorage 连接存储
func (h *StorageHandler) ConnectStorage(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.ConnectStorage(req.Name); err != nil {
		logger.LogError("connect storage", err, logger.GetField("name", req.Name))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage connected successfully")
}

// DisconnectStorage 断开存储连接
func (h *StorageHandler) DisconnectStorage(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.DisconnectStorage(req.Name); err != nil {
		logger.LogError("disconnect storage", err, logger.GetField("name", req.Name))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "storage disconnected successfully")
}

// SetDefaultStorage 设置默认KV存储
func (h *StorageHandler) SetDefaultStorage(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.SetDefaultKV(req.Name); err != nil {
		logger.LogError("set default storage", err, logger.GetField("name", req.Name))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "default storage set successfully")
}

// GetDefaultConfig 获取存储配置的默认值
func (h *StorageHandler) GetDefaultConfig(c *gin.Context) {
	storageType := c.Query("type")
	if storageType == "" {
		RespondBadRequest(c, "type parameter is required")
		return
	}

	// 从插件获取默认配置；不使用环境变量覆盖，保证与保存后运行时一致
	defaultConfig, err := h.manager.GetDefaultConfig(storageType)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"type":    storageType,
		"config":  defaultConfig,
		"success": true,
	})
}

// ──────────── KV 数据浏览 API ────────────

// getKVStore 获取指定名称或默认的 KVStore
func (h *StorageHandler) getKVStore(storageName string) (storage.KVStore, error) {
	if storageName != "" {
		return h.manager.GetKVStore(storageName)
	}
	return h.manager.GetDefaultKVStore()
}

// ListKVKeys 列出 KV 存储中的键
func (h *StorageHandler) ListKVKeys(c *gin.Context) {
	pattern := c.DefaultQuery("pattern", "*")
	storageName := c.Query("storage")

	kv, err := h.getKVStore(storageName)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	keys, err := kv.Keys(c.Request.Context(), pattern)
	if err != nil {
		logger.LogError("list kv keys", err)
		RespondInternalError(c, err.Error())
		return
	}

	count, _ := kv.Count(c.Request.Context(), pattern)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"storage_name": storageName,
		"total":        count,
		"keys":         keys,
	})
}

// GetKVValue 获取 KV 存储中指定键的值
func (h *StorageHandler) GetKVValue(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		RespondBadRequest(c, "key parameter is required")
		return
	}
	storageName := c.Query("storage")

	kv, err := h.getKVStore(storageName)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	value, err := kv.Get(c.Request.Context(), key)
	if err != nil {
		RespondBadRequest(c, fmt.Sprintf("key not found: %s", key))
		return
	}

	ttl, _ := kv.TTL(c.Request.Context(), key)
	exists, _ := kv.Exists(c.Request.Context(), key)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"storage_name": storageName,
		"key":          key,
		"value":        value,
		"ttl_seconds":  ttl.Seconds(),
		"exists":       exists,
	})
}

// DeleteKVKey 删除 KV 存储中的键
func (h *StorageHandler) DeleteKVKey(c *gin.Context) {
	var req struct {
		Key         string `json:"key" binding:"required"`
		StorageName string `json:"storage_name"`
	}
	if !BindJSON(c, &req) {
		return
	}

	kv, err := h.getKVStore(req.StorageName)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return
	}

	if err := kv.Delete(c.Request.Context(), req.Key); err != nil {
		logger.LogError("delete kv key", err, logger.GetField("key", req.Key))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, fmt.Sprintf("key '%s' deleted successfully", req.Key))
}

