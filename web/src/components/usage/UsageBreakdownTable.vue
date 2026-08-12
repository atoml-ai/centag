<template>
  <div class="usage-breakdown">
    <div class="breakdown-header">
      <span class="block-title">{{ t('usageBreakdown.title') }}</span>
      <el-button size="small" :loading="loading" @click="reload">
        {{ t('usageBreakdown.refresh') }}
      </el-button>
    </div>
    <el-table
      v-loading="loading"
      :data="records"
      size="small"
      stripe
      :max-height="mode === 'compact' ? 260 : 500"
      :empty-text="t('usageBreakdown.emptyText')"
    >
      <el-table-column prop="backend_id" :label="t('usageBreakdown.backend')" min-width="110" show-overflow-tooltip />
      <el-table-column prop="model" :label="t('usageBreakdown.model')" min-width="120" show-overflow-tooltip />
      <el-table-column :label="t('usageBreakdown.requests')" width="80" align="center">
        <template #default="{ row }">{{ formatNumber(row.request_count) }}</template>
      </el-table-column>
      <el-table-column :label="t('usageBreakdown.inputTokens')" width="100">
        <template #default="{ row }">{{ formatTokens(row.input_tokens) }}</template>
      </el-table-column>
      <el-table-column :label="t('usageBreakdown.outputTokens')" width="100">
        <template #default="{ row }">{{ formatTokens(row.output_tokens) }}</template>
      </el-table-column>
      <el-table-column :label="t('usageBreakdown.inputPrice')" width="110">
        <template #default="{ row }">${{ formatPrice(row.cost_input_price) }}</template>
      </el-table-column>
      <el-table-column :label="t('usageBreakdown.outputPrice')" width="110">
        <template #default="{ row }">${{ formatPrice(row.cost_output_price) }}</template>
      </el-table-column>
      <el-table-column :label="t('usageBreakdown.totalCost')" width="110">
        <template #default="{ row }">${{ formatCost(row.total_cost) }}</template>
      </el-table-column>
    </el-table>
    <div v-if="records.length" class="breakdown-total">
      <span>{{ t('usageBreakdown.totalCost') }}: ${{ formatCost(summary.total_cost) }}</span>
      <span class="price-unit">($/1M tokens)</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getUsageBreakdown } from '@/api/token-usage'
import { formatTokens } from '@/utils/format'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    mode?: 'compact' | 'full'
    from?: string
    to?: string
  }>(),
  {
    mode: 'compact',
    from: '',
    to: ''
  }
)

interface UsageBreakdownRecord {
  backend_id: string
  model: string
  request_count: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  cost_input_price: number
  cost_output_price: number
  input_cost: number
  output_cost: number
  total_cost: number
}

const loading = ref(false)
const records = ref<UsageBreakdownRecord[]>([])
const summary = ref({
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cost: 0
})

function formatNumber(n: number | undefined | null): string {
  if (!n) return '0'
  if (props.mode === 'full') return Number(n).toLocaleString()
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatPrice(n: number | undefined | null): string {
  return Number(n || 0).toFixed(6)
}

function formatCost(n: number | undefined | null): string {
  const v = Number(n || 0)
  if (v === 0) return '0.00'
  if (v < 0.01) return v.toFixed(6)
  if (v < 1000) return v.toFixed(4)
  return v.toFixed(2)
}

async function reload() {
  loading.value = true
  try {
    const params: { from?: string; to?: string } = {}
    if (props.from) params.from = props.from
    if (props.to) params.to = props.to
    const data: any = await getUsageBreakdown(
      Object.keys(params).length ? params : undefined
    )
    records.value = data?.records ?? []
    if (data?.summary) summary.value = data.summary
  } catch {
    records.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void reload()
})

defineExpose({ reload })
</script>

<style scoped>
.usage-breakdown {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.breakdown-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.block-title {
  font-size: 13px;
  font-weight: 600;
}
.breakdown-total {
  font-size: 12px;
  font-weight: 600;
  text-align: right;
}
.price-unit {
  margin-left: 6px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
</style>
