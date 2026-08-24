package server

// 配置归档导入（一键还原）：接收「配置导出」生成的 centag-initdata.zip，
// 应用其中的 initial-backends.yaml、pipeline-templates/*.yaml 与 system-config.yaml。
//
// 语义与首轮 seeding 完全一致：
//   - 后端走 bootstrap.ParseInitialBackendsFile（占位符解析 / bearer 剥离 / 模型映射）
//   - 流水线走 InitialPipelineTemplate → CreatePipelineFromTemplate → Register（upsert）
//   - 默认后端/模型走 PersistProxyConfig（与首页设置同一持久化路径）
//
// 还原为 upsert：已存在的同 ID 资源被覆盖，其余资源不受影响。

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"centag/core/pkg/backend"
	"centag/core/pkg/bootstrap"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
	"centag/core/pkg/pipeline"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// configImportMaxBytes 归档大小上限（20MB）：导出产物为纯 YAML 文本，实际远小于此。
const configImportMaxBytes = 20 << 20

type configImportResult struct {
	BackendsUpserted int      `json:"backends_upserted"`
	BackendsFailed   int      `json:"backends_failed"`
	PipelinesApplied int      `json:"pipelines_applied"`
	PipelinesFailed  int      `json:"pipelines_failed"`
	DefaultsApplied  bool     `json:"defaults_applied"`
	DefaultsSkipped  string   `json:"defaults_skipped,omitempty"`
	PipelineErrors   []string `json:"pipeline_errors,omitempty"`
}

func (s *Server) importConfigArchive(c *gin.Context) {
	if s.backendManager == nil || s.pipelineHandler == nil || s.pipelineHandler.pipelineRegistry == nil {
		RespondInternalError(c, "config import is unavailable in this edition")
		return
	}
	if user := s.pipelineHandler.accessUser(c); user != nil && !user.CanAddOwnBackends {
		RespondError(c, http.StatusForbidden, "adding or modifying own backends is disabled for this user")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		RespondBadRequest(c, "multipart field 'file' (zip archive) is required")
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		RespondBadRequest(c, "cannot read uploaded file: "+err.Error())
		return
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, configImportMaxBytes+1))
	if err != nil {
		RespondBadRequest(c, "cannot read uploaded file: "+err.Error())
		return
	}
	if len(data) > configImportMaxBytes {
		RespondBadRequest(c, fmt.Sprintf("archive too large (max %d bytes)", configImportMaxBytes))
		return
	}

	backendsData, templatesData, systemConfigData, err := parseConfigArchive(data)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return
	}
	if backendsData == nil && len(templatesData) == 0 && systemConfigData == nil {
		RespondBadRequest(c, "archive contains neither initial-backends.yaml nor pipeline-templates/*.yaml nor system-config.yaml")
		return
	}

	result := configImportResult{}
	if backendsData != nil {
		if !result.applyBackends(s, c, backendsData) {
			return
		}
	}
	for _, td := range templatesData {
		result.applyTemplate(s, td)
	}
	if systemConfigData != nil {
		result.applySystemConfig(s, c, systemConfigData)
	}
	logger.Infof("config import: backends upserted=%d failed=%d pipelines applied=%d failed=%d defaults_applied=%v",
		result.BackendsUpserted, result.BackendsFailed, result.PipelinesApplied, result.PipelinesFailed, result.DefaultsApplied)
	RespondSuccess(c, gin.H{
		"success": true,
		"result":  result,
	})
}

// applySystemConfig restores default backend/model (proxy defaults). It runs after
// backends are applied so the referenced backend usually already exists; a missing
// backend skips the restore with an explanatory result instead of dangling state.
func (r *configImportResult) applySystemConfig(s *Server, c *gin.Context, data []byte) {
	var sys struct {
		DefaultBackendID  string `json:"default_backend_id" yaml:"default_backend_id"`
		DefaultModel      string `json:"default_model" yaml:"default_model"`
		FallbackBackendID string `json:"fallback_backend_id" yaml:"fallback_backend_id"`
		FallbackModel     string `json:"fallback_model" yaml:"fallback_model"`
	}
	if err := yaml.Unmarshal(data, &sys); err != nil {
		r.DefaultsSkipped = "unparsable system-config.yaml: " + err.Error()
		return
	}
	cfg := config.Get()
	if cfg == nil {
		r.DefaultsSkipped = "system config not initialized"
		return
	}
	defBackend := strings.TrimSpace(sys.DefaultBackendID)
	if defBackend != "" {
		if _, err := s.backendManager.Get(defBackend); err != nil {
			r.DefaultsSkipped = fmt.Sprintf("default backend %q not present in archive or instance", defBackend)
			return
		}
	}
	fbBackend := strings.TrimSpace(sys.FallbackBackendID)
	if fbBackend != "" {
		if _, err := s.backendManager.Get(fbBackend); err != nil {
			r.DefaultsSkipped = fmt.Sprintf("fallback backend %q not present in archive or instance", fbBackend)
			return
		}
	}
	cfg.Proxy.DefaultBackendID = defBackend
	cfg.Proxy.DefaultModel = strings.TrimSpace(sys.DefaultModel)
	cfg.Proxy.FallbackBackendID = fbBackend
	cfg.Proxy.FallbackModel = strings.TrimSpace(sys.FallbackModel)
	// 与 handleSaveProxyConfig 一致：默认模型缺省时从后端首选模型回填
	if cfg.Proxy.DefaultModel == "" && cfg.Proxy.DefaultBackendID != "" {
		if filled := s.preferredModelForBackend(cfg.Proxy.DefaultBackendID); filled != "" {
			cfg.Proxy.DefaultModel = filled
		}
	}
	if err := config.PersistProxyConfig(c.Request.Context(), cfg.Proxy); err != nil {
		r.DefaultsSkipped = "persist proxy config: " + err.Error()
		return
	}
	r.DefaultsApplied = true
}

