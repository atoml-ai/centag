package cache

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestUnmarshalCacheEntry(t *testing.T) {
	// Test case 1: 直接 JSON
	entry := CacheEntry{Key: "test", Request: "req", Response: "resp"}
	data, _ := json.Marshal(entry)
	result, err := UnmarshalCacheEntry(data)
	if err != nil {
		t.Fatalf("Test 1 failed: %v", err)
	}
	if result.Key != "test" {
		t.Fatalf("Test 1 failed: key mismatch, got %s", result.Key)
	}
	t.Log("Test 1 passed: direct JSON")
	
	// Test case 2: base64 编码的 JSON（包装在 JSON 字符串中）
	data2, _ := json.Marshal(entry)
	str := base64.StdEncoding.EncodeToString(data2)
	data3, _ := json.Marshal(str)
	result2, err2 := UnmarshalCacheEntry(data3)
	if err2 != nil {
		t.Fatalf("Test 2 failed: %v", err2)
	}
	if result2.Key != "test" {
		t.Fatalf("Test 2 failed: key mismatch, got %s", result2.Key)
	}
	t.Log("Test 2 passed: base64 encoded JSON in JSON string")
	
	// Test case 3: 原始 base64 字符串（没有 JSON 包装）
	data4, _ := json.Marshal(entry)
	str2 := base64.StdEncoding.EncodeToString(data4)
	result3, err3 := UnmarshalCacheEntry([]byte(str2))
	if err3 != nil {
		t.Fatalf("Test 3 failed: %v", err3)
	}
	if result3.Key != "test" {
		t.Fatalf("Test 3 failed: key mismatch, got %s", result3.Key)
	}
	t.Log("Test 3 passed: raw base64 string")
}
