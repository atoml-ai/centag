import api from './index'

export interface PluginMetadata {
  id: string
  name: string
  version: string
  description?: string
  author?: string
  email?: string
  url?: string
  category?: string
  tags?: string[]
  permissions?: string[]
  download_url?: string
  checksum?: string
  signature?: string
  size?: number
  download_count?: number
  rating?: number
  rating_count?: number
  created_at?: string
  updated_at?: string
}

export interface PluginDependency {
  id: string
  version: string
  optional?: boolean
}

export interface RegisterPluginRequest {
  name: string
  version: string
  description?: string
  author?: string
  email?: string
  url?: string
  category?: string
  tags?: string[]
  permissions?: string[]
  dependencies?: PluginDependency[]
  download_url: string
  checksum: string
  signature?: string
  size?: number
}

export interface ListPluginsRequest {
  category?: string
  tags?: string[]
  author?: string
  search?: string
  sort_by?: 'name' | 'download_count' | 'rating' | 'created_at'
  sort_order?: 'asc' | 'desc'
  page?: number
  page_size?: number
}

export interface ListPluginsResponse {
  plugins: PluginMetadata[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface RatePluginRequest {
  score: number
  comment?: string
}

// 插件注册
export function registerPlugin(payload: RegisterPluginRequest) {
  return api.post('/api/v1/registry/plugins', payload)
}

// 获取插件列表
export function listPlugins(params?: ListPluginsRequest) {
  return api.get('/api/v1/registry/plugins', { params })
}

// 获取插件详情
export function getPlugin(id: string) {
  return api.get(`/api/v1/registry/plugins/${id}`)
}

// 删除插件
export function deletePlugin(id: string) {
  return api.delete(`/api/v1/registry/plugins/${id}`)
}

// 获取插件版本列表
export function listPluginVersions(id: string) {
  return api.get(`/api/v1/registry/plugins/${id}/versions`)
}

// 获取特定版本
export function getPluginVersion(id: string, version: string) {
  return api.get(`/api/v1/registry/plugins/${id}/versions/${version}`)
}

// 评分插件
export function ratePlugin(id: string, payload: RatePluginRequest) {
  return api.post(`/api/v1/registry/plugins/${id}/ratings`, payload)
}

// 获取插件评分
export function getPluginRating(id: string) {
  return api.get(`/api/v1/registry/plugins/${id}/ratings`)
}

// 下载插件
export function downloadPlugin(id: string) {
  return api.post(`/api/v1/registry/plugins/${id}/download`)
}

// 上传插件文件
export function uploadPlugin(file: File, metadata: Partial<RegisterPluginRequest>) {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('metadata', JSON.stringify(metadata))
  
  return api.post('/api/v1/registry/plugins/upload', formData, {
    headers: {
      'Content-Type': 'multipart/form-data'
    }
  })
}
