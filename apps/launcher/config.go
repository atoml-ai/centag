package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultPort = 20060

// Edition selects which prebuilt Centag binary/env profile the launcher starts.
// This launcher never imports Centag core — it only execs a sidecar.
type Edition string

const (
	EditionMinimal  Edition = "minimal"
	EditionPersonal Edition = "personal"
)

type Config struct {
	Edition   Edition
	BinPath   string
	Port      int
	DataDir   string
	NoOpen    bool
	NoSidecar bool   // connect to an already-running sidecar instead of starting one (debug)
	Headless  bool   // no system menu / systray (CI)
	Supervise bool   // restart sidecar on crash (default on except debug)
}

func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("centag-launcher", flag.ContinueOnError)
	edition := fs.String("edition", envOr("CENTAG_LAUNCHER_EDITION", "personal"), "sidecar edition: minimal | personal")
	binPath := fs.String("bin", envOr("CENTAG_BIN", ""), "path to centag sidecar binary")
	port := fs.Int("port", envIntOr("LLM_PROXY_SERVER_PORT", defaultPort), "sidecar listen port")
	dataDir := fs.String("data-dir", envOr("CENTAG_LAUNCHER_DATA_DIR", ""), "data directory (default: ~/.centag/lib/<edition>)")
	noOpen := fs.Bool("no-open", false, "do not open the system browser on start")
	headless := fs.Bool("headless", envOr("CENTAG_LAUNCHER_HEADLESS", "") == "1", "run sidecar only (no system menu; useful for CI)")
	noSidecar := fs.Bool("no-sidecar", false, "connect to an already-running sidecar instead of starting one (debug)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	ed := Edition(strings.ToLower(strings.TrimSpace(*edition)))
	switch ed {
	case EditionMinimal, EditionPersonal:
	default:
		return Config{}, fmt.Errorf("unsupported edition %q (want minimal|personal)", *edition)
	}
	if *port <= 0 || *port > 65535 {
		return Config{}, fmt.Errorf("invalid port %d", *port)
	}

	return Config{
		Edition:   ed,
		BinPath:   strings.TrimSpace(*binPath),
		Port:      *port,
		DataDir:   strings.TrimSpace(*dataDir),
		NoOpen:    *noOpen,
		NoSidecar: *noSidecar,
		Headless:  *headless,
		Supervise: launcherSuperviseEnabled(),
	}, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func (c Config) baseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", c.Port)
}
