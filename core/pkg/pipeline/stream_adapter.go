package pipeline

import (
	"context"
	"strings"
	"time"

	"centag/core/pkg/plugin"
)

// StreamAdapter 将完整字符串切分为流式 chunk 序列。
//
// 设计动机：流水线引擎内部所有节点统一非流式执行（Execute()），
// 完整 NodeOutput 由顶层流式适配器按需分块后下发给客户端。
// 这种"请求驱动 + 代理层统一适配"的模型可以让 optimize-mode / translate-mode
// 等最后节点是 processor 的流水线也能自然支持流式输出。
//
// 当前实现采用固定大小分块（带空白字符边界优先），
// 未来可替换为按语义边界（句子/段落）的智能分块算法，对外接口保持不变。
type StreamAdapter struct {
	// ChunkSize 每个 chunk 的字符数，0 或负值时使用默认值。
	ChunkSize int
	// ChunkDelay 相邻 chunk 之间的发送间隔，<=0 表示无延迟。
	ChunkDelay time.Duration
	// splitOnBoundary 优先在空白字符边界切分，避免切断单词/UTF-8 字符。
	splitOnBoundary bool
}

const (
	defaultStreamChunkSize = 16
	minStreamChunkSize     = 1
)

// NewStreamAdapter 创建默认配置的 StreamAdapter。
func NewStreamAdapter() *StreamAdapter {
	return &StreamAdapter{
		ChunkSize:       defaultStreamChunkSize,
		ChunkDelay:      0,
		splitOnBoundary: true,
	}
}

// Adapt 将完整内容切分为 chunk 序列。
// 返回的 channel 在所有 chunk 写入完成或 ctx 取消后关闭。
// 调用方应持续读取直到 channel 关闭。
//
// 注意：当前实现以 rune（Unicode 码点）为分块单位，避免切碎多字节字符。
// 当 ChunkDelay > 0 时，发送间隔用于模拟真实 LLM 输出的节流，
// 让客户端对分块的感知更接近原生流式协议。
func (a *StreamAdapter) Adapt(ctx context.Context, content string) <-chan plugin.StreamChunk {
	out := make(chan plugin.StreamChunk, 16)

	chunkSize := a.ChunkSize
	if chunkSize < minStreamChunkSize {
		chunkSize = defaultStreamChunkSize
	}

	go func() {
		defer close(out)

		if content == "" {
			select {
			case out <- plugin.StreamChunk{Content: "", Done: true}:
			case <-ctx.Done():
			}
			return
		}

		chunks := a.split(content, chunkSize)
		for i, c := range chunks {
			isLast := i == len(chunks)-1
			chunk := plugin.StreamChunk{
				Content: c,
				Done:    isLast,
			}
			select {
			case out <- chunk:
			case <-ctx.Done():
				return
			}

			if a.ChunkDelay > 0 && !isLast {
				timer := time.NewTimer(a.ChunkDelay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}
		}
	}()

	return out
}

// split 将 content 切分为固定大小的 chunk。
// 若 splitOnBoundary=true，则优先在空白字符（空格、换行）处切分，
// 避免把单词/标识符从中间切断。无法找到合适边界时回退到硬切分。
func (a *StreamAdapter) split(content string, chunkSize int) []string {
	runes := []rune(content)
	if len(runes) <= chunkSize {
		return []string{string(runes)}
	}

	chunks := make([]string, 0, len(runes)/chunkSize+1)
	for start := 0; start < len(runes); {
		end := start + chunkSize
		if end >= len(runes) {
			chunks = append(chunks, string(runes[start:]))
			break
		}

		if a.splitOnBoundary {
			// 在 [end-3, end) 区间寻找最近的空白字符作为切分点，
			// 保证至少向前推进 1 个字符
			cut := -1
			for i := end - 1; i > start; i-- {
				if isWhitespaceRune(runes[i]) {
					cut = i
					break
				}
			}
			if cut > 0 {
				chunks = append(chunks, string(runes[start:cut+1]))
				start = cut + 1
				continue
			}
		}

		chunks = append(chunks, string(runes[start:end]))
		start = end
	}
	return chunks
}

func isWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || strings.ContainsRune("\u3000", r)
}
