import api from './index'

// 获取后端列表（可选按能力筛选）
export const getBackends = (params?: { capability?: string }) => {
  return api.get('/api/v1/backends', { params })
}

// 获取后端模型列表
export const getBackendModels = (backendId: string, type?: 'all' | 'chat' | 'embedding') => {
  const params = type && type !== 'all' ? { type } : {}
  return api.get(`/api/v1/backends/${backendId}/models`, { params })
}

export interface BackendTypeMeta {
  type: string
  name: string
  default_base_url: string
  key_help: string
  config_schema?: Record<string, any>
  capabilities: string[]
  auth_schemes: string[]
}

// 获取已注册的后端类型元数据（WebUI 动态表单）
export const listBackendTypes = () => {
  return api.get('/api/v1/backends/types') as Promise<BackendTypeMeta[]>
}

// 熔断器实时状态（供 WebUI 展示）
export interface CircuitBreakerSnapshot {
  backend_id: string
  state: string // closed / open / half-open
  is_open: boolean
  consecutive_failures: number
  failure_count: number
  success_count: number
  request_count: number
  last_failure_at?: string
  open_since?: string
  last_state_change: string
  failure_threshold: number
  success_threshold: number
  timeout_sec: number
}

export interface CircuitBreakerStatus {
  circuit_breakers: CircuitBreakerSnapshot[]
  open_backends: string[]
  summary: { total: number; open: number; half_open: number; closed: number }
}

export const getCircuitBreakerStatus = () => {
  return api.get('/api/v1/backends/circuit-breaker') as Promise<CircuitBreakerStatus>
}

// 重置指定后端的熔断器
export const resetCircuitBreaker = (backendId: string) => {
  return api.post(`/api/v1/backends/circuit-breaker/${encodeURIComponent(backendId)}/reset`)
}

// Provider Catalog API
export interface ProviderCatalogEntry {
  id: string
  name: string
  type: string
  base_url: string
  env_key?: string
  icon?: string
  description?: string
  default_models?: Array<{
    name: string
    supports_tools: boolean
    supports_images: boolean
    supports_thinking: boolean
    max_context_tokens: number
  }>
  enabled: boolean
  updated_at: string
  remark?: string
}

export interface ProviderCatalogResponse {
  entries: ProviderCatalogEntry[]
  sync_time: string
  source: string
}

// 获取 Provider 目录（可用的 Provider 类型列表）
export const getProviderCatalog = () => {
  return api.get('/api/v1/provider-catalog') as Promise<ProviderCatalogResponse>
}

// 同步 Provider 目录（从飞书获取最新数据）
export const syncProviderCatalog = () => {
  return api.post('/api/v1/provider-catalog/sync') as Promise<ProviderCatalogResponse>
}

// 删除 Provider 目录条目
export const deleteProviderCatalogEntry = (id: string) => {
  return api.delete(`/api/v1/provider-catalog/${encodeURIComponent(id)}`)
}
