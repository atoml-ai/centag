package server

import (
	"net/http"

	"centag/core/pkg/configsync"
	"centag/core/pkg/logger"

	"github.com/gin-gonic/gin"
)

// ProviderCatalogHandler handles provider catalog API requests.
type ProviderCatalogHandler struct{}

// NewProviderCatalogHandler creates a new ProviderCatalogHandler.
func NewProviderCatalogHandler() *ProviderCatalogHandler {
	return &ProviderCatalogHandler{}
}

// getStore lazily resolves the global provider catalog store on each request,
// because the store is initialized in startConfigsync() which runs after
// route registration.
func (h *ProviderCatalogHandler) getStore() configsync.ProviderCatalogStore {
	return configsync.GetGlobalProviderCatalogStore()
}

// ListProviderCatalog returns all provider catalog entries.
func (h *ProviderCatalogHandler) ListProviderCatalog(c *gin.Context) {
	store := h.getStore()
	if store == nil {
		RespondError(c, http.StatusServiceUnavailable, "provider catalog not available")
		return
	}

	entries := store.GetAll()
	syncTime := store.GetSyncTime()

	c.JSON(http.StatusOK, gin.H{
		"entries":   entries,
		"sync_time": syncTime,
		"source":    "feishu",
	})
}

// SyncProviderCatalog triggers a sync from Feishu and returns the result.
func (h *ProviderCatalogHandler) SyncProviderCatalog(c *gin.Context) {
	store := h.getStore()
	if store == nil {
		RespondError(c, http.StatusServiceUnavailable, "provider catalog not available")
		return
	}

	// Get the scheduler and trigger a sync
	scheduler := configsync.GetScheduler()
	if scheduler == nil {
		RespondError(c, http.StatusServiceUnavailable, "configsync scheduler not available")
		return
	}

	// Trigger a sync
	if err := scheduler.SyncNow(c.Request.Context()); err != nil {
		logger.Warnf("provider catalog sync failed: %v", err)
		RespondError(c, http.StatusInternalServerError, "sync failed: "+err.Error())
		return
	}

	// Return the updated catalog
	entries := store.GetAll()
	syncTime := store.GetSyncTime()

	c.JSON(http.StatusOK, gin.H{
		"entries":   entries,
		"sync_time": syncTime,
		"source":    "feishu",
		"message":   "sync completed successfully",
	})
}

// DeleteProviderCatalogEntry removes a provider catalog entry by ID.
func (h *ProviderCatalogHandler) DeleteProviderCatalogEntry(c *gin.Context) {
	store := h.getStore()
	if store == nil {
		RespondError(c, http.StatusServiceUnavailable, "provider catalog not available")
		return
	}

	id := c.Param("id")
	if id == "" {
		RespondError(c, http.StatusBadRequest, "id is required")
		return
	}

	if err := store.Delete(id); err != nil {
		RespondError(c, http.StatusInternalServerError, "delete failed: "+err.Error())
		return
	}

	RespondSuccess(c, gin.H{"deleted": true})
}
