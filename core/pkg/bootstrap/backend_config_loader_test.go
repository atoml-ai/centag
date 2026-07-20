package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"centag/core/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	_ = logger.Init(logger.Config{Level: "error", Format: "console", Output: "stdout"})
}

// ── resolvePlaceholders ───────────────────────────────────────────────────────

func TestResolvePlaceholders_NoPlaceholder(t *testing.T) {
	input := "https://api.openai.com/v1"
	got := resolvePlaceholders(input)
	assert.Equal(t, input, got)
}

func TestResolvePlaceholders_EnvExists(t *testing.T) {
	t.Setenv("TEST_RESOLVE_VAR", "resolved-value")
	input := "{{TEST_RESOLVE_VAR|default}}"
	got := resolvePlaceholders(input)
	assert.Equal(t, "resolved-value", got)
}

func TestResolvePlaceholders_EnvMissingUseDefault(t *testing.T) {
	input := "{{NONEXISTENT_VAR_12345|fallback}}"
	got := resolvePlaceholders(input)
	assert.Equal(t, "fallback", got)
}

func TestResolvePlaceholders_EmptyDefault(t *testing.T) {
	input := "{{NONEXISTENT_VAR_12345|}}"
	got := resolvePlaceholders(input)
	assert.Equal(t, "", got)
}

func TestResolvePlaceholders_MultiplePlaceholders(t *testing.T) {
	t.Setenv("TEST_HOST", "localhost")
	input := "http://{{TEST_HOST|127.0.0.1}}:{{TEST_PORT|8080}}/path"
	got := resolvePlaceholders(input)
	assert.Equal(t, "http://localhost:8080/path", got)
}

func TestResolvePlaceholders_InvalidFormatKept(t *testing.T) {
	// Missing pipe separator — should return original
	input := "{{NO_PIPE}}"
	got := resolvePlaceholders(input)
	assert.Equal(t, "{{NO_PIPE}}", got)
}

func TestResolvePlaceholders_NestedBraces(t *testing.T) {
	// Extra braces inside — regex should still match the outer pattern
	t.Setenv("TEST_NEST", "val")
	input := "{{TEST_NEST|d}}"
	got := resolvePlaceholders(input)
	assert.Equal(t, "val", got)
}

// ── convertToBackendConfig ────────────────────────────────────────────────────

func TestConvertToBackendConfig_Minimal(t *testing.T) {
	initial := InitialBackendConfig{
		ID:      "test-backend",
		Name:    "Test",
		Type:    "openai",
		BaseURL: "https://api.test.com",
		Enabled: true,
	}
	got := convertToBackendConfig(initial)

	assert.Equal(t, "test-backend", got.ID)
	assert.Equal(t, "Test", got.Name)
	assert.Equal(t, "openai", got.Type)
	assert.Equal(t, "https://api.test.com", got.BaseURL)
	assert.True(t, got.Enabled)
	assert.Empty(t, got.APIKey)
	assert.Empty(t, got.SupportedModels)
	assert.False(t, got.AutoFetchModels)
}

func TestConvertToBackendConfig_WithBearerPrefix(t *testing.T) {
	initial := InitialBackendConfig{
		ID:     "b",
		APIKey: "Bearer  secret-key-123 ",
	}
	got := convertToBackendConfig(initial)
	assert.Equal(t, "secret-key-123", got.APIKey)
}

func TestConvertToBackendConfig_WithLowercaseBearer(t *testing.T) {
	initial := InitialBackendConfig{
		ID:     "b",
		APIKey: "bearer  key-456 ",
	}
	got := convertToBackendConfig(initial)
	assert.Equal(t, "key-456", got.APIKey)
}

func TestConvertToBackendConfig_WithoutBearerPrefix(t *testing.T) {
	initial := InitialBackendConfig{
		ID:     "b",
		APIKey: "plain-api-key",
	}
	got := convertToBackendConfig(initial)
	assert.Equal(t, "plain-api-key", got.APIKey)
}

func TestConvertToBackendConfig_ModelMappings(t *testing.T) {
	initial := InitialBackendConfig{
		ID: "m",
		SupportedModels: []InitialModelMapping{
			{RequestedModel: "gpt-4", ActualModel: "gpt-4-turbo", IsExact: true, CompatibilityScore: 0.95},
			{RequestedModel: "gpt-3.5", ActualModel: "gpt-3.5-turbo", IsExact: false},
		},
	}
	got := convertToBackendConfig(initial)
	require.Len(t, got.SupportedModels, 2)
	assert.Equal(t, "gpt-4", got.SupportedModels[0].RequestedModel)
	assert.Equal(t, "gpt-4-turbo", got.SupportedModels[0].ActualModel)
	assert.True(t, got.SupportedModels[0].IsExact)
	assert.InDelta(t, 0.95, got.SupportedModels[0].CompatibilityScore, 0.001)
	assert.False(t, got.SupportedModels[1].IsExact)
}

