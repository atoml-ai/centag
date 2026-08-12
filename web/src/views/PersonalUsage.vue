<template>
  <div class="personal-usage-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>{{ t('personalUsage.title') }}</span>
          <div class="header-actions">
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              range-separator="-"
              :start-placeholder="t('personalUsage.startDate')"
              :end-placeholder="t('personalUsage.endDate')"
              size="small"
              @change="loadUsage"
            />
            <el-button size="small" @click="loadUsage">{{ t('personalUsage.refresh') }}</el-button>
          </div>
        </div>
      </template>

      <!-- Usage Summary -->
      <el-row :gutter="16" class="usage-summary">
        <el-col :span="6">
          <div class="usage-stat">
            <div class="stat-value">{{ formatNumber(usageSummary.total_tokens) }}</div>
            <div class="stat-label">{{ t('personalUsage.totalTokens') }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="usage-stat">
            <div class="stat-value">{{ formatNumber(usageSummary.input_tokens) }}</div>
            <div class="stat-label">{{ t('personalUsage.inputTokens') }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="usage-stat">
            <div class="stat-value">{{ formatNumber(usageSummary.output_tokens) }}</div>
            <div class="stat-label">{{ t('personalUsage.outputTokens') }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="usage-stat">
            <div class="stat-value">${{ usageSummary.total_cost.toFixed(4) }}</div>
            <div class="stat-label">{{ t('personalUsage.totalCost') }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- Usage Details Table -->
      <el-table
        v-loading="loading"
        :data="usageRecords"
        stripe
        size="small"
        :empty-text="t('personalUsage.emptyText')"
        class="usage-table"
        max-height="500"
      >
        <el-table-column prop="model" :label="t('personalUsage.table.model')" width="140" />
        <el-table-column prop="backend_id" :label="t('personalUsage.table.backend')" width="120" />
        <el-table-column :label="t('personalUsage.table.inputTokens')" width="100">
          <template #default="{ row }">{{ formatNumber(row.input_tokens) }}</template>
        </el-table-column>
        <el-table-column :label="t('personalUsage.table.outputTokens')" width="100">
          <template #default="{ row }">{{ formatNumber(row.output_tokens) }}</template>
        </el-table-column>
        <el-table-column :label="t('personalUsage.table.inputPrice')" width="120">
          <template #default="{ row }">${{ row.cost_input_price.toFixed(6) }}</template>
        </el-table-column>
        <el-table-column :label="t('personalUsage.table.outputPrice')" width="120">
          <template #default="{ row }">${{ row.cost_output_price.toFixed(6) }}</template>
        </el-table-column>
        <el-table-column :label="t('personalUsage.table.totalCost')" width="120">
          <template #default="{ row }">${{ row.total_cost.toFixed(6) }}</template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Self Limit Card -->
    <el-card shadow="never" class="self-limit-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('personalUsage.selfLimit.title') }}</span>
          <el-tag type="info" size="small">{{ t('personalUsage.selfLimit.readonly') }}</el-tag>
        </div>
      </template>

      <div class="self-limit-info">
        <el-row :gutter="16">
          <el-col :span="12">
            <div class="limit-item">
              <div class="limit-label">{{ t('personalUsage.selfLimit.dailyTokenLimit') }}</div>
              <div class="limit-value">
                {{ selfLimit.daily_token_limit != null ? formatNumber(selfLimit.daily_token_limit) : t('personalUsage.selfLimit.notSet') }}
              </div>
            </div>
          </el-col>
          <el-col :span="12">
            <div class="limit-item">
              <div class="limit-label">{{ t('personalUsage.selfLimit.monthlyBudgetLimit') }}</div>
              <div class="limit-value">
                {{ selfLimit.monthly_budget_limit != null ? `$${selfLimit.monthly_budget_limit.toFixed(2)}` : t('personalUsage.selfLimit.notSet') }}
              </div>
            </div>
          </el-col>
        </el-row>
        <el-row :gutter="16" class="limit-row">
          <el-col :span="12">
            <div class="limit-item">
              <div class="limit-label">{{ t('personalUsage.selfLimit.enabled') }}</div>
              <div class="limit-value">
                <el-tag :type="selfLimit.enabled ? 'success' : 'info'" size="small">
                  {{ selfLimit.enabled ? t('personalUsage.selfLimit.enabledYes') : t('personalUsage.selfLimit.enabledNo') }}
                </el-tag>
              </div>
            </div>
          </el-col>
        </el-row>
      </div>

      <div class="self-limit-description">
        {{ t('personalUsage.selfLimit.description') }}
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getUsageBreakdown, getSelfLimit } from '@/api/token-usage'
import { formatTokens } from '@/utils/format'

const { t } = useI18n()

const loading = ref(false)
const dateRange = ref<[Date, Date] | null>(null)

interface UsageRecord {
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

interface UsageSummary {
  total_input_tokens: number
  total_output_tokens: number
  total_tokens: number
  total_cost: number
}

interface SelfLimit {
  enabled: boolean
  daily_token_limit?: number
  monthly_budget_limit?: number
}

const usageRecords = ref<UsageRecord[]>([])
const usageSummary = ref<UsageSummary>({
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_tokens: 0,
  total_cost: 0
})

const selfLimit = ref<SelfLimit>({
  enabled: false,
  daily_token_limit: undefined,
  monthly_budget_limit: undefined
})

function formatNumber(n: number): string {
  return formatTokens(n)
}

async function loadUsage() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (dateRange.value) {
      params.from = dateRange.value[0].toISOString().split('T')[0]
      params.to = dateRange.value[1].toISOString().split('T')[0]
    }

    // Load usage breakdown (per backend × model)
    const usageData = await getUsageBreakdown(params)
    usageRecords.value = usageData.records || []
    if (usageData.summary) {
      usageSummary.value = usageData.summary
    }

    // Load self-limit configuration
    selfLimit.value = await getSelfLimit()
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('personalUsage.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadUsage()
})
</script>

<style scoped>
.personal-usage-page {
  padding: 16px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.usage-summary {
  margin-bottom: 24px;
}
.usage-stat {
  text-align: center;
  padding: 16px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
}
.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--el-color-primary);
  margin-bottom: 4px;
}
.stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.usage-table {
  width: 100%;
}
.self-limit-card {
  margin-top: 16px;
}
.self-limit-info {
  margin-bottom: 16px;
}
.limit-item {
  padding: 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
  margin-bottom: 12px;
}
.limit-row {
  margin-top: 0;
}
.limit-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
.limit-value {
  font-size: 16px;
  font-weight: 500;
}
.self-limit-description {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  font-style: italic;
  padding: 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
}
</style>
