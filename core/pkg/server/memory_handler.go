package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"centag/core/pkg/agentmemory"
	"centag/core/internal/auth"
	"centag/core/pkg/embedding"
	"centag/core/pkg/logger"
	"centag/core/pkg/storage"

	"github.com/gin-gonic/gin"
)

// MemoryHandler 记忆服务处理器
type MemoryHandler struct {
	vectorStore  storage.VectorStore // 磁盘模式下的语义缓存向量（与 agent_memory_chunks 隔离）
	embeddingSvc embedding.EmbeddingService
	storeRoot    string
	ollamaURL    string
	httpClient   *http.Client
	// agentMem 非 nil 且库表已迁移时：正文与（PostgreSQL）向量走应用数据库，取代本地 memory-store 文件。
	agentMem *agentmemory.Service
	// 最小异步索引队列：仅 DB+PostgreSQL 模式生效。
	indexQueue chan memoryIndexTask
	// 队列可观测指标
	indexProcessed atomic.Int64
	indexFailed    atomic.Int64
	indexDropped   atomic.Int64
	indexErrMu     sync.RWMutex
	indexLastError string
}

type memoryIndexTask struct {
	userID  string
	agentID string
	path    string
}

// NewMemoryHandler 创建记忆处理器；agentMem 由 server 在 database.Get().GetDB() 可用时注入。
func NewMemoryHandler(vectorStore storage.VectorStore, embeddingSvc embedding.EmbeddingService, storeRoot string, agentMem *agentmemory.Service) *MemoryHandler {
	h := &MemoryHandler{
		vectorStore:  vectorStore,
		embeddingSvc: embeddingSvc,
		storeRoot:    storeRoot,
		ollamaURL:    "http://ollama:21434",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		agentMem:     agentMem,
	}
	if h.useAgentDB() && h.agentMem.Postgres() {
		h.indexQueue = make(chan memoryIndexTask, 512)
		go h.runAsyncIndexer()
	}

	return h
}

func (h *MemoryHandler) useAgentDB() bool {
	return h.agentMem != nil && h.agentMem.Enabled()
}

func (h *MemoryHandler) supportsAsyncIndex() bool {
	return h.indexQueue != nil
}

func (h *MemoryHandler) enqueueAsyncIndex(userID, agentID, path string) bool {
	if !h.supportsAsyncIndex() {
		return false
	}
	select {
	case h.indexQueue <- memoryIndexTask{userID: userID, agentID: agentID, path: path}:
		return true
	default:
		h.indexDropped.Add(1)
		return false
	}
}

func (h *MemoryHandler) runAsyncIndexer() {
	for task := range h.indexQueue {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		_, err := h.agentMem.ReindexDoc(ctx, task.userID, task.agentID, task.path)
		cancel()
		if err != nil {
			h.indexFailed.Add(1)
			h.setIndexLastError(err.Error())
			logger.Warnf("[memory] async reindex failed user=%s agent=%s path=%s err=%v", task.userID, task.agentID, task.path, err)
		} else {
			h.indexProcessed.Add(1)
		}
	}
}

func (h *MemoryHandler) setIndexLastError(msg string) {
	h.indexErrMu.Lock()
	h.indexLastError = msg
	h.indexErrMu.Unlock()
}

func (h *MemoryHandler) getIndexLastError() string {
	h.indexErrMu.RLock()
	defer h.indexErrMu.RUnlock()
	return h.indexLastError
}

type memoryIndexQueueMetrics struct {
	Enabled   bool
	Length    int
	Processed int64
	Failed    int64
	Dropped   int64
	LastError string
}

func (h *MemoryHandler) getIndexQueueMetrics() memoryIndexQueueMetrics {
	m := memoryIndexQueueMetrics{
		Enabled:   h.supportsAsyncIndex(),
		Processed: h.indexProcessed.Load(),
		Failed:    h.indexFailed.Load(),
		Dropped:   h.indexDropped.Load(),
		LastError: h.getIndexLastError(),
	}
	if h.indexQueue != nil {
		m.Length = len(h.indexQueue)
	}
	return m
}

// getUserID 从上下文获取用户 ID（目录名与向量 metadata 使用字符串）。
// ProxyAuthMiddleware / JWT 写入的 CtxKeyUserID 为 int64，不可断言为 string，否则会 panic → 500 Internal server error。
func (h *MemoryHandler) getUserID(c *gin.Context) string {
	v, exists := c.Get(auth.CtxKeyUserID)
	if !exists || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case int64:
		if x == 0 {
			return ""
		}
		return strconv.FormatInt(x, 10)
	case int:
		if x == 0 {
			return ""
		}
		return strconv.Itoa(x)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		// JSON 数字偶发以 float64 进上下文
		return strconv.FormatInt(int64(x), 10)
	default:
		return fmt.Sprint(x)
	}
}

// getUserAgentDir 获取用户+agent 的存储目录
func (h *MemoryHandler) getUserAgentDir(c *gin.Context, agentID string) string {
	userID := h.getUserID(c)
	if userID == "" {
		return ""
	}
	if agentID == "" {
		agentID = "main"
	}
	return filepath.Join(h.storeRoot, userID, agentID)
}

// isSafeMemoryRelPath 拒绝路径穿越与绝对路径（DB 模式与磁盘模式共用）。
func isSafeMemoryRelPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if p == "" || p == "." {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	if vol := filepath.VolumeName(p); vol != "" {
		return false
	}
	return true
}

// isPathWithinBase 校验目标路径是否严格位于 baseDir 内。
func isPathWithinBase(baseDir, targetPath string) bool {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func normalizeSyncMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "incremental" {
		return "incremental"
	}
	return "full"
}

func calcSyncAction(mode string, exists bool, currentContent, incomingContent []byte) string {
	if !exists {
		return "new"
	}
	if mode == "incremental" && bytes.Equal(currentContent, incomingContent) {
		return "skipped"
	}
	return "updated"
}

