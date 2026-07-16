package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"centag/core/pkg/plugin"
)

func TestStreamAdapter_EmptyContent(t *testing.T) {
	adapter := NewStreamAdapter()
	ch := adapter.Adapt(context.Background(), "")
	count := 0
	for chunk := range ch {
		count++
		if chunk.Content != "" {
			t.Errorf("empty content should yield empty chunk, got %q", chunk.Content)
		}
		if !chunk.Done {
			t.Errorf("empty content should yield Done=true")
		}
	}
	if count != 1 {
		t.Errorf("empty content should yield exactly 1 chunk, got %d", count)
	}
}

func TestStreamAdapter_SingleChunk(t *testing.T) {
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 100
	ch := adapter.Adapt(context.Background(), "hello")
	var collected []string
	doneSeen := false
	for chunk := range ch {
		collected = append(collected, chunk.Content)
		if chunk.Done {
			doneSeen = true
		}
	}
	joined := strings.Join(collected, "")
	if joined != "hello" {
		t.Errorf("expected %q, got %q", "hello", joined)
	}
	if !doneSeen {
		t.Errorf("expected at least one Done=true chunk")
	}
}

func TestStreamAdapter_MultipleChunks(t *testing.T) {
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 4
	adapter.splitOnBoundary = false // 强制硬切分便于断言

	content := "abcdefghij"
	ch := adapter.Adapt(context.Background(), content)

	var collected []string
	doneSeen := false
	for chunk := range ch {
		collected = append(collected, chunk.Content)
		if chunk.Done {
			doneSeen = true
		}
	}

	joined := strings.Join(collected, "")
	if joined != content {
		t.Errorf("expected reassembled %q, got %q", content, joined)
	}
	if !doneSeen {
		t.Errorf("expected at least one Done=true chunk")
	}
	if len(collected) < 3 {
		t.Errorf("expected multiple chunks for 10-char content with size=4, got %d chunks", len(collected))
	}
}

func TestStreamAdapter_SplitOnBoundary(t *testing.T) {
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 8
	adapter.splitOnBoundary = true

	// "hello world foo bar" → 期望在空格处切分
	content := "hello world foo bar"
	ch := adapter.Adapt(context.Background(), content)

	var collected []string
	for chunk := range ch {
		collected = append(collected, chunk.Content)
	}
	joined := strings.Join(collected, "")
	if joined != content {
		t.Errorf("expected reassembled %q, got %q", content, joined)
	}
	// 至少有一个 chunk 在中间以 " " 结尾（说明按空白边界切分）
	sawBoundaryCut := false
	for i, c := range collected {
		if i == len(collected)-1 {
			continue
		}
		if strings.HasSuffix(c, " ") {
			sawBoundaryCut = true
			break
		}
	}
	if !sawBoundaryCut {
		t.Errorf("expected at least one chunk to end on whitespace boundary; got chunks: %q", collected)
	}
}

func TestStreamAdapter_MultibyteUTF8(t *testing.T) {
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 4
	adapter.splitOnBoundary = false

	// 中英文混合，验证不会切碎多字节字符
	content := "你好世界hello"
	ch := adapter.Adapt(context.Background(), content)

	var collected []string
	for chunk := range ch {
		collected = append(collected, chunk.Content)
	}
	joined := strings.Join(collected, "")
	if joined != content {
		t.Errorf("expected reassembled %q, got %q", content, joined)
	}
}

func TestStreamAdapter_ZeroChunkSize(t *testing.T) {
	adapter := &StreamAdapter{ChunkSize: 0, splitOnBoundary: false}
	ch := adapter.Adapt(context.Background(), "abcdef")
	var collected []string
	for chunk := range ch {
		collected = append(collected, chunk.Content)
	}
	if strings.Join(collected, "") != "abcdef" {
		t.Errorf("zero chunk size should fall back to default")
	}
}

func TestStreamAdapter_ContextCancel(t *testing.T) {
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 1
	adapter.ChunkDelay = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := adapter.Adapt(ctx, "abcdefghij")

	count := 0
	for range ch {
		count++
		if count == 2 {
			cancel()
		}
	}
	if count >= 10 {
		t.Errorf("expected cancellation to stop chunks early, got %d", count)
	}
}

func TestStreamAdapter_NonEmptyTokensInFinalChunk(t *testing.T) {
	// 验证最后一个 chunk 同时携带 Done=true
	adapter := NewStreamAdapter()
	adapter.ChunkSize = 2
	ch := adapter.Adapt(context.Background(), "abc")
	var lastChunk *plugin.StreamChunk
	for chunk := range ch {
		c := chunk
		lastChunk = &c
	}
	if lastChunk == nil {
		t.Fatal("expected at least one chunk")
	}
	if !lastChunk.Done {
		t.Errorf("final chunk should have Done=true")
	}
}
