package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"centag/core/pkg/config"
	"centag/core/pkg/logger"
)

// stdoutOnlyLogViewerBody 当日志只写终端时，说明 Web「日志查看」与终端数据源不一致（前缀由进程内 log.output 拼接）
const stdoutOnlyLogViewerBody = "运行中的日志只出现在启动服务的终端，不会写入日志文件；本页读取的是磁盘上的历史文件，因此可能只有旧记录（例如上次关服）或与终端不一致。若要在网页中查看与终端一致的日志，请设置环境变量 LLM_PROXY_LOG_OUTPUT=file 或 both，并确认 LLM_PROXY_LOG_PATH / LLM_PROXY_LOG_FILENAME 与 zap 写入路径一致，然后重启服务。若 config/secrets/.env 已设为 file/both 仍见本提示，说明启动时进程实际拿到的不是该值（例如某启动脚本在 load_env 后又 export 覆盖了 LLM_PROXY_LOG_OUTPUT）。"

// LogEntry 日志条目结构
type LogEntry struct {
	Timestamp    string  `json:"timestamp"`
	Level        string  `json:"level"`
	RequestID    string  `json:"request_id,omitempty"`
	UserID       int64   `json:"user_id,omitempty"`
	APIKeyPrefix string  `json:"api_key_prefix,omitempty"`
	BackendID    string  `json:"backend_id,omitempty"`
	BackendType  string  `json:"backend_type,omitempty"`
	Model        string  `json:"model,omitempty"`
	Strategy     string  `json:"strategy,omitempty"`
	CacheHit     bool    `json:"cache_hit,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	StatusCode   int     `json:"status_code,omitempty"`
	Message      string            `json:"message"`
	ClientIP     string            `json:"client_ip,omitempty"`
	Path         string            `json:"path,omitempty"`
	Caller       string            `json:"caller,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

var logEntryStructuredKeys = map[string]struct{}{
	"timestamp": {}, "level": {}, "message": {}, "msg": {}, "caller": {},
	"request_id": {}, "user_id": {}, "api_key_prefix": {},
	"backend_id": {}, "backend": {}, "backend_type": {}, "selected_backend": {}, "cached_backend": {},
	"model": {}, "requested_model": {}, "strategy": {}, "cache_hit": {},
	"duration_ms": {}, "latency_ms": {}, "total_latency_ms": {},
	"status_code": {}, "status": {}, "client_ip": {}, "ip": {}, "path": {},
	"logger": {}, "stacktrace": {}, "function": {},
}

// LogQueryRequest 日志查询请求
type LogQueryRequest struct {
	From      string `form:"from"`
	To        string `form:"to"`
	UserID    string `form:"user_id"`
	BackendID string `form:"backend_id"`
	Model     string `form:"model"`
	Strategy  string `form:"strategy"`
	Level     string `form:"level"`
	// Q 全文关键词（消息、请求 ID、后端、模型、调用方等）
	Q string `form:"q"`
	// RequestID 按请求/对话 ID 筛选（支持部分匹配）
	RequestID string `form:"request_id"`
	// Category 预设：api / llm = 仅大模型服务（/v1 代理）相关日志；system = 排除 LLM 服务日志
	Category string `form:"category"`
	Limit    int    `form:"limit"`
	Page     int    `form:"page"`
}

// LogQueryResponse 日志查询响应
type LogQueryResponse struct {
	Logs       []LogEntry `json:"logs"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
	// NewestFirst 为 true 时表示列表按时间降序，第 1 页为最新
	NewestFirst bool `json:"newest_first"`
	// LogPath 当前读取的主日志文件路径（便于排查「无日志」）
	LogPath string `json:"log_path,omitempty"`
	// Warning 非空时表示：例如当前进程日志输出为 stdout，文件里只有历史/空，与终端所见不一致
	Warning string `json:"warning,omitempty"`
}

// LogHandler 日志处理器
type LogHandler struct {
	logPath         string
	stdoutOnly      bool
	logOutputRaw    string // 启动时 cfg.Log.Output 原始值，便于与 config/secrets/.env 对照
}

// NewLogHandler 创建日志处理器（路径与 zap 写入文件一致，来自 bootstrap/env 的 Log 配置）
func NewLogHandler(cfg *config.Config) *LogHandler {
	fn := strings.TrimSpace(cfg.Log.File.Filename)
	if fn == "" {
		fn = "centag.log"
	}
	dir := strings.TrimSpace(cfg.Log.File.Path)
	if dir == "" {
		dir = filepath.Join("bin", "logs")
	}
	logPath := filepath.Join(dir, fn)
	rawOut := strings.TrimSpace(cfg.Log.Output)
	out := strings.ToLower(rawOut)
	// file / both 会写文件；stdout 则当前运行日志不在该文件中
	stdoutOnly := out == "stdout"
	return &LogHandler{
		logPath:      logPath,
		stdoutOnly:   stdoutOnly,
		logOutputRaw: rawOut,
	}
}

func (h *LogHandler) stdoutOnlyWarning() string {
	if !h.stdoutOnly {
		return ""
	}
	display := h.logOutputRaw
	if display == "" {
		display = "(空，logger 将走默认分支，等同 stdout)"
	}
	return fmt.Sprintf("当前进程 log.output=%q：%s", display, stdoutOnlyLogViewerBody)
}

// GetLogs 获取日志列表
func (h *LogHandler) GetLogs(c *gin.Context) {
	// 解析查询参数
	var req LogQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid query parameters: "+err.Error())
		return
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = 1000
	}
	if req.Limit > 10000 {
		req.Limit = 10000 // 最大限制 10000 条
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	// 解析时间范围
	var fromTime, toTime time.Time
	var err error

	fromTime, toTime, err = parseLogQueryTimeRange(req.From, req.To)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 读取并过滤日志
	logs, total, err := h.readAndFilterLogs(fromTime, toTime, req, req.Page, req.Limit)
	if err != nil {
		logger.Errorf("Failed to read logs: %v", err)
		RespondError(c, http.StatusInternalServerError, "Failed to read logs: "+err.Error())
		return
	}

	// 计算总页数
	totalPages := (total + req.Limit - 1) / req.Limit
	if totalPages == 0 {
		totalPages = 1
	}

	resp := LogQueryResponse{
		Logs:        logs,
		Total:       total,
		Page:        req.Page,
		PageSize:    req.Limit,
		TotalPages:  totalPages,
		NewestFirst: true,
		LogPath:     h.logPath,
	}
	if w := h.stdoutOnlyWarning(); w != "" {
		resp.Warning = w
	}
	RespondSuccess(c, resp)
}

// parseLogQueryTimeRange 解析查询时间范围。from 为空表示不限制起始时间；to 为空表示截止到当前时间。
func parseLogQueryTimeRange(fromStr, toStr string) (fromTime, toTime time.Time, err error) {
	if fromStr != "" {
		fromTime, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from time format: %w", err)
		}
	}
	if toStr != "" {
		toTime, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to time format: %w", err)
		}
	} else {
		toTime = time.Now()
	}
	return fromTime, toTime, nil
}

// readAndFilterLogs 读取并过滤日志
func (h *LogHandler) readAndFilterLogs(fromTime, toTime time.Time, req LogQueryRequest, page, limit int) ([]LogEntry, int, error) {
	// 找到所有日志文件
	logFiles, err := h.findLogFiles()
	if err != nil {
		return nil, 0, err
	}

	// 读取所有匹配的日志条目
	var allLogs []LogEntry

	for _, logFile := range logFiles {
		logs, err := h.readLogFile(logFile, fromTime, toTime, req)
		if err != nil {
			logger.Warnf("Failed to read log file %s: %v", logFile, err)
			continue
		}
		allLogs = append(allLogs, logs...)
	}

	propagateRequestContext(allLogs)

	filtered := make([]LogEntry, 0, len(allLogs))
	for _, entry := range allLogs {
		if h.matchesFilters(entry, req) {
			filtered = append(filtered, entry)
		}
	}
	allLogs = filtered

	// 按时间排序（最新的在前）
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp > allLogs[j].Timestamp
	})

	total := len(allLogs)

	// limit <= 0 表示不分页（导出等场景返回全部匹配项）
	if limit <= 0 {
		return allLogs, total, nil
	}

	// 分页
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		return []LogEntry{}, total, nil
	}
	if end > total {
		end = total
	}

	return allLogs[start:end], total, nil
}

// findLogFiles 找到所有日志文件（包括轮转文件）
func (h *LogHandler) findLogFiles() ([]string, error) {
	var files []string

	// 主日志文件
	if _, err := os.Stat(h.logPath); err == nil {
		files = append(files, h.logPath)
	}

	// 查找轮转文件（centag.log.1, centag.log.2.gz 等）
	dir := filepath.Dir(h.logPath)
	baseName := filepath.Base(h.logPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return files, nil // 忽略错误，返回已找到的文件
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 匹配 centag.log.* 或 centag.log.*.gz
		if strings.HasPrefix(name, baseName+".") {
			files = append(files, filepath.Join(dir, name))
		}
	}

	// 按文件名排序（最新的在前）
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	return files, nil
}

// readLogFile 读取单个日志文件并过滤
func (h *LogHandler) readLogFile(logFile string, fromTime, toTime time.Time, req LogQueryRequest) ([]LogEntry, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logs []LogEntry

	// 处理 gzip 压缩文件
	var reader io.Reader = file
	if strings.HasSuffix(logFile, ".gz") {
		// TODO: 如果需要支持 gzip 文件，这里添加 gzip 解压
		// 暂时跳过压缩文件
		return logs, nil
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := parseLogLine(line)
		if err != nil {
			continue // 跳过无法解析的行
		}

		// 时间过滤
		if !h.matchesTimeRange(entry, fromTime, toTime) {
			continue
		}

		logs = append(logs, entry)
	}

	return logs, scanner.Err()
}

// zap 控制台编码一行：时间 + 级别 + 调用位置 + 消息（与 internal/logger 的 console encoder 一致）
var zapConsoleLogLine = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:Z|[+-]\d{4}|[+-]\d{2}:\d{2}))\s+(\S+)\s+(\S+)\s+(.*)$`,
)

// zapCompactConsoleLogLine 桌面端等场景：console 字段间无空格，尾部 JSON 紧跟 message
// 例：2026-06-10T12:49:29.539+0800infologger/logger.go:128request{"method":"GET","path":"/api/..."}
var zapCompactConsoleLogLine = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+(?:Z|[+-]\d{4}|[+-]\d{2}:\d{2}))(debug|info|warn|error|fatal|dpanic|panic)([\w./-]+:\d+)(.*)$`,
)

var (
	reRequestIDInMsg = regexp.MustCompile(`(?i)(?:\[request\]\s*)?id:\s*(\S+)`)
	reClientIPInMsg  = regexp.MustCompile(`(?i)client\s*ip:\s*(\S+)`)
	reModelInMsg     = regexp.MustCompile(`(?i)(?:\[request details\]\s*)?model:\s*(\S+)`)
	reBackendInMsg   = regexp.MustCompile(`(?i)(?:backend|selected_backend|cached_backend):\s*(\S+)`)
)

func parseFlexibleLogTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05.999999999Z0700",
		"2006-01-02T15:04:05.000-07:00",
	}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

func normalizeZapTimestamp(s string) string {
	if t, err := parseFlexibleLogTime(s); err == nil {
		return t.Format(time.RFC3339Nano)
	}
	return s
}

func stringField(raw map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func intField(raw map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		switch v := raw[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return 0
}

func int64Field(raw map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		switch v := raw[k].(type) {
		case float64:
			return int64(v)
		case int:
			return int64(v)
		case int64:
			return v
		}
	}
	return 0
}

func applyTailFields(entry *LogEntry, fields map[string]string) {
	if entry == nil || len(fields) == 0 {
		return
	}
	if entry.RequestID == "" {
		entry.RequestID = fields["request_id"]
	}
	if entry.BackendID == "" {
		entry.BackendID = firstNonEmpty(fields["backend_id"], fields["backend"], fields["selected_backend"], fields["cached_backend"])
	}
	if entry.Model == "" {
		entry.Model = firstNonEmpty(fields["model"], fields["requested_model"])
	}
	if entry.ClientIP == "" {
		entry.ClientIP = fields["client_ip"]
	}
	if entry.Strategy == "" {
		entry.Strategy = fields["strategy"]
	}
	if entry.DurationMs == 0 {
		if v := fields["duration_ms"]; v != "" {
			if n, err := parseInt64Loose(v); err == nil {
				entry.DurationMs = n
			}
		} else if v := fields["latency_ms"]; v != "" {
			if n, err := parseInt64Loose(v); err == nil {
				entry.DurationMs = n
			}
		} else if v := fields["total_latency_ms"]; v != "" {
			if n, err := parseInt64Loose(v); err == nil {
				entry.DurationMs = n
			}
		}
	}
	if entry.StatusCode == 0 {
		if v := fields["status_code"]; v != "" {
			if n, err := parseInt64Loose(v); err == nil {
				entry.StatusCode = int(n)
			}
		} else if v := fields["status"]; v != "" {
			if n, err := parseInt64Loose(v); err == nil {
				entry.StatusCode = int(n)
			}
		}
	}
	if entry.Path == "" {
		entry.Path = fields["path"]
	}
	if entry.ClientIP == "" {
		entry.ClientIP = firstNonEmpty(fields["ip"], fields["client_ip"])
	}
	mergeExtraFields(entry, fields)
}

func mergeExtraFields(entry *LogEntry, fields map[string]string) {
	if entry == nil || len(fields) == 0 {
		return
	}
	if entry.Extra == nil {
		entry.Extra = make(map[string]string)
	}
	for k, v := range fields {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, structured := logEntryStructuredKeys[k]; structured {
			continue
		}
		entry.Extra[k] = v
	}
	if len(entry.Extra) == 0 {
		entry.Extra = nil
	}
}

func mergeExtraFieldsFromRaw(entry *LogEntry, raw map[string]interface{}) {
	if entry == nil || len(raw) == 0 {
		return
	}
	fields := make(map[string]string)
	for k, v := range raw {
		if _, structured := logEntryStructuredKeys[k]; structured {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			continue
		}
		fields[k] = s
	}
	mergeExtraFields(entry, fields)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseInt64Loose(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseCompactConsoleTail(rest string) (string, map[string]string) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", nil
	}
	idx := strings.Index(rest, "{")
	if idx < 0 {
		return rest, nil
	}
	message := strings.TrimSpace(rest[:idx])
	tail := strings.TrimSpace(rest[idx:])
	if tail == "" {
		return message, nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(tail), &raw); err != nil {
		return message, nil
	}
	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		fields[k] = fmt.Sprint(v)
	}
	return message, fields
}

func parseConsoleTailFields(msg string) (string, map[string]string) {
	parts := strings.Split(msg, "\t")
	if len(parts) <= 1 {
		return msg, nil
	}
	fields := make(map[string]string)
	for _, p := range parts[1:] {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "{") {
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(p), &raw); err == nil {
				for k, v := range raw {
					fields[k] = fmt.Sprint(v)
				}
				continue
			}
		}
		if idx := strings.Index(p, "="); idx > 0 {
			fields[strings.TrimSpace(p[:idx])] = strings.TrimSpace(p[idx+1:])
		}
	}
	return parts[0], fields
}

func enrichLogEntry(entry *LogEntry) {
	if entry == nil {
		return
	}
	if entry.RequestID == "" {
		if m := reRequestIDInMsg.FindStringSubmatch(entry.Message); len(m) == 2 {
			entry.RequestID = strings.TrimRight(m[1], "|")
		}
	}
	if entry.ClientIP == "" {
		if m := reClientIPInMsg.FindStringSubmatch(entry.Message); len(m) == 2 {
			entry.ClientIP = strings.TrimRight(m[1], "|")
		}
	}
	if entry.Model == "" {
		if m := reModelInMsg.FindStringSubmatch(entry.Message); len(m) == 2 {
			entry.Model = strings.TrimRight(m[1], "|")
		}
	}
	if entry.BackendID == "" {
		if m := reBackendInMsg.FindStringSubmatch(entry.Message); len(m) == 2 {
			entry.BackendID = strings.TrimRight(strings.Trim(m[1], `"`), "|")
		}
	}
}

