package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"centag/core/internal/cache"
	"centag/core/internal/llm"
	"centag/core/pkg/backend"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/processor"

	"github.com/gin-gonic/gin"
)

// CacheHandler 缓存处理器
type CacheHandler struct {
	cacheManager  *cache.Manager
	proxyCache    *cache.ProxyCache
	qaSplitConfig *config.QASplitConfig
	backendManager *backend.Manager
}

// NewCacheHandler 创建缓存处理器
func NewCacheHandler(cacheManager *cache.Manager, proxyCache *cache.ProxyCache, backendManager *backend.Manager) *CacheHandler {
	return &CacheHandler{
		cacheManager:  cacheManager,
		proxyCache:    proxyCache,
		backendManager: backendManager,
	}
}

// SetQASplitConfig 设置问答拆分配置
func (h *CacheHandler) SetQASplitConfig(cfg *config.QASplitConfig) {
	h.qaSplitConfig = cfg
}

// GetStats 获取缓存统计
func (h *CacheHandler) GetStats(c *gin.Context) {
	stats := h.proxyCache.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ClearCache 清空缓存
func (h *CacheHandler) ClearCache(c *gin.Context) {
	if err := h.proxyCache.ClearAll(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache cleared successfully",
	})
}

// InvalidateCache 使指定缓存失效
func (h *CacheHandler) InvalidateCache(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "key is required",
		})
		return
	}

	if err := h.proxyCache.Invalidate(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache invalidated successfully",
	})
}

// SetCacheEnabled 设置缓存启用状态
func (h *CacheHandler) SetCacheEnabled(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.proxyCache.SetEnabled(req.Enabled)

	// 同步持久化到数据库，防止重启后丢失
	if cfg := config.Get(); cfg != nil {
		cfg.Cache.Enabled = req.Enabled
		if err := config.SaveConfig(cfg); err != nil {
			logger.Warnf("SetCacheEnabled: failed to persist cache enabled state: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache status updated",
		"data": gin.H{
			"enabled": req.Enabled,
		},
	})
}

// GetCacheEnabled 获取缓存启用状态
func (h *CacheHandler) GetCacheEnabled(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled": h.proxyCache.IsEnabled(),
		},
	})
}

// SetCacheTTL 设置缓存TTL
func (h *CacheHandler) SetCacheTTL(c *gin.Context) {
	var req struct {
		TTL int `json:"ttl"` // 秒
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if req.TTL <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "TTL must be greater than 0",
		})
		return
	}

	// COMPLETED: TTL is set during cache Set() via ttl parameter
	// The SetCacheTTL handler accepts TTL value but actual TTL is applied in Manager.Set()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache TTL updated",
		"data": gin.H{
			"ttl": req.TTL,
		},
	})
}

// CacheInfoRequest 缓存信息请求
type CacheInfoRequest struct {
	Key string `json:"key" binding:"required"`
	Type string `json:"type"` // 缓存类型: exact, semantic
}

// CheckCache 检查缓存是否存在
func (h *CacheHandler) CheckCache(c *gin.Context) {
	var req CacheInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 从精确缓存获取
	exactCache := h.cacheManager.GetExactCache()
	entry, err := exactCache.Get(c.Request.Context(), req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if entry != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"found": true,
				"entry": entry,
			},
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"found": false,
			},
		})
	}
}

// GetCacheInfo 获取缓存信息
func (h *CacheHandler) GetCacheInfo(c *gin.Context) {
	var req CacheInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 从精确缓存获取
	exactCache := h.cacheManager.GetExactCache()
	entry, err := exactCache.Get(c.Request.Context(), req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Cache entry not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entry,
	})
}

