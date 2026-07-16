// Package thinksplit 提供从响应内容中分离 <think>...</think> 标签的纯函数。
//
// 当上游把思考写在 <think>...</think> 时，拆到 ReasoningContent，
// Content 不含标签；流式不泄漏。
package thinksplit

import (
	"strings"
)

const (
	// thinkOpen 标签开始标记
	thinkOpen = "<think>"
	// thinkClose 标签结束标记
	thinkClose = "</think>"
)

// Split 从 content 中分离 <think>...</think> 标签。
// 返回值:
//   - visible: 不含 <think> 标签的可见内容
//   - reasoning: 从 <think> 标签中提取的推理内容
//
// 处理逻辑:
//   - 无标签: visible=content, reasoning=""
//   - 完整标签: visible=标签前内容+标签后内容, reasoning=标签内内容
//   - 只有 open 没有 close: visible=标签前内容, reasoning=标签后内容（进入推理缓冲）
//   - 嵌套标签: 仅处理最外层（使用计数器匹配）
func Split(content string) (visible string, reasoning string) {
	if content == "" {
		return "", ""
	}

	// 查找开始标签
	openIdx := strings.Index(content, thinkOpen)
	if openIdx == -1 {
		// 没有开始标签，返回原始内容
		return content, ""
	}

	// 查找匹配的结束标签（使用计数器处理嵌套）
	closeIdx := -1
	nesting := 0
	remaining := content[openIdx+len(thinkOpen):]
	
	for i := 0; i < len(remaining); {
		if strings.HasPrefix(remaining[i:], thinkOpen) {
			nesting++
			i += len(thinkOpen)
		} else if strings.HasPrefix(remaining[i:], thinkClose) {
			if nesting == 0 {
				// 找到匹配的close标签
				closeIdx = openIdx + len(thinkOpen) + i
				break
			}
			nesting--
			i += len(thinkClose)
		} else {
			i++
		}
	}

	if closeIdx == -1 {
		// 没有结束标签，只有 open 标签
		// visible = open 标签之前的内容
		// reasoning = open 标签之后的内容（进入推理缓冲）
		visible = content[:openIdx]
		reasoning = content[openIdx+len(thinkOpen):]
		return visible, reasoning
	}

	// 提取各部分
	beforeOpen := content[:openIdx]
	insideThink := content[openIdx+len(thinkOpen) : closeIdx]
	afterClose := content[closeIdx+len(thinkClose):]

	// visible = beforeOpen + afterClose
	visible = beforeOpen + afterClose
	reasoning = insideThink

	return visible, reasoning
}

// StreamSplitter 流式状态机，用于处理跨 chunk 的 <think> 标签。
// 设计：只处理第一个 <think>...</think> 标签，后续的 <think> 标签作为可见内容。
type StreamSplitter struct {
	// 状态
	buffer      string // 累积的未处理内容
	inThinkTag  bool   // 是否在 <think> 标签内
	thinkBuffer string // 推理内容缓冲
	foundFirst  bool   // 是否已找到第一个 <think> 标签
}

// NewStreamSplitter 创建新的流式分割器。
func NewStreamSplitter() *StreamSplitter {
	return &StreamSplitter{}
}

// Feed 处理一个流式 delta，返回分离后的 visibleDelta 和 reasoningDelta。
func (s *StreamSplitter) Feed(delta string) (visibleDelta, reasoningDelta string) {
	if delta == "" {
		return "", ""
	}

	s.buffer += delta
	return s.process()
}

// Flush 刷新缓冲区，处理剩余内容。
// 当流结束时调用，确保所有内容都被处理。
func (s *StreamSplitter) Flush() (visibleDelta, reasoningDelta string) {
	if s.buffer == "" {
		return "", ""
	}

	// 处理剩余缓冲区
	visible, reasoning := s.processBuffer()

	// 重置状态
	s.buffer = ""
	s.thinkBuffer = ""

	return visible, reasoning
}