func logEntryFromZapMap(raw map[string]interface{}) LogEntry {
	e := LogEntry{Level: "info"}
	if v := stringField(raw, "timestamp"); v != "" {
		e.Timestamp = normalizeZapTimestamp(v)
	}
	if v := stringField(raw, "level"); v != "" {
		e.Level = strings.ToLower(v)
	}
	if v := stringField(raw, "message", "msg"); v != "" {
		e.Message = v
	}
	if v := stringField(raw, "caller"); v != "" {
		e.Caller = v
	}
	e.RequestID = stringField(raw, "request_id")
	e.BackendID = firstNonEmpty(stringField(raw, "backend_id"), stringField(raw, "backend"), stringField(raw, "selected_backend"))
	e.BackendType = stringField(raw, "backend_type")
	e.Model = firstNonEmpty(stringField(raw, "model"), stringField(raw, "requested_model"))
	e.Strategy = stringField(raw, "strategy")
	e.ClientIP = stringField(raw, "client_ip")
	e.Path = stringField(raw, "path")
	e.APIKeyPrefix = stringField(raw, "api_key_prefix")
	if e.StatusCode == 0 {
		e.StatusCode = intField(raw, "status")
	}
	e.StatusCode = intField(raw, "status_code")
	e.DurationMs = int64Field(raw, "duration_ms", "latency_ms", "total_latency_ms")
	if uid := int64Field(raw, "user_id"); uid != 0 {
		e.UserID = uid
	}
	mergeExtraFieldsFromRaw(&e, raw)
	enrichLogEntry(&e)
	return e
}

