package cache

import "time"

// EnrichCacheWriteMetadata ensures management-console fields are present on write (best-effort).
// Does not overwrite existing non-empty values.
func EnrichCacheWriteMetadata(metadata map[string]interface{}, cacheType string) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if cacheType != "" {
		if _, ok := metadata["cache_type"]; !ok {
			metadata["cache_type"] = cacheType
		}
	}
	if _, ok := metadata["created_at"]; !ok {
		metadata["created_at"] = time.Now().UTC().Format(time.RFC3339)
	}
	return metadata
}

// AttachRequestContextMetadata copies session/request identifiers into cache metadata.
func AttachRequestContextMetadata(metadata map[string]interface{}, sessionID, requestID, backendName string) map[string]interface{} {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if sessionID != "" {
		if v, _ := metadata["session_id"].(string); v == "" {
			metadata["session_id"] = sessionID
		}
	}
	if requestID != "" {
		if v, _ := metadata["request_id"].(string); v == "" {
			metadata["request_id"] = requestID
		}
	}
	if backendName != "" {
		if v, _ := metadata["backend_name"].(string); v == "" {
			metadata["backend_name"] = backendName
		}
	}
	return metadata
}
