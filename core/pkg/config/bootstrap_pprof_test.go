package config

import "testing"

func TestLoadBootstrap_PprofEnabledFromEnv(t *testing.T) {
	t.Run("CENTAG_PPROF", func(t *testing.T) {
		t.Setenv("CENTAG_PPROF", "true")
		t.Setenv("LLM_PROXY_PPROF_ENABLED", "")
		cfg := LoadBootstrap()
		if !cfg.Server.PprofEnabled {
			t.Fatal("expected PprofEnabled=true when CENTAG_PPROF=true")
		}
	})

	t.Run("LLM_PROXY_PPROF_ENABLED", func(t *testing.T) {
		t.Setenv("CENTAG_PPROF", "")
		t.Setenv("LLM_PROXY_PPROF_ENABLED", "1")
		cfg := LoadBootstrap()
		if !cfg.Server.PprofEnabled {
			t.Fatal("expected PprofEnabled=true when LLM_PROXY_PPROF_ENABLED=1")
		}
	})

	t.Run("default_off", func(t *testing.T) {
		t.Setenv("CENTAG_PPROF", "")
		t.Setenv("LLM_PROXY_PPROF_ENABLED", "")
		cfg := LoadBootstrap()
		if cfg.Server.PprofEnabled {
			t.Fatal("expected PprofEnabled=false by default")
		}
	})
}
