package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const version = 1

// Snapshot records OS proxy + CA state before enable.
type Snapshot struct {
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	OS         string    `json:"os"`
	ClientMode string    `json:"client_mode"` // local|remote
	Proxy      ProxyState `json:"proxy"`
	CA         CAState   `json:"ca"`
	Centag     CentagRef `json:"centag"`
}

type ProxyState struct {
	Mode       string `json:"mode"` // off|manual|pac
	PACURL     string `json:"pac_url"`
	HTTP       string `json:"http"`
	HTTPS      string `json:"https"`
	Exceptions string `json:"exceptions"`
}

type CAState struct {
	FingerprintSHA256 string `json:"fingerprint_sha256"`
	InstalledByUs     bool   `json:"installed_by_us"`
}

type CentagRef struct {
	APIBase     string `json:"api_base"`
	MITMProxy   string `json:"mitm_proxy"`
	ServerLabel string `json:"server_label"`
}

func Dir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.TempDir()
		}
		return filepath.Join(base, "Centag"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".centag"), nil
	}
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "proxy-snapshot.json"), nil
}

func Save(s *Snapshot) error {
	if s == nil {
		return fmt.Errorf("nil snapshot")
	}
	s.Version = version
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.OS = runtime.GOOS
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func Load() (*Snapshot, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Exists() bool {
	path, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func Remove() error {
	path, err := Path()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
