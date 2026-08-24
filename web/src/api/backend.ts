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
