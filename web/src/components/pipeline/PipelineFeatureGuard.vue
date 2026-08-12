<template>
  <el-tooltip
    v-if="support.visible"
    :content="tooltipText"
    :placement="placement"
    :disabled="!showTooltipWhenEnabled && support.enabled"
  >
    <span class="feature-guard-trigger">
      <slot :enabled="support.enabled" :disabled="!support.enabled" :support="support" />
    </span>
  </el-tooltip>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { resolvePipelineFeatureSupport, getPipelineFeatureLabel, type PipelineFeatureKey } from '@/utils/pipeline/features'
import type { Pipeline } from '@/api/pipeline'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  feature: PipelineFeatureKey
  pipeline: Pipeline
  unrestricted: boolean
  actionLabel?: string
  placement?: 'top' | 'top-start' | 'top-end' | 'bottom' | 'bottom-start' | 'bottom-end' | 'left' | 'left-start' | 'left-end' | 'right' | 'right-start' | 'right-end'
  showTooltipWhenEnabled?: boolean
}>(), {
  actionLabel: '',
  placement: 'top',
  showTooltipWhenEnabled: true
})

const support = computed(() =>
  resolvePipelineFeatureSupport(props.feature, props.pipeline, { unrestricted: props.unrestricted })
)

const tooltipText = computed(() => {
  if (!support.value.enabled) {
    return support.value.reason || t('pipelineFeatureGuard.notAvailable', { label: getPipelineFeatureLabel(props.feature) })
  }
  return props.actionLabel || getPipelineFeatureLabel(props.feature)
})
</script>

<style scoped>
.feature-guard-trigger {
  display: inline-flex;
}
</style>