// listMemoryMarkdownRelPaths 枚举记忆根目录下全部 .md，路径相对于 userAgentDir（正斜杠，与 Put/Get 的 path 一致）。
func listMemoryMarkdownRelPaths(userAgentDir string) ([]string, error) {
	fi, err := os.Stat(userAgentDir)
	if err != nil || !fi.IsDir() {
		return nil, err
	}
	var out []string
	err = filepath.WalkDir(userAgentDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, e := filepath.Rel(userAgentDir, p)
		if e != nil {
			return e
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// searchMemoryKeyword 无向量库或向量无命中时，在磁盘 .md 中做简单子串匹配（不调用 embedding）。
func (h *MemoryHandler) searchMemoryKeyword(userAgentDir, query string, limit int) []MemorySearchResult {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" || limit <= 0 {
		return nil
	}
	paths, err := listMemoryMarkdownRelPaths(userAgentDir)
	if err != nil {
		return nil
	}
	var results []MemorySearchResult
	for _, rel := range paths {
		full := filepath.Join(userAgentDir, filepath.FromSlash(rel))
		b, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		lines := strings.Split(string(b), "\n")
		firstLine := -1
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), q) {
				firstLine = i + 1
				break
			}
		}
		if firstLine < 0 && strings.Contains(strings.ToLower(string(b)), q) {
			firstLine = 1
		}
		if firstLine < 0 {
			continue
		}
		lo := firstLine - 2
		if lo < 1 {
			lo = 1
		}
		hi := firstLine + 4
		if hi > len(lines) {
			hi = len(lines)
		}
		snippet := strings.Join(lines[lo-1:hi], "\n")
		runes := []rune(snippet)
		if len(runes) > 2000 {
			snippet = string(runes[:2000])
		}
		results = append(results, MemorySearchResult{
			Path:      rel,
			Content:   snippet,
			Score:     0.55,
			StartLine: lo,
			EndLine:   hi,
		})
		if len(results) >= limit {
			break
		}
	}
	return results
}

// saveVersion 保存文件版本
func (h *MemoryHandler) saveVersion(userID, agentID, filePath, content string) (string, error) {
	versionDir := filepath.Join(h.storeRoot, ".versions", userID, agentID)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return "", err
	}

	// 生成版本 ID（时间戳 + 随机字符串）
	versionID := fmt.Sprintf("%d_%s", time.Now().Unix(), strings.Split(filePath, ".")[0])

	// 版本文件命名: {原始路径}_{版本ID}.md
	safePath := strings.ReplaceAll(filePath, "/", "_")
	versionFile := filepath.Join(versionDir, fmt.Sprintf("%s_%s.md", safePath, versionID))

	if err := os.WriteFile(versionFile, []byte(content), 0644); err != nil {
		return "", err
	}

	return versionID, nil
}

// listVersions 列出文件的所有版本
func (h *MemoryHandler) listVersions(userID, agentID, filePath string) ([]MemoryVersionItem, error) {
	versionDir := filepath.Join(h.storeRoot, ".versions", userID, agentID)

	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return []MemoryVersionItem{}, nil
	}

	safePath := strings.ReplaceAll(filePath, "/", "_")
	prefix := safePath + "_"

	entries, err := os.ReadDir(versionDir)
	if err != nil {
		return nil, err
	}

	versions := []MemoryVersionItem{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".md") {
			info, _ := entry.Info()
			versionID := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), ".md")

			// 读取文件计算行数
			fullPath := filepath.Join(versionDir, entry.Name())
			content, _ := os.ReadFile(fullPath)
			lines := len(strings.Split(string(content), "\n"))

			versions = append(versions, MemoryVersionItem{
				VersionID: versionID,
				CreatedAt: info.ModTime(),
				Size:      int(info.Size()),
				Lines:     lines,
			})
		}
	}

	// 按时间倒序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	return versions, nil
}

// getVersion 读取指定版本
func (h *MemoryHandler) getVersion(userID, agentID, filePath, versionID string) (string, error) {
	versionDir := filepath.Join(h.storeRoot, ".versions", userID, agentID)
	safePath := strings.ReplaceAll(filePath, "/", "_")
	versionFile := filepath.Join(versionDir, fmt.Sprintf("%s_%s.md", safePath, versionID))

	content, err := os.ReadFile(versionFile)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// MemoryPullRequest 增量同步请求（云端→本地）
type MemoryPullRequest struct {
	AgentID  string `json:"agent_id"`
	DestPath string `json:"dest_path"` // 目标本地路径
	Since    int64  `json:"since"`     // 只同步 since 之后的文件（时间戳）
}

// MemoryPullResponse 增量同步响应
type MemoryPullResponse struct {
	Success     bool             `json:"success"`
	Message     string           `json:"message"`
	UserID      string           `json:"user_id"`
	AgentID     string           `json:"agent_id"`
	FilesPulled int              `json:"files_pulled"`
	Details     []PullFileDetail `json:"details"`
}

// PullFileDetail 同步的文件详情
type PullFileDetail struct {
	Path      string `json:"path"`
	Action    string `json:"action"` // "updated" | "new" | "unchanged"
	LocalPath string `json:"local_path"`
}

// SyncFileDetail 同步的文件详情。
type SyncFileDetail struct {
	Path   string `json:"path"`
	Action string `json:"action"` // "new" | "updated" | "skipped"
}

// MemoryIndexRequest 索引请求
type MemoryIndexRequest struct {
	AgentID string `json:"agent_id"`
}

// MemorySearchRequest 搜索请求
type MemorySearchRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	AgentID string `json:"agent_id"`
}