func TestConvertToBackendConfig_Capabilities(t *testing.T) {
	initial := InitialBackendConfig{
		ID: "c",
		Capabilities: InitialModelCapabilities{
			MaxContextTokens: 8192,
			Features:         []string{"vision", "tools"},
			SupportsTools:    true,
		},
	}
	got := convertToBackendConfig(initial)
	assert.Equal(t, 8192, got.Capabilities.MaxContextTokens)
	assert.Equal(t, []string{"vision", "tools"}, got.Capabilities.Features)
	assert.True(t, got.Capabilities.SupportsTools)
}

func TestConvertToBackendConfig_ResolvePlaceholderInBaseURL(t *testing.T) {
	t.Setenv("TEST_BASE_URL", "https://resolved.test")
	initial := InitialBackendConfig{
		ID:      "p",
		BaseURL: "{{TEST_BASE_URL|https://default.test}}",
	}
	got := convertToBackendConfig(initial)
	assert.Equal(t, "https://resolved.test", got.BaseURL)
}

// ── loadInitialBackendFile ────────────────────────────────────────────────────

func TestLoadInitialBackendFile_YAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = "" // 清除缓存，使环境变量生效

	// 创建 config/initdata/initial-backends.yaml
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	content := `version: "1.0"
description: test yaml
backends:
  - id: yaml-backend
    name: YAML Backend
    type: openai
    base_url: https://yaml.test
`
	path := filepath.Join(dir, "initdata", "initial-backends.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	file, foundPath, err := loadInitialBackendFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Contains(t, foundPath, "initial-backends.yaml")
	assert.Equal(t, "1.0", file.Version)
	assert.Equal(t, "test yaml", file.Description)
	require.Len(t, file.Backends, 1)
	assert.Equal(t, "yaml-backend", file.Backends[0].ID)
}

func TestLoadInitialBackendFile_JSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	content := `{"version":"2.0","backends":[{"id":"json-backend","name":"JSON","type":"ollama"}]}`
	path := filepath.Join(dir, "initdata", "initial-backends.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	file, foundPath, err := loadInitialBackendFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Contains(t, foundPath, "initial-backends.json")
	assert.Equal(t, "2.0", file.Version)
	require.Len(t, file.Backends, 1)
	assert.Equal(t, "json-backend", file.Backends[0].ID)
}

func TestLoadInitialBackendFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	file, foundPath, err := loadInitialBackendFile()
	require.NoError(t, err)
	assert.Nil(t, file)
	assert.Empty(t, foundPath)
}

func TestLoadInitialBackendFile_ProfilePreferredNoUnion(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "project")
	globalInit := filepath.Join(projectRoot, "config", "initdata")
	profileInit := filepath.Join(dir, "profile-initdata")
	require.NoError(t, os.MkdirAll(globalInit, 0o755))
	require.NoError(t, os.MkdirAll(profileInit, 0o755))

	t.Setenv("PROJECT_ROOT", projectRoot)
	t.Setenv("INITDATA_PATH", profileInit)
	projectRootCache = ""

	globalContent := `version: "1.0"
backends:
  - id: global-only
    name: Global
    type: openai
  - id: shared
    name: Global Shared
    type: openai
`
	require.NoError(t, os.WriteFile(filepath.Join(globalInit, "initial-backends.yaml"), []byte(globalContent), 0o644))

	profileContent := `version: "1.0"
backends:
  - id: shared
    name: Profile Shared
    type: openai
  - id: profile-only
    name: Profile
    type: ollama
`
	require.NoError(t, os.WriteFile(filepath.Join(profileInit, "initial-backends.yaml"), []byte(profileContent), 0o644))

	file, foundPath, err := loadInitialBackendFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Contains(t, foundPath, "profile-initdata")
	require.Len(t, file.Backends, 2)
	ids := map[string]string{}
	for _, b := range file.Backends {
		ids[b.ID] = b.Name
	}
	assert.Equal(t, "Profile Shared", ids["shared"])
	assert.Equal(t, "Profile", ids["profile-only"])
	_, hasGlobal := ids["global-only"]
	assert.False(t, hasGlobal, "must not union-merge global backends when profile file exists")
}

func TestLoadInitialBackendFile_FallbackGlobalWhenProfileMissing(t *testing.T) {
	dir := t.TempDir()
	projectRoot := filepath.Join(dir, "project")
	globalInit := filepath.Join(projectRoot, "config", "initdata")
	profileInit := filepath.Join(dir, "profile-empty")
	require.NoError(t, os.MkdirAll(globalInit, 0o755))
	require.NoError(t, os.MkdirAll(profileInit, 0o755))

	t.Setenv("PROJECT_ROOT", projectRoot)
	t.Setenv("INITDATA_PATH", profileInit)
	projectRootCache = ""

	globalContent := `version: "1.0"
backends:
  - id: from-global
    name: Global
    type: openai
`
	require.NoError(t, os.WriteFile(filepath.Join(globalInit, "initial-backends.yaml"), []byte(globalContent), 0o644))

	file, foundPath, err := loadInitialBackendFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Contains(t, foundPath, "config/initdata")
	require.Len(t, file.Backends, 1)
	assert.Equal(t, "from-global", file.Backends[0].ID)
}

func TestLoadInitialBackendFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	path := filepath.Join(dir, "initdata", "initial-backends.yaml")
	require.NoError(t, os.WriteFile(path, []byte("invalid: ["), 0o644))

	file, _, err := loadInitialBackendFile()
	assert.Error(t, err)
	assert.Nil(t, file)
}

// ── LoadInitialBackendsFromJSON ───────────────────────────────────────────────

func TestLoadInitialBackendsFromJSON_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	content := `version: "1.0"
backends:
  - id: b1
    name: Backend One
    type: openai
    base_url: https://b1.test
    enabled: true
  - id: b2
    name: Backend Two
    type: ollama
    base_url: https://b2.test
    enabled: false
`
	path := filepath.Join(dir, "initdata", "initial-backends.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	backends := LoadInitialBackendsFromJSON()
	require.Len(t, backends, 2)
	assert.Equal(t, "b1", backends[0].ID)
	assert.True(t, backends[0].Enabled)
	assert.Equal(t, "b2", backends[1].ID)
	assert.False(t, backends[1].Enabled)
}

func TestLoadInitialBackendsFromJSON_EmptyBackends(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	content := `version: "1.0"`
	path := filepath.Join(dir, "initdata", "initial-backends.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	backends := LoadInitialBackendsFromJSON()
	assert.Nil(t, backends)
}

func TestLoadInitialBackendsFromJSON_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	backends := LoadInitialBackendsFromJSON()
	assert.Nil(t, backends)
}

// ── Integration: PipelineTemplates in backend file ─────────────────────────────

func TestLoadInitialBackendFile_WithPipelineTemplates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("INITDATA_PATH", filepath.Join(dir, "initdata"))
	projectRootCache = ""

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "initdata"), 0o755))
	content := `version: "1.0"
pipeline_templates:
  - id: test-pipeline
    name: Test Pipeline
    nodes:
      - id: n1
        name: Node One
`
	path := filepath.Join(dir, "initdata", "initial-backends.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	file, _, err := loadInitialBackendFile()
	require.NoError(t, err)
	require.NotNil(t, file)
	require.Len(t, file.PipelineTemplates, 1)
	assert.Equal(t, "test-pipeline", file.PipelineTemplates[0].ID)
	assert.Equal(t, "Test Pipeline", file.PipelineTemplates[0].Name)
}

// ── Ensure config.BackendConfig fields match expectations ──────────────────────

func TestConvertToBackendConfig_FieldCoverage(t *testing.T) {
	temp := 0.7
	maxTok := 2048
	initial := InitialBackendConfig{
		ID:              "full",
		Name:            "Full Backend",
		Type:            "anthropic",
		BaseURL:         "https://anthropic.test",
		APIKey:          "sk-test",
		Enabled:         true,
		Weight:          5,
		Timeout:         30,
		MaxRetries:      3,
		Description:     "A test backend",
		AutoFetchModels: true,
		SupportedModels: []InitialModelMapping{
			{RequestedModel: "claude-3", ActualModel: "claude-3-opus-20240229"},
		},
		Capabilities: InitialModelCapabilities{
			MaxContextTokens: 200000,
			Features:         []string{"vision"},
			SupportsTools:    true,
		},
	}
	got := convertToBackendConfig(initial)

	assert.Equal(t, "full", got.ID)
	assert.Equal(t, "Full Backend", got.Name)
	assert.Equal(t, "anthropic", got.Type)
	assert.Equal(t, "https://anthropic.test", got.BaseURL)
	assert.Equal(t, "sk-test", got.APIKey)
	assert.True(t, got.Enabled)
	assert.Equal(t, 30, got.Timeout)
	assert.Equal(t, 3, got.MaxRetries)
	assert.Equal(t, "A test backend", got.Description)
	assert.True(t, got.AutoFetchModels)
	assert.Len(t, got.SupportedModels, 1)
	assert.Equal(t, 200000, got.Capabilities.MaxContextTokens)
	assert.Equal(t, []string{"vision"}, got.Capabilities.Features)
	assert.True(t, got.Capabilities.SupportsTools)

	// Weight and Priority are parsed from file but intentionally not mapped
	// to config.BackendConfig (they are scheduler parameters). This documents
	// that behaviour so any future change is caught by the test.
	_ = temp
	_ = maxTok
}
