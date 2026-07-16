package server

import (
	"fmt"
	"net/http"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"github.com/gin-gonic/gin"
)

type DataStoreHandler struct {
	manager *storage.Manager
}

func NewDataStoreHandler(storageManager *storage.Manager) *DataStoreHandler {
	return &DataStoreHandler{
		manager: storageManager,
	}
}

func (h *DataStoreHandler) ListDataStores(c *gin.Context) {
	statuses := h.manager.ListDataStoreStatuses()
	defaultNames := h.manager.GetDefaultDataStoreNames()

	availStorages := func() []map[string]interface{} {
		cfg := config.Get()
		if cfg == nil {
			return nil
		}
		result := make([]map[string]interface{}, 0, len(cfg.Storages))
		for _, s := range cfg.Storages {
			result = append(result, map[string]interface{}{
				"name":    s.Name,
				"type":    s.Type,
				"enabled": s.Enabled,
			})
		}
		return result
	}()

	c.JSON(http.StatusOK, gin.H{
		"data_stores":       statuses,
		"default_data_stores": defaultNames,
		"available_storages":  availStorages,
	})
}

func (h *DataStoreHandler) GetDataStore(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		RespondBadRequest(c, "name parameter is required")
		return
	}

	ds, err := h.manager.GetDataStoreConfig(name)
	if err != nil {
		RespondNotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, ds)
}

func (h *DataStoreHandler) AddDataStore(c *gin.Context) {
	var ds config.DataStoreConfig
	if !BindJSON(c, &ds) {
		return
	}

	if err := h.manager.AddDataStore(&ds); err != nil {
		logger.LogError("add data store", err, logger.GetField("name", ds.Name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "data store added successfully")
}

func (h *DataStoreHandler) UpdateDataStore(c *gin.Context) {
	var ds config.DataStoreConfig
	if !BindJSON(c, &ds) {
		return
	}

	oldName := c.Query("name")
	if oldName == "" {
		oldName = ds.Name
	}

	if err := h.manager.UpdateDataStore(oldName, &ds); err != nil {
		logger.LogError("update data store", err, logger.GetField("name", oldName))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "data store updated successfully")
}

func (h *DataStoreHandler) DeleteDataStore(c *gin.Context) {
	name := c.Query("name")

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

	if err := h.manager.DeleteDataStore(name); err != nil {
		logger.LogError("delete data store", err, logger.GetField("name", name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "data store deleted successfully")
}

func (h *DataStoreHandler) ToggleDataStore(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.ToggleDataStore(req.Name, req.Enabled); err != nil {
		logger.LogError("toggle data store", err, logger.GetField("name", req.Name))
		RespondInternalError(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "data store toggled successfully")
}

func (h *DataStoreHandler) TestConnection(c *gin.Context) {
	var ds config.DataStoreConfig
	if !BindJSON(c, &ds) {
		return
	}

	if err := h.manager.TestDataStoreConnection(&ds); err != nil {
		logger.LogError("data store connection test", err,
			logger.GetField("name", ds.Name),
			logger.GetField("storage_name", ds.StorageName))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "connection test successful")
}

func (h *DataStoreHandler) GetStatus(c *gin.Context) {
	statuses := h.manager.ListDataStoreStatuses()
	c.JSON(http.StatusOK, gin.H{"statuses": statuses})
}

func (h *DataStoreHandler) SetDefault(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.SetDefaultDataStore(req.Name); err != nil {
		logger.LogError("set default data store", err, logger.GetField("name", req.Name))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, "default data store set successfully")
}

func (h *DataStoreHandler) RemoveDefault(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if !BindJSON(c, &req) {
		return
	}

	if err := h.manager.RemoveDefaultDataStore(req.Name); err != nil {
		logger.LogError("remove default data store", err, logger.GetField("name", req.Name))
		RespondBadRequest(c, err.Error())
		return
	}

	RespondSuccessWithMessage(c, fmt.Sprintf("data store '%s' removed from defaults", req.Name))
}
