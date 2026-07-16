import api from '@/api'

export interface BackendTestOptions {
  apiKey?: string
}

/** Test backend connection via /api/v1/backends/test and return the updated backend config. */
export async function testBackendConnection(
  backend: { id: string },
  options?: BackendTestOptions
): Promise<any> {
  const payload: Record<string, string> = { id: backend.id }
  if (options?.apiKey) {
    payload.api_key = options.apiKey
  }
  return api.post('/api/v1/backends/test', payload) as Promise<any>
}

/** Merge probe/test response into a backend list (same as Backends page). */
export function mergeBackendUpdate(list: any[], updated: any): void {
  if (!updated?.id) return
  const index = list.findIndex((b) => b.id === updated.id)
  if (index !== -1) {
    list[index] = { ...list[index], ...updated }
  }
}

export type BackendTestMessage = { level: 'success' | 'warning'; text: string }

/** User-facing message after /backends/test — shared by dashboard and backends page. */
export function getBackendTestMessage(updatedBackend: any, fallbackName: string): BackendTestMessage {
  const name = updatedBackend?.name || fallbackName
  if (updatedBackend?.enabled && updatedBackend?.health_status?.status === 'healthy') {
    return { level: 'success', text: `${name} 连接成功，已启用` }
  }
  if (updatedBackend?.enabled) {
    return { level: 'warning', text: `${name} 连接成功，但健康状态异常` }
  }
  return { level: 'warning', text: `${name} 连接失败，请检查配置` }
}