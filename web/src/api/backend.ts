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