// process 处理缓冲区中的内容。
func (s *StreamSplitter) process() (visibleDelta, reasoningDelta string) {
	// 累积本轮找到的推理内容
	pendingReasoning := ""

	for {
		if s.buffer == "" {
			break
		}

		if s.inThinkTag {
			// 在 <think> 标签内，查找匹配的结束标签（使用计数器处理嵌套）
			closeIdx := -1
			nesting := 0
			for i := 0; i < len(s.buffer); {
				if strings.HasPrefix(s.buffer[i:], thinkOpen) {
					nesting++
					i += len(thinkOpen)
				} else if strings.HasPrefix(s.buffer[i:], thinkClose) {
					if nesting == 0 {
						// 找到匹配的close标签
						closeIdx = i
						break
					}
					nesting--
					i += len(thinkClose)
				} else {
					i++
				}
			}

			if closeIdx == -1 {
				// 没有匹配的结束标签，整个缓冲区都是推理内容
				newContent := s.buffer
				s.buffer = ""
				return "", newContent
			}

			// 找到匹配的结束标签
			insideThink := s.buffer[:closeIdx]
			s.buffer = s.buffer[closeIdx+len(thinkClose):]
			s.inThinkTag = false

			// 累积推理内容，继续处理剩余缓冲区
			pendingReasoning += s.thinkBuffer + insideThink
			s.thinkBuffer = ""
			continue
		} else {
			// 不在 <think> 标签内，查找开始标签
			// 只有在找到第一个 <think> 标签时才处理，后续的作为可见内容
			if !s.foundFirst {
				openIdx := strings.Index(s.buffer, thinkOpen)
				if openIdx != -1 {
					// 找到开始标签
					visibleDelta = s.buffer[:openIdx]
					s.buffer = s.buffer[openIdx+len(thinkOpen):]
					s.inThinkTag = true
					s.thinkBuffer = ""
					s.foundFirst = true

					if visibleDelta != "" || pendingReasoning != "" {
						return visibleDelta, pendingReasoning
					}
					// 如果 visibleDelta 和 pendingReasoning 都为空，继续处理
					continue
				}
			}

			// 没有开始标签，整个缓冲区都是可见内容
			visibleDelta = s.buffer
			s.buffer = ""
			return visibleDelta, pendingReasoning
		}
	}

	return "", pendingReasoning
}

// processBuffer 处理缓冲区中的内容（用于 Flush）。
func (s *StreamSplitter) processBuffer() (visible string, reasoning string) {
	if s.inThinkTag {
		// 在 <think> 标签内，查找匹配的结束标签
		closeIdx := -1
		nesting := 0
		for i := 0; i < len(s.buffer); {
			if strings.HasPrefix(s.buffer[i:], thinkOpen) {
				nesting++
				i += len(thinkOpen)
			} else if strings.HasPrefix(s.buffer[i:], thinkClose) {
				if nesting == 0 {
					closeIdx = i
					break
				}
				nesting--
				i += len(thinkClose)
			} else {
				i++
			}
		}

		if closeIdx == -1 {
			// 没有匹配的结束标签，整个缓冲区都是推理内容
			return "", s.buffer
		}

		// 找到匹配的结束标签
		insideThink := s.buffer[:closeIdx]
		afterClose := s.buffer[closeIdx+len(thinkClose):]

		reasoning = s.thinkBuffer + insideThink
		s.thinkBuffer = ""
		s.inThinkTag = false

		return afterClose, reasoning
	}

	// 不在 <think> 标签内，查找开始标签（只处理第一个）
	if !s.foundFirst {
		openIdx := strings.Index(s.buffer, thinkOpen)
		if openIdx != -1 {
			visible = s.buffer[:openIdx]
			reasoning = s.buffer[openIdx+len(thinkOpen):]
			s.foundFirst = true
			return visible, reasoning
		}
	}

	// 没有开始标签，整个缓冲区都是可见内容
	return s.buffer, ""
}