// parseLogLine 解析日志行（JSON 一行、zap 控制台一行、或其它纯文本）
func parseLogLine(line string) (LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return LogEntry{}, fmt.Errorf("empty line")
	}

	if strings.HasPrefix(line, "{") {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			e := logEntryFromZapMap(raw)
			if e.Message != "" || e.Timestamp != "" {
				return e, nil
			}
		}
	}

	if m := zapConsoleLogLine.FindStringSubmatch(line); len(m) == 5 {
		message, tailFields := parseConsoleTailFields(m[4])
		entry := LogEntry{
			Timestamp: normalizeZapTimestamp(m[1]),
			Level:     strings.ToLower(m[2]),
			Caller:    m[3],
			Message:   message,
		}
		applyTailFields(&entry, tailFields)
		enrichLogEntry(&entry)
		return entry, nil
	}

	if m := zapCompactConsoleLogLine.FindStringSubmatch(line); len(m) == 5 {
		message, tailFields := parseCompactConsoleTail(m[4])
		entry := LogEntry{
			Timestamp: normalizeZapTimestamp(m[1]),
			Level:     strings.ToLower(m[2]),
			Caller:    m[3],
			Message:   message,
		}
		applyTailFields(&entry, tailFields)
		enrichLogEntry(&entry)
		return entry, nil
	}

	// 无法解析时间：不参与按时间筛选的排除（matchesTimeRange 对空时间戳返回 true）
	entry := LogEntry{
		Timestamp: "",
		Level:     "info",
		Message:   line,
	}
	enrichLogEntry(&entry)
	return entry, nil
}

