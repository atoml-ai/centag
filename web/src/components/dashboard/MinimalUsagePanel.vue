<template>
  <div class="minimal-usage">
    <UsageMetricsSummary
      ref="metricsRef"
      mode="compact"
      :hint="hint"
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

withDefaults(
  defineProps<{
    hint?: string
  }>(),
  {
    hint: '计量与成本估算（按当前服务存储策略保留）。'
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
