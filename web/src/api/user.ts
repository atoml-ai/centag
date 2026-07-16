import api from './index'
import type { UserInfo } from './auth'

export interface CreateUserRequest {
  username: string
  password: string
  role?: 'admin' | 'normal'
  display_name?: string
  email?: string
}

export interface UpdateUserRequest {
  display_name?: string
  email?: string
  role?: 'admin' | 'normal'
  enabled?: boolean
}

export interface UpdateProfileRequest {
  display_name: string
  email: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface AdminResetPasswordRequest {
  new_password: string
}

export interface APIKey {
  id: number
  name: string
  key_prefix: string
  masked_key: string
  /** 为 true 时可 GET /apikeys/:id 拉取完整密钥（需服务端已配置加密存储） */
  reveal_available: boolean
  enabled: boolean
  budget_usd: number
  used_usd: number
  rate_limit_rpm: number
  rate_limit_tpm: number
  model_whitelist: string
  expires_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface APIKeyWithFull extends APIKey {
  full_key: string
}

export interface APIKeyDetail extends APIKey {
  full_key?: string
}

export interface CreateAPIKeyRequest {
  name: string
  expires_in?: number // days, undefined = no expiry
  budget_usd?: number
  rate_limit_rpm?: number
  rate_limit_tpm?: number
  model_whitelist?: string
}

export interface UpdateAPIKeyRequest {
  name?: string
  enabled?: boolean
  budget_usd?: number
  rate_limit_rpm?: number
  rate_limit_tpm?: number
  model_whitelist?: string
}

// ── Admin APIs ──────────────────────────────────────────────────────────────

export function listUsers(): Promise<UserInfo[]> {
  return api.get('/api/v1/admin/users')
}

export function createUser(req: CreateUserRequest): Promise<UserInfo> {
  return api.post('/api/v1/admin/users', req)
}

export function updateUser(id: number, req: UpdateUserRequest): Promise<UserInfo> {
  return api.put(`/api/v1/admin/users/${id}`, req)
}

export function deleteUser(id: number): Promise<void> {
  return api.delete(`/api/v1/admin/users/${id}`)
}

export function adminResetPassword(id: number, req: AdminResetPasswordRequest): Promise<void> {
  return api.put(`/api/v1/admin/users/${id}/password`, req)
}

// ── Self Profile APIs ───────────────────────────────────────────────────────

export function getProfile(): Promise<UserInfo> {
  return api.get('/api/v1/user/profile')
}

export function updateProfile(req: UpdateProfileRequest): Promise<UserInfo> {
  return api.put('/api/v1/user/profile', req)
}

export function changePassword(req: ChangePasswordRequest): Promise<void> {
  return api.put('/api/v1/user/password', req)
}

// ── API Key APIs ─────────────────────────────────────────────────────────────

export function listAPIKeys(): Promise<APIKey[]> {
  return api.get('/api/v1/user/apikeys')
}

export function getAPIKey(id: number): Promise<APIKeyDetail> {
  return api.get(`/api/v1/user/apikeys/${id}`)
}

export function createAPIKey(req: CreateAPIKeyRequest): Promise<APIKeyWithFull> {
  return api.post('/api/v1/user/apikeys', req)
}

export function updateAPIKey(id: number, req: UpdateAPIKeyRequest): Promise<APIKey> {
  return api.put(`/api/v1/user/apikeys/${id}`, req)
}

export function deleteAPIKey(id: number): Promise<void> {
  return api.delete(`/api/v1/user/apikeys/${id}`)
}

// ── Admin API Key APIs ───────────────────────────────────────────────────────────

export function listUserAPIKeys(userId: number): Promise<APIKey[]> {
  return api.get(`/api/v1/admin/users/${userId}/apikeys`)
}

export function getAdminAPIKey(userId: number, keyId: number): Promise<APIKeyDetail> {
  return api.get(`/api/v1/admin/users/${userId}/apikeys/${keyId}`)
}