type requestLogContext struct {
	RequestID  string
	Model      string
	BackendID  string
	ClientIP   string
	StatusCode int
	DurationMs int64
}

var httpAccessPathInMessage = regexp.MustCompile(`"path"\s*:\s*"([^"]+)"`)

func isHTTPAccessLogEntry(entry *LogEntry) bool {
	msg := strings.TrimSpace(entry.Message)
	lower := strings.ToLower(msg)
	if lower == "request" {
		return true
	}
	// zap console / sugared 格式：request {"method":"GET","path":"/api/..."}
	if strings.HasPrefix(lower, "request ") && (strings.Contains(msg, `"path"`) || strings.Contains(msg, `"method"`)) {
		return true
	}
	if strings.HasPrefix(lower, "request{") || strings.Contains(msg, `request{"method"`) || strings.Contains(msg, `request {"method"`) {
		return strings.Contains(msg, `"path"`) || strings.Contains(msg, `"method"`) || strings.TrimSpace(entry.Path) != ""
	}
	return false
}

func logHTTPPath(entry LogEntry) string {
	if p := strings.TrimSpace(entry.Path); p != "" {
		return p
	}
	if m := httpAccessPathInMessage.FindStringSubmatch(entry.Message); len(m) == 2 {
		return m[1]
	}
	return ""
}

func isLLMProxyHTTPPath(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	if path == "" {
		return false
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	llmPaths := []string{
		"/v1/chat/completions",
		"/v1/messages",
		"/v1/completions",
		"/v1/embeddings",
		"/v1/models",
		"/v1/backends",
		"/api/v1/openai/",
	}
	for _, p := range llmPaths {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

func isRequestStartMessage(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "[chat completions]") && strings.Contains(lower, "request started") ||
		strings.Contains(lower, "request started")
}

func mergeLogIntoContext(e *LogEntry, ctx *requestLogContext) {
	if e.RequestID != "" {
		ctx.RequestID = e.RequestID
	}
	// 避免启动/初始化日志（如 QA splitter 的 ollama 配置）污染无 request_id 的行。
	if e.RequestID == "" && ctx.RequestID == "" {
		return
	}
	if e.Model != "" {
		ctx.Model = e.Model
	}
	if e.BackendID != "" {
		ctx.BackendID = e.BackendID
	}
	if e.ClientIP != "" {
		ctx.ClientIP = e.ClientIP
	}
	if e.StatusCode != 0 {
		ctx.StatusCode = e.StatusCode
	}
	if e.DurationMs != 0 {
		ctx.DurationMs = e.DurationMs
	}
}

func applyContextToEntry(e *LogEntry, ctx *requestLogContext) {
	if e.RequestID == "" && ctx.RequestID != "" {
		e.RequestID = ctx.RequestID
	}
	if e.Model == "" && ctx.Model != "" {
		e.Model = ctx.Model
	}
	if e.BackendID == "" && ctx.BackendID != "" {
		e.BackendID = ctx.BackendID
	}
	if e.ClientIP == "" && ctx.ClientIP != "" {
		e.ClientIP = ctx.ClientIP
	}
}

func isRequestEndMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "[request] completed") ||
		strings.Contains(msg, "策略决策流程结束") ||
		strings.Contains(msg, "后端响应接收")
}

