package webhook

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"centag/core/pkg/pipeline"
)

// Engine executes registered pipelines.
type Engine interface {
	Execute(ctx context.Context, pipelineID string, input *pipeline.PipelineInput) (*pipeline.PipelineOutput, error)
	GetPipelineConfig(pipelineID string) *pipeline.AgentPatternPipeline
}

// EventType represents the type of webhook event.
type EventType string

const (
	EventTypePipelineTrigger EventType = "pipeline.trigger"
	EventTypePipelineComplete EventType = "pipeline.complete"
	EventTypePipelineError   EventType = "pipeline.error"
)

// WebhookEvent represents a webhook event for logging.
type WebhookEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	PipelineID string                `json:"pipeline_id"`
	Source    string                 `json:"source"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Handler triggers pipeline execution from external CI/CD webhooks.
type Handler struct {
	engine Engine
	secret string
	events []WebhookEvent
	mu     sync.RWMutex
}

// NewHandler creates a webhook handler. Secret from WEBHOOK_SECRET env when empty.
func NewHandler(engine Engine, secret string) *Handler {
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("WEBHOOK_SECRET"))
	}
	return &Handler{
		engine: engine,
		secret: secret,
		events: make([]WebhookEvent, 0),
	}
}

type triggerRequest struct {
	Content  string                 `json:"content"`
	Messages []pipeline.Message     `json:"messages,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TriggerPipeline POST /api/v1/webhooks/pipeline/:id
func (h *Handler) TriggerPipeline(c *gin.Context) {
	if h.engine == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "pipeline engine not initialized"})
		return
	}
	if !h.authorize(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	pipelineID := strings.TrimSpace(c.Param("id"))
	if pipelineID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "pipeline id required"})
		return
	}

	var req triggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Content) == "" && len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "content or messages required"})
		return
	}

	// Log trigger event
	h.logEvent(WebhookEvent{
		ID:         fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Type:       EventTypePipelineTrigger,
		PipelineID: pipelineID,
		Source:     c.GetHeader("X-Webhook-Source"),
		Success:    true,
		Metadata:   req.Metadata,
		Timestamp:  time.Now(),
	})

	timeout := 10 * time.Minute
	if p := h.engine.GetPipelineConfig(pipelineID); p != nil && p.GlobalConfig.Timeout > 0 {
		timeout = time.Duration(p.GlobalConfig.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	input := &pipeline.PipelineInput{
		Content:   req.Content,
		Messages:  req.Messages,
		Metadata:  req.Metadata,
		SessionID: c.GetHeader("X-Request-ID"),
	}
	if input.Metadata == nil {
		input.Metadata = map[string]interface{}{}
	}
	input.Metadata["webhook_trigger"] = true
	input.Metadata["webhook_source"] = c.GetHeader("X-Webhook-Source")

	output, err := h.engine.Execute(ctx, pipelineID, input)
	if err != nil {
		// Log error event
		h.logEvent(WebhookEvent{
			ID:         fmt.Sprintf("wh-%d", time.Now().UnixNano()),
			Type:       EventTypePipelineError,
			PipelineID: pipelineID,
			Source:     c.GetHeader("X-Webhook-Source"),
			Success:    false,
			Error:      err.Error(),
			Metadata:   req.Metadata,
			Timestamp:  time.Now(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Log complete event
	h.logEvent(WebhookEvent{
		ID:         fmt.Sprintf("wh-%d", time.Now().UnixNano()),
		Type:       EventTypePipelineComplete,
		PipelineID: pipelineID,
		Source:     c.GetHeader("X-Webhook-Source"),
		Success:    true,
		Metadata:   req.Metadata,
		Timestamp:  time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "data": output})
}

// GetEvents returns recent webhook events.
func (h *Handler) GetEvents(limit int) []WebhookEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.events) {
		limit = len(h.events)
	}

	// Return most recent events
	events := make([]WebhookEvent, limit)
	copy(events, h.events[len(h.events)-limit:])
	return events
}

// logEvent logs a webhook event.
func (h *Handler) logEvent(event WebhookEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)

	// Keep only last 1000 events
	if len(h.events) > 1000 {
		h.events = h.events[len(h.events)-1000:]
	}
}

func (h *Handler) authorize(c *gin.Context) bool {
	if h.secret != "" {
		if c.GetHeader("X-Webhook-Secret") == h.secret {
			return true
		}
	}
	// ProxyAuthMiddleware already validated Bearer / API key when route is protected.
	if uid, ok := c.Get("user_id"); ok && uid != nil {
		switch v := uid.(type) {
		case int:
			if v > 0 {
				return true
			}
		case int64:
			if v > 0 {
				return true
			}
		case uint:
			if v > 0 {
				return true
			}
		case uint64:
			if v > 0 {
				return true
			}
		default:
			if fmt.Sprintf("%v", v) != "" && fmt.Sprintf("%v", v) != "0" {
				return true
			}
		}
	}
	// Secure default: reject when neither webhook secret nor authenticated caller is present.
	return false
}