// MemorySearchResult 搜索结果
type MemorySearchResult struct {
	Path      string  `json:"path"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
}

// MemoryGetRequest 读取请求
type MemoryGetRequest struct {
	Path    string `json:"path"`
	From    int    `json:"from"`
	Lines   int    `json:"lines"`
	AgentID string `json:"agent_id"`
}

// MemoryStats 统计信息
type MemoryStats struct {
	AgentID      string   `json:"agent_id"`
	VectorCount  int      `json:"vector_count"`
	IndexedFiles []string `json:"indexed_files"`
}

// MemoryPutRequest 写入请求
type MemoryPutRequest struct {
	Path    string   `json:"path"`
	Content string   `json:"content"`
	AgentID string   `json:"agent_id"`
	Tags    []string `json:"tags,omitempty"`
}

// MemoryAppendRequest 追加请求
type MemoryAppendRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	AgentID string `json:"agent_id"`
}

// MemoryWriteResponse 写入响应
type MemoryWriteResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	Path             string `json:"path"`
	UserID           string `json:"user_id"`
	AgentID          string `json:"agent_id"`
	Vectors          int    `json:"vectors_indexed"`
	IndexQueued      bool   `json:"index_queued,omitempty"`
	IndexQueueLength int    `json:"index_queue_length,omitempty"`
}

// MemorySyncRequest 同步请求 - 从本地工作区同步到云端
type MemorySyncRequest struct {
	AgentID    string `json:"agent_id"`
	SourcePath string `json:"source_path"` // 本地工作区路径
	Mode       string `json:"mode"`        // "full" | "incremental"
}

// MemorySyncResponse 同步响应
type MemorySyncResponse struct {
	Success          bool             `json:"success"`
	Message          string           `json:"message"`
	UserID           string           `json:"user_id"`
	AgentID          string           `json:"agent_id"`
	Mode             string           `json:"mode"`
	FilesSynced      int              `json:"files_synced"`
	FilesNew         int              `json:"files_new"`
	FilesUpdated     int              `json:"files_updated"`
	FilesSkipped     int              `json:"files_skipped"`
	Vectors          int              `json:"vectors_indexed"`
	IndexQueued      int              `json:"index_queued"`
	IndexQueueLength int              `json:"index_queue_length"`
	Details          []string         `json:"details"`
	FileDetails      []SyncFileDetail `json:"file_details"`
}

// MemoryVersionInfo 版本信息
type MemoryVersionInfo struct {
	VersionID string    `json:"version_id"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	AgentID   string    `json:"agent_id"`
}

// MemoryVersionsResponse 版本列表响应
type MemoryVersionsResponse struct {
	Success  bool                `json:"success"`
	Path     string              `json:"path"`
	Versions []MemoryVersionItem `json:"versions"`
}

// MemoryVersionItem 单个版本
type MemoryVersionItem struct {
	VersionID string    `json:"version_id"`
	CreatedAt time.Time `json:"created_at"`
	Size      int       `json:"size"`
	Lines     int       `json:"lines"`
}

// getOllamaEmbedding 获取 Ollama embedding
func (h *MemoryHandler) getOllamaEmbedding(text string) ([]float32, error) {
	// 优先使用配置的 embedding 服务
	if h.embeddingSvc != nil {
		return h.embeddingSvc.GetEmbedding(context.TODO(), text)
	}

	// 回退到直接调用 Ollama
	type EmbedRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}

	reqBody, _ := json.Marshal(EmbedRequest{
		Model:  "bge-m3",
		Prompt: text,
	})

	resp, err := h.httpClient.Post(h.ollamaURL+"/api/embeddings", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama error: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddingData, ok := result["embedding"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid embedding response")
	}

	embedding := make([]float32, len(embeddingData))
	for i, v := range embeddingData {
		embedding[i] = float32(v.(float64))
	}

	return embedding, nil
}

// indexMemoryFile 索引单个文件
func (h *MemoryHandler) indexMemoryFile(userID, agentID, filePath string) (int, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	chunkSize := 10
	indexed := 0

	// 生成带 user 前缀的 namespace ID
	namespace := fmt.Sprintf("%s:%s", userID, agentID)

	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunkText := strings.Join(lines[i:end], "\n")
		chunkText = strings.TrimSpace(chunkText)

		// 跳过空块或纯标题
		if chunkText == "" || strings.HasPrefix(chunkText, "# ") {
			continue
		}

		// 获取 embedding
		emb, err := h.getOllamaEmbedding(chunkText)
		if err != nil {
			continue
		}

		if len(emb) == 0 {
			continue
		}

		// 生成向量 ID（包含 user 和 agent 信息）
		vectorID := fmt.Sprintf("%s:%s:%d", namespace, filepath.Base(filePath), i+1)

		// 存入向量存储（带 user_id 元数据用于隔离）
		if h.vectorStore != nil {
			err := h.vectorStore.Insert(context.TODO(), []storage.Vector{{
				ID:     vectorID,
				Vector: emb,
				Metadata: map[string]interface{}{
					"user_id":   userID,
					"agent_id":  agentID,
					"path":      filePath,
					"content":   chunkText,
					"startLine": i + 1,
					"endLine":   end,
				},
			}})
			if err == nil {
				indexed++
			}
		}
	}

	return indexed, nil
}

// BuildIndex 构建向量索引
// POST /api/v1/memory/index
func (h *MemoryHandler) BuildIndex(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemoryIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.AgentID = "main"
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	// 使用 user_id + agent_id 构建隔离的存储路径
	agentDir := h.getUserAgentDir(c, agentID)
	if agentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	if h.useAgentDB() {
		ctx := c.Request.Context()
		if !h.agentMem.Postgres() {
			c.JSON(200, gin.H{
				"success":       true,
				"message":       "SQLite 模式：无向量分块表，语义索引跳过；搜索使用关键词。",
				"user_id":       userID,
				"agent_id":      agentID,
				"indexed_files": []string{},
				"vector_count":  0,
			})
			return
		}
		nDoc, nVec, err := h.agentMem.ReindexAllDocs(ctx, userID, agentID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Reindex failed", "detail": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"success":       true,
			"message":       fmt.Sprintf("Reindexed %d documents, %d vector chunks", nDoc, nVec),
			"user_id":       userID,
			"agent_id":      agentID,
			"indexed_files": []string{}, // 与磁盘模式字段对齐；具体路径见 GET /memory/stats
			"vector_count":  nVec,
		})
		return
	}

	// 检查 store 目录（自动创建用户目录）
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create directory", "path": agentDir})
		return
	}

	indexedFiles := []string{}
	totalVectors := 0

	// 索引 MEMORY.md
	memFile := filepath.Join(agentDir, "MEMORY.md")
	if _, err := os.Stat(memFile); err == nil {
		count, err := h.indexMemoryFile(userID, agentID, memFile)
		if err == nil && count > 0 {
			indexedFiles = append(indexedFiles, "MEMORY.md")
			totalVectors += count
		}
	}

	// 索引 memory/*.md
	memDir := filepath.Join(agentDir, "memory")
	if entries, err := os.ReadDir(memDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				filePath := filepath.Join(memDir, entry.Name())
				count, err := h.indexMemoryFile(userID, agentID, filePath)
				if err == nil && count > 0 {
					indexedFiles = append(indexedFiles, entry.Name())
					totalVectors += count
				}
			}
		}
	}

	c.JSON(200, gin.H{
		"success":       true,
		"message":       "Index built successfully",
		"user_id":       userID,
		"agent_id":      agentID,
		"indexed_files": indexedFiles,
		"vector_count":  totalVectors,
	})
}

