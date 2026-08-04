<template>
  <div class="minimal-usage">
    <UsageMetricsSummary
      ref="metricsRef"
      mode="compact"
      :hint="hint || t('minimalUsagePanel.hint')"
      show-billing-button
      @open-billing="openBillingRules"
    />
    <SessionBrowser ref="sessionsRef" mode="compact" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'
import SessionBrowser from '@/components/usage/SessionBrowser.vue'

const { t } = useI18n()
const router = useRouter()

withDefaults(
  defineProps<{
    hint?: string
  }>(),
  {
    hint: ''
  }
)

const metricsRef = ref<InstanceType<typeof UsageMetricsSummary> | null>(null)
const sessionsRef = ref<InstanceType<typeof SessionBrowser> | null>(null)

function openBillingRules() {
  router.push('/billing')
}

async function reload() {
  await Promise.all([metricsRef.value?.reload(), sessionsRef.value?.reload()])
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
