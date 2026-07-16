// Package plugin provides the plugin interface for cache evaluation
package plugin

import (
	"context"
	"time"
)

// PluginType defines the type of evaluation plugin
type PluginType string

const (
	// PluginTypeInput evaluates the input question
	PluginTypeInput PluginType = "input"
	// PluginTypeProcess evaluates the generated answer
	PluginTypeProcess PluginType = "process"
	// PluginTypeOutput makes the final caching decision
	PluginTypeOutput PluginType = "output"
)

// EvaluatorPlugin is the interface that all evaluation plugins must implement
type EvaluatorPlugin interface {
	// ========== Basic Information ==========

	// Name returns the unique identifier of the plugin
	Name() string

	// Version returns the plugin version
	Version() string

	// Type returns the plugin type (input/process/output)
	Type() PluginType

	// Description returns a human-readable description
	Description() string

	// ========== Core Evaluation ==========

	// Evaluate performs the evaluation
	// Input contains all necessary context for evaluation
	// Output contains the evaluation result
	Evaluate(ctx context.Context, input *EvalInput) (*EvalOutput, error)

	// ========== Configuration Management ==========

	// GetConfigSchema returns the configuration schema for this plugin
	GetConfigSchema() *ConfigSchema

	// ValidateConfig validates the provided configuration
	ValidateConfig(config map[string]interface{}) error

	// SetConfig sets the plugin configuration
	SetConfig(config map[string]interface{}) error

	// GetConfig returns the current configuration
	GetConfig() map[string]interface{}

	// ========== Lifecycle ==========

	// Init initializes the plugin
	Init() error

	// Close cleans up resources
	Close() error

	// HealthCheck checks if the plugin is healthy
	HealthCheck() error
}

// EvalInput contains all information needed for evaluation
type EvalInput struct {
	// Basic information
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`

	// Question related
	Question         string    `json:"question"`          // Expanded question
	OriginalQuestion string    `json:"original_question"` // Original user query
	IsExpanded       bool      `json:"is_expanded"`
	HistoryMessages  []Message `json:"history_messages"`

	// Answer related
	Answer     string `json:"answer"`
	AnswerType string `json:"answer_type"` // text | json | stream
	TokenCount int    `json:"token_count"`
	LatencyMs  int64  `json:"latency_ms"`

	// Model information
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`

	// Results from previous plugins in the pipeline
	PreviousResults map[string]*EvalOutput `json:"previous_results"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EvalOutput contains the evaluation result
type EvalOutput struct {
	// Score from 0 to 100
	Score float64 `json:"score"`

	// Passed indicates if the evaluation passed the threshold
	Passed bool `json:"passed"`

	// Labels are tags assigned by the plugin
	Labels []string `json:"labels"`

	// Details contains additional evaluation information
	Details map[string]interface{} `json:"details"`

	// ProcessTimeMs is the time taken to process
	ProcessTimeMs int64 `json:"process_time_ms"`

	// Metadata contains arbitrary metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// ConfigSchema defines the configuration structure
type ConfigSchema struct {
	Fields []ConfigField `json:"fields"`
}

// ConfigField defines a single configuration field
type ConfigField struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // string | number | boolean | array | object
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Min         *float64    `json:"min,omitempty"`
	Max         *float64    `json:"max,omitempty"`
	Options     []string    `json:"options,omitempty"` // For enum types
}

// BasePlugin provides common functionality for plugins
type BasePlugin struct {
	name        string
	version     string
	pluginType  PluginType
	description string
	config      map[string]interface{}
	schema      *ConfigSchema
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(name, version string, pluginType PluginType, description string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		version:     version,
		pluginType:  pluginType,
		description: description,
		config:      make(map[string]interface{}),
	}
}

// Name returns the plugin name
func (bp *BasePlugin) Name() string { return bp.name }

// Version returns the plugin version
func (bp *BasePlugin) Version() string { return bp.version }

// Type returns the plugin type
func (bp *BasePlugin) Type() PluginType { return bp.pluginType }

// Description returns the plugin description
func (bp *BasePlugin) Description() string { return bp.description }

// GetConfigSchema returns the config schema
func (bp *BasePlugin) GetConfigSchema() *ConfigSchema { return bp.schema }

// GetConfig returns the current config
func (bp *BasePlugin) GetConfig() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range bp.config {
		result[k] = v
	}
	return result
}

// SetConfig sets the configuration with validation
func (bp *BasePlugin) SetConfig(config map[string]interface{}) error {
	if err := bp.ValidateConfig(config); err != nil {
		return err
	}
	bp.config = config
	return nil
}

// ValidateConfig validates configuration against schema
func (bp *BasePlugin) ValidateConfig(config map[string]interface{}) error {
	if bp.schema == nil {
		return nil
	}

	for _, field := range bp.schema.Fields {
		value, exists := config[field.Name]

		// Check required fields
		if field.Required && !exists {
			return &ValidationError{
				Field:   field.Name,
				Message: "required field missing",
			}
		}

		// Skip validation if field not provided and not required
		if !exists {
			continue
		}

		// Type validation
		if !validateType(value, field.Type) {
			return &ValidationError{
				Field:   field.Name,
				Message: "invalid type",
			}
		}

		// Range validation for numbers
		if field.Type == "number" {
			if num, ok := value.(float64); ok {
				if field.Min != nil && num < *field.Min {
					return &ValidationError{
						Field:   field.Name,
						Message: "value below minimum",
					}
				}
				if field.Max != nil && num > *field.Max {
					return &ValidationError{
						Field:   field.Name,
						Message: "value above maximum",
					}
				}
			}
		}
	}

	return nil
}

// Init performs default initialization
func (bp *BasePlugin) Init() error { return nil }

// Close performs default cleanup
func (bp *BasePlugin) Close() error { return nil }

// HealthCheck performs default health check
func (bp *BasePlugin) HealthCheck() error { return nil }

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error for field '" + e.Field + "': " + e.Message
}

// Helper functions

func validateType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		return false
	}
}

// PtrFloat64 returns a pointer to a float64
func PtrFloat64(f float64) *float64 {
	return &f
}

// PtrInt returns a pointer to an int
func PtrInt(i int) *int {
	return &i
}

// Clamp restricts a value to a range
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// NowMs returns current timestamp in milliseconds
func NowMs() int64 {
	return time.Now().UnixMilli()
}