// Search 语义搜索
// POST /api/v1/memory/search
func (h *MemoryHandler) Search(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemorySearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	query := req.Query
	if query == "" {
		c.JSON(400, gin.H{"error": "Query is required"})
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	if h.useAgentDB() {
		ctx := c.Request.Context()
		results, mode, err := h.agentMem.Search(ctx, userID, agentID, query, limit)
		if err != nil {
			c.JSON(500, gin.H{"error": "Search failed", "detail": err.Error()})
			return
		}
		searchResults := make([]MemorySearchResult, 0, len(results))
		for _, r := range results {
			searchResults = append(searchResults, MemorySearchResult{
				Path:      r.Path,
				Content:   r.Content,
				Score:     r.Score,
				StartLine: r.StartLine,
				EndLine:   r.EndLine,
			})
		}
		msg := ""
		if mode == "keyword" {
			msg = "keyword match (no pgvector hits or embedding unavailable)"
		}
		logger.Infof("[memory] agent DB search user_id=%s namespace=%s results=%d mode=%s (query_len=%d)",
			userID, agentID, len(searchResults), mode, len(query))
		c.JSON(200, gin.H{"success": true, "results": searchResults, "message": msg})
		return
	}

	userAgentDir := h.getUserAgentDir(c, agentID)

	queryEmb, embErr := h.getOllamaEmbedding(query)

	if h.vectorStore != nil && embErr == nil {
		results, err := h.vectorStore.Search(context.TODO(), queryEmb, limit, map[string]interface{}{
			"user_id":  userID,
			"agent_id": agentID,
		})
		if err == nil && len(results) > 0 {
			searchResults := make([]MemorySearchResult, 0, len(results))
			for _, r := range results {
				pathStr, _ := r.Metadata["path"].(string)
				contentStr, _ := r.Metadata["content"].(string)
				startL, endL := 1, 1
				if sl, ok := r.Metadata["startLine"].(float64); ok {
					startL = int(sl)
				}
				if el, ok := r.Metadata["endLine"].(float64); ok {
					endL = int(el)
				}
				searchResults = append(searchResults, MemorySearchResult{
					Path:      pathStr,
					Content:   contentStr,
					Score:     float64(r.Score),
					StartLine: startL,
					EndLine:   endL,
				})
			}
			c.JSON(200, gin.H{
				"success": true,
				"results": searchResults,
			})
			return
		}
	}

	if userAgentDir != "" {
		if kw := h.searchMemoryKeyword(userAgentDir, query, limit); len(kw) > 0 {
			msg := "keyword match (no vector hits or vector store unavailable)"
			if embErr != nil {
				msg = fmt.Sprintf("keyword match (embedding unavailable: %v)", embErr)
			}
			c.JSON(200, gin.H{"success": true, "results": kw, "message": msg})
			return
		}
	}

	if embErr != nil {
		c.JSON(200, gin.H{
			"success": true,
			"results": []MemorySearchResult{},
			"message": fmt.Sprintf("No results; embedding failed: %v", embErr),
		})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"results": []MemorySearchResult{},
		"message": "No results found - try POST /api/v1/memory/index or use a query that matches file text",
	})
}

// Get 读取记忆文件
// GET /api/v1/memory/get
func (h *MemoryHandler) Get(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	agentID := c.DefaultQuery("agent_id", "main")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if h.useAgentDB() {
		if !isSafeMemoryRelPath(filePath) {
			c.JSON(403, gin.H{"error": "Invalid path"})
			return
		}
		content, err := h.agentMem.GetDoc(c.Request.Context(), userID, agentID, filePath)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "File not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to read document", "detail": err.Error()})
			return
		}
		from := c.DefaultQuery("from", "1")
		lines := c.DefaultQuery("lines", "0")
		c.JSON(200, gin.H{
			"success": true,
			"path":    filePath,
			"content": content,
			"from":    from,
			"lines":   lines,
		})
		return
	}

	// 获取用户隔离的目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 拼接完整路径（限制在用户目录下）
	fullPath := filepath.Join(userAgentDir, filePath)
	// 安全检查：确保路径在用户目录内
	if !isPathWithinBase(userAgentDir, fullPath) {
		c.JSON(403, gin.H{"error": "Access denied"})
		return
	}

	// 读取文件
	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(404, gin.H{"error": "File not found"})
		return
	}

	// 处理行范围
	from := c.DefaultQuery("from", "1")
	lines := c.DefaultQuery("lines", "0")

	c.JSON(200, gin.H{
		"success": true,
		"path":    filePath,
		"content": string(content),
		"from":    from,
		"lines":   lines,
	})
}

