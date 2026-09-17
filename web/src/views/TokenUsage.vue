<template>
  <div class="token-usage">
    <div v-if="!embedded" class="usage-header">
      <h2>{{ pageTitle }}</h2>
    </div>

    <UsageMetricsSummary
      ref="metricsRef"
      mode="full"
      :hint="pageHint"
      show-billing-button
      @open-billing="openBillingRules"
    />

    <UsageAnalyticsTabs />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'
import UsageAnalyticsTabs from '@/components/usage/UsageAnalyticsTabs.vue'

defineProps<{
  /** When true, hide page title (embedded in metering-billing tabs). */
  embedded?: boolean
}>()

const { t } = useI18n()

const router = useRouter()
const authStore = useAuthStore()
const { isTeam } = useEdition()
const canManageBilling = computed(() => !(isTeam.value && !authStore.isAdmin))
const pageTitle = computed(() => (canManageBilling.value ? t('tokenUsage.titleBilling') : t('tokenUsage.titleUsage')))
const pageHint = computed(() =>
  canManageBilling.value
    ? t('tokenUsage.hintBilling')
    : t('tokenUsage.hintUsage')
)

const metricsRef = ref<InstanceType<typeof UsageMetricsSummary> | null>(null)

function openBillingRules() {
  if (!canManageBilling.value) return
  router.push('/billing')
}

onMounted(() => {
  metricsRef.value?.reload()
})
</script>

<style scoped>
.token-usage {
  width: 100%;
  padding: 0 0 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.usage-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .usage-header {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
