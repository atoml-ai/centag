package pipeline

import "time"

// ═══════════════════════════════════════════════════════════
// 存储键格式约定
//
// 通用格式：pipeline:{namespace}:{category}:{detail}
//
// 示例：
//   pipeline:education-scene:history                    — 教育流水线对话历史
//   pipeline:education-scene:scene_context               — 教育流水线场景上下文
//   pipeline:education-scene:user:123:progress            — 用户学习进度
//   pipeline:coding-agent:code_snippets                  — 编程流水线代码片段
//   pipeline:coding-agent:solutions                      — 编程流水线解决方案
//   pipeline:{namespace}:execution:{timestamp}           — 执行记录归档
//   pipeline:{namespace}:node:{node_id}:output           — 节点输出
// ═══════════════════════════════════════════════════════════

// EducationStorageSchema 教育流水线存储 Schema
type EducationStorageSchema struct {
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	Scene       string                 `json:"scene"`
	History     []SceneHistoryEntry    `json:"history"`
	Progress    map[string]interface{} `json:"progress"`
	LastUpdated time.Time              `json:"last_updated"`
}

// SceneHistoryEntry 教育场景历史记录条目
type SceneHistoryEntry struct {
	Scene     string    `json:"scene"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	Score     float64   `json:"score,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// CodingStorageSchema 编程流水线存储 Schema
type CodingStorageSchema struct {
	UserID       string                 `json:"user_id"`
	SessionID    string                 `json:"session_id"`
	CodeSnippets []CodeSnippet          `json:"code_snippets"`
	Solutions    []CodingSolution       `json:"solutions"`
	Progress     map[string]interface{} `json:"progress"`
	LastUpdated  time.Time              `json:"last_updated"`
}

// CodeSnippet 代码片段
type CodeSnippet struct {
	ID          string    `json:"id"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	SessionID   string    `json:"session_id,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CodingSolution 编程解决方案
type CodingSolution struct {
	TaskDescription string    `json:"task_description"`
	Solution        string    `json:"solution"`
	Files           []string  `json:"files,omitempty"`
	Success         bool      `json:"success"`
	SessionID       string    `json:"session_id,omitempty"`
	RequestID       string    `json:"request_id,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}
