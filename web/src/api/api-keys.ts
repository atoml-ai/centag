import api from './index'
import type { APIKey, CreateAPIKeyRequest, UpdateAPIKeyRequest } from './user'

export interface AdminCreateAPIKeyRequest extends CreateAPIKeyRequest {
  user_id: number
}

export interface ListAllAPIKeysResult {
  keys: APIKey[]
  total: number
  limit: number
  offset: number
}

export interface APIKeyStats {
  total_used_usd: number
  budget_usd: number
}

export function listAllAPIKeys(offset = 0, limit = 20): Promise<ListAllAPIKeysResult> {
  return api.get('/api/v1/admin/api-keys', { params: { offset, limit } })
}

export function createAdminAPIKey(req: AdminCreateAPIKeyRequest): Promise<APIKey & { full_key: string }> {
  return api.post('/api/v1/admin/api-keys', req)
}

export function updateAdminAPIKey(id: number, req: UpdateAPIKeyRequest): Promise<APIKey> {
  return api.put(`/api/v1/admin/api-keys/${id}`, req)
}

export function deleteAdminAPIKey(id: number): Promise<void> {
  return api.delete(`/api/v1/admin/api-keys/${id}`)
}

export function getAPIKeyStats(id: number): Promise<APIKeyStats> {
  return api.get(`/api/v1/admin/api-keys/${id}/stats`)
}
