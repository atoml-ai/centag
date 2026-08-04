package ota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckLatest_MatchesAsset(t *testing.T) {
	sum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	asset := "update-package-centag-team-0.2.9-linux-amd64.tar.gz"

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/repos/atoml-ai/centag/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.2.9",
			"body": "notes",
			"assets": [
				{"name": "` + asset + `", "size": 12, "browser_download_url": "` + srv.URL + `/pkg"},
				{"name": "checksums.txt", "size": 80, "browser_download_url": "` + srv.URL + `/checksums.txt"}
			]
		}`))
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sum + "  " + asset + "\n"))
	})
	mux.HandleFunc("/pkg", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pkg"))
	})

	c := &Client{
		APIBase:    srv.URL,
		Repo:       "atoml-ai/centag",
		Edition:    "team",
		GOOS:       "linux",
		GOARCH:     "amd64",
		HTTPClient: srv.Client(),
	}
	res, err := c.CheckLatest(context.Background(), "0.2.8")
	if err != nil {
		t.Fatal(err)
	}
	if !res.UpdateAvailable {
		t.Fatalf("expected update available: %+v", res)
	}
	if res.AssetName != asset {
		t.Fatalf("asset=%q", res.AssetName)
	}
	if res.SHA256 != sum {
		t.Fatalf("sha=%q want %q", res.SHA256, sum)
	}
}

func TestCheckLatest_NoMatchingAsset(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"tag_name": "v0.2.9",
			"assets": [
				{"name": "centag-cli-personal-linux-amd64.tar.gz", "size": 1, "browser_download_url": "http://x"}
			]
		}`))
	}))
	defer api.Close()

	c := &Client{
		APIBase: api.URL, Repo: "atoml-ai/centag", Edition: "team",
		GOOS: "linux", GOARCH: "amd64", HTTPClient: api.Client(),
	}
	res, err := c.CheckLatest(context.Background(), "0.2.8")
	if err != nil {
		t.Fatal(err)
	}
	if res.UpdateAvailable {
		t.Fatalf("should not be available without team asset: %+v", res)
	}
}

func TestDownloadToFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello-ota"))
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client()}
	dir := t.TempDir()
	dest := filepath.Join(dir, "pkg.bin")
	if err := c.DownloadToFile(context.Background(), srv.URL, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello-ota" {
		t.Fatalf("got %q", data)
	}
}
