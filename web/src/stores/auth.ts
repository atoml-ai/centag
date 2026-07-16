import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as apiLogin, logout as apiLogout, refreshToken as apiRefresh, setupAdmin as apiSetup } from '@/api/auth'
import type { UserInfo } from '@/api/auth'

const REFRESH_TOKEN_KEY = 'centag_refresh_token'

export const useAuthStore = defineStore('auth', () => {
  // Access token is kept in memory only (never persisted to localStorage)
  const accessToken = ref<string>('')
  const user = ref<UserInfo | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!accessToken.value && !!user.value)
  const isAdmin = computed(() => {
    const r = (user.value?.role ?? '').toLowerCase()
    return r === 'admin' || r === 'administrator'
  })
  const displayName = computed(() => user.value?.display_name || user.value?.username || '')

  function applyAuth(resp: { access_token: string; refresh_token: string; user: UserInfo }) {
    accessToken.value = resp.access_token
    user.value = resp.user
    localStorage.setItem(REFRESH_TOKEN_KEY, resp.refresh_token)
  }

  // ── Actions ────────────────────────────────────────────────────────────────

  async function login(username: string, password: string) {
    loading.value = true
    try {
      const resp = await apiLogin({ username, password })
      applyAuth(resp)
    } finally {
      loading.value = false
    }
  }

  async function setup(password: string) {
    loading.value = true
    try {
      const resp = await apiSetup(password)
      applyAuth(resp)
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    const rt = localStorage.getItem(REFRESH_TOKEN_KEY) ?? undefined
    try {
      await apiLogout(rt)
    } catch {
      // ignore
    } finally {
      clearAuth()
    }
  }

  // Called on app startup: try to restore session via refresh token.
  async function restoreSession(): Promise<boolean> {
    const rt = localStorage.getItem(REFRESH_TOKEN_KEY)
    if (!rt) return false
    try {
      const resp = await apiRefresh(rt)
      applyAuth(resp)
      return true
    } catch {
      clearAuth()
      return false
    }
  }

  // Called by the axios response interceptor to silently refresh the access token.
  async function silentRefresh(): Promise<string> {
    const rt = localStorage.getItem(REFRESH_TOKEN_KEY)
    if (!rt) throw new Error('no refresh token')
    const resp = await apiRefresh(rt)
    applyAuth(resp)
    return resp.access_token
  }

  function clearAuth() {
    accessToken.value = ''
    user.value = null
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  function updateUser(updated: Partial<UserInfo>) {
    if (user.value) {
      user.value = { ...user.value, ...updated }
    }
  }

  return {
    accessToken,
    user,
    loading,
    isAuthenticated,
    isAdmin,
    displayName,
    login,
    setup,
    logout,
    restoreSession,
    silentRefresh,
    clearAuth,
    updateUser,
  }
})
