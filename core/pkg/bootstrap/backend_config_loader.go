package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"centag/core/pkg/config"
	"centag/core/pkg/logger"

	"gopkg.in/yaml.v3"
)

// initialBackendFileOnce ensures we only log the file path discovery once
var initialBackendFileOnce sync.Once

// InitialBackendConfig represents a backend configuration from the JSON/YAML file
type InitialBackendConfig struct {
	ID              string                   `json:"id" yaml:"id"`
	Name            string                   `json:"name" yaml:"name"`
	Type            string                   `json:"type" yaml:"type"`
	BaseURL         string                   `json:"base_url" yaml:"base_url"`
	APIKey          string                   `json:"api_key" yaml:"api_key"`
	Enabled         bool                     `json:"enabled" yaml:"enabled"`
	Weight          int                      `json:"weight" yaml:"weight"`
	Timeout         int                      `json:"timeout" yaml:"timeout"`
	MaxRetries      int                      `json:"max_retries" yaml:"max_retries"`
	Description     string                   `json:"description" yaml:"description"`
	ProbeModel      string                   `json:"probe_model,omitempty" yaml:"probe_model,omitempty"`
	SupportedModels []InitialModelMapping    `json:"supported_models" yaml:"supported_models"`
	Capabilities    InitialModelCapabilities `json:"capabilities" yaml:"capabilities"`
	Priority        int                      `json:"priority" yaml:"priority"`
	AutoFetchModels bool                     `json:"auto_fetch_models" yaml:"auto_fetch_models"`
}

// InitialModelMapping represents a model mapping from the JSON/YAML file
type InitialModelMapping struct {
	RequestedModel     string  `json:"requested_model" yaml:"requested_model"`
	ActualModel        string  `json:"actual_model" yaml:"actual_model"`
	IsExact            bool    `json:"is_exact" yaml:"is_exact"`
	CompatibilityScore float64 `json:"compatibility_score,omitempty" yaml:"compatibility_score,omitempty"`
}

// InitialModelCapabilities represents model capabilities from the JSON/YAML file
type InitialModelCapabilities struct {
	MaxContextTokens int      `json:"max_context_tokens" yaml:"max_context_tokens"`
	Features         []string `json:"features" yaml:"features"`
	SupportsTools    bool     `json:"supports_tools" yaml:"supports_tools"`
}

// InitialBackendFile represents the structure of the initial backends JSON/YAML file
type InitialBackendFile struct {
	Version           string                    `json:"version" yaml:"version"`
	Description       string                    `json:"description" yaml:"description"`
	Backends          []InitialBackendConfig    `json:"backends" yaml:"backends"`
	PipelineTemplates []InitialPipelineTemplate `json:"pipeline_templates,omitempty" yaml:"pipeline_templates,omitempty"`
}

