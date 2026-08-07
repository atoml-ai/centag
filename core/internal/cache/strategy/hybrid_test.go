package strategy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"centag/core/pkg/storage"
)

type memKV struct {
	data map[string][]byte
}

func newMemKV() *memKV { return &memKV{data: map[string][]byte{}} }

func (m *memKV) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	switch v := value.(type) {
	case []byte:
		m.data[key] = v
	case string:
		m.data[key] = []byte(v)
	default:
		m.data[key] = []byte(fmt.Sprint(v))
	}
	return nil
}
func (m *memKV) Get(ctx context.Context, key string) (interface{}, error) {
	b, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	return b, nil
}
func (m *memKV) GetBytes(ctx context.Context, key string) ([]byte, error) {
	b, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	return b, nil
}
func (m *memKV) GetString(ctx context.Context, key string) (string, error) {
	b, err := m.GetBytes(ctx, key)
	return string(b), err
}
func (m *memKV) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}
func (m *memKV) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}
func (m *memKV) Expire(ctx context.Context, key string, ttl time.Duration) error { return nil }
func (m *memKV) TTL(ctx context.Context, key string) (time.Duration, error)     { return 0, nil }
func (m *memKV) SetBatch(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	return nil
}
func (m *memKV) GetBatch(ctx context.Context, keys []string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *memKV) DeleteBatch(ctx context.Context, keys []string) error { return nil }
func (m *memKV) Keys(ctx context.Context, pattern string) ([]string, error) {
	return nil, nil
}
func (m *memKV) Count(ctx context.Context, pattern string) (int64, error) { return 0, nil }
func (m *memKV) GetAll(ctx context.Context, pattern string) (map[string][]byte, error) {
	return nil, nil
}
func (m *memKV) FlushDB(ctx context.Context) error           { return nil }
func (m *memKV) Close() error                                { return nil }
func (m *memKV) GetStoreInfo() storage.StoreInfo             { return storage.StoreInfo{Type: "memory"} }

func TestHybridStrategy_ReadExactFirstThenSemantic(t *testing.T) {
	kv := newMemKV()
	exact := NewExactStrategy(&ExactConfig{})
	exact.SetKVStore(kv)
	_ = kv.Set(context.Background(), "exact-key", []byte("from-exact"), 0)

	sem := NewSemanticStrategy(&SemanticConfig{Threshold: 0.8, TopK: 3})
	sem.SetEmbeddingService(&mockEmbeddingService{dimension: 8})
	store := &mockVectorStore{}
	sem.SetVectorStore(store)
	_ = store.Insert(context.Background(), []storage.Vector{{
		ID: "sem-key", Vector: []float32{1}, Metadata: map[string]interface{}{"response": "from-semantic"},
	}})

	h := NewHybridStrategy(exact, sem)
	if h.Name() != "hybrid" || !h.SupportsSemantic() {
		t.Fatalf("name/supports: %s %v", h.Name(), h.SupportsSemantic())
	}

	r, err := h.Read(context.Background(), "exact-key", ReadOptions{})
	if err != nil || r == nil || !r.Hit || r.Content != "from-exact" || r.SourceStrategy != "exact" {
		t.Fatalf("exact hit first: %+v err=%v", r, err)
	}

	r, err = h.Read(context.Background(), "missing-exact", ReadOptions{Threshold: 0.8})
	if err != nil || r == nil || !r.Hit || r.Content != "from-semantic" || r.SourceStrategy != "semantic" {
		t.Fatalf("semantic fallthrough: %+v err=%v", r, err)
	}
}

func TestHybridStrategy_WriteAndDeleteBoth(t *testing.T) {
	kv := newMemKV()
	exact := NewExactStrategy(&ExactConfig{})
	exact.SetKVStore(kv)
	sem := NewSemanticStrategy(&SemanticConfig{Threshold: 0.8, TopK: 3})
	sem.SetEmbeddingService(&mockEmbeddingService{dimension: 8})
	store := &mockVectorStore{}
	sem.SetVectorStore(store)

	h := NewHybridStrategy(exact, sem)
	err := h.Write(context.Background(), &Entry{
		Key: "k1", Request: "hello world", Response: "hi",
	}, WriteOptions{TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.GetBytes(context.Background(), "k1"); err != nil {
		t.Fatalf("exact write missing: %v", err)
	}
	if len(store.vectors) == 0 {
		t.Fatal("semantic write missing")
	}
	if err := h.Delete(context.Background(), "k1"); err != nil {
		t.Fatal(err)
	}
}
