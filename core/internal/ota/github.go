package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	defaultAPIBase = "https://api.github.com"
	defaultRepo    = "atoml-ai/centag"
	userAgent     = "centag-ota"
)

// Client checks and downloads update packages from a public GitHub Release.
type Client struct {
	APIBase    string
	Repo       string
	Edition    string
	GOOS       string
	GOARCH     string
	HTTPClient *http.Client
}

// CheckResult is the outcome of a remote version check.
type CheckResult struct {
	CurrentVersion string `json:"current_version"`
	RemoteVersion  string `json:"remote_version"`
	TagName        string `json:"tag_name"`
	UpdateAvailable bool  `json:"update_available"`
	AssetName      string `json:"asset_name,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Size           int64  `json:"size,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	Message        string `json:"message,omitempty"`
}

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// NewClientFromEnv builds a Client using env overrides when set.
func NewClientFromEnv(edition string) *Client {
	api := strings.TrimSpace(os.Getenv("CENTAG_UPDATE_GITHUB_API"))
	if api == "" {
		api = defaultAPIBase
	}
	repo := strings.TrimSpace(os.Getenv("CENTAG_UPDATE_GITHUB_REPO"))
	if repo == "" {
		repo = defaultRepo
	}
	ed := strings.TrimSpace(strings.ToLower(edition))
	if ed == "" {
		ed = strings.TrimSpace(strings.ToLower(os.Getenv("CENTAG_EDITION")))
	}
	if ed == "" {
		ed = "team"
	}
	return &Client{
		APIBase: strings.TrimRight(api, "/"),
		Repo:    repo,
		Edition: ed,
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) http() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *Client) apiBase() string {
	if c != nil && c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return defaultAPIBase
}

func (c *Client) repo() string {
	if c != nil && c.Repo != "" {
		return c.Repo
	}
	return defaultRepo
}

func (c *Client) edition() string {
	if c != nil && c.Edition != "" {
		return c.Edition
	}
	return "team"
}

func (c *Client) goos() string {
	if c != nil && c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *Client) goarch() string {
	if c != nil && c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

// CheckLatest fetches the latest non-draft release and matches the OTA asset for this edition/platform.
func (c *Client) CheckLatest(ctx context.Context, currentVersion string) (*CheckResult, error) {
	rel, err := c.fetchLatest(ctx)
	if err != nil {
		return nil, err
	}
	remoteVer := NormalizeVersion(rel.TagName)
	wantPrefix := fmt.Sprintf("update-package-centag-%s-", c.edition())
	wantSuffix := fmt.Sprintf("-%s-%s.tar.gz", c.goos(), c.goarch())

	var matched *ghAsset
	var checksumsURL string
	for i := range rel.Assets {
		a := &rel.Assets[i]
		name := a.Name
		if name == "checksums.txt" {
			checksumsURL = a.BrowserDownloadURL
			continue
		}
		if strings.HasPrefix(name, wantPrefix) && strings.HasSuffix(name, wantSuffix) {
			matched = a
		}
	}

	result := &CheckResult{
		CurrentVersion:  FormatTag(currentVersion),
		RemoteVersion:   FormatTag(remoteVer),
		TagName:         rel.TagName,
		UpdateAvailable: IsUpgrade(currentVersion, remoteVer),
		ReleaseNotes:    strings.TrimSpace(rel.Body),
	}
	if matched == nil {
		result.UpdateAvailable = false
		result.Message = fmt.Sprintf("release %s has no %s OTA asset for %s/%s",
			rel.TagName, c.edition(), c.goos(), c.goarch())
		return result, nil
	}

	result.AssetName = matched.Name
	result.DownloadURL = matched.BrowserDownloadURL
	result.Size = matched.Size

	// Prefer side-car .sha256 asset; fall back to checksums.txt.
	var sha string
	for i := range rel.Assets {
		if rel.Assets[i].Name == matched.Name+".sha256" {
			sha, _ = c.fetchChecksum(ctx, rel.Assets[i].BrowserDownloadURL, matched.Name)
			break
		}
	}
	if sha == "" && checksumsURL != "" {
		sha, _ = c.fetchChecksum(ctx, checksumsURL, matched.Name)
	}
	result.SHA256 = sha

	if !result.UpdateAvailable {
		result.Message = "already up to date"
	}
	return result, nil
}

func (c *Client) fetchLatest(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase(), c.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("github release request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github release API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("decode github release: %w", err)
	}
	if rel.Draft {
		return nil, fmt.Errorf("latest release is a draft")
	}
	return &rel, nil
}

func (c *Client) fetchChecksum(ctx context.Context, url, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return ParseChecksum(string(data), assetName), nil
}

// ParseChecksum extracts the hex digest for assetName from sha256sum / checksums.txt content.
func ParseChecksum(content, assetName string) string {
	base := assetName
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Formats: "<sha>  <file>" or "<sha> *<file>"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if name == base || strings.HasSuffix(name, "/"+base) {
			if looksLikeSHA256(sum) {
				return sum
			}
		}
		// Single-file .sha256 may be just the digest.
		if len(fields) == 1 && looksLikeSHA256(sum) {
			return sum
		}
	}
	// Bare digest file
	trimmed := strings.TrimSpace(content)
	if looksLikeSHA256(trimmed) {
		return strings.ToLower(trimmed)
	}
	first := strings.Fields(trimmed)
	if len(first) >= 1 && looksLikeSHA256(first[0]) {
		return strings.ToLower(first[0])
	}
	return ""
}

func looksLikeSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// DownloadToFile downloads url into destPath.
func (c *Client) DownloadToFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	const maxBytes = 500 * 1024 * 1024
	written, err := io.Copy(f, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if written > maxBytes {
		return fmt.Errorf("download exceeds 500MB limit")
	}
	return nil
}