// propagateRequestContext fills request_id/model/backend on lines that belong to the
// same request but were logged without structured fields (legacy Infof, strategy steps).
func propagateRequestContext(logs []LogEntry) {
	if len(logs) == 0 {
		return
	}
	order := make([]int, len(logs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		ti, _ := parseFlexibleLogTime(logs[order[i]].Timestamp)
		tj, _ := parseFlexibleLogTime(logs[order[j]].Timestamp)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return order[i] < order[j]
	})

	var ctx requestLogContext
	for _, idx := range order {
		e := &logs[idx]
		if isHTTPAccessLogEntry(e) || isSystemNoiseLog(*e) {
			ctx = requestLogContext{}
			continue
		}
		if isRequestStartMessage(e.Message) {
			if e.RequestID != "" && e.RequestID != ctx.RequestID {
				ctx = requestLogContext{RequestID: e.RequestID}
			}
		}
		mergeLogIntoContext(e, &ctx)
		applyContextToEntry(e, &ctx)
		if isRequestEndMessage(e.Message) {
			ctx = requestLogContext{}
		}
	}
}

// matchesTimeRange 检查日志是否在时间范围内
func (h *LogHandler) matchesTimeRange(entry LogEntry, fromTime, toTime time.Time) bool {
	if entry.Timestamp == "" {
		return true // 没有时间戳的日志默认包含
	}

	logTime, err := parseFlexibleLogTime(entry.Timestamp)
	if err != nil {
		return true // 无法解析的时间默认包含
	}

	if fromTime.IsZero() {
		if toTime.IsZero() {
			return true
		}
		return !logTime.After(toTime)
	}
	if toTime.IsZero() {
		return !logTime.Before(fromTime)
	}

	return (logTime.Equal(fromTime) || logTime.After(fromTime)) &&
		(logTime.Equal(toTime) || logTime.Before(toTime))
}

// filterHaystack 用于后端/模型等筛选：实际日志多为 zap 文本或仅 message 中含 ID，不仅靠结构化字段。
func filterHaystack(entry LogEntry) string {
	var b strings.Builder
	b.WriteString(entry.BackendID)
	b.WriteByte(' ')
	b.WriteString(entry.BackendType)
	b.WriteByte(' ')
	b.WriteString(entry.Model)
	b.WriteByte(' ')
	b.WriteString(entry.Strategy)
	b.WriteByte(' ')
	b.WriteString(entry.Message)
	b.WriteByte(' ')
	b.WriteString(entry.Caller)
	b.WriteByte(' ')
	b.WriteString(entry.RequestID)
	b.WriteByte(' ')
	b.WriteString(entry.ClientIP)
	b.WriteByte(' ')
	b.WriteString(entry.APIKeyPrefix)
	b.WriteByte(' ')
	b.WriteString(fmt.Sprint(entry.StatusCode))
	b.WriteByte(' ')
	b.WriteString(fmt.Sprint(entry.DurationMs))
	return b.String()
}

