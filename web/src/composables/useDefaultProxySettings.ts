import { ref, watch } from 'vue'
import api, { getBackends } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { storeToRefs } from 'pinia'

function unwrapProxyConfig(res: unknown): { default_backend_id?: string; default_model?: string } {
  if (!res || typeof res !== 'object') return {}
  const obj = res as Record<string, unknown>
  // 拦截器已解包时字段在顶层；少数路径仍可能套一层 data
  if (
    !('default_backend_id' in obj) &&
    !('default_model' in obj) &&
    obj.data &&
    typeof obj.data === 'object' &&
    !Array.isArray(obj.data)
  ) {
    return obj.data as { default_backend_id?: string; default_model?: string }
  }
  return obj as { default_backend_id?: string; default_model?: string }
}

function unwrapList(res: unknown): Array<{ id: string; name?: string }> {
  if (Array.isArray(res)) return res
  if (res && typeof res === 'object') {
    const obj = res as Record<string, unknown>
    if (Array.isArray(obj.data)) return obj.data as Array<{ id: string; name?: string }>
    if (Array.isArray(obj.backends)) return obj.backends as Array<{ id: string; name?: string }>
  }
  return []
}

/** 状态栏等：读取系统默认后端 / 默认模型（鉴权就绪后加载） */
export function useDefaultProxySettings(options?: { enabled?: boolean }) {
  const authStore = useAuthStore()
  const { isAuthenticated } = storeToRefs(authStore)
  const enabled = options?.enabled ?? true

  const backendId = ref('')
  const backendName = ref('')
  const model = ref('')
  let loading = false

  async function loadDefaultProxySettings() {
    if (!enabled || !isAuthenticated.value || loading) return
    loading = true
    try {
      const [proxyRes, backendsRes] = await Promise.all([
        api.get('/api/v1/config/proxy'),
        getBackends()
      ])
      const proxy = unwrapProxyConfig(proxyRes)
      const id = String(proxy.default_backend_id || '').trim()
      const defaultModel = String(proxy.default_model || '').trim()
      backendId.value = id
      model.value = defaultModel

      const list = unwrapList(backendsRes)
      const found = list.find((b) => b.id === id)
      backendName.value = found?.name || id || '未设置'
    } catch (error) {
      console.error('Failed to load default proxy settings:', error)
      if (!backendName.value) backendName.value = '未设置'
    } finally {
      loading = false
    }
  }

  watch(
    isAuthenticated,
    (ok) => {
      if (ok) void loadDefaultProxySettings()
    },
    { immediate: true }
  )

  return {
    backendId,
    backendName,
    model,
    loadDefaultProxySettings
  }
}