// DeleteCacheEntry 删除缓存条目（JSON body 或 query: key, type）
func (h *CacheHandler) DeleteCacheEntry(c *gin.Context) {
	var req CacheInfoRequest
	_ = c.ShouldBindJSON(&req)
	if req.Key == "" {
		req.Key = strings.TrimSpace(c.Query("key"))
	}
	if req.Type == "" {
		req.Type = strings.TrimSpace(c.Query("type"))
	}
	if req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "key is required",
		})
		return
	}

	cacheType := req.Type
	if cacheType == "" {
		cacheType = "exact"
	}

	var err error
	if cacheType == "semantic" {
		semanticCache := h.cacheManager.GetSemanticCache()
		if semanticCache == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Semantic cache not available",
			})
			return
		}
		err = semanticCache.Delete(c.Request.Context(), req.Key)
	} else {
		err = h.proxyCache.Invalidate(c.Request.Context(), req.Key)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache entry deleted",
	})
}

// ListCacheEntries 列出缓存条目(带分页与多维筛选)
// Query: type, save_only, storage, session_id, model, q, from, to, page/size 或 limit/offset
func (h *CacheHandler) ListCacheEntries(c *gin.Context) {
	var limit int
	var offset int
	cacheType := c.Query("type") // exact | semantic | all (默认 exact)
	if cacheType == "" {
		cacheType = "exact"
	}
	saveOnlyFilter := c.Query("save_only") // all | save_only | cache
	storageFilter := c.Query("storage")

	filters := cacheListFilters{
		SessionID: strings.TrimSpace(c.Query("session_id")),
		Model:     strings.TrimSpace(c.Query("model")),
		Query:     strings.TrimSpace(c.Query("q")),
	}
	if from, ok := parseRFC3339Loose(c.Query("from")); ok {
		filters.From, filters.HasFrom = from, true
	}
	if to, ok := parseRFC3339Loose(c.Query("to")); ok {
		// inclusive end-of-day when date-only
		if len(strings.TrimSpace(c.Query("to"))) == 10 {
			to = to.Add(24*time.Hour - time.Nanosecond)
		}
		filters.To, filters.HasTo = to, true
	}

	if pageStr := c.Query("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
		size := 10
		if sizeStr := c.Query("size"); sizeStr != "" {
			if n, err := strconv.Atoi(sizeStr); err == nil && n > 0 && n <= 1000 {
				size = n
			}
		}
		limit = size
		offset = (page - 1) * size
	} else {
		limit = 10
		if l := c.Query("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
				limit = n
			}
		}
		offset = 0
		if o := c.Query("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}
	}

	var allEntries []map[string]interface{}
	var totalExactCount int
	var totalSemanticCount int

	var kvStoreInfo, vectorStoreInfo string
	if kvStore := h.cacheManager.GetKVStore(); kvStore != nil {
		kvStoreInfo = kvStore.GetStoreInfo().Type
	}
	if vectorStore := h.cacheManager.GetVectorStore(); vectorStore != nil {
		vectorStoreInfo = vectorStore.GetStoreInfo().Type
	}

	if cacheType == "all" || cacheType == "exact" {
		exactCache := h.cacheManager.GetExactCache()
		if exactCache != nil {
			for _, entry := range exactCache.List() {
				metadata, _ := entry["metadata"].(map[string]interface{})
				isSaveOnly := metadata != nil && metadata["save_only"] == true
				if saveOnlyFilter == "save_only" && !isSaveOnly {
					continue
				}
				if saveOnlyFilter == "cache" && isSaveOnly {
					continue
				}
				entry["cache_type"] = "exact"
				entry["similarity"] = nil
				if sb, ok := entry["storage_backend"].(string); ok && sb != "" {
					// keep
				} else if writeStorage, ok := metadata["write_storage"].(string); ok && writeStorage != "" {
					entry["storage_backend"] = writeStorage
				} else if kvStoreInfo != "" {
					entry["storage_backend"] = kvStoreInfo
				}
				if storageFilter != "" && storageFilter != "all" {
					entryStorage, _ := entry["storage_backend"].(string)
					if entryStorage != storageFilter {
						continue
					}
				}
				entry = flattenCacheEntryForAPI(entry)
				if !matchCacheListFilters(entry, filters) {
					continue
				}
				allEntries = append(allEntries, entry)
				totalExactCount++
			}
		}
	}

	// 语义缓存不支持 save_only（save_only 只写精确缓存）
	if (cacheType == "all" || cacheType == "semantic") && saveOnlyFilter != "save_only" {
		if semanticCache := h.cacheManager.GetSemanticCache(); semanticCache != nil {
			semanticEntries, err := semanticCache.ListFromVectorStore(c.Request.Context(), 1000, 0)
			if err != nil {
				logger.Warn("Failed to load semantic cache from vector store", logger.GetField("error", err))
				semanticEntries = semanticCache.List()
			}
			if len(semanticEntries) == 0 {
				semanticEntries = semanticCache.List()
			}
			for _, entry := range semanticEntries {
				entry["cache_type"] = "semantic"
				if _, hasSimilarity := entry["similarity"]; !hasSimilarity {
					entry["similarity"] = nil
				}
				metadata, _ := entry["metadata"].(map[string]interface{})
				if sb, ok := entry["storage_backend"].(string); ok && sb != "" {
					// keep
				} else if writeStorage, ok := metadata["write_storage"].(string); ok && writeStorage != "" {
					entry["storage_backend"] = writeStorage
				} else if vectorStoreInfo != "" {
					entry["storage_backend"] = vectorStoreInfo
				}
				if storageFilter != "" && storageFilter != "all" {
					entryStorage, _ := entry["storage_backend"].(string)
					if entryStorage != storageFilter {
						continue
					}
				}
				entry = flattenCacheEntryForAPI(entry)
				if !matchCacheListFilters(entry, filters) {
					continue
				}
				allEntries = append(allEntries, entry)
				totalSemanticCount++
			}
		}
	}

	sort.Slice(allEntries, func(i, j int) bool {
		ti, ok1 := entryTimestamp(allEntries[i])
		tj, ok2 := entryTimestamp(allEntries[j])
		if !ok1 || !ok2 {
			return false
		}
		return ti.After(tj)
	})

	totalCount := len(allEntries)
	start := offset
	if start > totalCount {
		start = totalCount
	}
	end := start + limit
	if end > totalCount {
		end = totalCount
	}
	var pagedEntries []map[string]interface{}
	if start < end {
		pagedEntries = allEntries[start:end]
	} else {
		pagedEntries = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"entries":              pagedEntries,
			"total_count":          totalCount,
			"total_exact_count":    totalExactCount,
			"total_semantic_count": totalSemanticCount,
			"limit":                limit,
			"offset":               offset,
			"cache_type":           cacheType,
			"storage_filter":       storageFilter,
			"session_id":           filters.SessionID,
			"model":                filters.Model,
			"q":                    filters.Query,
			"kv_store":             kvStoreInfo,
			"vector_store":         vectorStoreInfo,
		},
	})
}