// GetStats 获取记忆统计
// GET /api/v1/memory/stats
func (h *MemoryHandler) GetStats(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	agentID := c.DefaultQuery("agent_id", "main")
	queueMetrics := h.getIndexQueueMetrics()

	if h.useAgentDB() {
		paths, vcount, err := h.agentMem.Stats(c.Request.Context(), userID, agentID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Stats failed", "detail": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"success":               true,
			"user_id":               userID,
			"agent_id":              agentID,
			"vector_count":          vcount,
			"indexed_files":         paths,
			"index_queue_enabled":   queueMetrics.Enabled,
			"index_queue_length":    queueMetrics.Length,
			"index_tasks_processed": queueMetrics.Processed,
			"index_tasks_failed":    queueMetrics.Failed,
			"index_tasks_dropped":   queueMetrics.Dropped,
			"index_last_error":      queueMetrics.LastError,
		})
		return
	}

	// 获取用户隔离的目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 查询向量数量（带 user_id 过滤）
	vectorCount := 0
	if h.vectorStore != nil {
		// TODO: 实现带 user_id 过滤的计数查询
		// 目前返回 0，因为需要 VectorStore 接口支持过滤计数
	}

	indexedFiles := []string{}
	if paths, err := listMemoryMarkdownRelPaths(userAgentDir); err == nil {
		indexedFiles = paths
	}

	c.JSON(200, gin.H{
		"success":               true,
		"user_id":               userID,
		"agent_id":              agentID,
		"vector_count":          vectorCount,
		"indexed_files":         indexedFiles,
		"index_queue_enabled":   queueMetrics.Enabled,
		"index_queue_length":    queueMetrics.Length,
		"index_tasks_processed": queueMetrics.Processed,
		"index_tasks_failed":    queueMetrics.Failed,
		"index_tasks_dropped":   queueMetrics.Dropped,
		"index_last_error":      queueMetrics.LastError,
	})
}

// Put 写入/更新记忆文件
// PUT /api/v1/memory/put
func (h *MemoryHandler) Put(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemoryPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "detail": err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	if h.useAgentDB() {
		if !isSafeMemoryRelPath(req.Path) {
			c.JSON(403, gin.H{"error": "Invalid path"})
			return
		}
		nVec := 0
		indexQueued := false
		var err error
		if h.supportsAsyncIndex() {
			_, err = h.agentMem.PutDocWithoutIndex(c.Request.Context(), userID, agentID, req.Path, req.Content)
			if err == nil {
				indexQueued = h.enqueueAsyncIndex(userID, agentID, req.Path)
				if !indexQueued {
					nVec, err = h.agentMem.ReindexDoc(c.Request.Context(), userID, agentID, req.Path)
				}
			}
		} else {
			nVec, err = h.agentMem.PutDoc(c.Request.Context(), userID, agentID, req.Path, req.Content)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to persist document", "detail": err.Error()})
			return
		}
		c.JSON(200, MemoryWriteResponse{
			Success:          true,
			Message:          "Document saved (database)",
			Path:             req.Path,
			UserID:           userID,
			AgentID:          agentID,
			Vectors:          nVec,
			IndexQueued:      indexQueued,
			IndexQueueLength: h.getIndexQueueMetrics().Length,
		})
		return
	}

	// 获取用户隔离的目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(userAgentDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create directory", "detail": err.Error()})
		return
	}

	// 拼接完整路径
	fullPath := filepath.Join(userAgentDir, req.Path)
	// 安全检查：确保路径在用户目录内
	if !isPathWithinBase(userAgentDir, fullPath) {
		c.JSON(403, gin.H{"error": "Access denied"})
		return
	}

	// memory/foo/bar.md 等子路径需先创建父目录（仅 MkdirAll(userAgentDir) 不够，否则会 WriteFile ENOENT → 500）
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create parent directories", "detail": err.Error()})
		return
	}

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to write file", "detail": err.Error()})
		return
	}

	// 写入成功后自动建索引
	vectorCount := 0
	if h.vectorStore != nil {
		count, _ := h.indexMemoryFile(userID, agentID, fullPath)
		vectorCount = count
	}

	c.JSON(200, MemoryWriteResponse{
		Success:          true,
		Message:          "File written successfully",
		Path:             req.Path,
		UserID:           userID,
		AgentID:          agentID,
		Vectors:          vectorCount,
		IndexQueueLength: h.getIndexQueueMetrics().Length,
	})
}

// Append 追加记忆内容
// POST /api/v1/memory/append
func (h *MemoryHandler) Append(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemoryAppendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "detail": err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if req.Content == "" {
		c.JSON(400, gin.H{"error": "content is required"})
		return
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	if h.useAgentDB() {
		if !isSafeMemoryRelPath(req.Path) {
			c.JSON(403, gin.H{"error": "Invalid path"})
			return
		}
		nVec := 0
		indexQueued := false
		var err error
		if h.supportsAsyncIndex() {
			_, err = h.agentMem.AppendDocWithoutIndex(c.Request.Context(), userID, agentID, req.Path, req.Content)
			if err == nil {
				indexQueued = h.enqueueAsyncIndex(userID, agentID, req.Path)
				if !indexQueued {
					nVec, err = h.agentMem.ReindexDoc(c.Request.Context(), userID, agentID, req.Path)
				}
			}
		} else {
			nVec, err = h.agentMem.AppendDoc(c.Request.Context(), userID, agentID, req.Path, req.Content)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to append document", "detail": err.Error()})
			return
		}
		c.JSON(200, MemoryWriteResponse{
			Success:          true,
			Message:          "Content appended (database)",
			Path:             req.Path,
			UserID:           userID,
			AgentID:          agentID,
			Vectors:          nVec,
			IndexQueued:      indexQueued,
			IndexQueueLength: h.getIndexQueueMetrics().Length,
		})
		return
	}

	// 获取用户隔离的目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(userAgentDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create directory", "detail": err.Error()})
		return
	}

	// 拼接完整路径
	fullPath := filepath.Join(userAgentDir, req.Path)
	// 安全检查：确保路径在用户目录内
	if !isPathWithinBase(userAgentDir, fullPath) {
		c.JSON(403, gin.H{"error": "Access denied"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create parent directories", "detail": err.Error()})
		return
	}

	// 读取现有内容并追加
	var newContent string
	existingContent, err := os.ReadFile(fullPath)
	if err != nil {
		// 文件不存在，从头创建
		newContent = req.Content
	} else {
		// 追加内容（留空行分隔）
		existing := string(existingContent)
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			newContent = existing + "\n\n" + req.Content
		} else {
			newContent = existing + req.Content
		}
	}

	// 写入文件
	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to append content", "detail": err.Error()})
		return
	}

	// 追加成功后自动建索引（只索引新增部分，这里简化为重新索引整个文件）
	vectorCount := 0
	if h.vectorStore != nil {
		count, _ := h.indexMemoryFile(userID, agentID, fullPath)
		vectorCount = count
	}

	c.JSON(200, MemoryWriteResponse{
		Success:          true,
		Message:          "Content appended successfully",
		Path:             req.Path,
		UserID:           userID,
		AgentID:          agentID,
		Vectors:          vectorCount,
		IndexQueueLength: h.getIndexQueueMetrics().Length,
	})
}

