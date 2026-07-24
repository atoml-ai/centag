import { ref, watch } from 'vue'
import { getPipelineDefaults, getPipelines, parsePipelinesResponse } from '@/api/pipeline'
import { useAuthStore } from '@/stores/auth'
import { storeToRefs } from 'pinia'

/** 读取当前默认流水线（鉴权就绪后加载，避免 StatusBar 早于 restoreSession） */
export function useActivePipeline(options?: { enabled?: boolean }) {
  const authStore = useAuthStore()
  const { isAuthenticated } = storeToRefs(authStore)
  const enabled = options?.enabled ?? true

  const pipelineId = ref('')
  const pipelineName = ref('')
  let loading = false

  async function loadActivePipeline() {
    if (!enabled || !isAuthenticated.value || loading) return
    loading = true
    try {
      const [defaultsRes, pipelinesRes] = await Promise.all([
        getPipelineDefaults(),
        getPipelines()
      ])
      const defaults =
        (defaultsRes as { data?: { default_pipeline_id?: string } })?.data ?? defaultsRes
      const id = String(
        (defaults as { default_pipeline_id?: string })?.default_pipeline_id || ''
      ).trim()
      pipelineId.value = id
      const list = parsePipelinesResponse(pipelinesRes)
      const found = list.find((p) => p.id === id)
      pipelineName.value = found?.name || id || '未设置'
    } catch (error) {
      console.error('Failed to load active pipeline:', error)
      if (!pipelineName.value) pipelineName.value = '未设置'
    } finally {
      loading = false
    }
  }

  watch(
    isAuthenticated,
    (ok) => {
      if (ok) void loadActivePipeline()
    },
    { immediate: true }
  )

  return {
    pipelineId,
    pipelineName,
    loadActivePipeline
  }
}