// InitialPipelineTemplate represents a pipeline template from the JSON file
type InitialPipelineTemplate struct {
	ID            string                       `json:"id" yaml:"id"`
	SchemaVersion string                       `json:"schema_version,omitempty" yaml:"schema_version,omitempty"`
	Name          string                       `json:"name" yaml:"name"`
	Description   string                       `json:"description" yaml:"description"`
	ShortcutCode  string                       `json:"shortcut_code,omitempty" yaml:"shortcut_code,omitempty"`
	Nodes         []InitialPipelineNodeConfig  `json:"nodes" yaml:"nodes"`
	GlobalConfig  *InitialGlobalPipelineConfig `json:"global_config,omitempty" yaml:"global_config,omitempty"`
	Metadata      map[string]interface{}       `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type InitialPipelineNodeConfig struct {
	ID              string                 `json:"id" yaml:"id"`
	Type            string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Kind            string                 `json:"kind,omitempty" yaml:"kind,omitempty"`
	Implementation  string                 `json:"implementation,omitempty" yaml:"implementation,omitempty"`
	Name            string                 `json:"name" yaml:"name"`
	Backend         string                 `json:"backend,omitempty" yaml:"backend,omitempty"`
	Model           string                 `json:"model,omitempty" yaml:"model,omitempty"`
	Config          InitialNodeConfig      `json:"config" yaml:"config"`
	Inputs          map[string]string      `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs         map[string]interface{} `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	ConfigSchemaRef string                 `json:"config_schema_ref,omitempty" yaml:"config_schema_ref,omitempty"`
	SecretsRef      map[string]string      `json:"secrets_ref,omitempty" yaml:"secrets_ref,omitempty"`
	Permissions     []string               `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Timeout         int                    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retry           *InitialRetryConfig    `json:"retry,omitempty" yaml:"retry,omitempty"`
	Condition       string                 `json:"condition,omitempty" yaml:"condition,omitempty"`
	NextNodes       []string               `json:"next_nodes,omitempty" yaml:"next_nodes,omitempty"`
	DependsOn       []string               `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	RouteConfig     *InitialRouteConfig     `json:"route_config,omitempty" yaml:"route_config,omitempty"`
}

type InitialNodeConfig struct {
	Backend        string                 `json:"backend,omitempty" yaml:"backend,omitempty"`
	Model          string                 `json:"model,omitempty" yaml:"model,omitempty"`
	PromptTemplate string                 `json:"prompt_template,omitempty" yaml:"prompt_template,omitempty"`
	SystemPrompt   string                 `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	Temperature    *float64               `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	MaxTokens      *int                   `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty" yaml:"custom_config,omitempty"`
	TemplateVars   map[string]string      `json:"template_vars,omitempty" yaml:"template_vars,omitempty"`
}

type InitialRetryConfig struct {
	MaxAttempts     int    `json:"max_attempts" yaml:"max_attempts"`
	BackoffStrategy string `json:"backoff_strategy" yaml:"backoff_strategy"`
	InitialDelay    int    `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay        int    `json:"max_delay" yaml:"max_delay"`
}

type InitialRouteConfig struct {
	RouterNodeID string `json:"router_node_id" yaml:"router_node_id"`
	RouteValue   string `json:"route_value" yaml:"route_value"`
	IsDefault    bool   `json:"is_default" yaml:"is_default"`
}

type InitialGlobalPipelineConfig struct {
	Timeout         int                         `json:"timeout" yaml:"timeout"`
	MaxRetries      int                         `json:"max_retries" yaml:"max_retries"`
	BypassOnError   bool                        `json:"bypass_on_error" yaml:"bypass_on_error"`
	ParallelLimit   int                         `json:"parallel_limit" yaml:"parallel_limit"`
	LogLevel        string                      `json:"log_level,omitempty" yaml:"log_level,omitempty"`
	SystemPrompt    string                      `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	FallbackGroups  []InitialFallbackGroup     `json:"fallback_groups,omitempty" yaml:"fallback_groups,omitempty"`
	Storage         *InitialStorageHookConfig   `json:"storage,omitempty" yaml:"storage,omitempty"`
	Hooks           []InitialHookConfig         `json:"hooks,omitempty" yaml:"hooks,omitempty"`
}

type InitialStorageHookConfig struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	AutoSave      bool   `json:"auto_save" yaml:"auto_save"`
	SaveInterval  int    `json:"save_interval" yaml:"save_interval"`
	RetentionDays int    `json:"retention_days" yaml:"retention_days"`
}

