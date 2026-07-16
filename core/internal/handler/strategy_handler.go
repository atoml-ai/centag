package handler

import (
	"net/http"

	"centag/core/pkg/logger"
	"centag/core/internal/strategy"

	"github.com/gin-gonic/gin"
)

// StrategyHandler 匹配策略处理器
type StrategyHandler struct{}

// NewStrategyHandler 创建策略处理器
func NewStrategyHandler() *StrategyHandler {
	return &StrategyHandler{}
}

// ListStrategies 获取所有策略（内置 + 自定义）
// GET /api/v1/strategies
func (h *StrategyHandler) ListStrategies(c *gin.Context) {
	items := strategy.ListAll()
	RespondSuccess(c, map[string]interface{}{
		"strategies": items,
		"count":      len(items),
	})
}

// CreateStrategy 创建自定义策略
// POST /api/v1/strategies
func (h *StrategyHandler) CreateStrategy(c *gin.Context) {
	var req strategy.CustomStrategy
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	item, err := strategy.GetStore().Create(&req)
	if err != nil {
		logger.Warnf("[Strategy] Create failed: %v", err)
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    item,
	})
}

// UpdateStrategy 更新自定义策略
// PUT /api/v1/strategies/:id
func (h *StrategyHandler) UpdateStrategy(c *gin.Context) {
	id := c.Param("id")

	var req strategy.CustomStrategy
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "无效的请求数据: "+err.Error())
		return
	}

	item, err := strategy.GetStore().Update(id, &req)
	if err != nil {
		logger.Warnf("[Strategy] Update failed id=%s: %v", id, err)
		RespondNotFound(c, err.Error())
		return
	}

	RespondSuccess(c, item)
}

// DeleteStrategy 删除自定义策略
// DELETE /api/v1/strategies/:id
func (h *StrategyHandler) DeleteStrategy(c *gin.Context) {
	id := c.Param("id")

	if err := strategy.GetStore().Delete(id); err != nil {
		logger.Warnf("[Strategy] Delete failed id=%s: %v", id, err)
		RespondNotFound(c, err.Error())
		return
	}

	RespondSuccess(c, map[string]string{"message": "策略已删除"})
}
