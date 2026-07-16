package server

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"centag/core/pkg/logger"
)

type autoBuildRebuildConfig struct {
	Enabled       bool
	Interval      time.Duration
	PipelineID    string
	Strategy      string
	ProbeBackends bool
	Canary        bool
	MaxUpdates    int
}

func (h *PipelineHandler) StartAutoBuildRebuildLoopFromEnv() {
	cfg := loadAutoBuildRebuildConfigFromEnv()
	if !cfg.Enabled {
		return
	}
	h.StartAutoBuildRebuildLoop(cfg)
}

func (h *PipelineHandler) StartAutoBuildRebuildLoop(cfg autoBuildRebuildConfig) {
	if h == nil || !cfg.Enabled || cfg.Interval <= 0 {
		return
	}
	if h.autoBuildScheduler == nil {
		logger.Warnf("auto-build periodic rebuild disabled: scheduler not initialized")
		return
	}
	if strings.TrimSpace(cfg.PipelineID) == "" {
		cfg.PipelineID = "router-mode"
	}
	if normalizeAutoBuildStrategy(cfg.Strategy) == "" {
		cfg.Strategy = "balance"
	}
	h.autoBuildMu.Lock()
	if h.autoBuildStop != nil {
		h.autoBuildStop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.autoBuildStop = cancel
	h.autoBuildMu.Unlock()

	logger.Infof("auto-build periodic rebuild enabled: pipeline=%s strategy=%s interval=%s canary=%v max_updates=%d probe_backends=%v",
		cfg.PipelineID, cfg.Strategy, cfg.Interval.String(), cfg.Canary, cfg.MaxUpdates, cfg.ProbeBackends)

	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.runAutoBuildRebuildOnce(ctx, cfg)
			}
		}
	}()
}

func (h *PipelineHandler) runAutoBuildRebuildOnce(ctx context.Context, cfg autoBuildRebuildConfig) {
	if h == nil || h.pipelineRegistry == nil || h.autoBuildScheduler == nil {
		return
	}
	if cfg.ProbeBackends && h.autoBuildBackendMgr != nil {
		probeCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		_, err := h.autoBuildBackendMgr.ProbeAllBackends(probeCtx, true)
		cancel()
		if err != nil {
			logger.Warnf("auto-build periodic rebuild probe failed: %v", err)
			return
		}
		if err := h.autoBuildBackendMgr.Save(); err != nil {
			logger.Warnf("auto-build periodic rebuild save probe failed: %v", err)
			return
		}
	}

	current := h.pipelineRegistry.Get(strings.TrimSpace(cfg.PipelineID))
	if current == nil {
		logger.Warnf("auto-build periodic rebuild skipped: pipeline %s not found", cfg.PipelineID)
		return
	}
	cloned, err := clonePipeline(current)
	if err != nil {
		logger.Warnf("auto-build periodic rebuild clone failed: %v", err)
		return
	}

	maxUpdates := normalizeAutoBuildMaxUpdates(cfg.Canary, cfg.MaxUpdates)
	updates, warnings, err := h.buildAutoRoutePlan(cloned, normalizeAutoBuildStrategy(cfg.Strategy), nil, maxUpdates)
	if err != nil {
		logger.Warnf("auto-build periodic rebuild build failed: %v", err)
		return
	}
	if len(updates) == 0 {
		return
	}

	h.pushAutoBuildHistory(cfg.PipelineID, current, cfg.Strategy, len(updates))
	for i := range cloned.Nodes {
		cloned.Nodes[i].Normalize()
	}
	if err := h.pipelineRegistry.Register(cloned); err != nil {
		logger.Warnf("auto-build periodic rebuild apply failed: %v", err)
		return
	}
	h.syncModesFromRegistry()
	logger.Infof("auto-build periodic rebuild applied: pipeline=%s updates=%d warnings=%d", cfg.PipelineID, len(updates), len(warnings))
}

func loadAutoBuildRebuildConfigFromEnv() autoBuildRebuildConfig {
	interval := parseDurationEnv("LLM_PROXY_ROUTER_AUTOBUILD_REBUILD_INTERVAL", 0)
	if interval <= 0 {
		return autoBuildRebuildConfig{Enabled: false}
	}
	cfg := autoBuildRebuildConfig{
		Enabled:       true,
		Interval:      interval,
		PipelineID:    strings.TrimSpace(getEnvOrDefault("LLM_PROXY_ROUTER_AUTOBUILD_PIPELINE_ID", "router-mode")),
		Strategy:      normalizeAutoBuildStrategy(getEnvOrDefault("LLM_PROXY_ROUTER_AUTOBUILD_STRATEGY", "balance")),
		ProbeBackends: parseBoolEnv("LLM_PROXY_ROUTER_AUTOBUILD_PROBE_BACKENDS", false),
		Canary:        parseBoolEnv("LLM_PROXY_ROUTER_AUTOBUILD_CANARY", false),
		MaxUpdates:    parseIntEnv("LLM_PROXY_ROUTER_AUTOBUILD_MAX_UPDATES", 0),
	}
	if cfg.Strategy == "" {
		cfg.Strategy = "balance"
	}
	return cfg
}

func parseDurationEnv(key string, defaultValue time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return defaultValue
	}
	return d
}

func parseBoolEnv(key string, defaultValue bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return defaultValue
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultValue
	}
}

func parseIntEnv(key string, defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return v
}

