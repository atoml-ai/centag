package configsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SnapshotProvider is a Provider backed by a public HTTP snapshot (no auth).
// Used by Personal edition where no Feishu credentials are available.
type SnapshotProvider struct {
	urls  []string
	httpc *http.Client
}

// NewSnapshotProvider creates a snapshot provider with one or more URLs.
// URLs are tried in order; first successful response wins.
func NewSnapshotProvider(urls []string) *SnapshotProvider {
	return &SnapshotProvider{
		urls:  urls,
		httpc: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *SnapshotProvider) FetchConfig(ctx context.Context, q Query) ([]Row, error) {
	snap, err := p.fetchSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snap.Config, nil
}

func (p *SnapshotProvider) FetchModelPrices(ctx context.Context) ([]ProviderPrice, error) {
	snap, err := p.fetchSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snap.Prices, nil
}

func (p *SnapshotProvider) fetchSnapshot(ctx context.Context) (*Snapshot, error) {
	var lastErr error
	for _, u := range p.urls {
		snap, err := p.fetchOne(ctx, u)
		if err != nil {
			lastErr = err
			continue
		}
		return snap, nil
	}
	if lastErr != nil {
		return nil, fmt.Errorf("snapshot: all sources failed: %w", lastErr)
	}
	return nil, fmt.Errorf("snapshot: no URLs configured")
}

// maxSnapshotBody caps snapshot response reads. A var so tests can shrink it.
var maxSnapshotBody int64 = 10 << 20

// DefaultSnapshotURLs are the well-known public snapshot endpoints tried in
// order when no explicit URL is configured.
var DefaultSnapshotURLs = []string{}

// ResolveSnapshotURLs returns the snapshot URLs to use: the env override
// CENTAG_CONFIGSYNC_SNAPSHOT_URL (space-separated for multi-source) wins,
// otherwise the build-time defaults are used (TC-SNP-006).
func ResolveSnapshotURLs(defaults []string) []string {
	if v := trimSpaces(os.Getenv("CENTAG_CONFIGSYNC_SNAPSHOT_URL")); v != "" {
		return strings.Fields(v)
	}
	return defaults
}

func trimSpaces(s string) string { return strings.TrimSpace(s) }

func (p *SnapshotProvider) fetchOne(ctx context.Context, url string) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snapshot fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("snapshot fetch %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBody))
	if err != nil {
		return nil, fmt.Errorf("snapshot read %s: %w", url, err)
	}
	// An over-cap response was truncated — reject rather than mis-parse
	// (TC-SNP-004 oversize protection).
	if int64(len(body)) >= maxSnapshotBody {
		return nil, fmt.Errorf("snapshot %s exceeds size cap (%d bytes)", url, maxSnapshotBody)
	}
	var snap Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		return nil, fmt.Errorf("snapshot decode %s: %w", url, err)
	}
	if snap.Schema != snapshotSchema {
		return nil, fmt.Errorf("snapshot schema %d (expected %d)", snap.Schema, snapshotSchema)
	}
	return &snap, nil
}