func isSystemNoiseLog(entry LogEntry) bool {
	msg := strings.ToLower(entry.Message)
	noise := []string{
		"bootstrap:", "product edition:", "listening on", "server started",
		"starting server", "starting host proxy", "host proxy server started",
		"host proxy is disabled", "host proxy initialized", "host proxy enabled",
		"mode dispatcher initialized", "defaultpipelineresolver injected",
		"defaultpipelineresolver initialized", "proxy mode manager initialized",
		"proxy mode middleware registered", "migration", "sqlite", "postgresql",
		"gin mode", "health/ready", "wails", "desktop shell",
		"semantic cache", "exact match cache", "cache manager initialized",
		"cache strategies registered", "storage config loaded", "embedding service",
		"embedding:", "no vector store", "no default kv store", "语义缓存初始化",
		"loaded backend configs", "已从数据库加载", "plugin registry", "plugin marketplace",
		"business plugin", "builtin nodes registered", "pipeline templates loaded",
		"pipelines from store", "capabilitybroker", "evaluation manager",
		"agent memory", "saveonly mode", "qa splitter", "ca certificate",
		"auth/refresh", "plugin security validator", "listenandserve failed",
		"server error:", "server listen",
		"[handler]", "handler] modedispatcher", "handler] defaultpipelineresolver",
	}
	for _, n := range noise {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func matchesLLMServiceCategory(entry LogEntry) bool {
	if isSystemNoiseLog(entry) {
		return false
	}
	if isHTTPAccessLogEntry(&entry) {
		return isLLMProxyHTTPPath(logHTTPPath(entry))
	}
	msg := strings.ToLower(entry.Message)
	markers := []string{
		"[chat completions]", "request started", "[request]", "[config] proxy mode",
		"chat completions", "[request] details", "[request] completed",
		"策略决策", "请求转发", "后端响应", "策略错误", "策略降级",
		"/v1/chat", "/v1/messages", "/v1/completions", "/v1/embeddings",
		"transparent proxy", "cache hit mode", "cache mode", "proxy auth rejected",
		"无效的 api key", "需要认证", "requested_model", "selected_backend",
		"后端选择", "缓存命中", "缓存未命中",
	}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	// 仅有 model/backend 字段（如语义缓存初始化）不足以判定为大模型代理请求。
	if strings.TrimSpace(entry.RequestID) != "" {
		return true
	}
	return false
}

func normalizeLogCategory(cat string) string {
	c := strings.ToLower(strings.TrimSpace(cat))
	switch c {
	case "llm", "api", "proxy", "service":
		return "llm"
	case "system", "other", "infra":
		return "system"
	default:
		return c
	}
}

func levelMatchesFilter(entryLevel, filter string) bool {
	if filter == "" {
		return true
	}
	el := strings.ToLower(strings.TrimSpace(entryLevel))
	fl := strings.ToLower(strings.TrimSpace(filter))
	if fl == "warn" {
		return el == "warn" || el == "warning"
	}
	return el == fl
}

// matchesFilters 检查日志是否匹配筛选条件
func (h *LogHandler) matchesFilters(entry LogEntry, req LogQueryRequest) bool {
	// 用户 ID 筛选（仅当请求显式带非空 user_id 时生效）
	if want := strings.TrimSpace(req.UserID); want != "" {
		if entry.UserID == 0 {
			return false
		}
		if fmt.Sprint(entry.UserID) != want {
			return false
		}
	}

	hay := strings.ToLower(filterHaystack(entry))

	// 后端 ID 筛选（结构化字段 + 全文，匹配如 ID: bigmodel、后端名等）
	if q := strings.TrimSpace(req.BackendID); q != "" {
		if !strings.Contains(hay, strings.ToLower(q)) {
			return false
		}
	}

	// 模型筛选（结构化字段 + 全文）
	if q := strings.TrimSpace(req.Model); q != "" {
		if !strings.Contains(hay, strings.ToLower(q)) {
			return false
		}
	}

	// 策略筛选
	if q := strings.TrimSpace(req.Strategy); q != "" {
		if !strings.Contains(hay, strings.ToLower(q)) {
			return false
		}
	}

	// 日志级别筛选（兼容 zap 的 warning / warn）
	if req.Level != "" && !levelMatchesFilter(entry.Level, req.Level) {
		return false
	}

	// 请求 ID 筛选
	if q := strings.TrimSpace(req.RequestID); q != "" {
		rq := strings.ToLower(q)
		rid := strings.ToLower(strings.TrimSpace(entry.RequestID))
		if rid != "" {
			if !strings.Contains(rid, rq) {
				return false
			}
		} else if !strings.Contains(hay, rq) {
			return false
		}
	}

	// 全文关键词
	if q := strings.TrimSpace(req.Q); q != "" {
		if !strings.Contains(hay, strings.ToLower(q)) {
			return false
		}
	}

	// 预设分类
	switch normalizeLogCategory(req.Category) {
	case "llm":
		if !matchesLLMServiceCategory(entry) {
			return false
		}
	case "system":
		if matchesLLMServiceCategory(entry) {
			return false
		}
	}

	return true
}

// ExportLogs 导出日志
func (h *LogHandler) ExportLogs(c *gin.Context) {
	var req struct {
		From      string `json:"from"`
		To        string `json:"to"`
		UserID    string `json:"user_id"`
		Backend   string `json:"backend_id"`
		Model     string `json:"model"`
		Strategy  string `json:"strategy"`
		Level     string `json:"level"`
		Q         string `json:"q"`
		RequestID string `json:"request_id"`
		Category  string `json:"category"`
		Format    string `json:"format"` // json, csv, txt
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// 默认格式
	if req.Format == "" {
		req.Format = "json"
	}

	fromTime, toTime, err := parseLogQueryTimeRange(req.From, req.To)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 构建筛选条件
	queryReq := LogQueryRequest{
		From:      req.From,
		To:        req.To,
		UserID:    req.UserID,
		BackendID: req.Backend,
		Model:     req.Model,
		Strategy:  req.Strategy,
		Level:     req.Level,
		Q:         req.Q,
		RequestID: req.RequestID,
		Category:  req.Category,
		Limit:     0, // 0 = 全部
		Page:      1,
	}

	// 读取所有匹配的日志
	logs, _, err := h.readAndFilterLogs(fromTime, toTime, queryReq, 1, 0)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to read logs: "+err.Error())
		return
	}

	// 根据格式生成文件内容
	var content []byte
	var filename string
	var contentType string

	switch req.Format {
	case "csv":
		content, filename, contentType = h.exportCSV(logs)
	case "txt":
		content, filename, contentType = h.exportTXT(logs)
	default: // json
		content, filename, contentType = h.exportJSON(logs)
	}

	// 设置响应头
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, contentType, content)
}

// exportJSON 导出为 JSON 格式
func (h *LogHandler) exportJSON(logs []LogEntry) ([]byte, string, string) {
	data, _ := json.MarshalIndent(logs, "", "  ")
	filename := fmt.Sprintf("logs_%s.json", time.Now().Format("20060102_150405"))
	return data, filename, "application/json"
}

// exportCSV 导出为 CSV 格式
func (h *LogHandler) exportCSV(logs []LogEntry) ([]byte, string, string) {
	var sb strings.Builder
	sb.WriteString("timestamp,level,request_id,user_id,backend_id,model,strategy,duration_ms,status_code,message\n")

	for _, log := range logs {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%d,%s,%s,%s,%d,%d,\"%s\"\n",
			log.Timestamp,
			log.Level,
			log.RequestID,
			log.UserID,
			log.BackendID,
			log.Model,
			log.Strategy,
			log.DurationMs,
			log.StatusCode,
			strings.ReplaceAll(log.Message, "\"", "\"\""),
		))
	}

	filename := fmt.Sprintf("logs_%s.csv", time.Now().Format("20060102_150405"))
	return []byte(sb.String()), filename, "text/csv"
}

// exportTXT 导出为 TXT 格式
func (h *LogHandler) exportTXT(logs []LogEntry) ([]byte, string, string) {
	var sb strings.Builder
	for _, log := range logs {
		sb.WriteString(fmt.Sprintf("[%s] [%s] %s\n", log.Timestamp, log.Level, log.Message))
	}

	filename := fmt.Sprintf("logs_%s.txt", time.Now().Format("20060102_150405"))
	return []byte(sb.String()), filename, "text/plain"
}

