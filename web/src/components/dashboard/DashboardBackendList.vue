<!-- 兼容入口：日常 Provider 管理请用 ProviderManagerPanel -->
<template>
  <ProviderManagerPanel
    ref="panelRef"
    :backends="backends"
    @refresh="emit('refresh')"
    @backend-updated="(b) => emit('backend-updated', b)"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ProviderManagerPanel from '@/components/backends/ProviderManagerPanel.vue'

defineProps<{
  backends: any[]
}>()

const emit = defineEmits<{
  refresh: []
  'backend-updated': [backend: any]
}>()

const panelRef = ref<InstanceType<typeof ProviderManagerPanel> | null>(null)

function openCreate() {
  panelRef.value?.openCreate()
}
function reloadDefault() {
  panelRef.value?.reloadDefault()
}

defineExpose({ openCreate, reloadDefault })
</script>
