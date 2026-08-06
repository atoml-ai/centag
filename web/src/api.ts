import api from './api/index'

// Dashboard API
export function getDashboard() {
  return api.get('/api/v1/monitor/dashboard')
}

export function getStatus() {
  return api.get('/api/v1/status')
}

// Backend API
export function getBackends() {
  return api.get('/api/v1/backends')
}

export function getBackend(id: string) {
  return api.get(`/api/v1/backends/${id}`)
}

export function createBackend(payload: any) {
  return api.post('/api/v1/backends', payload)
}

export function updateBackend(id: string, payload: any) {
  return api.put(`/api/v1/backends/${id}`, payload)
}

export function deleteBackend(id: string) {
  return api.delete(`/api/v1/backends/${id}`)
}

export function setDefaultBackend(id: string) {
  return api.put(`/api/v1/backends/${id}/default`)
}

export function testConnection(payload: any) {
  return api.post('/api/v1/backends/test', payload)
}

// Config API
export function getConfig() {
  return api.get('/api/v1/config')
}

export function saveConfig(payload: any) {
  return api.put('/api/v1/config', payload)
}

// Cache API
export function getCacheStats() {
  return api.get('/api/v1/cache/stats')
}

export function clearCache() {
  return api.post('/api/v1/cache/clear')
}

export type CacheListParams = {
  page?: number
  size?: number
  limit?: number
  offset?: number
  type?: string
  save_only?: string
  storage?: string
  session_id?: string
  model?: string
  q?: string
  from?: string
  to?: string
}

export function getCacheList(params?: CacheListParams) {
  return api.get('/api/v1/cache/list', { params })
}

export function getCacheEntry(params: { key: string; type?: string }) {
  return api.get('/api/v1/cache/entry', { params })
}

export function deleteCacheEntry(key: string, type?: string) {
  return api.delete('/api/v1/cache/entry', { data: { key, type: type || 'exact' } })
}

// Chat API
export function chatCompletions(payload: any, config?: { headers?: Record<string, string> }) {
  return api.post('/v1/chat/completions', payload, config)
}

// Plugin API
export function getPlugins() {
  return api.get('/api/v1/plugins')
}

export function getPlugin(name: string) {
  return api.get(`/api/v1/plugins/${name}`)
}

export function updatePlugin(name: string, payload: any) {
  return api.put(`/api/v1/plugins/${name}`, payload)
}

// Storage API
export function getStorages() {
  return api.get('/api/v1/storage')
}

export function getStorage(name?: string) {
  return api.get('/api/v1/storage/get', { params: { name } })
}

export function getDefaultStorageConfig() {
  return api.get('/api/v1/storage/default-config')
}

export function addStorage(payload: any) {
  return api.post('/api/v1/storage/add', payload)
}

export function updateStorage(payload: any) {
  return api.post('/api/v1/storage/update', payload)
}

export function deleteStorage(name: string) {
  return api.delete('/api/v1/storage', { params: { name } })
}

export function toggleStorage(name: string, enabled: boolean) {
  return api.post('/api/v1/storage/toggle', { name, enabled })
}

export function testStorage(payload: any) {
  return api.post('/api/v1/storage/test', payload)
}

export function getStorageStatus() {
  return api.get('/api/v1/storage/status')
}

export function connectStorage(name: string) {
  return api.post('/api/v1/storage/connect', { name })
}

export function disconnectStorage(name: string) {
  return api.post('/api/v1/storage/disconnect', { name })
}

export function setDefaultStorage(name: string) {
  return api.post('/api/v1/storage/set-default', { name })
}

// DataStore API
export function getDataStores() {
  return api.get('/api/v1/data-store')
}

export function getDataStore(name: string) {
  return api.get('/api/v1/data-store/get', { params: { name } })
}

export function getDataStoreStatus() {
  return api.get('/api/v1/data-store/status')
}

export function addDataStore(payload: any) {
  return api.post('/api/v1/data-store/add', payload)
}

export function updateDataStore(payload: any) {
  return api.post('/api/v1/data-store/update', payload)
}