// applyBackends parses initial-backends content and upserts into backendManager.
// Returns false when it responded with an unrecoverable error (caller must abort).
func (r *configImportResult) applyBackends(s *Server, c *gin.Context, data []byte) bool {
	cfgs, err := bootstrap.ParseInitialBackendsFile(data)
	if err != nil {
		RespondBadRequest(c, err.Error())
		return false
	}
	now := time.Now().Format(time.RFC3339)
	for i := range cfgs {
		bc := backendConfigFromInitial(&cfgs[i], now)
		if _, err := s.backendManager.Get(bc.ID); err == nil {
			err = s.backendManager.Update(bc)
		} else {
			err = s.backendManager.Add(bc)
		}
		if err != nil {
			logger.Warnf("config import: backend %s: %v", bc.ID, err)
			r.BackendsFailed++
			continue
		}
		r.BackendsUpserted++
		// 若该后端是系统默认，同步 proxy 默认模型（与 UpdateBackend 行为一致）
		syncProxyDefaultModelFromBackend(bc.ID)
	}
	if r.BackendsUpserted > 0 {
		if err := s.backendManager.Save(); err != nil {
			RespondInternalError(c, "failed to persist backends: "+err.Error())
			return false
		}
	}
	return true
}

// applyTemplate converts one template and upserts it via the pipeline registry.
func (r *configImportResult) applyTemplate(s *Server, data []byte) {
	tmpl, err := bootstrap.ParseInitialPipelineTemplate(data)
	if err != nil {
		logger.Warnf("config import: %v", err)
		r.PipelinesFailed++
		r.PipelineErrors = append(r.PipelineErrors, err.Error())
		return
	}
	converted := convertInitialTemplates([]bootstrap.InitialPipelineTemplate{*tmpl})
	if len(converted) == 0 {
		r.PipelinesFailed++
		r.PipelineErrors = append(r.PipelineErrors, "template produced no pipeline")
		return
	}
	p := pipeline.CreatePipelineFromTemplate(converted[0], nil)
	if p == nil {
		r.PipelinesFailed++
		r.PipelineErrors = append(r.PipelineErrors, fmt.Sprintf("template %s conversion failed", tmpl.ID))
		return
	}
	reg := s.pipelineHandler.pipelineRegistry
	if err := reg.Register(p); err != nil {
		logger.Warnf("config import: pipeline %s: %v", p.ID, err)
		r.PipelinesFailed++
		r.PipelineErrors = append(r.PipelineErrors, fmt.Sprintf("%s: %v", p.ID, err))
		return
	}
	if s.pipelineHandler.engine != nil {
		s.pipelineHandler.engine.InvalidateStorageHookCache(p.ID)
	}
	r.PipelinesApplied++
}

// backendConfigFromInitial mirrors entrypoint_full.go's backendConfigFromConfig.
func backendConfigFromInitial(c *config.BackendConfig, now string) *backend.BackendConfig {
	sms := make([]backend.ModelMapping, len(c.SupportedModels))
	for i := range c.SupportedModels {
		sms[i] = backend.ModelMapping{
			RequestedModel:     c.SupportedModels[i].RequestedModel,
			ActualModel:        c.SupportedModels[i].ActualModel,
			CompatibilityScore: c.SupportedModels[i].CompatibilityScore,
			IsExact:            c.SupportedModels[i].IsExact,
		}
	}
	createdAt := c.CreatedAt
	if createdAt == "" {
		createdAt = now
	}
	return &backend.BackendConfig{
		ID:              c.ID,
		Name:            c.Name,
		Type:            c.Type,
		BaseURL:         c.BaseURL,
		APIKey:          c.APIKey,
		Enabled:         c.Enabled,
		Timeout:         c.Timeout,
		MaxRetries:      c.MaxRetries,
		Description:     c.Description,
		ProbeModel:      c.ProbeModel,
		Metadata:        c.Metadata,
		SupportedModels: sms,
		Capabilities: backend.ModelCapabilities{
			MaxContextTokens: c.Capabilities.MaxContextTokens,
			Features:         c.Capabilities.Features,
			SupportsImages:   c.Capabilities.SupportsImages,
			SupportsTools:    c.Capabilities.SupportsTools,
		},
		AutoFetchModels: c.AutoFetchModels,
		CreatedAt:       createdAt,
		UpdatedAt:       now,
		Weight:          c.Weight,
		Priority:        c.Priority,
	}
}

// parseConfigArchive extracts initdata members from a zip archive.
// Returns initial-backends content (or nil), pipeline template contents and
// system-config content (or nil).
func parseConfigArchive(data []byte) ([]byte, [][]byte, []byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid zip archive: %w", err)
	}
	var backends []byte
	var systemConfig []byte
	var templates [][]byte
	readMember := func(zf *zip.File) ([]byte, error) {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, configImportMaxBytes))
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", zf.Name, err)
		}
		return content, nil
	}
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.TrimPrefix(filepath.ToSlash(zf.Name), "./"))
		switch {
		case name == "initial-backends.yaml" || name == "initial-backends.yml" || name == "initial-backends.json":
			content, err := readMember(zf)
			if err != nil {
				return nil, nil, nil, err
			}
			backends = content
		case name == "system-config.yaml" || name == "system-config.yml":
			content, err := readMember(zf)
			if err != nil {
				return nil, nil, nil, err
			}
			systemConfig = content
		case strings.HasPrefix(name, "pipeline-templates/") &&
			(strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")):
			content, err := readMember(zf)
			if err != nil {
				return nil, nil, nil, err
			}
			templates = append(templates, content)
		}
	}
	return backends, templates, systemConfig, nil
}
