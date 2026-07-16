import axios, { type AxiosRequestConfig } from 'axios'

const api = axios.create({
  baseURL: '/',
  timeout: 30000
})

// ── Auth store accessor ──────────────────────────────────────────────────────
// The auth store is set lazily after Pinia is initialised.  This avoids
// the circular-dependency warning from Vite when both the store and the api
// module import each other.

type AuthStoreRef = {
  accessToken: string
  silentRefresh: () => Promise<string>
  clearAuth: () => void
}

let _authStore: AuthStoreRef | null = null

/** Call this from main.ts after createPinia() to wire up the auth store. */
export function setAuthStore(store: AuthStoreRef) {
  _authStore = store
}

function getAuthStore(): AuthStoreRef | null {
  if (_authStore) return _authStore
  // Lazy import as a fallback when main.ts hasn't called setAuthStore yet.
  try {
    const { useAuthStore } = require('@/stores/auth')
    _authStore = useAuthStore()
    return _authStore
  } catch {
    return null
  }
}

// ── Request interceptor: attach JWT Bearer token ────────────────────────────
api.interceptors.request.use(
  (config) => {
    const store = getAuthStore()
    if (store?.accessToken) {
      config.headers = config.headers ?? {}
      config.headers['Authorization'] = `Bearer ${store.accessToken}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Track whether a token refresh is in progress to avoid parallel refresh calls.
let refreshing = false
let refreshQueue: Array<{ resolve: (token: string) => void; reject: (err: unknown) => void }> = []
let redirectingToLogin = false

function processQueue(error: unknown, token: string | null) {
  refreshQueue.forEach((p) => {
    if (error) p.reject(error)
    else p.resolve(token!)
  })
  refreshQueue = []
}

function isLoginPath(pathname: string): boolean {
  // Support both "/login" and deployments under "/static/login".
  return pathname === '/login' || pathname.endsWith('/login')
}

function buildLoginPath(pathname: string): string {
  // Keep compatibility with createWebHistory('/static/').
  if (pathname.startsWith('/static/')) return '/static/login'
  return '/login'
}

/** 从统一 Response 或 OpenAI 风格嵌套 error 中提取可读文案 */
function extractApiErrorMessage(data: unknown): string | undefined {
  if (data == null) return undefined
  if (typeof data === 'string') return data
  if (typeof data !== 'object') return undefined
  const o = data as Record<string, unknown>
  if (typeof o.error === 'string') return o.error
  if (o.error && typeof o.error === 'object' && o.error !== null) {
    const inner = o.error as Record<string, unknown>
    if (typeof inner.message === 'string') return inner.message
  }
  if (typeof o.message === 'string') return o.message
  return undefined
}

// ── Response interceptor: handle errors + 401 auto-refresh ─────────────────
api.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob' || response.config.responseType === 'arraybuffer') {
      return response
    }
    const responseData = response.data
    if (responseData && typeof responseData === 'object') {
      if ('success' in responseData) {
        if (!responseData.success) {
          const errorMsg = extractApiErrorMessage(responseData) || '请求失败'
          return Promise.reject(new Error(errorMsg))
        }
        if ('data' in responseData) return responseData.data
        return responseData
      }
    }
    return responseData
  },
  async (error) => {
    const originalRequest: AxiosRequestConfig & { _retry?: boolean } = error.config

    if (
      error.response?.status === 401 &&
      !originalRequest._retry &&
      originalRequest.url !== '/api/auth/login' &&
      originalRequest.url !== '/api/auth/refresh'
    ) {
      if (refreshing) {
        return new Promise((resolve, reject) => {
          refreshQueue.push({
            resolve: (token) => {
              originalRequest.headers = originalRequest.headers ?? {}
              originalRequest.headers['Authorization'] = `Bearer ${token}`
              originalRequest._retry = true
              resolve(api(originalRequest))
            },
            reject,
          })
        })
      }

      originalRequest._retry = true
      refreshing = true

      try {
        const store = getAuthStore()
        if (!store) throw new Error('auth store not available')
        const newToken = await store.silentRefresh()
        processQueue(null, newToken)
        originalRequest.headers = originalRequest.headers ?? {}
        originalRequest.headers['Authorization'] = `Bearer ${newToken}`
        return api(originalRequest)
      } catch (refreshError) {
        processQueue(refreshError, null)
        const store = getAuthStore()
        store?.clearAuth()
        if (typeof window !== 'undefined') {
          const { pathname } = window.location
          if (!isLoginPath(pathname) && !redirectingToLogin) {
            redirectingToLogin = true
            window.location.href = buildLoginPath(pathname)
          }
        }
        return Promise.reject(refreshError)
      } finally {
        refreshing = false
      }
    }

    const message =
      extractApiErrorMessage(error.response?.data) || error.message || '请求失败'
    return Promise.reject(new Error(message))
  }
)

export default api

// Export all API modules for convenience
export * from './auth'
export * from './backend'
export * from './clash'
export * from './evaluation'
export * from './strategy'
export * from './user'
