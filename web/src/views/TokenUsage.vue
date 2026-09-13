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

    <el-row :gutter="20" class="chart-row">
      <el-col :span="24">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('tokenUsage.dailyTrend') }}</span>
              <el-radio-group v-model="chartDays" size="small" @change="loadDailyUsage">
                <el-radio-button value="7">{{ $t('tokenUsage.last7Days') }}</el-radio-button>
                <el-radio-button value="30">{{ $t('tokenUsage.last30Days') }}</el-radio-button>
                <el-radio-button value="90">{{ $t('tokenUsage.last90Days') }}</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <v-chart :option="dailyChartOption" style="height: 420px" autoresize />
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="chart-row">
      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('tokenUsage.modelTop5') }}</span>
              <el-button type="primary" link size="small" @click="loadModelStats">{{ $t('tokenUsage.refresh') }}</el-button>
            </div>
          </template>
          <v-chart :option="modelChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('tokenUsage.backendTop5') }}</span>
              <el-button type="primary" link size="small" @click="loadBackendStats">{{ $t('tokenUsage.refresh') }}</el-button>
            </div>
          </template>
          <v-chart :option="backendChartOption" style="height: 300px" autoresize @click="onBackendChartClick" />
          <div v-if="expandedBackend" class="account-detail">
            <div class="detail-title">
              {{ expandedBackend }} · {{ $t('tokenUsage.keyDetail') }}
            </div>
            <el-table :data="expandedAccountRows" stripe size="small" max-height="240">
              <el-table-column prop="account_id" :label="$t('tokenUsage.colKey')" min-width="160" />
              <el-table-column prop="request_count" :label="$t('tokenUsage.colRequests')" width="110" />
              <el-table-column :label="$t('tokenUsage.colTokens')" width="120">
                <template #default="{ row }">{{ formatTokenCount(row.total_tokens) }}</template>
              </el-table-column>
              <el-table-column v-if="expandedHasSuccessRate" :label="$t('tokenUsage.colSuccessRate')" width="110">
                <template #default="{ row }">
                  {{ row.success_rate == null ? '-' : (row.success_rate * 100).toFixed(1) + '%' }}
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" class="chart-row">
      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ $t('tokenUsage.accountTop5') }}</span>
              <div class="card-header-actions">
                <el-select
                  v-model="accountBackend"
                  clearable
                  filterable
                  size="small"
                  :placeholder="$t('tokenUsage.allBackends')"
                  style="width: 160px"
                >
                  <el-option
                    v-for="b in accountBackendOptions"
                    :key="b"
                    :label="b"
                    :value="b"
                  />
                </el-select>
                <el-button type="primary" link size="small" @click="loadAccountStats">{{ $t('tokenUsage.refresh') }}</el-button>
              </div>
            </div>
          </template>
          <v-chart :option="accountChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>
    </el-row>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import * as tokenApi from '@/api/token-usage'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'

echarts.use([
  CanvasRenderer,
  BarChart,
  LineChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent
])

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

const chartDays = ref('30')
const dailyStats = ref<any[]>([])
const modelStats = ref<any[]>([])
const backendStats = ref<any[]>([])
const accountStats = ref<any[]>([])

const dailyChartOption = computed(() => {
  const rows = [...dailyStats.value].reverse()
  const n = rows.length
  const windowSize = 14
  const start = n > windowSize ? ((n - windowSize) / n) * 100 : 0
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      orient: 'vertical',
      left: 8,
      top: 'middle',
      data: [t('tokenUsage.inputToken'), t('tokenUsage.outputToken'), t('tokenUsage.totalToken')]
    },
    grid: {
      left: 120,
      right: 28,
      top: 28,
      bottom: 72,
      containLabel: true
    },
    dataZoom: [
      {
        type: 'inside',
        xAxisIndex: 0,
        filterMode: 'filter',
        zoomOnMouseWheel: true,
        moveOnMouseMove: true,
        moveOnMouseWheel: false
      },
      {
        type: 'slider',
        xAxisIndex: 0,
        height: 22,
        bottom: 12,
        start,
        end: 100,
        brushSelect: false
      }
    ],
    xAxis: {
      type: 'category',
      data: rows.map((s) => s.date),
      axisLabel: {
        hideOverlap: true,
        rotate: n > 14 ? 35 : 0
      },
      axisTick: { alignWithLabel: true }
    },
    yAxis: {
      type: 'value',
      name: t('tokenUsage.tokenCount')
    },
    series: [
      {
        name: t('tokenUsage.inputToken'),
        type: 'bar',
        stack: 'total',
        data: rows.map((s) => s.prompt_tokens),
        itemStyle: { color: '#5470c6' },
        barMaxWidth: 28
      },
      {
        name: t('tokenUsage.outputToken'),
        type: 'bar',
        stack: 'total',
        data: rows.map((s) => s.comp_tokens),
        itemStyle: { color: '#91cc75' },
        barMaxWidth: 28
      },
      {
        name: t('tokenUsage.totalToken'),
        type: 'line',
        data: rows.map((s) => s.total_tokens),
        itemStyle: { color: '#fac858' },
        lineStyle: { width: 3 },
        symbolSize: 6
      }
    ]
  }
})

