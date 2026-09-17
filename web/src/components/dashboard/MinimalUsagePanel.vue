<template>
  <div class="minimal-usage">
    <UsageMetricsSummary
      ref="metricsRef"
      mode="compact"
      :hint="hint || t('minimalUsagePanel.hint')"
      :show-billing-button="!metricsOnly"
      :metrics-only="metricsOnly"
      @open-billing="openBillingRules"
    />
    <UsageAnalyticsTabs v-if="analytics" ref="analyticsRef" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'
import UsageAnalyticsTabs from '@/components/usage/UsageAnalyticsTabs.vue'

const { t } = useI18n()
const router = useRouter()

withDefaults(
  defineProps<{
    hint?: string
    metricsOnly?: boolean
    analytics?: boolean
  }>(),
  {
    hint: '',
    metricsOnly: false,
    analytics: false
  }
)

const metricsRef = ref<InstanceType<typeof UsageMetricsSummary> | null>(null)
const analyticsRef = ref<InstanceType<typeof UsageAnalyticsTabs> | null>(null)

function openBillingRules() {
  router.push('/billing')
}

async function reload() {
  await Promise.all([metricsRef.value?.reload(), analyticsRef.value?.reload()])
}

defineExpose({ reload })
</script>

<style scoped>
.minimal-usage {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>
