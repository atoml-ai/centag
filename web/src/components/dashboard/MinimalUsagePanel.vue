<template>
  <div class="minimal-usage">
    <UsageMetricsSummary
      ref="metricsRef"
      mode="compact"
      :hint="hint || t('minimalUsagePanel.hint')"
      show-billing-button
      @open-billing="billingVisible = true"
    />
    <SessionBrowser ref="sessionsRef" mode="compact" />
    <BillingRulesDialog v-model="billingVisible" @saved="onBillingSaved" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'
import SessionBrowser from '@/components/usage/SessionBrowser.vue'
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'

import { useI18n } from 'vue-i18n'

const { t } = useI18n()

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
const billingVisible = ref(false)

function onBillingSaved() {
  metricsRef.value?.reload()
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
