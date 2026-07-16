package server

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/pkg/logger"
)

// TraceTimelineEvent 请求追踪时间轴事件
type TraceTimelineEvent struct {
	Phase      string            `json:"phase"`
	Label      string            `json:"label"`
	Timestamp  string            `json:"ts"`
	Level      string            `json:"level,omitempty"`
	DurationMs *int64            `json:"duration_ms,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Backend    string            `json:"backend,omitempty"`
	Model      string            `json:"model,omitempty"`
	StatusCode *int              `json:"status_code,omitempty"`
	Nodes      []string          `json:"nodes,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// TraceSummary 请求追踪摘要
type TraceSummary struct {
	ProxyMode  string `json:"proxy_mode,omitempty"`
	PipelineID string `json:"pipeline_id,omitempty"`
	Model      string `json:"model,omitempty"`
	BackendID  string `json:"backend_id,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	ClientIP   string `json:"client_ip,omitempty"`
	Level      string `json:"level,omitempty"`
	Success    bool   `json:"success"`
}

// TraceRouting 路由决策信息
type TraceRouting struct {
	DetectedMode   string `json:"detected_mode,omitempty"`
	Source         string `json:"source,omitempty"`
	ResolvedMode   string `json:"resolved_mode,omitempty"`
	ResolvedSource string `json:"resolved_source,omitempty"`
}

// TracePipelineGraph 流水线执行图（P1 摘要，P2 可扩展拓扑高亮）
type TracePipelineGraph struct {
	PipelineID    string         `json:"pipeline_id,omitempty"`
	ExecutedNodes []string       `json:"executed_nodes,omitempty"`
	NodeDetails   map[string]any `json:"node_details,omitempty"`
	TotalNodes    int            `json:"total_nodes,omitempty"`
	TotalTokens   int64          `json:"total_tokens,omitempty"`
}

// TraceResponse 请求追踪响应
type TraceResponse struct {
	RequestID     string               `json:"request_id"`
	Summary       TraceSummary         `json:"summary"`
	Routing       TraceRouting         `json:"routing"`
	Timeline      []TraceTimelineEvent `json:"timeline"`
	PipelineGraph TracePipelineGraph   `json:"pipeline_graph"`
	RawLogCount   int                  `json:"raw_log_count"`
}

func traceBodyPreview(entry LogEntry, keys ...string) string {
	for _, k := range keys {
		if v := traceExtra(entry, k); v != "" {
			return v
		}
	}
	return ""
}

func traceExtra(entry LogEntry, keys ...string) string {
	for _, k := range keys {
		if entry.Extra != nil {
			if v := strings.TrimSpace(entry.Extra[k]); v != "" {
				return v
			}
		}
	}
	return ""
}

func traceInt64Ptr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func traceIntPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func classifyTraceEvent(entry LogEntry) (phase, label, detail string, nodes []string) {
	msg := entry.Message
	lower := strings.ToLower(msg)

	switch {
	case isRequestStartMessage(msg):
		method := traceExtra(entry, "method")
		path := firstNonEmpty(entry.Path, traceExtra(entry, "path"))
		if method != "" && path != "" {
			detail = method + " " + path
		} else if path != "" {
			detail = path
		}
		return "request", "请求进入", detail, nil

	case strings.Contains(msg, "[Config] Resolved pipeline"):
		resolved := traceExtra(entry, "resolved_mode")
		source := traceExtra(entry, "resolved_source")
		if resolved == "" {
			resolved = extractAfterColon(msg)
		}
		if source != "" {
			detail = resolved + " (source: " + source + ")"
		} else {
			detail = resolved
		}
		return "routing", "解析默认流水线", detail, nil

	case strings.Contains(msg, "[Config] proxy mode"):
		mode := traceExtra(entry, "proxy_mode")
		source := traceExtra(entry, "source")
		if mode != "" && source != "" {
			detail = mode + " ← " + source
		} else {
			detail = firstNonEmpty(mode, extractAfterColon(msg))
		}
		return "routing", "检测代理模式", detail, nil

	case strings.Contains(lower, "pipeline execution started"):
		pid := traceExtra(entry, "pipeline_id")
		total := traceExtra(entry, "total_nodes")
		if pid != "" && total != "" {
			detail = pid + " (" + total + " nodes)"
		} else {
			detail = firstNonEmpty(pid, extractAfterColon(msg))
		}
		return "pipeline", "流水线启动", detail, nil

	case strings.Contains(lower, "pipeline execution finished"):
		pid := traceExtra(entry, "pipeline_id")
		tokens := traceExtra(entry, "total_tokens")
		if pid != "" && tokens != "" {
			detail = pid + ", tokens=" + tokens
		} else {
			detail = pid
		}
		return "pipeline", "流水线完成", detail, nil

	case strings.Contains(msg, "[generator]"):
		nodeID := traceExtra(entry, "node_id")
		if nodeID == "" {
			nodeID = "generator"
		}
		nodes = []string{nodeID}
		if strings.Contains(lower, "响应") || strings.Contains(lower, "response") {
			return "backend", "生成节点响应", traceBodyPreview(entry, "response_preview"), nodes
		}
		return "backend", "生成节点请求", traceBodyPreview(entry, "messages_preview", "user_input_preview"), nodes

	case strings.Contains(msg, "[backend]"):
		if strings.Contains(lower, "response") {
			return "backend", "后端响应", traceBodyPreview(entry, "response_preview"), nil
		}
		return "backend", "后端请求", traceBodyPreview(entry, "messages_preview"), nil

	case strings.Contains(msg, "[Response] details"):
		return "complete", "响应详情", traceBodyPreview(entry, "response_preview"), nil

	case strings.Contains(lower, "[request] completed"):
		return "complete", "请求完成", msg, nil

	case strings.Contains(lower, "[request] failed") || entry.Level == "error":
		return "error", "请求失败", msg, nil

	case strings.Contains(msg, "[Request] details"):
		return "request", "请求详情", traceBodyPreview(entry, "messages_preview"), nil

	default:
		if entry.BackendID != "" || entry.Model != "" {
			parts := make([]string, 0, 2)
			if entry.BackendID != "" {
				parts = append(parts, entry.BackendID)
			}
			if entry.Model != "" {
				parts = append(parts, entry.Model)
			}
			if nodeID := traceExtra(entry, "node_id"); nodeID != "" {
				return "backend", "后端调用", strings.Join(parts, " / "), []string{nodeID}
			}
			return "backend", "后端调用", strings.Join(parts, " / "), nil
		}
		return "info", shortenTraceMessage(msg), msg, nil
	}
}

func extractAfterColon(msg string) string {
	if idx := strings.Index(msg, ":"); idx >= 0 && idx+1 < len(msg) {
		return strings.TrimSpace(msg[idx+1:])
	}
	return strings.TrimSpace(msg)
}

func shortenTraceMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) <= 80 {
		return msg
	}
	return msg[:77] + "..."
}

func buildTraceFromLogs(requestID string, logs []LogEntry) TraceResponse {
	sort.Slice(logs, func(i, j int) bool {
		ti, _ := parseFlexibleLogTime(logs[i].Timestamp)
		tj, _ := parseFlexibleLogTime(logs[j].Timestamp)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return i < j
	})

	trace := TraceResponse{
		RequestID: requestID,
		PipelineGraph: TracePipelineGraph{
			NodeDetails: make(map[string]any),
		},
	}
	if len(logs) == 0 {
		return trace
	}
	trace.RawLogCount = len(logs)

	executedNodes := make([]string, 0)
	nodeSeen := make(map[string]struct{})
	timeline := make([]TraceTimelineEvent, 0, len(logs))

	for _, entry := range logs {
		phase, label, detail, nodes := classifyTraceEvent(entry)
		ev := TraceTimelineEvent{
			Phase:     phase,
			Label:     label,
			Timestamp: entry.Timestamp,
			Level:     entry.Level,
			Detail:    detail,
			Backend:   entry.BackendID,
			Model:     entry.Model,
			Nodes:     nodes,
		}
		if entry.DurationMs > 0 {
			ev.DurationMs = traceInt64Ptr(entry.DurationMs)
		}
		if entry.StatusCode > 0 {
			ev.StatusCode = traceIntPtr(entry.StatusCode)
		}
		if len(entry.Extra) > 0 {
			ev.Extra = entry.Extra
		}
		timeline = append(timeline, ev)

		// Summary fields
		if trace.Summary.StartedAt == "" && entry.Timestamp != "" {
			trace.Summary.StartedAt = entry.Timestamp
		}
		if entry.Timestamp != "" {
			trace.Summary.FinishedAt = entry.Timestamp
		}
		if entry.Path != "" {
			trace.Summary.Path = entry.Path
		}
		if entry.ClientIP != "" {
			trace.Summary.ClientIP = entry.ClientIP
		}
		if entry.Model != "" {
			trace.Summary.Model = entry.Model
		}
		if entry.BackendID != "" {
			trace.Summary.BackendID = entry.BackendID
		}
		if entry.StatusCode > 0 {
			trace.Summary.StatusCode = entry.StatusCode
		}
		if entry.DurationMs > trace.Summary.DurationMs {
			trace.Summary.DurationMs = entry.DurationMs
		}
		if entry.Level == "error" {
			trace.Summary.Level = "error"
		}

		method := traceExtra(entry, "method")
		if method != "" {
			trace.Summary.Method = method
		}

		if strings.Contains(entry.Message, "[Config] proxy mode") {
			if mode := traceExtra(entry, "proxy_mode"); mode != "" {
				trace.Routing.DetectedMode = mode
				if trace.Summary.ProxyMode == "" {
					trace.Summary.ProxyMode = mode
				}
			}
			if source := traceExtra(entry, "source"); source != "" {
				trace.Routing.Source = source
			}
		}
		if strings.Contains(entry.Message, "[Config] Resolved pipeline") {
			if mode := traceExtra(entry, "resolved_mode"); mode != "" {
				trace.Routing.ResolvedMode = mode
				trace.Summary.ProxyMode = mode
			}
			if source := traceExtra(entry, "resolved_source"); source != "" {
				trace.Routing.ResolvedSource = source
			}
		}
		if strings.Contains(strings.ToLower(entry.Message), "pipeline execution started") {
			if pid := traceExtra(entry, "pipeline_id"); pid != "" {
				trace.Summary.PipelineID = pid
				trace.PipelineGraph.PipelineID = pid
			}
			if total := traceExtra(entry, "total_nodes"); total != "" {
				if n, err := strconv.Atoi(total); err == nil && n > 0 {
					trace.PipelineGraph.TotalNodes = n
				}
			}
		}
		if strings.Contains(strings.ToLower(entry.Message), "pipeline execution finished") {
			if tokens := traceExtra(entry, "total_tokens"); tokens != "" {
				if n, err := strconv.ParseInt(tokens, 10, 64); err == nil && n > 0 {
					trace.PipelineGraph.TotalTokens = n
				}
			}
		}
		for _, node := range nodes {
			if node == "" {
				continue
			}
			if _, ok := nodeSeen[node]; !ok {
				nodeSeen[node] = struct{}{}
				executedNodes = append(executedNodes, node)
			}
			trace.PipelineGraph.NodeDetails[node] = map[string]any{
				"backend": entry.BackendID,
				"model":   entry.Model,
			}
		}
	}

	trace.Timeline = timeline
	trace.PipelineGraph.ExecutedNodes = executedNodes
	trace.Summary.Success = trace.Summary.Level != "error" &&
		(trace.Summary.StatusCode == 0 || (trace.Summary.StatusCode >= 200 && trace.Summary.StatusCode < 400))

	return trace
}

// GetTrace 按 request_id 聚合请求追踪
func (h *LogHandler) GetTrace(c *gin.Context) {
	requestID := strings.TrimSpace(c.Param("request_id"))
	if requestID == "" {
		RespondError(c, http.StatusBadRequest, "request_id is required")
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	fromTime, toTime, err := parseLogQueryTimeRange(from, to)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if from == "" {
		fromTime = time.Now().Add(-7 * 24 * time.Hour)
	}

	req := LogQueryRequest{
		From:      from,
		To:        to,
		RequestID: requestID,
		Limit:     5000,
		Page:      1,
	}
	logs, total, err := h.readAndFilterLogs(fromTime, toTime, req, 1, req.Limit)
	if err != nil {
		logger.Errorf("Failed to read trace logs: %v", err)
		RespondError(c, http.StatusInternalServerError, "Failed to read trace logs: "+err.Error())
		return
	}
	if total == 0 {
		RespondError(c, http.StatusNotFound, fmt.Sprintf("no logs found for request_id %q", requestID))
		return
	}

	propagateRequestContext(logs)
	trace := buildTraceFromLogs(requestID, logs)
	RespondSuccess(c, trace)
}
