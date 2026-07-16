import api from './index'

export interface LoginRequest {
  username: string
  password: string
}

export interface UserInfo {
  id: number
  username: string
  role: 'admin' | 'normal'
  display_name: string
  email: string
  enabled: boolean
  created_at: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  user: UserInfo
}

export interface RefreshResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  user: UserInfo
}

export function login(req: LoginRequest): Promise<LoginResponse> {
  return api.post('/api/auth/login', req)
}

export function setupAdmin(password: string): Promise<LoginResponse> {
  return api.post('/api/v1/auth/setup', { password })
}

export function getBootstrapStatus(): Promise<{ initialized: boolean; username: string; edition: string }> {
  return api.get('/api/v1/auth/bootstrap-status')
}

export function logout(refreshToken?: string): Promise<void> {
  return api.post('/api/auth/logout', { refresh_token: refreshToken })
}

export function refreshToken(token: string): Promise<RefreshResponse> {
  return api.post('/api/auth/refresh', { refresh_token: token })
}

export function getMe(): Promise<UserInfo> {
  return api.get('/api/auth/me')
}
