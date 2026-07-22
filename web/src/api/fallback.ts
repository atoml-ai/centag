import api from './index'

export interface FallbackRule {
  priority: number
  backend_id: string
  model: string
  timeout_sec?: number
  max_retries?: number
}

export type FallbackStrategyType = 'same_model_different_backend' | 'same_backend_different_model' | 'custom_chain'

export interface FallbackPolicy {
  id: string
  name: string
  description?: string
  strategy: FallbackStrategyType
  rules: FallbackRule[]
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface FallbackPolicyTestResult {
  policy_id: string
  strategy: FallbackStrategyType
  rules: FallbackRule[]
  note: string
}

// 获取所有降级策略
export const getFallbackPolicies = () => {
  return api.get('/api/v1/fallback-policies') as Promise<FallbackPolicy[]>
}

// 获取单个策略
export const getFallbackPolicy = (id: string) => {
  return api.get(`/api/v1/fallback-policies/${id}`) as Promise<FallbackPolicy>
}

// 创建策略
export const createFallbackPolicy = (policy: Omit<FallbackPolicy, 'created_at' | 'updated_at'>) => {
  return api.post('/api/v1/fallback-policies', policy) as Promise<FallbackPolicy>
}

// 更新策略
export const updateFallbackPolicy = (id: string, policy: Omit<FallbackPolicy, 'created_at' | 'updated_at'>) => {
  return api.put(`/api/v1/fallback-policies/${id}`, policy) as Promise<FallbackPolicy>
}

// 删除策略
export const deleteFallbackPolicy = (id: string) => {
  return api.delete(`/api/v1/fallback-policies/${id}`)
}

// 测试策略（预览降级路径）
export const testFallbackPolicy = (id: string) => {
  return api.post(`/api/v1/fallback-policies/${id}/test`) as Promise<FallbackPolicyTestResult>
}
