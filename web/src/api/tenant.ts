import api from './index'

export interface TenantQuota {
  tenant_id: string
  daily_token_limit?: number
  monthly_token_limit?: number
  daily_request_limit?: number
  monthly_request_limit?: number
  max_backends?: number
  max_api_keys?: number
  used_today_tokens?: number
  used_today_requests?: number
  used_month_tokens?: number
  used_month_requests?: number
  reset_date?: string
  updated_at?: string
}

export interface Tenant {
  id: string
  user_id?: number
  name: string
  description?: string
  status: 'active' | 'suspended' | 'deleted'
  created_at?: string
  updated_at?: string
  quota?: TenantQuota
}

export interface TenantDetail {
  tenant: Tenant
  quota?: TenantQuota
}

export interface UpdateTenantRequest {
  name?: string
  description?: string
  status?: 'active' | 'suspended'
}

export interface UpdateTenantQuotaRequest {
  daily_token_limit?: number
  monthly_token_limit?: number
  daily_request_limit?: number
  monthly_request_limit?: number
  max_backends?: number
  max_api_keys?: number
}

// ── 管理员 API（/api/v1/admin/tenants）────────────────────────────────────

export function listTenants() {
  return api.get<Tenant[]>('/api/v1/admin/tenants')
}

export function getTenant(id: string) {
  return api.get<TenantDetail>(`/api/v1/admin/tenants/${id}`)
}

export function updateTenant(id: string, payload: UpdateTenantRequest) {
  return api.put(`/api/v1/admin/tenants/${id}`, payload)
}

export function deleteTenant(id: string) {
  return api.delete(`/api/v1/admin/tenants/${id}`)
}

export function getTenantQuota(id: string) {
  return api.get<TenantQuota>(`/api/v1/admin/tenants/${id}/quota`)
}

export function updateTenantQuota(id: string, payload: UpdateTenantQuotaRequest) {
  return api.put(`/api/v1/admin/tenants/${id}/quota`, payload)
}

export function resetTenantQuota(id: string) {
  return api.put(`/api/v1/admin/tenants/${id}/quota/reset`)
}

// ── 当前用户 API（/api/v1/user/tenant）──────────────────────────────────────

export function getCurrentTenant() {
  return api.get<TenantDetail>('/api/v1/user/tenant')
}

export function getCurrentQuota() {
  return api.get<TenantQuota>('/api/v1/user/tenant/quota')
}