package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	healthTimeout  = 45 * time.Second
	healthInterval = 300 * time.Millisecond
)

type sidecarProcess struct {
	cmd      *exec.Cmd
	port     int
	logFile  *os.File
	waitOnce sync.Once
	waitErr  error
}

func (p *sidecarProcess) baseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", p.port)
}

func startSidecar(ctx context.Context, cfg Config, binary, dataDir string) (*sidecarProcess, error) {
	if err := ensureDirs(dataDir); err != nil {
		return nil, err
	}
	if err := terminateListenerOnPort(cfg.Port); err != nil {
		return nil, fmt.Errorf("clear port %d: %w", cfg.Port, err)
	}

	logPath := filepath.Join(dataDir, "logs", "centag-sidecar.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open sidecar log: %w", err)
	}

	workDir := filepath.Dir(binary)
	// Explicit `serve` subcommand: bare `centag` now prints help (CLI convention).
	cmd := exec.CommandContext(ctx, binary, "serve")
	// Run with dataDir as CWD so relative ./data paths stay under the user data dir.
	cmd.Dir = dataDir
	// Default: logs only to sidecar file. Debug/dev (parent exported LOG_OUTPUT=both|stdout):
	// also tee to the launcher console so `./start.sh debug … --desktop` stays useful.
	outW := io.Writer(logFile)
	errW := io.Writer(logFile)
	if sidecarConsoleTeeEnabled() {
		outW = io.MultiWriter(logFile, os.Stdout)
		errW = io.MultiWriter(logFile, os.Stderr)
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	cmd.Env = buildSidecarEnv(cfg, binary, workDir, dataDir)
	hideSidecarWindow(cmd)

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("start sidecar: %w", err)
	}

	proc := &sidecarProcess{cmd: cmd, port: cfg.Port, logFile: logFile}
	if err := waitHealthy(ctx, proc.baseURL()); err != nil {
		_ = proc.stop()
		return nil, err
	}
	return proc, nil
}

func sidecarConsoleTeeEnabled() bool {
	out := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_PROXY_LOG_OUTPUT")))
	if out == "both" || out == "stdout" || out == "console" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("LLM_PROXY_SERVER_MODE")), "debug")
}

func buildSidecarEnv(cfg Config, binary, workDir, dataDir string) []string {
	env := append(os.Environ(),
		"LLM_PROXY_SERVER_HOST=127.0.0.1",
		fmt.Sprintf("LLM_PROXY_SERVER_PORT=%d", cfg.Port),
		fmt.Sprintf("CENTAG_EDITION=%s", cfg.Edition),
		fmt.Sprintf("LLM_PROXY_LOG_PATH=%s", filepath.Join(dataDir, "logs")),
	)
	// Preserve parent overrides (e.g. start.sh debug → console/both); default to file/json.
	if strings.TrimSpace(os.Getenv("LLM_PROXY_LOG_OUTPUT")) == "" {
		env = append(env, "LLM_PROXY_LOG_OUTPUT=file")
	}
	if strings.TrimSpace(os.Getenv("LLM_PROXY_LOG_FORMAT")) == "" {
		env = append(env, "LLM_PROXY_LOG_FORMAT=json")
	}

	staticPath := firstExistingDir(
		filepath.Join(workDir, "static"),
		filepath.Join(filepath.Dir(binary), "static"),
	)
	if staticPath != "" {
		env = append(env, "STATIC_PATH="+staticPath)
	}

	initdata := firstExistingDir(
		filepath.Join(workDir, "config", "initdata"),
		filepath.Join(filepath.Dir(binary), "config", "initdata"),
		filepath.Join(workDir, "..", "..", "config", "initdata"),
	)
	if cfg.Edition == EditionMinimal {
		minimalInit := firstExistingDir(
			filepath.Join(workDir, "config", "profiles", "minimal", "initdata"),
			filepath.Join(filepath.Dir(binary), "..", "..", "config", "profiles", "minimal", "initdata"),
		)
		if minimalInit != "" {
			initdata = minimalInit
		}
	}
	if initdata != "" {
		env = append(env, "INITDATA_PATH="+initdata)
	}

	if cfg.Edition == EditionPersonal {
		env = append(env,
			"LLM_PROXY_DB_DRIVER=sqlite",
			fmt.Sprintf("SQLITE_PATH=%s", filepath.Join(dataDir, "storage", "centag.db")),
		)
	}
	return env
}

func waitHealthy(ctx context.Context, baseURL string) error {
	deadline := time.Now().Add(healthTimeout)
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("health status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return fmt.Errorf("sidecar not healthy within %s: %w", healthTimeout, lastErr)
}

func (p *sidecarProcess) wait() error {
	if p == nil || p.cmd == nil {
		return nil
	}
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
	})
	return p.waitErr
}

func (p *sidecarProcess) stop() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	stopProcess(p.cmd.Process)
	done := make(chan error, 1)
	go func() { done <- p.wait() }()
	select {
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	case <-done:
	}
	if p.logFile != nil {
		_ = p.logFile.Close()
		p.logFile = nil
	}
	return nil
}

func terminateListenerOnPort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		_ = ln.Close()
		return nil
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	for _, line := range strings.Fields(string(out)) {
		_ = exec.Command("kill", "-TERM", line).Run()
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func firstExistingDir(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
	}
	return ""
}