// Delete 删除记忆文件
// DELETE /api/v1/memory/doc
func (h *MemoryHandler) Delete(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	agentID := c.DefaultQuery("agent_id", "main")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if h.useAgentDB() {
		if !isSafeMemoryRelPath(filePath) {
			c.JSON(403, gin.H{"error": "Invalid path"})
			return
		}
		err := h.agentMem.DeleteDoc(c.Request.Context(), userID, agentID, filePath)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"error": "File not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to delete document", "detail": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"success":         true,
			"message":         "Document deleted (database)",
			"path":            filePath,
			"user_id":         userID,
			"agent_id":        agentID,
			"vectors_deleted": 0,
		})
		return
	}

	// 获取用户隔离的目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 拼接完整路径
	fullPath := filepath.Join(userAgentDir, filePath)
	// 安全检查：确保路径在用户目录内
	if !isPathWithinBase(userAgentDir, fullPath) {
		c.JSON(403, gin.H{"error": "Access denied"})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "File not found"})
		return
	}

	// 删除文件
	if err := os.Remove(fullPath); err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete file", "detail": err.Error()})
		return
	}

	// 删除向量索引
	vectorsDeleted := 0
	if h.vectorStore != nil {
		if err := h.deleteVectorsForFile(userID, agentID, fullPath); err != nil {
			// 记录错误但不影响返回成功
			logger.Warnf("[memory] Warning: failed to delete vectors for %s: %v", fullPath, err)
		} else {
			vectorsDeleted = 1 // 简化处理
		}
	}

	c.JSON(200, gin.H{
		"success":         true,
		"message":         "File deleted successfully",
		"path":            filePath,
		"user_id":         userID,
		"agent_id":        agentID,
		"vectors_deleted": vectorsDeleted,
	})
}

// Sync 从本地工作区同步记忆到云端
// POST /api/v1/memory/sync
func (h *MemoryHandler) Sync(c *gin.Context) {
	// 获取用户 ID（已由 middleware 验证）
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemorySyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认同步模式
		req.Mode = "full"
	}
	mode := normalizeSyncMode(req.Mode)

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	// 源路径：如果未指定，使用环境变量或默认路径
	sourcePath := req.SourcePath
	if sourcePath == "" {
		sourcePath = os.Getenv("OPENCLAW_WORKSPACE")
	}
	if sourcePath == "" {
		// 尝试从 OpenClaw 默认工作区获取
		homeDir := os.Getenv("HOME")
		sourcePath = filepath.Join(homeDir, ".openclaw", "workspace")
	}

	// 检查源目录是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		c.JSON(404, gin.H{"error": "Source path not found", "path": sourcePath})
		return
	}

	if h.useAgentDB() {
		ctx := c.Request.Context()
		details := []string{}
		fileDetails := []SyncFileDetail{}
		filesSynced := 0
		totalVectors := 0
		filesNew := 0
		filesUpdated := 0
		filesSkipped := 0
		indexQueued := 0

		processDBFile := func(rel string, content []byte) {
			if !isSafeMemoryRelPath(rel) {
				return
			}
			exists := false
			current := []byte{}
			cur, _, err := h.agentMem.GetDocWithUpdatedAt(ctx, userID, agentID, rel)
			if err == nil {
				exists = true
				current = []byte(cur)
			} else if err != sql.ErrNoRows {
				return
			}
			action := calcSyncAction(mode, exists, current, content)
			if action == "skipped" {
				filesSkipped++
				fileDetails = append(fileDetails, SyncFileDetail{Path: rel, Action: action})
				return
			}

			nv := 0
			if h.supportsAsyncIndex() {
				if _, err := h.agentMem.PutDocWithoutIndex(ctx, userID, agentID, rel, string(content)); err != nil {
					return
				}
				if h.enqueueAsyncIndex(userID, agentID, rel) {
					indexQueued++
				} else {
					n, err := h.agentMem.ReindexDoc(ctx, userID, agentID, rel)
					if err != nil {
						return
					}
					nv = n
				}
			} else {
				n, err := h.agentMem.PutDoc(ctx, userID, agentID, rel, string(content))
				if err != nil {
					return
				}
				nv = n
			}
			filesSynced++
			if action == "new" {
				filesNew++
			} else {
				filesUpdated++
			}
			details = append(details, rel)
			fileDetails = append(fileDetails, SyncFileDetail{Path: rel, Action: action})
			totalVectors += nv
		}

		memFile := filepath.Join(sourcePath, "MEMORY.md")
		if _, err := os.Stat(memFile); err == nil {
			content, err := os.ReadFile(memFile)
			if err == nil {
				processDBFile("MEMORY.md", content)
			}
		}

		memDir := filepath.Join(sourcePath, "memory")
		if entries, err := os.ReadDir(memDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					sourceFile := filepath.Join(memDir, entry.Name())
					content, err := os.ReadFile(sourceFile)
					if err != nil {
						continue
					}
					rel := "memory/" + entry.Name()
					processDBFile(rel, content)
				}
			}
		}

		c.JSON(200, MemorySyncResponse{
			Success:          true,
			Message:          "Sync completed (database)",
			UserID:           userID,
			AgentID:          agentID,
			Mode:             mode,
			FilesSynced:      filesSynced,
			FilesNew:         filesNew,
			FilesUpdated:     filesUpdated,
			FilesSkipped:     filesSkipped,
			Vectors:          totalVectors,
			IndexQueued:      indexQueued,
			IndexQueueLength: h.getIndexQueueMetrics().Length,
			Details:          details,
			FileDetails:      fileDetails,
		})
		return
	}

	// 获取用户云端目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	// 确保云端目录存在
	if err := os.MkdirAll(userAgentDir, 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create directory", "detail": err.Error()})
		return
	}

	details := []string{}
	fileDetails := []SyncFileDetail{}
	filesSynced := 0
	totalVectors := 0
	filesNew := 0
	filesUpdated := 0
	filesSkipped := 0

	// 1. 同步 MEMORY.md
	processDiskFile := func(rel string, content []byte) {
		if !isSafeMemoryRelPath(rel) {
			return
		}
		destPath := filepath.Join(userAgentDir, filepath.FromSlash(rel))
		if !isPathWithinBase(userAgentDir, destPath) {
			return
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return
		}
		exists := false
		current := []byte{}
		if b, err := os.ReadFile(destPath); err == nil {
			exists = true
			current = b
		}
		action := calcSyncAction(mode, exists, current, content)
		if action == "skipped" {
			filesSkipped++
			fileDetails = append(fileDetails, SyncFileDetail{Path: rel, Action: action})
			return
		}
		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return
		}
		filesSynced++
		if action == "new" {
			filesNew++
		} else {
			filesUpdated++
		}
		details = append(details, rel)
		fileDetails = append(fileDetails, SyncFileDetail{Path: rel, Action: action})
		if h.vectorStore != nil {
			count, _ := h.indexMemoryFile(userID, agentID, destPath)
			totalVectors += count
		}
	}

	memFile := filepath.Join(sourcePath, "MEMORY.md")
	if _, err := os.Stat(memFile); err == nil {
		content, err := os.ReadFile(memFile)
		if err == nil {
			processDiskFile("MEMORY.md", content)
		}
	}

	// 2. 同步 memory/*.md
	memDir := filepath.Join(sourcePath, "memory")
	if entries, err := os.ReadDir(memDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				sourceFile := filepath.Join(memDir, entry.Name())
				content, err := os.ReadFile(sourceFile)
				if err != nil {
					continue
				}

				processDiskFile("memory/"+entry.Name(), content)
			}
		}
	}

	c.JSON(200, MemorySyncResponse{
		Success:          true,
		Message:          "Sync completed",
		UserID:           userID,
		AgentID:          agentID,
		Mode:             mode,
		FilesSynced:      filesSynced,
		FilesNew:         filesNew,
		FilesUpdated:     filesUpdated,
		FilesSkipped:     filesSkipped,
		Vectors:          totalVectors,
		IndexQueued:      0,
		IndexQueueLength: h.getIndexQueueMetrics().Length,
		Details:          details,
		FileDetails:      fileDetails,
	})
}

