package pipeline

import (
	"context"
	"strings"
	"sync"
)

// IntentResolver resolves a light-weight intent/category for router fallback.
// Implementations must not create import cycles with scheduler; inject from server.
type IntentResolver interface {
	ResolveCategory(ctx context.Context, content string, categories []string) (category string, confidence float64, err error)
}

var (
	intentResolverMu sync.RWMutex
	intentResolver   IntentResolver = CategoryKeywordResolver{}
)

// SetIntentResolver sets the process-wide intent resolver used by keyword_then_intent.
func SetIntentResolver(r IntentResolver) {
	intentResolverMu.Lock()
	defer intentResolverMu.Unlock()
	if r == nil {
		intentResolver = CategoryKeywordResolver{}
		return
	}
	intentResolver = r
}

// GetIntentResolver returns the current resolver.
func GetIntentResolver() IntentResolver {
	intentResolverMu.RLock()
	defer intentResolverMu.RUnlock()
	return intentResolver
}

// CategoryKeywordResolver matches route category keys as substrings of the user content.
type CategoryKeywordResolver struct{}

// ResolveCategory returns the longest matching category key (case-insensitive contains).
func (CategoryKeywordResolver) ResolveCategory(ctx context.Context, content string, categories []string) (string, float64, error) {
	_ = ctx
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" || len(categories) == 0 {
		return "", 0, nil
	}
	best := ""
	for _, cat := range categories {
		c := strings.ToLower(strings.TrimSpace(cat))
		if c == "" {
			continue
		}
		if strings.Contains(text, c) && len(c) > len(best) {
			best = strings.TrimSpace(cat)
		}
	}
	if best == "" {
		return "", 0, nil
	}
	// heuristic confidence: longer keyword relative to content
	conf := float64(len(best)) / float64(len(text))
	if conf > 1 {
		conf = 1
	}
	if conf < 0.35 {
		conf = 0.55
	}
	return best, conf, nil
}
