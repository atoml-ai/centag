package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: %v\n", err)
		os.Exit(2)
	}

	dataDir, err := resolveDataDir(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: data dir: %v\n", err)
		os.Exit(1)
	}
	cfg.DataDir = dataDir

	binary, err := resolveSidecarBinary(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "centag-launcher: starting %s (%s) on :%d\ndata dir: %s\nsupervise: %v\n",
		binary, cfg.Edition, cfg.Port, dataDir, cfg.Supervise)

	var sidecar *sidecarProcess
	if cfg.NoSidecar {
		fmt.Fprintf(os.Stderr, "centag-launcher: --no-sidecar mode, connecting to existing sidecar on :%d\n", cfg.Port)
		sidecar = &sidecarProcess{port: cfg.Port}
	} else {
		var err error
		sidecar, err = startSidecar(ctx, cfg, binary, dataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: bootstrap failed: %v\n", err)
			os.Exit(1)
		}
	}

	hub := newSidecarHub(cfg, binary, dataDir, sidecar)
	app := &launcherApp{cfg: cfg, hub: hub}
	defer app.shutdown()

	// Auto-trust the sidecar CA on first launch so wrapped apps' HTTPS works
	// out of the box (macOS: one admin dialog; Windows/Linux: silent).
	go ensureCATrusted(cfg)

	if cfg.Supervise {
		go hub.watch(ctx)
	}

	if !cfg.NoOpen {
		if err := openBrowser(cfg.baseURL()); err != nil {
			fmt.Fprintf(os.Stderr, "centag-launcher: open browser: %v\n", err)
		}
	}

	go func() {
		<-ctx.Done()
		app.shutdown()
		quitMenu(!cfg.Headless)
	}()

	if err := app.run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "centag-launcher: %v\n", err)
		os.Exit(1)
	}
}
