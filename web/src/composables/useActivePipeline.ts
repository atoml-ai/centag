import { ref, onMounted } from 'vue'
import { getPipelineDefaults, getPipelines } from '@/api/pipeline'
import { useAuthStore } from '@/stores/auth'

export function useActivePipeline(options?: { enabled?: boolean }) {
  const authStore = useAuthStore()
  const enabled = options?.enabled ?? true

  const pipelineId = ref('')
  const pipelineName = ref('')

  async function loadActivePipeline() {
    if (!authStore.isAuthenticated) return
    try {
      const [defaultsRes, pipelinesRes] = await Promise.all([
        getPipelineDefaults(),
        getPipelines()
      ])
      const defaults = defaultsRes?.data ?? defaultsRes
      const id = defaults?.default_pipeline_id || ''
      pipelineId.value = id
      const list = Array.isArray(pipelinesRes) ? pipelinesRes : pipelinesRes?.data || []
      const found = list.find((p: { id: string; name?: string }) => p.id === id)
      pipelineName.value = found?.name || id || '未设置'
    } catch (error) {
      console.error('Failed to load active pipeline:', error)
      pipelineName.value = pipelineId.value || '未设置'
    }
  }

  onMounted(() => {
    if (enabled) loadActivePipeline()
  })

  return {
    pipelineId,
    pipelineName,
    loadActivePipeline
  }
}