// ListVersions 列出文件版本
// GET /api/v1/memory/versions
func (h *MemoryHandler) ListVersions(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	agentID := c.DefaultQuery("agent_id", "main")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(400, gin.H{"error": "path is required"})
		return
	}

	if h.useAgentDB() {
		c.JSON(200, MemoryVersionsResponse{
			Success:  true,
			Path:     filePath,
			Versions: nil,
		})
		return
	}

	versions, err := h.listVersions(userID, agentID, filePath)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list versions", "detail": err.Error()})
		return
	}

	c.JSON(200, MemoryVersionsResponse{
		Success:  true,
		Path:     filePath,
		Versions: versions,
	})
}

// GetVersion 获取指定版本的内容
// GET /api/v1/memory/version
func (h *MemoryHandler) GetVersion(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	agentID := c.DefaultQuery("agent_id", "main")
	filePath := c.Query("path")
	versionID := c.Query("version_id")

	if filePath == "" || versionID == "" {
		c.JSON(400, gin.H{"error": "path and version_id are required"})
		return
	}

	if h.useAgentDB() {
		c.JSON(404, gin.H{
			"error":  "Version not found",
			"detail": "database-backed memory has no file version history",
		})
		return
	}

	content, err := h.getVersion(userID, agentID, filePath, versionID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Version not found", "detail": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":    true,
		"path":       filePath,
		"version_id": versionID,
		"content":    content,
	})
}