export function deleteDataStore(name: string) {
  return api.delete('/api/v1/data-store', { params: { name } })
}

export function toggleDataStore(name: string, enabled: boolean) {
  return api.post('/api/v1/data-store/toggle', { name, enabled })
}

export function testDataStore(payload: any) {
  return api.post('/api/v1/data-store/test', payload)
}

export function setDefaultDataStore(name: string) {
  return api.post('/api/v1/data-store/set-default', { name })
}

export function removeDefaultDataStore(name: string) {
  return api.post('/api/v1/data-store/remove-default', { name })
}

// KV 数据浏览
export function listKVKeys(params?: { pattern?: string; storage?: string }) {
  return api.get('/api/v1/storage/kv/keys', { params })
}

export function getKVValue(params: { key: string; storage?: string }) {
  return api.get('/api/v1/storage/kv/get', { params })
}

export function deleteKVKey(payload: { key: string; storage_name?: string }) {
  return api.post('/api/v1/storage/kv/delete', payload)
}

export interface LogEntry {
  timestamp: string
  level: string
  request_id?: string
  backend_id?: string
  model?: string
  status_code?: number
  duration_ms?: number
  client_ip?: string
  path?: string
  message: string
  caller?: string
  extra?: Record<string, string>
}

export interface LogQueryResult {
  logs: LogEntry[]
  total: number
  page: number
  page_size: number
  total_pages: number
  newest_first?: boolean
  log_path?: string
  warning?: string
}

export interface TraceTimelineEvent {
  phase: string
  label: string
  ts: string
  level?: string
  duration_ms?: number
  detail?: string
  backend?: string
  model?: string
  status_code?: number
  nodes?: string[]
  extra?: Record<string, string>
}

export interface TraceSummary {
  proxy_mode?: string
  pipeline_id?: string
  model?: string
  backend_id?: string
  status_code?: number
  duration_ms?: number
  started_at?: string
  finished_at?: string
  method?: string
  path?: string
  client_ip?: string
  level?: string
  success: boolean
}

export interface TraceRouting {
  detected_mode?: string
  source?: string
  resolved_mode?: string
  resolved_source?: string
}

export interface TracePipelineGraph {
  pipeline_id?: string
  executed_nodes?: string[]
  node_details?: Record<string, unknown>
  total_nodes?: number
  total_tokens?: number
}

export interface TraceResult {
  request_id: string
  summary: TraceSummary
  routing: TraceRouting
  timeline: TraceTimelineEvent[]
  pipeline_graph: TracePipelineGraph
  raw_log_count: number
}

export const tracesApi = {
  getTrace(requestId: string, params?: Record<string, unknown>) {
    return api.get(`/api/v1/traces/${encodeURIComponent(requestId)}`, { params }) as Promise<TraceResult>
  }
}

// Logs API
export const logsApi = {
  getLogs(params?: Record<string, unknown>) {
    return api.get('/api/v1/logs', { params }) as Promise<LogQueryResult>
  },
  getStats(params?: Record<string, unknown>) {
    return api.get('/api/v1/logs/stats', { params })
  },
  clearLogs() {
    return api.post('/api/v1/logs/clear') as Promise<{ cleared_files?: number; log_path?: string; warning?: string; message?: string }>
  },
  async exportLogs(params?: Record<string, unknown>) {
    const response = await api.post('/api/v1/logs/export', params, {
      responseType: 'blob'
    })
    const blob = response.data as Blob
    const disposition = String(
      response.headers?.['content-disposition'] ?? response.headers?.['Content-Disposition'] ?? ''
    )
    if (disposition.includes('attachment')) {
      return blob
    }
    const text = await blob.text()
    try {
      const parsed = JSON.parse(text) as { success?: boolean; error?: string }
      if (parsed?.success === false) {
        throw new Error(parsed.error || '导出失败')
      }
    } catch (e) {
      if (e instanceof Error && (e.message === '导出失败' || e.message.includes('导出'))) {
        throw e
      }
    }
    throw new Error('导出失败')
  }
}

export { default as api } from './api/index'
export default api

export * from './api/pipeline'