// StreamLogs SSE 实时日志流
func (h *LogHandler) StreamLogs(c *gin.Context) {
	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 解析筛选参数
	var streamReq LogQueryRequest
	if err := c.ShouldBindQuery(&streamReq); err != nil {
		c.SSEvent("error", "Invalid query parameters: "+err.Error())
		return
	}

	// 打开日志文件
	file, err := os.Open(h.logPath)
	if err != nil {
		c.SSEvent("error", "Failed to open log file")
		return
	}
	defer file.Close()

	// 移动到文件末尾
	file.Seek(0, 2)

	// 发送初始消息
	c.SSEvent("connected", "Log stream connected")
	c.Writer.Flush()

	// 持续读取新日志
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// 读取新行
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}

				entry, _ := parseLogLine(line)

				// 应用筛选
				if !h.matchesFilters(entry, streamReq) {
					continue
				}

				c.SSEvent("log", entry)
				c.Writer.Flush()
			}
		}
	}
}

// GetLogStats 获取日志统计信息（支持与列表相同的 from/to/category 筛选；未传 from 时默认最近 24 小时）
func (h *LogHandler) GetLogStats(c *gin.Context) {
	var req LogQueryRequest
	_ = c.ShouldBindQuery(&req)

	fromTime := time.Now().Add(-24 * time.Hour)
	toTime := time.Now()
	if strings.TrimSpace(req.From) != "" || strings.TrimSpace(req.To) != "" {
		var err error
		fromTime, toTime, err = parseLogQueryTimeRange(req.From, req.To)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	logFiles, err := h.findLogFiles()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to find log files: "+err.Error())
		return
	}

	stats := LogStats{
		TotalLogs:     0,
		ErrorCount:    0,
		WarnCount:     0,
		InfoCount:     0,
		DebugCount:    0,
		BackendStats:  make(map[string]int),
		ModelStats:    make(map[string]int),
		HourlyStats:   make(map[string]int),
	}

	for _, logFile := range logFiles {
		fileStats, err := h.collectFileStats(logFile, fromTime, toTime, req)
		if err != nil {
			continue
		}
		stats.TotalLogs += fileStats.TotalLogs
		stats.ErrorCount += fileStats.ErrorCount
		stats.WarnCount += fileStats.WarnCount
		stats.InfoCount += fileStats.InfoCount
		stats.DebugCount += fileStats.DebugCount

		for k, v := range fileStats.BackendStats {
			stats.BackendStats[k] += v
		}
		for k, v := range fileStats.ModelStats {
			stats.ModelStats[k] += v
		}
		for k, v := range fileStats.HourlyStats {
			stats.HourlyStats[k] += v
		}
	}

	if w := h.stdoutOnlyWarning(); w != "" {
		stats.Warning = w
	}

	RespondSuccess(c, stats)
}

// LogStats 日志统计信息
type LogStats struct {
	TotalLogs    int            `json:"total_logs"`
	ErrorCount   int            `json:"error_count"`
	WarnCount    int            `json:"warn_count"`
	InfoCount    int            `json:"info_count"`
	DebugCount   int            `json:"debug_count"`
	BackendStats map[string]int `json:"backend_stats"`
	ModelStats   map[string]int `json:"model_stats"`
	HourlyStats  map[string]int `json:"hourly_stats"`
	// Warning 与列表接口一致：stdout 模式下文件与终端不同步时的说明
	Warning string `json:"warning,omitempty"`
}

// collectFileStats 收集单个文件的统计信息（与列表接口共用时间/分类筛选）
func (h *LogHandler) collectFileStats(logFile string, fromTime, toTime time.Time, req LogQueryRequest) (LogStats, error) {
	stats := LogStats{
		BackendStats: make(map[string]int),
		ModelStats:   make(map[string]int),
		HourlyStats:  make(map[string]int),
	}

	file, err := os.Open(logFile)
	if err != nil {
		return stats, err
	}
	defer file.Close()

	var reader io.Reader = file
	if strings.HasSuffix(logFile, ".gz") {
		return stats, nil
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry, err := parseLogLine(line)
		if err != nil {
			continue
		}

		// 时间范围检查
		if !h.matchesTimeRange(entry, fromTime, toTime) {
			continue
		}
		if !h.matchesFilters(entry, req) {
			continue
		}

		stats.TotalLogs++

		// 按级别统计
		switch strings.ToLower(entry.Level) {
		case "error":
			stats.ErrorCount++
		case "warn", "warning":
			stats.WarnCount++
		case "info":
			stats.InfoCount++
		case "debug":
			stats.DebugCount++
		}

		// 按后端统计
		if entry.BackendID != "" {
			stats.BackendStats[entry.BackendID]++
		}

		// 按模型统计
		if entry.Model != "" {
			stats.ModelStats[entry.Model]++
		}

		// 按小时统计
		if len(entry.Timestamp) >= 13 {
			hour := entry.Timestamp[:13] // YYYY-MM-DDTHH
			stats.HourlyStats[hour]++
		}
	}

	return stats, scanner.Err()
}

