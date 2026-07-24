<template>
  <div class="usage-metrics" :class="mode">
    <div class="filter-bar">
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        size="small"
        :range-separator="t('usageMetricsSummary.dateSeparator')"
        :start-placeholder="t('usageMetricsSummary.dateStart')"
        :end-placeholder="t('usageMetricsSummary.dateEnd')"
        value-format="YYYY-MM-DD"
        style="width: 260px"
        @change="reload"
      />
      <el-select v-model="groupBy" size="small" style="width: 120px" @change="reload">
        <el-option :label="t('usageMetricsSummary.groupByModel')" value="model" />
        <el-option :label="t('usageMetricsSummary.groupByBackend')" value="backend" />
        <el-option :label="t('usageMetricsSummary.groupByDate')" value="date" />
      </el-select>
      <el-button size="small" :loading="loading" @click="reload">{{ t('usageMetricsSummary.refresh') }}</el-button>
      <el-button v-if="effectiveShowBilling" size="small" type="primary" plain @click="emit('open-billing')">
        {{ t('usageMetricsSummary.billingRules') }}
      </el-button>
      <el-radio-group v-model="displayCurrency" size="small" @change="onDisplayCurrencyChange">
        <el-radio-button value="USD">{{ t('usageMetricsSummary.currencyUSD') }}</el-radio-button>
        <el-radio-button value="CNY">{{ t('usageMetricsSummary.currencyCNY') }}</el-radio-button>
      </el-radio-group>
      <slot name="actions" />
    </div>

    <div class="usage-stats">
      <div class="stat">
        <div class="stat-value">{{ currencySymbol }}{{ formatCost(summary.total_cost_usd) }}</div>
        <div class="stat-label">{{ t('usageMetricsSummary.totalCost', { currency: displayCurrency }) }}</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(summary.total_tokens || stats.total_tokens) }}</div>
        <div class="stat-label">{{ t('usageMetricsSummary.tokens') }}</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.request_count) }}</div>
        <div class="stat-label">{{ t('usageMetricsSummary.requests') }}</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.total_prompt_tokens) }}</div>
        <div class="stat-label">{{ t('usageMetricsSummary.input') }}</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ formatNumber(stats.total_completion_tokens) }}</div>
        <div class="stat-label">{{ t('usageMetricsSummary.output') }}</div>
      </div>
    </div>

    <div v-if="summary.groups?.length" class="groups-block">
      <div class="block-title">{{ t('usageMetricsSummary.costDistribution') }}</div>
      <el-table :data="summary.groups" size="small" stripe :max-height="mode === 'compact' ? 180 : 320">
        <el-table-column prop="key" :label="groupByLabel" min-width="120" />
        <el-table-column :label="t('usageMetricsSummary.costColumn')" width="110">
          <template #default="{ row }">{{ currencySymbol }}{{ formatCost(row.cost_usd) }}</template>
        </el-table-column>
        <el-table-column prop="tokens" :label="t('usageMetricsSummary.tokenColumn')" width="100">
          <template #default="{ row }">{{ formatNumber(row.tokens) }}</template>
        </el-table-column>
        <el-table-column prop="request_count" :label="t('usageMetricsSummary.requestColumn')" width="80" />
      </el-table>
    </div>

    <p v-if="hint" class="hint">{{ hint }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUserUsage } from '@/api/token-usage'
import * as costApi from '@/api/cost'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import {
  currencySymbol as symbolOf,
  formatDisplayCost,
  getDisplayCurrency,
  setDisplayCurrency,
  type DisplayCurrency
} from '@/utils/billing-currency'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    mode?: 'compact' | 'full'
    hint?: string
    showBillingButton?: boolean
  }>(),
  {
    mode: 'compact',
    hint: '',
    showBillingButton: true
  }
)

const authStore = useAuthStore()
const { isTeam } = useEdition()
const effectiveShowBilling = computed(
  () => props.showBillingButton && !(isTeam.value && !authStore.isAdmin)
)

const emit = defineEmits<{
  'open-billing': []
  loaded: []
}>()

const loading = ref(false)
const dateRange = ref<[string, string] | null>(null)
const groupBy = ref<'model' | 'backend' | 'date'>('model')

const stats = reactive({
  total_tokens: 0,
  total_prompt_tokens: 0,
  total_completion_tokens: 0,
  request_count: 0
})

const summary = ref<costApi.CostSummary>({
  total_cost_usd: 0,
  total_tokens: 0,
  cache_saved_usd: 0,
  currency: 'USD',
  usd_to_cny: 7.2,
  groups: [],
  from: '',
  to: '',
  group_by: 'model'
})

const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
const usdToCny = computed(() => summary.value.usd_to_cny || 7.2)
const currencySymbol = computed(() => symbolOf(displayCurrency.value))
const groupByLabel = computed(() => {
  if (groupBy.value === 'backend') return t('usageMetricsSummary.groupByBackendLabel')
  if (groupBy.value === 'date') return t('usageMetricsSummary.groupByDateLabel')
  return t('usageMetricsSummary.groupByModelLabel')
})

function formatNumber(n: number | undefined | null): string {
  if (!n) return '0'
  if (props.mode === 'full') return Number(n).toLocaleString()
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatCost(n: number | undefined | null): string {
  return formatDisplayCost(n, displayCurrency.value, usdToCny.value)
}

function onDisplayCurrencyChange(v: DisplayCurrency | string | number | boolean | undefined) {
  const c = v === 'CNY' ? 'CNY' : 'USD'
  displayCurrency.value = c
  setDisplayCurrency(c)
}

async function loadUsage() {
  try {
    const res: any = await getUserUsage(
      dateRange.value ? { from: dateRange.value[0], to: dateRange.value[1] } : undefined
    )
    const data = res?.stats ?? res?.data?.stats ?? res
    stats.total_tokens = Number(data?.total_tokens || 0)
    stats.total_prompt_tokens = Number(data?.total_prompt_tokens || 0)
    stats.total_completion_tokens = Number(data?.total_completion_tokens || 0)
    stats.request_count = Number(data?.request_count || 0)
  } catch {
  }
}

async function loadCostSummary() {
  try {
    const params: costApi.CostSummaryParams = { group_by: groupBy.value }
    if (dateRange.value) {
      params.from = dateRange.value[0]
      params.to = dateRange.value[1]
    }
    summary.value = await costApi.getCostSummary(params)
  } catch (err: any) {
    const status = err?.response?.status ?? err?.status
    summary.value = {
      ...summary.value,
      total_cost_usd: 0,
      groups: []
    }
    if (status === 403) {
      console.warn('[usage] cost/summary unavailable (403); rebuild/update if on personal edition')
    }
  }
}

async function reload() {
  loading.value = true
  try {
    await Promise.all([loadUsage(), loadCostSummary()])
    emit('loaded')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void reload()
})

defineExpose({
  reload,
  dateRange,
  stats
})
</script>

<style scoped>
.usage-metrics {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.usage-stats {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}
.usage-metrics.full .usage-stats {
  gap: 16px;
}
.stat {
  background: var(--el-fill-color-light);
  border-radius: 8px;
  padding: 10px 8px;
  text-align: center;
}
.usage-metrics.full .stat {
  padding: 16px 12px;
}
.stat-value {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.2;
}
.usage-metrics.full .stat-value {
  font-size: 22px;
}
.stat-label {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.groups-block {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 8px;
}
.block-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
}
.hint {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
@media (max-width: 900px) {
  .usage-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
