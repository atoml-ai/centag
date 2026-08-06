package strategy

import (
	"context"
	"testing"

	"centag/core/pkg/storage"
)

func TestSemanticStrategyRead_AppliesThreshold(t *testing.T) {
	embed := &mockEmbeddingService{dimension: 8}
	store := &mockVectorStore{}
	s := NewSemanticStrategy(&SemanticConfig{Threshold: 0.95, TopK: 3})
	s.SetEmbeddingService(embed)
	s.SetVectorStore(store)

	_ = store.Insert(context.Background(), []storage.Vector{{
		ID:       "k1",
		Vector:   []float32{1},
		Metadata: map[string]interface{}{"response": "cached"},
	}})

	// mockVectorStore.Search returns score 0.9 — below default threshold 0.95
	r, err := s.Read(context.Background(), "query", ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Hit {
		t.Fatal("expected miss when score 0.9 < threshold 0.95")
	}

	r, err = s.Read(context.Background(), "query", ReadOptions{Threshold: 0.8})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Hit || r.Content != "cached" {
		t.Fatalf("expected hit with lower threshold, got %+v", r)
	}
}

func TestSemanticStrategyWrite_RequiresRequest(t *testing.T) {
	s := NewSemanticStrategy(&SemanticConfig{})
	s.SetEmbeddingService(&mockEmbeddingService{dimension: 8})
	s.SetVectorStore(&mockVectorStore{})
	err := s.Write(context.Background(), &Entry{Key: "k", Response: "r"}, WriteOptions{})
	if err == nil {
		t.Fatal("expected error for empty Request")
	}
}
