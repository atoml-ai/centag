package server

import (
	"strings"
	"time"
)

// cacheListFilters holds multi-dimensional query params for cache management list/detail.
type cacheListFilters struct {
	SessionID string
	Model     string
	Query     string // keyword against request/response/key/request_id
	From      time.Time
	To        time.Time
	HasFrom   bool
	HasTo     bool
}

func parseRFC3339Loose(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func entryMetaString(entry map[string]interface{}, key string) string {
	if entry == nil {
		return ""
	}
	if v, ok := entry[key].(string); ok && v != "" {
		return v
	}
	if meta, ok := entry["metadata"].(map[string]interface{}); ok && meta != nil {
		if v, ok := meta[key].(string); ok {
			return v
		}
	}
	return ""
}

func entryTimestamp(entry map[string]interface{}) (time.Time, bool) {
	if entry == nil {
		return time.Time{}, false
	}
	if t, ok := entry["timestamp"].(time.Time); ok {
		return t, true
	}
	if s, ok := entry["timestamp"].(string); ok {
		return parseRFC3339Loose(s)
	}
	if meta, ok := entry["metadata"].(map[string]interface{}); ok {
		if s, ok := meta["created_at"].(string); ok {
			return parseRFC3339Loose(s)
		}
	}
	return time.Time{}, false
}

func matchCacheListFilters(entry map[string]interface{}, f cacheListFilters) bool {
	if f.SessionID != "" {
		if !strings.EqualFold(entryMetaString(entry, "session_id"), f.SessionID) {
			return false
		}
	}
	if f.Model != "" {
		model := entryMetaString(entry, "model")
		if model == "" {
			return false
		}
		if !strings.Contains(strings.ToLower(model), strings.ToLower(f.Model)) {
			return false
		}
	}
	if f.HasFrom || f.HasTo {
		ts, ok := entryTimestamp(entry)
		if !ok {
			return false
		}
		if f.HasFrom && ts.Before(f.From) {
			return false
		}
		if f.HasTo && ts.After(f.To) {
			return false
		}
	}
	if f.Query != "" {
		q := strings.ToLower(f.Query)
		hay := strings.ToLower(strings.Join([]string{
			entryMetaString(entry, "request_id"),
			entryMetaString(entry, "session_id"),
			entryMetaString(entry, "model"),
			stringifyEntryField(entry, "key"),
			stringifyEntryField(entry, "request"),
			stringifyEntryField(entry, "response"),
			entryMetaString(entry, "request_text"),
		}, " "))
		if !strings.Contains(hay, q) {
			return false
		}
	}
	return true
}

func stringifyEntryField(entry map[string]interface{}, key string) string {
	if entry == nil {
		return ""
	}
	switch v := entry[key].(type) {
	case string:
		return v
	default:
		return ""
	}
}

// flattenCacheEntryForAPI normalizes fields used by the unified cache console.
func flattenCacheEntryForAPI(entry map[string]interface{}) map[string]interface{} {
	if entry == nil {
		return nil
	}
	if sid := entryMetaString(entry, "session_id"); sid != "" {
		entry["session_id"] = sid
	}
	if model := entryMetaString(entry, "model"); model != "" {
		entry["model"] = model
	}
	if rid := entryMetaString(entry, "request_id"); rid != "" {
		entry["request_id"] = rid
	}
	if ct := entryMetaString(entry, "cache_type"); ct != "" {
		if _, ok := entry["cache_type"]; !ok {
			entry["cache_type"] = ct
		}
	}
	return entry
}
