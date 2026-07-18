import api from './index'

export interface CostGroup {
  key: string
  cost_usd: number
  tokens: number
  request_count: number
}

export interface CostSummary {
  total_cost_usd: number
  total_tokens: number
  cache_saved_usd: number
  currency?: string
  usd_to_cny?: number
  groups: CostGroup[]
  from: string
  to: string
  group_by: string
}

export interface CostSummaryParams {
  from?: string
  to?: string
  group_by?: 'tenant' | 'backend' | 'model' | 'date' | 'dept'
  tenant_id?: string
}

export function getCostSummary(params: CostSummaryParams = {}) {
  return api({
    url: '/api/v1/admin/cost/summary',
    method: 'get',
    params,
  }) as Promise<CostSummary>
}