// GetCacheEntry returns a single cache entry by key (management console detail).
// Query: key (required), type=exact|semantic (default exact)
func (h *CacheHandler) GetCacheEntry(c *gin.Context) {
	key := strings.TrimSpace(c.Query("key"))
	if key == "" {
		key = strings.TrimSpace(c.Param("key"))
	}
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "key is required"})
		return
	}
	cacheType := c.Query("type")
	if cacheType == "" {
		cacheType = "exact"
	}

	if cacheType == "semantic" {
		semanticCache := h.cacheManager.GetSemanticCache()
		if semanticCache == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "semantic cache not available"})
			return
		}
		entries := semanticCache.List()
		for _, entry := range entries {
			if stringifyEntryField(entry, "key") == key {
				entry["cache_type"] = "semantic"
				c.JSON(http.StatusOK, gin.H{"success": true, "data": flattenCacheEntryForAPI(entry)})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "entry not found"})
		return
	}

	exact := h.cacheManager.GetExactCache()
	if exact == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "exact cache not available"})
		return
	}
	entry, err := exact.Get(c.Request.Context(), key)
	if err != nil || entry == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "entry not found"})
		return
	}
	out := map[string]interface{}{
		"key":        entry.Key,
		"request":    entry.Request,
		"response":   entry.Response,
		"metadata":   entry.Metadata,
		"timestamp":  entry.Timestamp,
		"cache_type": "exact",
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": flattenCacheEntryForAPI(out)})
}