type InitialHookConfig struct {
	Type        string                 `json:"type" yaml:"type"`
	On          []string               `json:"on" yaml:"on"`
	StorageName string                 `json:"storage_name,omitempty" yaml:"storage_name,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

type InitialFallbackGroup struct {
	PrimaryNodeID  string   `json:"primary_node_id" yaml:"primary_node_id"`
	FallbackNodes   []string `json:"fallback_nodes" yaml:"fallback_nodes"`
	MaxAttempts     int      `json:"max_attempts" yaml:"max_attempts"`
}

// loadInitialBackendFile loads backend seed from the edition/customer initdata.
//
// Priority (no union merge):
//  1. Profile / INITDATA_PATH directory — if it contains initial-backends.yaml|json, use it alone
//  2. Global ProjectRoot()/config/initdata — fallback when profile has no backends file
//     (e.g. custom --initdata zip placed at config/initdata)
//
// A universal vendor catalog must not live under config/initdata as a runtime seed;
// see config/profiles/_shared/initdata/backends-catalog.yaml for reference only.
func loadInitialBackendFile() (*InitialBackendFile, string, error) {
	globalRoot, profileRoot := InitdataRoots()
	if globalRoot == "" && profileRoot == "" {
		logger.Info("bootstrap: INITDATA_PATH / ProjectRoot 未确定，跳过初始后端配置加载")
		return nil, "", nil
	}

	sameRoot := globalRoot != "" && profileRoot != "" &&
		filepath.Clean(globalRoot) == filepath.Clean(profileRoot)

	// 1. Prefer profile / INITDATA_PATH when it has a backends file
	if profileRoot != "" {
		profileConfig, profilePath, profileLoaded, profileErr := loadBackendFileFromDir(profileRoot)
		if profileErr != nil {
			return nil, profilePath, profileErr
		}
		if profileLoaded {
			initialBackendFileOnce.Do(func() {
				logger.Infof("bootstrap: 从 Profile/INITDATA 加载初始后端: %s (%d backends)", profilePath, len(profileConfig.Backends))
			})
			return profileConfig, profilePath, nil
		}
	}

	// 2. Fallback to global config/initdata (customer zip or bare install)
	if !sameRoot && globalRoot != "" {
		globalConfig, globalPath, globalLoaded, globalErr := loadBackendFileFromDir(globalRoot)
		if globalErr != nil {
			return nil, globalPath, globalErr
		}
		if globalLoaded {
			initialBackendFileOnce.Do(func() {
				logger.Infof("bootstrap: Profile 无 initial-backends，回退全局: %s (%d backends)", globalPath, len(globalConfig.Backends))
			})
			return globalConfig, globalPath, nil
		}
	}

	logger.Info("bootstrap: 未找到初始后端配置文件（Profile 或全局），使用空配置")
	return nil, "", nil
}

// loadBackendFileFromDir attempts to load initial-backends.yaml then initial-backends.json from a directory.
// Returns (config, path, loaded, error). If a file exists but fails to parse, returns the error.
func loadBackendFileFromDir(root string) (*InitialBackendFile, string, bool, error) {
	if root == "" {
		return nil, "", false, nil
	}

	yamlPath := filepath.Join(root, "initial-backends.yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		var cfg InitialBackendFile
		unmarshalErr := yaml.Unmarshal(data, &cfg)
		if unmarshalErr != nil {
			logger.Warnf("bootstrap: 解析初始后端配置文件失败 (YAML): %v", unmarshalErr)
			return nil, yamlPath, false, unmarshalErr
		}
		return &cfg, yamlPath, true, nil
	}

	jsonPath := filepath.Join(root, "initial-backends.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		var cfg InitialBackendFile
		if err := json.Unmarshal(data, &cfg); err == nil {
			return &cfg, jsonPath, true, nil
		}
		logger.Warnf("bootstrap: 解析初始后端配置文件失败 (JSON): %v", err)
		return nil, jsonPath, false, err
	}

	return nil, "", false, nil
}

// LoadInitialBackendsFromJSON loads backend seed configurations for first-run bootstrap.
// Source is edition/customer initial-backends.yaml|json via loadInitialBackendFile (profile-first).
// Returns nil if no file exists or it contains no backends.
func LoadInitialBackendsFromJSON() []config.BackendConfig {
	fileConfig, foundPath, err := loadInitialBackendFile()
	if err != nil || fileConfig == nil {
		return nil
	}

	// Convert to BackendConfig
	backends := make([]config.BackendConfig, 0, len(fileConfig.Backends))
	for _, initial := range fileConfig.Backends {
		backend := convertToBackendConfig(initial)
		backends = append(backends, backend)
	}

	if len(backends) == 0 {
		logger.Warnf("bootstrap: %s 为空", foundPath)
		return nil
	}

	initialBackendFileOnce.Do(func() {
		logger.Infof("bootstrap: 成功加载 %d 个后端配置", len(backends))
	})
	return backends
}

// ParseInitialBackendsFile parses in-memory initial-backends.yaml|json content and
// converts entries to runtime backend configs. It reuses the exact conversion path
// of first-run seeding (placeholder resolution / bearer prefix strip / model mapping),
// so config archive import (一键还原) behaves identically to a fresh seed.
func ParseInitialBackendsFile(data []byte) ([]config.BackendConfig, error) {
	var file InitialBackendFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse initial backends content: %w", err)
	}
	out := make([]config.BackendConfig, 0, len(file.Backends))
	for _, initial := range file.Backends {
		out = append(out, convertToBackendConfig(initial))
	}
	return out, nil
}

// convertToBackendConfig converts InitialBackendConfig to config.BackendConfig
func convertToBackendConfig(initial InitialBackendConfig) config.BackendConfig {
	// Resolve base_url placeholders
	baseURL := resolvePlaceholders(initial.BaseURL)

	apiKey := resolvePlaceholders(strings.TrimSpace(initial.APIKey))
	if len(apiKey) >= 7 && strings.EqualFold(apiKey[:7], "bearer ") {
		apiKey = strings.TrimSpace(apiKey[7:])
	}

	// Convert supported models
	supportedModels := make([]config.ModelMapping, 0, len(initial.SupportedModels))
	for _, model := range initial.SupportedModels {
		supportedModels = append(supportedModels, config.ModelMapping{
			RequestedModel:     resolvePlaceholders(model.RequestedModel),
			ActualModel:        resolvePlaceholders(model.ActualModel),
			IsExact:            model.IsExact,
			CompatibilityScore: model.CompatibilityScore,
		})
	}

	return config.BackendConfig{
		ID:              initial.ID,
		Name:            initial.Name,
		Type:            initial.Type,
		BaseURL:         baseURL,
		APIKey:          apiKey,
		Enabled:         initial.Enabled,
		Timeout:         initial.Timeout,
		MaxRetries:      initial.MaxRetries,
		Description:     initial.Description,
		ProbeModel:      strings.TrimSpace(initial.ProbeModel),
		SupportedModels: supportedModels,
		Capabilities: config.ModelCapabilities{
			MaxContextTokens: initial.Capabilities.MaxContextTokens,
			Features:         initial.Capabilities.Features,
			SupportsTools:    initial.Capabilities.SupportsTools,
		},
		AutoFetchModels: initial.AutoFetchModels,
	}
}

// resolvePlaceholders replaces {{ENV_VAR|default}} and ${VAR} patterns with actual values
func resolvePlaceholders(s string) string {
	// First handle {{ENV_VAR|default}} patterns
	pattern := regexp.MustCompile(`\{\{([^|}]+)\|([^|}]*)\}\}`)
	result := pattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract env var name and default
		parts := strings.Split(match[2:len(match)-2], "|")
		if len(parts) != 2 {
			return match // Return original if invalid
		}
		envVar := strings.TrimSpace(parts[0])
		defaultValue := strings.TrimSpace(parts[1])

		// Try to get from environment
		if val := os.Getenv(envVar); val != "" {
			return val
		}
		return defaultValue
	})

	// Then handle ${VAR} patterns
	pattern2 := regexp.MustCompile(`\$\{([^}]+)\}`)
	result = pattern2.ReplaceAllStringFunc(result, func(match string) string {
		// Extract env var name
		envVar := strings.TrimSpace(match[2 : len(match)-1])
		if val := os.Getenv(envVar); val != "" {
			return val
		}
		return "" // Return empty string if not found (consistent with {{VAR|}} behavior)
	})

	return result
}
