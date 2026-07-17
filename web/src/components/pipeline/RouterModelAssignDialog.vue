<!-- Compatibility wrapper: v0.2.2 名称。请优先使用 CapabilitySlotsDialog。 -->
<template>
  <CapabilitySlotsDialog v-model="visible" :pipeline-id="pipelineId" @saved="onSaved" />
</template>

<script setup lang="ts">
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import type { AgentPatternPipeline } from '@/api/pipeline'

const visible = defineModel<boolean>({ default: false })

defineProps<{
  pipelineId: string
}>()

const emit = defineEmits<{
  saved: [pipeline: AgentPatternPipeline]
}>()

function onSaved(pipeline: AgentPatternPipeline) {
  emit('saved', pipeline)
}
</script>
