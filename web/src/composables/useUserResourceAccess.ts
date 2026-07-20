import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'

/** Team 普通用户资源自建 / 改默认流水线开关（超管与 personal/minimal 不受限） */
export function useUserResourceAccess() {
  const authStore = useAuthStore()
  const { isTeam, isPersonal, isMinimal } = useEdition()

  const unrestricted = computed(
    () => authStore.isAdmin || isPersonal.value || isMinimal.value || !isTeam.value
  )

  const canAddOwnBackends = computed(() => {
    if (unrestricted.value) return true
    return authStore.user?.can_add_own_backends !== false
  })

  const canAddOwnPipelines = computed(() => {
    if (unrestricted.value) return true
    return authStore.user?.can_add_own_pipelines !== false
  })

  const canChangeDefaultPipeline = computed(() => {
    if (unrestricted.value) return true
    return authStore.user?.can_change_default_pipeline !== false
  })

  return {
    canAddOwnBackends,
    canAddOwnPipelines,
    canChangeDefaultPipeline
  }
}
