import api from './index'

export interface ClashRule {
  id: number
  name: string
  rule_content: string
  has_custom_rule: boolean
  subscribe_token: string
  subscribe_url: string
  created_at: string
  updated_at: string
}

export interface CreateClashRuleRequest {
  name: string
  rule_content?: string
}

export interface UpdateClashRuleRequest {
  name?: string
  rule_content?: string
}

export interface RegenerateTokenResponse {
  subscribe_token: string
  subscribe_url: string
}

export function listClashRules(): Promise<ClashRule[]> {
  return api.get('/api/v1/user/clash/rules')
}

export function getClashRule(id: number): Promise<ClashRule> {
  return api.get(`/api/v1/user/clash/rules/${id}`)
}

export function createClashRule(req: CreateClashRuleRequest): Promise<ClashRule> {
  return api.post('/api/v1/user/clash/rules', req)
}

export function updateClashRule(id: number, req: UpdateClashRuleRequest): Promise<ClashRule> {
  return api.put(`/api/v1/user/clash/rules/${id}`, req)
}

export function deleteClashRule(id: number): Promise<void> {
  return api.delete(`/api/v1/user/clash/rules/${id}`)
}

export function resetClashRuleContent(id: number): Promise<ClashRule> {
  return api.post(`/api/v1/user/clash/rules/${id}/reset`)
}

export function regenerateClashToken(id: number): Promise<RegenerateTokenResponse> {
  return api.post(`/api/v1/user/clash/rules/${id}/token`)
}

export function getDefaultClashRule(): Promise<{ content: string }> {
  return api.get('/api/v1/user/clash/default-rule')
}
