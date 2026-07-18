import api from './index'

export interface PricingRule {
  id?: number
  name: string
  backend_id: string
  model: string
  input_price_per_m: number
  output_price_per_m: number
  currency?: string
  priority: number
  enabled: boolean
}

export function listPricingRules() {
  return api({
    url: '/api/v1/admin/billing/rules',
    method: 'get'
  }) as Promise<PricingRule[]>
}

export function createPricingRule(data: PricingRule) {
  return api({
    url: '/api/v1/admin/billing/rules',
    method: 'post',
    data
  }) as Promise<PricingRule>
}

export function updatePricingRule(id: number, data: PricingRule) {
  return api({
    url: `/api/v1/admin/billing/rules/${id}`,
    method: 'put',
    data
  }) as Promise<PricingRule>
}

export function deletePricingRule(id: number) {
  return api({
    url: `/api/v1/admin/billing/rules/${id}`,
    method: 'delete'
  })
}

export function importPricingRules(yamlText: string) {
  return api({
    url: '/api/v1/admin/billing/rules/import',
    method: 'post',
    data: yamlText,
    headers: { 'Content-Type': 'application/x-yaml' },
    transformRequest: [(data) => data]
  }) as Promise<{ imported: number }>
}

export function exportPricingRules() {
  return api({
    url: '/api/v1/admin/billing/rules/export',
    method: 'get',
    responseType: 'text'
  }) as Promise<string>
}
