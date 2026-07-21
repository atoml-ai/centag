package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SetupStatus mirrors GET /api/v1/proxy/setup/status.
type SetupStatus struct {
	Mode               string `json:"mode"`
	MITMEnabled        bool   `json:"mitm_enabled"`
	ListenAddr         string `json:"listen_addr"`
	ListenIsLoopback   bool   `json:"listen_is_loopback"`
	AllowLANClients    bool   `json:"allow_lan_clients"`
	AdvertiseHost      string `json:"advertise_host"`
	PACEnabled         bool   `json:"pac_enabled"`
	PACURL             string `json:"pac_url"`
	CADownloadURL      string `json:"ca_download_url"`
	CAFingerprintSHA256 string `json:"ca_fingerprint_sha256"`
	MITMProxy          string `json:"mitm_proxy"`
}

type Client struct {
	Base   string
	Token  string
	HTTP   *http.Client
}

func New(apiBase string) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return nil, fmt.Errorf("empty api base")
	}
	if _, err := url.Parse(base); err != nil {
		return nil, err
	}
	return &Client{
		Base: base,
		HTTP: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *Client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.HTTP.Do(req)
}

func (c *Client) SetupStatus() (*SetupStatus, error) {
	resp, err := c.get("/api/v1/proxy/setup/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("setup/status requires auth (HTTP %d); set CENTAG_WRAP_TOKEN", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setup/status HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var st SetupStatus
	if err := json.Unmarshal(body, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Client) DownloadCA() ([]byte, error) {
	resp, err := c.get("/api/v1/proxy/ca.crt")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ca.crt HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	return body, nil
}

func (c *Client) FetchPAC() (string, error) {
	resp, err := c.get("/api/v1/proxy/pac")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pac HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	return string(body), nil
}

// ValidateRemoteReady ensures team server is configured for LAN clients.
func ValidateRemoteReady(st *SetupStatus, pacBody string) error {
	if st == nil {
		return fmt.Errorf("nil setup status")
	}
	if !st.AllowLANClients {
		return fmt.Errorf("server allow_lan_clients=false; admin must enable LAN egress first")
	}
	if st.AdvertiseHost == "" || strings.Contains(st.AdvertiseHost, "127.0.0.1") {
		return fmt.Errorf("server advertise_host invalid: %q", st.AdvertiseHost)
	}
	if strings.Contains(pacBody, "PROXY 127.0.0.1:") {
		return fmt.Errorf("PAC still points to 127.0.0.1; server PAC advertise misconfigured")
	}
	if st.PACURL == "" {
		return fmt.Errorf("empty pac_url from server")
	}
	return nil
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