const modelChartOption = computed(() => ({
  tooltip: {
    trigger: 'item',
    formatter: '{b}: {c} ({d}%)'
  },
  legend: {
    orient: 'vertical',
    left: 'left'
  },
  series: [
    {
      name: t('tokenUsage.modelUsage'),
      type: 'pie',
      radius: '50%',
      data: modelStats.value.map((s) => ({
        name: s.model,
        value: s.total_tokens
      })),
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)'
        }
      }
    }
  ]
}))

const backendChartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'value',
    name: t('tokenUsage.tokenCount')
  },
  yAxis: {
    type: 'category',
    data: backendStats.value.map((s) => s.backend_id).reverse()
  },
  series: [
    {
      name: t('tokenUsage.totalToken'),
      type: 'bar',
      data: backendStats.value.map((s) => s.total_tokens).reverse(),
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
          { offset: 0, color: '#83bff6' },
          { offset: 0.5, color: '#188df0' },
          { offset: 1, color: '#188df0' }
        ])
      }
    }
  ]
}))

const accountBackend = ref('')
// 点击后端条形图展开该后端下的各 Key 计量明细（数据与 Key 卡片同源、同一 30 天窗口）
const expandedBackend = ref('')
const expandedAccountRows = computed(() => {
  if (!expandedBackend.value) return []
  return accountStats.value
    .filter((s) => String(s.account_id || '').trim() && String(s.backend_id || '') === expandedBackend.value)
    .sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0))
})
const expandedHasSuccessRate = computed(() =>
  expandedAccountRows.value.some((r) => r.success_rate != null)
)

function formatTokenCount(n: number): string {
  if (n == null) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k'
  return String(n)
}

function onBackendChartClick(params: { name?: string }) {
  const name = String(params?.name ?? '').trim()
  if (!name) return
  expandedBackend.value = expandedBackend.value === name ? '' : name
}
const accountBackendOptions = computed(() =>
  [...new Set(accountStats.value.filter((s) => String(s.account_id || '').trim()).map((s) => String(s.backend_id || '')))]
    .filter(Boolean)
    .sort()
)

// 只统计真正归属某个 Key 的行（account_id 为空的行是该后端未按 Key 计量的
// 合计，不属于 "Key 使用 TOP5" 语义）。
const accountRows = computed(() => {
  const scope = accountBackend.value?.trim()
  return accountStats.value.filter(
    (s) => String(s.account_id || '').trim() && (!scope || String(s.backend_id || '') === scope)
  )
})

// 图表恒为全局 Token 数 Top5 的 Key，横轴标签固定 "key · backend" 以表明归属
const accountTopRows = computed(() =>
  [...accountRows.value].sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0)).slice(0, 5)
)

const accountChartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' }
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'value',
    name: t('tokenUsage.tokenCount')
  },
  yAxis: {
    type: 'category',
    data: accountTopRows.value
      .map((s) => `${s.account_id} · ${s.backend_id}`)
      .reverse()
  },
  series: [
    {
      name: t('tokenUsage.totalToken'),
      type: 'bar',
      data: accountTopRows.value.map((s) => s.total_tokens).reverse(),
      itemStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
          { offset: 0, color: '#83e6c0' },
          { offset: 1, color: '#2fbf71' }
        ])
      }
    }
  ]
}))

const loadDailyUsage = async () => {
  try {
    const res = await tokenApi.getDailyUsage({ days: parseInt(chartDays.value) })
    dailyStats.value = res.daily_stats ?? []
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadDailyTrendFailed') + '：' + error.message)
  }
}

const loadModelStats = async () => {
  try {
    const res = await tokenApi.getModelStats({ days: 30 })
    modelStats.value = (res.model_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadModelStatsFailed') + '：' + error.message)
  }
}

const loadBackendStats = async () => {
  try {
    const res = await tokenApi.getBackendStats({ days: 30 })
    backendStats.value = (res.backend_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadBackendStatsFailed') + '：' + error.message)
  }
}

const loadAccountStats = async () => {
  try {
    const res = await tokenApi.getAccountStats({ days: 30 })
    accountStats.value = res.account_stats ?? []
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadAccountStatsFailed') + '：' + error.message)
  }
}

onMounted(() => {
  metricsRef.value?.reload()
  loadDailyUsage()
  loadModelStats()
  loadBackendStats()
  loadAccountStats()
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

.chart-row {
  margin-bottom: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.account-detail {
  margin-top: 8px;
  border-top: 1px solid #ebeef5;
  padding-top: 8px;
}

.detail-title {
  font-size: 13px;
  font-weight: 600;
  color: #606266;
  margin-bottom: 6px;
}

@media (max-width: 768px) {
  .usage-header {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
