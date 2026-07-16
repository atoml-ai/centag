import api from './index'

export interface WeightBreakdown {
  name_similarity: number  // 0-1
  capacity_match: number   // 0-1
  family_match: number     // 0-1
}

export interface StrategyListItem {
  id: string
  name: string
  description: string
  weights: WeightBreakdown
  is_builtin: boolean
  strictness?: number
  tolerance?: number
  created_at?: string
}

export interface CreateStrategyRequest {
  name: string
  description: string
  weights: WeightBreakdown
  strictness: number
  tolerance: number
}

export interface UpdateStrategyRequest {
  description: string
  weights: WeightBreakdown
  strictness: number
  tolerance: number
}

// 获取所有策略（内置 + 自定义）
export function listStrategies() {
  return api.get<{ strategies: StrategyListItem[]; count: number }>('/api/v1/strategies')
}

// 创建自定义策略
export function createStrategy(req: CreateStrategyRequest) {
  return api.post<StrategyListItem>('/api/v1/strategies', req)
}

// 更新自定义策略
export function updateStrategy(id: string, req: UpdateStrategyRequest) {
  return api.put<StrategyListItem>(`/api/v1/strategies/${id}`, req)
}

// 删除自定义策略
export function deleteStrategy(id: string) {
  return api.delete(`/api/v1/strategies/${id}`)
}
