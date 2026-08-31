<template>
  <div class="billing-rules-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ t('billingRules.title') }}</h2>
        <p class="page-sub">{{ t('billingRules.subtitle') }}</p>
      </div>
    </div>

    <PricingRulesPanel ref="rulesPanelRef" @saved="onRulesSaved" />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PricingRulesPanel from '@/components/billing/PricingRulesPanel.vue'

const { t } = useI18n()
const rulesPanelRef = ref<InstanceType<typeof PricingRulesPanel> | null>(null)

function onRulesSaved() {
  /* reserved for future cross-tab refresh */
}
</script>

<style scoped>
.billing-rules-page {
  width: 100%;
  padding: 0 0 32px;
}
.page-header {
  margin-bottom: 8px;
}
.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.page-sub {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
</style>