// ClearLogs 清空磁盘上的日志文件（主文件截断，轮转文件删除）。
func (h *LogHandler) ClearLogs(c *gin.Context) {
	// 先刷盘，避免截断后 zap/lumberjack 缓冲区把旧内容写回文件。
	_ = logger.Sync()

	files, err := h.findLogFiles()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "Failed to list log files: "+err.Error())
		return
	}

	cleared := 0
	var errs []string
	for _, path := range files {
		if path == h.logPath {
			if err := os.Truncate(path, 0); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", path, err))
				continue
			}
		} else if err := os.Remove(path); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		cleared++
	}

	// 截断可能触发 lumberjack 轮转，再清一次残留轮转文件并确保主文件为空。
	if remaining, err := h.findLogFiles(); err == nil {
		for _, path := range remaining {
			if path == h.logPath {
				if err := os.Truncate(path, 0); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", path, err))
				}
				continue
			}
			if err := os.Remove(path); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", path, err))
			}
		}
	}

	if len(errs) > 0 {
		RespondError(c, http.StatusInternalServerError, strings.Join(errs, "; "))
		return
	}

	resp := gin.H{
		"cleared_files": cleared,
		"log_path":      h.logPath,
		"message":       fmt.Sprintf("已清空 %d 个日志文件", cleared),
	}
	if w := h.stdoutOnlyWarning(); w != "" {
		resp["warning"] = w
	}
	RespondSuccess(c, resp)
}

// TailLogs 轻量日志尾部读取，供桌面版日志边栏实时轮询使用。
// GET /api/v1/logs/tail?offset=<bytes>&tail=<true|false>&limit=<bytes>
//   - offset=0&tail=true：返回文件末尾最近 32KB 内容（首次打开）
//   - offset>0：从该偏移量读取新增内容
// 返回格式化后的可读文本（JSON 行被解析为 "时间 [级别] 消息" 格式）。
func (h *LogHandler) TailLogs(c *gin.Context) {
	path := h.logPath
	if path == "" {
		RespondError(c, http.StatusServiceUnavailable, "log path not configured")
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		RespondSuccess(c, gin.H{
			"content":   "",
			"offset":    0,
			"eof":       true,
			"file_size": 0,
			"log_path":  path,
			"warning":   h.stdoutOnlyWarning(),
		})
		return
	}
	size := info.Size()

	offsetStr := c.Query("offset")
	var offset int64
	if offsetStr != "" {
		fmt.Sscanf(offsetStr, "%d", &offset)
	}
	tail := c.Query("tail") == "true"
	limit := int64(64 * 1024)
	if l := c.Query("limit"); l != "" {
		var n int64
		if _, err := fmt.Sscanf(l, "%d", &n); err == nil && n > 0 && n < 1024*1024 {
			limit = n
		}
	}

	// 首次打开且请求尾部：定位到末尾前 32KB
	if offset == 0 && tail {
		if size > 32*1024 {
			offset = size - 32*1024
		} else {
			offset = 0
		}
	}

	// 文件被截断或轮转：重置到 0
	if offset > size {
		offset = 0
	}

	if offset >= size {
		RespondSuccess(c, gin.H{
			"content":   "",
			"offset":    offset,
			"eof":       true,
			"file_size": size,
			"log_path":  path,
		})
		return
	}

	f, err := os.Open(path)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to open log: "+err.Error())
		return
	}
	defer f.Close()

	if _, err := f.Seek(offset, 0); err != nil {
		RespondError(c, http.StatusInternalServerError, "failed to seek log: "+err.Error())
		return
	}

	buf := make([]byte, limit)
	n, _ := f.Read(buf)
	raw := string(buf[:n])

	// 首次 tail 时，第一行可能是不完整的（从文件中间截断），跳过到第一个换行符
	if offset == 0 && tail && n > 0 {
		if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
			skipped := int64(idx + 1)
			offset += skipped
			raw = raw[skipped:]
			n -= int(skipped)
		}
	}

	newOffset := offset + int64(n)
	formatted := formatTailLogLines(raw)

	resp := gin.H{
		"content":   formatted,
		"offset":    newOffset,
		"eof":       newOffset >= size,
		"file_size": size,
		"log_path":  path,
	}
	if w := h.stdoutOnlyWarning(); w != "" {
		resp["warning"] = w
	}
	RespondSuccess(c, resp)
}

// formatTailLogLines 将 JSON 格式的日志行格式化为可读文本。
// 每行格式：2006-01-02 15:04:05 [LEVEL] message
func formatTailLogLines(raw string) string {
	lines := strings.Split(raw, "\n")
	var builder strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// 尝试解析为 JSON
		var entry map[string]interface{}
		if json.Unmarshal([]byte(line), &entry) == nil {
			ts, _ := entry["timestamp"].(string)
			level, _ := entry["level"].(string)
			if level == "" {
				level, _ = entry["lvl"].(string)
			}
			msg, _ := entry["message"].(string)
			if msg == "" {
				msg, _ = entry["msg"].(string)
			}
			caller, _ := entry["caller"].(string)

			// 格式化时间：截取到毫秒
			short := ts
			if len(ts) >= 19 {
				// 2006-01-02T15:04:05 -> 2006-01-02 15:04:05
				short = strings.Replace(ts[:19], "T", " ", 1)
				if len(ts) >= 23 {
					short += ts[19:23] // 毫秒
				}
			}

			builder.WriteString(short)
			builder.WriteString(" [")
			if level != "" {
				builder.WriteString(strings.ToUpper(level))
			} else {
				builder.WriteString("INFO")
			}
			builder.WriteString("] ")
			if caller != "" {
				builder.WriteString(caller)
				builder.WriteString(" ")
			}
			builder.WriteString(msg)

			// 附加关键字段（非结构化键）
			for _, k := range []string{"request_id", "model", "backend_id", "strategy", "duration_ms", "status_code"} {
				if v, ok := entry[k]; ok && v != nil {
					builder.WriteString(" ")
					builder.WriteString(k)
					builder.WriteString("=")
					builder.WriteString(fmt.Sprintf("%v", v))
				}
			}
			builder.WriteString("\n")
		} else {
			// 非 JSON 行原样保留
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