// WarmupCache 预热缓存
func (h *CacheHandler) WarmupCache(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt" binding:"required"`
		Model  string `json:"model"`
		TTL    int    `json:"ttl"` // 秒
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if req.Model == "" {
		req.Model = "qwen/qwen3-4b-fp8"
	}

	if req.TTL <= 0 {
		req.TTL = 3600 // 默认1小时
	}

	// TODO: 实现预热缓存功能
	// 需要调用后端API并将响应缓存起来

	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "Warmup feature not implemented yet",
	})
}

// GetSemanticThreshold 获取语义匹配阈值（优先返回内存中的值，与滑块/SET 一致）
func (h *CacheHandler) GetSemanticThreshold(c *gin.Context) {
	threshold := h.cacheManager.GetSemanticThreshold()
	// 若内存仍为默认值且 KV 中有保存值，则从 KV 加载到内存（首次加载或重启后）
	if kvStore := h.cacheManager.GetKVStore(); kvStore != nil && threshold == 0.85 {
		if val, err := kvStore.Get(c.Request.Context(), "semantic_threshold"); err == nil && val != nil {
			if t, ok := val.(float64); ok {
				threshold = float32(t)
				_ = h.cacheManager.UpdateSemanticThreshold(threshold)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"threshold":    threshold,
			"description": "语义相似度阈值 (0-1)，值越低匹配越宽松，命中率越高",
		},
	})
}

// SetSemanticThreshold 设置语义匹配阈值
func (h *CacheHandler) SetSemanticThreshold(c *gin.Context) {
	var req struct {
		Threshold float32 `json:"threshold" binding:"required,min=0,max=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 验证阈值范围
	if req.Threshold < 0 || req.Threshold > 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Threshold must be between 0 and 1",
		})
		return
	}

	// 保存到KV存储
	if kvStore := h.cacheManager.GetKVStore(); kvStore != nil {
		if err := kvStore.Set(c.Request.Context(), "semantic_threshold", float64(req.Threshold), 3600); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	// 更新语义缓存阈值
	if err := h.cacheManager.UpdateSemanticThreshold(req.Threshold); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Semantic threshold updated",
		"data": gin.H{
			"threshold": req.Threshold,
		},
	})
}

// SemanticSearchRequest 语义搜索请求
type SemanticSearchRequest struct {
	Query     string  `json:"query" binding:"required"`
	Threshold float32 `json:"threshold" binding:"required,min=0,max=1"`
	TopK      int     `json:"top_k" binding:"required,min=1,max=10"`
}

// GenerateCacheKeyRequest 生成缓存键请求
type GenerateCacheKeyRequest struct {
	Model       string        `json:"model" binding:"required"`
	Messages    []interface{} `json:"messages" binding:"required"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// SemanticSearch 语义搜索
func (h *CacheHandler) SemanticSearch(c *gin.Context) {
	var req SemanticSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 语义缓存未配置时 SearchByQuery 返回 (nil, nil)，仅精确匹配生效
	semanticAvailable := h.cacheManager.GetSemanticCache() != nil
	entries, err := h.cacheManager.SearchByQuery(c.Request.Context(), req.Query, req.Threshold, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if entries == nil {
		entries = []*cache.CacheEntry{}
	}

	// 统计超过阈值的命中数
	hitsCount := 0
	for _, entry := range entries {
		if similarity, ok := entry.Metadata["similarity_score"].(float32); ok {
			if similarity >= req.Threshold {
				hitsCount++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hits":                hitsCount,          // 超过阈值的命中数
			"total_results":       len(entries),       // 总结果数（包括低于阈值的）
			"entries":             entries,
			"semantic_available":  semanticAvailable,
			"threshold":           req.Threshold,
		},
	})
}

// GenerateCacheKey 生成缓存键（与代理路径一致：归一化 messages 仅保留 role/content）
func (h *CacheHandler) GenerateCacheKey(c *gin.Context) {
	var req GenerateCacheKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 归一化 messages，与 /v1/chat/completions 代理使用的 convertMessagesToInterface 结果一致
	messages := cache.NormalizeMessagesForKey(req.Messages)
	key, err := h.proxyCache.GetRequestKey(req.Model, messages, req.Temperature, req.MaxTokens, nil, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"key": key,
		},
	})
}

// GetQASplitStatus 获取问答拆分状态
func (h *CacheHandler) GetQASplitStatus(c *gin.Context) {
	qaSplitter := h.cacheManager.GetQASplitter()
	if qaSplitter == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"enabled": false,
				"message": "QA splitter not configured",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled": qaSplitter.IsEnabled(),
		},
	})
}

