import { ref, computed, onMounted } from 'vue'
import { getPipelines, getPipelineDefaults, type AgentPatternPipeline } from '@/api/pipeline'

export function useChatPipelines() {
  const pipelines = ref<AgentPatternPipeline[]>([])
  const defaultPipelineId = ref('')
  const pipelinesLoading = ref(false)

  const defaultPipeline = computed(() =>
    pipelines.value.find((p) => p.id === defaultPipelineId.value) || null
  )

  const pipelineShortcuts = computed(() =>
    pipelines.value
      .filter((p) => (p.shortcut_code || '').trim())
      .map((p) => ({
        code: (p.shortcut_code || '').trim(),
        pipelineId: p.id,
        name: p.name || p.id
      }))
  )

  async function loadPipelines() {
    pipelinesLoading.value = true
    try {
      const [listRes, defaultsRes] = await Promise.all([getPipelines(), getPipelineDefaults()])
      pipelines.value = Array.isArray(listRes) ? listRes : listRes?.data || []
      const defaults = defaultsRes?.data ?? defaultsRes
      defaultPipelineId.value = defaults?.default_pipeline_id || ''
    } catch (error) {
      console.error('Failed to load pipelines:', error)
      pipelines.value = []
    } finally {
      pipelinesLoading.value = false
    }
  }

  onMounted(loadPipelines)

  return {
    pipelines,
    defaultPipelineId,
    defaultPipeline,
    pipelineShortcuts,
    pipelinesLoading,
    loadPipelines
  }
}