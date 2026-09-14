package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func wrapHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// prepareWrapApp calls POST /api/v1/wrap/apps/:id/prepare on the local sidecar.
// The sidecar resolves the Centag model name and (optionally) writes the app's
// local model config; the launcher never builds argv itself.
func prepareWrapApp(baseURL, id string, writeConfig bool) (map[string]any, error) {
	payload, _ := json.Marshal(map[string]any{"write_config": writeConfig})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/wrap/apps/"+id+"/prepare", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := wrapHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prepare %s HTTP %d: %s", id, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// wrapDoctor calls GET /api/v1/wrap/doctor on the local sidecar.
func wrapDoctor(baseURL string) (map[string]any, error) {
	resp, err := wrapHTTPClient().Get(strings.TrimRight(baseURL, "/") + "/api/v1/wrap/doctor")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctor HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// warningsText joins the prepare response `warnings` array into a short string.
func warningsText(res map[string]any) string {
	ws, _ := res["warnings"].([]any)
	parts := make([]string, 0, len(ws))
	for _, w := range ws {
		if s, ok := w.(string); ok && strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "；")
}
