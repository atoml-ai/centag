package storage

import (
	"testing"
)

func TestStorageType_Constants(t *testing.T) {
	tests := []struct {
		name  string
		st    StorageType
		want  string
	}{
		{"Redis", StorageTypeRedis, "redis"},
		{"Milvus", StorageTypeMilvus, "milvus"},
		{"Chroma", StorageTypeChroma, "chroma"},
		{"Postgresql", StorageTypePostgresql, "postgresql"},
		{"Elasticsearch", StorageTypeElasticsearch, "elasticsearch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.st) != tt.want {
				t.Errorf("StorageType = %s, want %s", tt.st, tt.want)
			}
		})
	}
}

func TestStorageConfigItem_Fields(t *testing.T) {
	config := StorageConfigItem{
		Name:        "test-storage",
		Type:        StorageTypeRedis,
		Enabled:     true,
		Config:      map[string]interface{}{"host": "localhost"},
		Description: "Test Redis storage",
	}

	if config.Name != "test-storage" {
		t.Errorf("StorageConfigItem.Name = %s, want test-storage", config.Name)
	}
	if config.Type != StorageTypeRedis {
		t.Errorf("StorageConfigItem.Type = %s, want redis", config.Type)
	}
	if !config.Enabled {
		t.Error("StorageConfigItem.Enabled should be true")
	}
}

func TestStorageConfig_Fields(t *testing.T) {
	config := &StorageConfig{
		Storages: []StorageConfigItem{
			{Name: "redis", Type: StorageTypeRedis, Enabled: true},
			{Name: "milvus", Type: StorageTypeMilvus, Enabled: false},
		},
		DefaultKV: "redis",
	}

	if len(config.Storages) != 2 {
		t.Errorf("StorageConfig.Storages length = %d, want 2", len(config.Storages))
	}
	if config.DefaultKV != "redis" {
		t.Errorf("StorageConfig.DefaultKV = %s, want redis", config.DefaultKV)
	}
}

func TestStoreInfo_Fields(t *testing.T) {
	info := StoreInfo{
		Type: "redis",
	}

	if info.Type != "redis" {
		t.Errorf("StoreInfo.Type = %s, want redis", info.Type)
	}
}

func TestVectorEntry_Fields(t *testing.T) {
	entry := VectorEntry{
		ID: "vec1",
		Metadata: map[string]interface{}{
			"model": "gpt-4",
		},
	}

	if entry.ID != "vec1" {
		t.Errorf("VectorEntry.ID = %s, want vec1", entry.ID)
	}
	if entry.Metadata["model"] != "gpt-4" {
		t.Errorf("VectorEntry.Metadata[model] = %v, want gpt-4", entry.Metadata["model"])
	}
}

func TestVector_Fields(t *testing.T) {
	vector := Vector{
		ID:     "vec1",
		Vector: []float32{0.1, 0.2, 0.3},
		Metadata: map[string]interface{}{
			"key": "value",
		},
	}

	if vector.ID != "vec1" {
		t.Errorf("Vector.ID = %s, want vec1", vector.ID)
	}
	if len(vector.Vector) != 3 {
		t.Errorf("Vector.Vector length = %d, want 3", len(vector.Vector))
	}
}

func TestSearchResult_Fields(t *testing.T) {
	result := SearchResult{
		ID:       "result1",
		Vector:   []float32{0.1, 0.2},
		Score:    0.95,
		Metadata: map[string]interface{}{"source": "test"},
	}

	if result.ID != "result1" {
		t.Errorf("SearchResult.ID = %s, want result1", result.ID)
	}
	if result.Score != 0.95 {
		t.Errorf("SearchResult.Score = %f, want 0.95", result.Score)
	}
}

func TestCollectionInfo_Fields(t *testing.T) {
	info := CollectionInfo{
		Name:       "test_collection",
		Dimension:  1536,
		Count:      100,
		IndexType:  "HNSW",
		MetricType: "COSINE",
	}

	if info.Name != "test_collection" {
		t.Errorf("CollectionInfo.Name = %s, want test_collection", info.Name)
	}
	if info.Dimension != 1536 {
		t.Errorf("CollectionInfo.Dimension = %d, want 1536", info.Dimension)
	}
}

func TestMetricType_Constants(t *testing.T) {
	tests := []struct {
		name string
		mt   MetricType
		want string
	}{
		{"L2", MetricTypeL2, "L2"},
		{"IP", MetricTypeIP, "IP"},
		{"Cosine", MetricTypeCosine, "COSINE"},
		{"Hamming", MetricTypeHamming, "HAMMING"},
		{"Jaccard", MetricTypeJaccard, "JACCARD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mt) != tt.want {
				t.Errorf("MetricType = %s, want %s", tt.mt, tt.want)
			}
		})
	}
}

func TestIndexType_Constants(t *testing.T) {
	tests := []struct {
		name string
		it   IndexType
		want string
	}{
		{"Flat", IndexTypeFlat, "FLAT"},
		{"IVFFlat", IndexTypeIVFFlat, "IVFFLAT"},
		{"IVFPQ", IndexTypeIVFPQ, "IVFPQ"},
		{"HNSW", IndexTypeHNSW, "HNSW"},
		{"DISKANN", IndexTypeDISKANN, "DISKANN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.it) != tt.want {
				t.Errorf("IndexType = %s, want %s", tt.it, tt.want)
			}
		})
	}
}

func TestKVItem_Fields(t *testing.T) {
	item := KVItem{
		Key:   "test_key",
		Value: "test_value",
		TTL:   3600,
	}

	if item.Key != "test_key" {
		t.Errorf("KVItem.Key = %s, want test_key", item.Key)
	}
	if item.Value != "test_value" {
		t.Errorf("KVItem.Value = %v, want test_value", item.Value)
	}
	if item.TTL != 3600 {
		t.Errorf("KVItem.TTL = %d, want 3600", item.TTL)
	}
}

func TestStorageStatus_Fields(t *testing.T) {
	status := StorageStatus{
		Name:    "redis",
		Type:    StorageTypeRedis,
		Healthy: true,
		Error:   nil,
	}

	if status.Name != "redis" {
		t.Errorf("StorageStatus.Name = %s, want redis", status.Name)
	}
	if !status.Healthy {
		t.Error("StorageStatus.Healthy should be true")
	}
}
