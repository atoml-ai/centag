<template>
  <div class="billing-rules-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">{{ t('billingRules.title') }}</h2>
        <p class="page-sub">{{ t('billingRules.subtitle') }}</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="page-tabs" @tab-change="onTabChange">
      <el-tab-pane :label="t('billingRules.tabRules')" name="rules">
        <PricingRulesPanel ref="rulesPanelRef" @saved="onRulesSaved" />
      </el-tab-pane>
      <el-tab-pane v-if="showSyncTab" :label="t('billingRules.tabSync')" name="sync" lazy>
        <PricingSyncPanel @applied="onSyncApplied" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useEdition } from '@/composables/useEdition'
import PricingRulesPanel from '@/components/billing/PricingRulesPanel.vue'
import PricingSyncPanel from '@/components/billing/PricingSyncPanel.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { isTeam } = useEdition()

const showSyncTab = computed(() => isTeam.value)
const activeTab = ref('rules')
const rulesPanelRef = ref<InstanceType<typeof PricingRulesPanel> | null>(null)

function tabFromQuery(): string {
  const q = String(route.query.tab || '')
  if (q === 'sync' && showSyncTab.value) return 'sync'
  return 'rules'
}

function syncTabToRoute(tab: string) {
  const nextQuery = { ...route.query }
  if (tab === 'sync') nextQuery.tab = 'sync'
  else delete nextQuery.tab
  router.replace({ query: nextQuery }).catch(() => undefined)
}

function onTabChange(name: string | number) {
  syncTabToRoute(String(name))
}

function onRulesSaved() {
  /* reserved for future cross-tab refresh */
}

function onSyncApplied() {
  rulesPanelRef.value?.reload()
  activeTab.value = 'rules'
  syncTabToRoute('rules')
}

onMounted(() => {
  activeTab.value = tabFromQuery()
})

watch(
  () => route.query.tab,
  () => {
    activeTab.value = tabFromQuery()
  }
)

watch(showSyncTab, (ok) => {
  if (!ok && activeTab.value === 'sync') {
    activeTab.value = 'rules'
    syncTabToRoute('rules')
  }
})
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
.page-tabs :deep(.el-tabs__content),
.page-tabs :deep(.el-tab-pane) {
  overflow: visible;
}
</style>
