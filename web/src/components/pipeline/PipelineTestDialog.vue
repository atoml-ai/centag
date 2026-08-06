<template>
  <!-- 兼容旧入口：与 Personal/Minimal 共用 MinimalChat 右侧抽屉 -->
  <MinimalChat
    v-model="visible"
    :initial-pipeline-id="pipelineId"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MinimalChat from '@/views/MinimalChat.vue'
import type { Pipeline } from '@/api/pipeline'

const props = defineProps<{
  modelValue: boolean
  pipeline: Pipeline | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const pipelineId = computed(() => props.pipeline?.id || '')
</script>