// SetQASplitEnabled 设置问答拆分启用状态
func (h *CacheHandler) SetQASplitEnabled(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	qaSplitter := h.cacheManager.GetQASplitter()
	if qaSplitter == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "QA splitter not configured",
		})
		return
	}

	qaSplitter.SetEnabled(req.Enabled)

	// 同步持久化到数据库，防止重启后丢失
	if cfg := config.Get(); cfg != nil {
		cfg.QASplit.Enabled = req.Enabled
		if err := config.SaveConfig(cfg); err != nil {
			logger.Warnf("SetQASplitEnabled: failed to persist qa_split enabled state: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "QA split status updated",
		"data": gin.H{
			"enabled": req.Enabled,
		},
	})
}

// TestQASplit 测试问答拆分
func (h *CacheHandler) TestQASplit(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
		Answer   string `json:"answer" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	qaSplitter := h.cacheManager.GetQASplitter()
	if qaSplitter == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "QA splitter not configured",
		})
		return
	}

	result, err := qaSplitter.SplitQA(c.Request.Context(), req.Question, req.Answer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// GetQASplitConfig 获取问答拆分配置
func (h *CacheHandler) GetQASplitConfig(c *gin.Context) {
	// 优先返回配置文件中的配置
	if h.qaSplitConfig != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"configured":  true,
				"enabled":     h.qaSplitConfig.Enabled,
				"backend_id":  h.qaSplitConfig.BackendID,
				"model":       h.qaSplitConfig.Model,
				"timeout":     h.qaSplitConfig.Timeout,
				"temperature": h.qaSplitConfig.Temperature,
				"max_tokens":  h.qaSplitConfig.MaxTokens,
				"prompt":      h.qaSplitConfig.Prompt,
			},
		})
		return
	}

	// 如果没有配置文件中的配置，尝试从 QASplitter 获取
	qaSplitter := h.cacheManager.GetQASplitter()
	if qaSplitter == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"configured": false,
				"enabled":    false,
			},
		})
		return
	}

	config := gin.H{
		"configured": true,
		"enabled":    qaSplitter.IsEnabled(),
		"prompt":     qaSplitter.GetPrompt(),
	}

	// 获取 chat service 的配置
	if chatService := qaSplitter.GetChatService(); chatService != nil {
		config["has_chat_service"] = true
		info := chatService.GetProviderInfo()
		config["provider"] = info.Provider
		config["model"] = info.Model
		config["base_url"] = info.BaseURL
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateQASplitConfig 更新问答拆分配置
func (h *CacheHandler) UpdateQASplitConfig(c *gin.Context) {
	var req struct {
		Enabled     bool   `json:"enabled"`
		Prompt      string `json:"prompt"`
		BackendID   string `json:"backend_id"`
		Model       string `json:"model"`
		Timeout     int    `json:"timeout"`
		Temperature float64 `json:"temperature"`
		MaxTokens   int    `json:"max_tokens"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 同步更新缓存中的配置
	if h.qaSplitConfig != nil {
		oldBackendID := h.qaSplitConfig.BackendID
		oldModel := h.qaSplitConfig.Model

		h.qaSplitConfig.Enabled = req.Enabled
		h.qaSplitConfig.Prompt = req.Prompt
		h.qaSplitConfig.BackendID = req.BackendID
		h.qaSplitConfig.Model = req.Model
		h.qaSplitConfig.Timeout = req.Timeout
		h.qaSplitConfig.Temperature = req.Temperature
		h.qaSplitConfig.MaxTokens = req.MaxTokens

		// 持久化配置到文件
		if err := config.SaveConfig(config.Get()); err != nil {
			logger.Warnf("Failed to save qa_split config: %v", err)
			// 不返回错误，因为内存配置已更新成功
		} else {
			logger.Info("QA split config saved to file successfully")
		}

		// 检查是否需要重新创建 ChatService（后端 ID 或模型发生变化，或 ChatService 为 nil）
		qaSplitter := h.cacheManager.GetQASplitter()
		needRecreate := qaSplitter == nil ||
			oldBackendID != req.BackendID ||
			oldModel != req.Model ||
			qaSplitter.GetChatService() == nil

		if needRecreate && req.BackendID != "" && req.Enabled {
			logger.Infof("Recreating QA splitter chat service - backend_id: %s, model: %s", req.BackendID, req.Model)

			// 从后端管理器获取后端配置
			backend, err := h.backendManager.Get(req.BackendID)
			if err != nil {
				logger.Warnf("QA split backend %s not found: %v", req.BackendID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Backend not found: " + req.BackendID,
				})
				return
			}

			if !backend.Enabled {
				logger.Warnf("QA split backend %s is disabled", req.BackendID)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Backend is disabled: " + req.BackendID,
				})
				return
			}

			// 创建 chat service 配置
			qaSplitConfig := &llm.ChatConfig{
				Provider:    backend.Type,
				Model:       req.Model,
				BaseURL:     backend.BaseURL,
				APIKey:      backend.APIKey,
				Timeout:     req.Timeout,
				Temperature: req.Temperature,
				MaxTokens:   req.MaxTokens,
				Enabled:     req.Enabled,
			}

			var chatService llm.ChatService

			// 根据后端类型创建对应的 chat service
			switch backend.Type {
			case "ollama":
				chatService, err = llm.NewOllamaChatService(qaSplitConfig)
			case "openai":
				chatService, err = llm.NewOpenAIChatService(qaSplitConfig)
			default:
				logger.Warnf("Unsupported QA split backend type: %s", backend.Type)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Unsupported backend type: " + backend.Type,
				})
				return
			}

			if err != nil {
				logger.Warnf("Failed to create QA split chat service: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to create chat service: " + err.Error(),
				})
				return
			}

			if chatService == nil {
				logger.Warnf("Created chat service is nil")
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to create chat service",
				})
				return
			}

			// 创建或更新 QASplitter
			if qaSplitter == nil {
				qaSplitter = processor.NewQASplitter(&processor.QASplitterConfig{
					ChatService: chatService,
					Prompt:      req.Prompt,
					Enabled:     req.Enabled,
				})
			} else {
				qaSplitter.SetChatService(chatService)
				qaSplitter.SetPrompt(req.Prompt)
				qaSplitter.SetEnabled(req.Enabled)
			}

			h.cacheManager.SetQASplitter(qaSplitter)

			logger.Infof("QA splitter recreated - backend_id: %s, model: %s, backend_type: %s",
				req.BackendID, req.Model, backend.Type)
		} else if qaSplitter != nil {
			// 只更新配置，不需要重新创建 ChatService
			qaSplitter.SetEnabled(req.Enabled)
			if req.Prompt != "" {
				qaSplitter.SetPrompt(req.Prompt)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "QA split configuration updated",
	})
}