// RestoreVersion 恢复指定版本
// POST /api/v1/memory/restore
func (h *MemoryHandler) RestoreVersion(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req struct {
		Path      string `json:"path"`
		VersionID string `json:"version_id"`
		AgentID   string `json:"agent_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "detail": err.Error()})
		return
	}

	if req.Path == "" || req.VersionID == "" {
		c.JSON(400, gin.H{"error": "path and version_id are required"})
		return
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	if h.useAgentDB() {
		c.JSON(400, gin.H{"error": "Version restore is only supported when memory is stored on disk (MEMORY_STORE_ROOT)"})
		return
	}

	// 获取版本内容
	content, err := h.getVersion(userID, agentID, req.Path, req.VersionID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Version not found", "detail": err.Error()})
		return
	}

	// 先保存当前版本作为备份
	userAgentDir := h.getUserAgentDir(c, agentID)
	currentFile := filepath.Join(userAgentDir, req.Path)
	if _, err := os.Stat(currentFile); err == nil {
		currentContent, _ := os.ReadFile(currentFile)
		if len(currentContent) > 0 {
			h.saveVersion(userID, agentID, req.Path, string(currentContent))
		}
	}

	// 恢复版本内容
	fullPath := filepath.Join(userAgentDir, req.Path)
	if !isPathWithinBase(userAgentDir, fullPath) {
		c.JSON(403, gin.H{"error": "Access denied"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create parent directories", "detail": err.Error()})
		return
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to restore version", "detail": err.Error()})
		return
	}

	// 重建索引
	vectorCount := 0
	if h.vectorStore != nil {
		count, _ := h.indexMemoryFile(userID, agentID, fullPath)
		vectorCount = count
	}

	c.JSON(200, gin.H{
		"success":         true,
		"message":         "Version restored successfully",
		"path":            req.Path,
		"version_id":      req.VersionID,
		"vectors_indexed": vectorCount,
	})
}

// deleteVectorsForFile 删除文件相关的所有向量
func (h *MemoryHandler) deleteVectorsForFile(userID, agentID, filePath string) error {
	if h.vectorStore == nil {
		return nil
	}

	// 列出所有向量（简化处理：获取全部）
	allVectors, _, err := h.vectorStore.ListAll(context.TODO(), "", 10000, 0)
	if err != nil {
		return err
	}

	idsToDelete := []string{}
	for _, v := range allVectors {
		// 检查 metadata 中的 path 是否匹配
		if path, ok := v.Metadata["path"].(string); ok {
			if path == filePath || strings.HasSuffix(path, filePath) || strings.HasSuffix(filePath, filepath.Base(path)) {
				idsToDelete = append(idsToDelete, v.ID)
			}
		}
	}

	if len(idsToDelete) > 0 {
		return h.vectorStore.Delete(context.TODO(), idsToDelete)
	}

	return nil
}

// Pull 增量同步（云端→本地）
// POST /api/v1/memory/pull
func (h *MemoryHandler) Pull(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == "" {
		c.JSON(401, gin.H{"error": "Unauthorized: user ID not found"})
		return
	}

	var req MemoryPullRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Since = 0 // 默认同步全部
	}

	agentID := req.AgentID
	if agentID == "" {
		agentID = "main"
	}

	// 目标路径
	destPath := req.DestPath
	if destPath == "" {
		homeDir := os.Getenv("HOME")
		destPath = filepath.Join(homeDir, ".openclaw", "workspace")
	}

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create directory", "detail": err.Error()})
		return
	}

	if h.useAgentDB() {
		ctx := c.Request.Context()
		var paths []string
		var err error
		if req.Since > 0 {
			paths, err = h.agentMem.ListPathsAfter(ctx, userID, agentID, time.Unix(req.Since, 0))
		} else {
			paths, err = h.agentMem.ListPaths(ctx, userID, agentID)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to list cloud documents", "detail": err.Error()})
			return
		}
		details := []PullFileDetail{}
		filesPulled := 0
		for _, rel := range paths {
			if !isSafeMemoryRelPath(rel) {
				continue
			}
			content, remoteUpdatedAt, err := h.agentMem.GetDocWithUpdatedAt(ctx, userID, agentID, rel)
			if err != nil {
				continue
			}
			localPath := filepath.Join(destPath, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				continue
			}
			action := "new"
			if info, err := os.Stat(localPath); err == nil {
				localContent, readErr := os.ReadFile(localPath)
				if readErr == nil && string(localContent) == content {
					action = "unchanged"
				} else if !remoteUpdatedAt.IsZero() && info.ModTime().After(remoteUpdatedAt) {
					action = "conflict_local_newer"
				} else {
					action = "updated"
				}
			}
			if action == "unchanged" || action == "conflict_local_newer" {
				details = append(details, PullFileDetail{
					Path:      rel,
					Action:    action,
					LocalPath: localPath,
				})
				continue
			}
			if err := os.WriteFile(localPath, []byte(content), 0644); err == nil {
				filesPulled++
				details = append(details, PullFileDetail{
					Path:      rel,
					Action:    action,
					LocalPath: localPath,
				})
			}
		}
		c.JSON(200, MemoryPullResponse{
			Success:     true,
			Message:     "Pull completed (database)",
			UserID:      userID,
			AgentID:     agentID,
			FilesPulled: filesPulled,
			Details:     details,
		})
		return
	}

	// 获取云端用户目录
	userAgentDir := h.getUserAgentDir(c, agentID)
	if userAgentDir == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	details := []PullFileDetail{}
	filesPulled := 0
	sinceTime := time.Unix(req.Since, 0)

	// 1. 同步 MEMORY.md
	memFile := filepath.Join(userAgentDir, "MEMORY.md")
	if info, err := os.Stat(memFile); err == nil {
		// 检查是否需要同步（修改时间 > since）
		if req.Since == 0 || info.ModTime().After(sinceTime) {
			content, err := os.ReadFile(memFile)
			if err == nil {
				localPath := filepath.Join(destPath, "MEMORY.md")
				action := "new"

				// 检查本地文件是否存在及是否需要更新
				if localInfo, err := os.Stat(localPath); err == nil {
					if localInfo.ModTime().After(info.ModTime()) {
						action = "unchanged" // 本地更新，不覆盖
					} else {
						action = "updated"
					}
				}

				if action != "unchanged" {
					if err := os.WriteFile(localPath, content, 0644); err == nil {
						filesPulled++
						details = append(details, PullFileDetail{
							Path:      "MEMORY.md",
							Action:    action,
							LocalPath: localPath,
						})
					}
				}
			}
		}
	}

	// 2. 同步 memory/*.md
	memDir := filepath.Join(userAgentDir, "memory")
	if entries, err := os.ReadDir(memDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				sourceFile := filepath.Join(memDir, entry.Name())
				info, err := os.Stat(sourceFile)
				if err != nil {
					continue
				}

				// 检查是否需要同步
				if req.Since > 0 && !info.ModTime().After(sinceTime) {
					continue
				}

				content, err := os.ReadFile(sourceFile)
				if err != nil {
					continue
				}

				// 确保本地 memory 目录存在
				localMemDir := filepath.Join(destPath, "memory")
				if err := os.MkdirAll(localMemDir, 0755); err != nil {
					continue
				}

				localPath := filepath.Join(localMemDir, entry.Name())
				action := "new"

				// 检查本地文件
				if localInfo, err := os.Stat(localPath); err == nil {
					if localInfo.ModTime().After(info.ModTime()) {
						action = "unchanged"
					} else {
						action = "updated"
					}
				}

				if action != "unchanged" {
					if err := os.WriteFile(localPath, content, 0644); err == nil {
						filesPulled++
						details = append(details, PullFileDetail{
							Path:      "memory/" + entry.Name(),
							Action:    action,
							LocalPath: localPath,
						})
					}
				}
			}
		}
	}

	c.JSON(200, MemoryPullResponse{
		Success:     true,
		Message:     "Pull completed",
		UserID:      userID,
		AgentID:     agentID,
		FilesPulled: filesPulled,
		Details:     details,
	})
}
