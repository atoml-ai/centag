<template>
  <div class="usage-analytics">
    <el-tabs v-model="activeTab" class="usage-analytics__tabs">
      <el-tab-pane :label="t('tokenUsage.dailyTrend')" name="trend">
        <div class="analytics-toolbar">
          <el-radio-group v-model="chartDays" size="small" @change="loadDailyUsage">
            <el-radio-button value="7">{{ t('tokenUsage.last7Days') }}</el-radio-button>
            <el-radio-button value="30">{{ t('tokenUsage.last30Days') }}</el-radio-button>
            <el-radio-button value="90">{{ t('tokenUsage.last90Days') }}</el-radio-button>
          </el-radio-group>
        </div>
        <v-chart :option="dailyChartOption" style="height: 380px" autoresize />
      </el-tab-pane>

      <el-tab-pane :label="t('tokenUsage.usageDistribution')" name="backend">
        <div class="analytics-toolbar">
          <el-select
            v-model="selectedBackend"
            clearable
            filterable
            size="small"
            :placeholder="t('tokenUsage.allBackends')"
            class="backend-select"
          >
            <el-option
              v-for="b in backendOptions"
              :key="b.backend_id"
              :label="backendLabel(b)"
              :value="b.backend_id"
            />
          </el-select>
        </div>
        <el-empty
          v-if="!selectedBackend"
          :description="t('tokenUsage.selectBackend')"
          :image-size="80"
        />
        <el-row v-else :gutter="20">
          <el-col :span="12">
            <div class="chart-block-title">{{ t('tokenUsage.modelUsage') }}</div>
            <el-empty v-if="!modelRows.length" :image-size="60" />
            <v-chart v-else :option="modelChartOption" style="height: 320px" autoresize />
          </el-col>
          <el-col :span="12">
            <div class="chart-block-title">{{ t('tokenUsage.keyUsage') }}</div>
            <el-empty v-if="!keyRows.length" :image-size="60" />
            <v-chart v-else :option="keyChartOption" style="height: 320px" autoresize />
          </el-col>
        </el-row>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import { formatTokens } from '@/utils/format'
import * as tokenApi from '@/api/token-usage'

echarts.use([
  CanvasRenderer,
  BarChart,
  LineChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  DataZoomComponent
])

interface BackendStat {
  backend_id: string
  total_tokens: number
  request_count: number
}

interface UsageRecord {
  backend_id: string
  model: string
  total_tokens: number
}

interface AccountStat {
  backend_id: string
  account_id: string
  total_tokens: number
  request_count: number
  success_rate?: number | null
}

const { t } = useI18n()

const activeTab = ref('trend')
const chartDays = ref('30')
const dailyStats = ref<any[]>([])
const backendStats = ref<BackendStat[]>([])
const usageRecords = ref<UsageRecord[]>([])
const accountStats = ref<AccountStat[]>([])
const selectedBackend = ref('')

const backendOptions = computed(() =>
  [...backendStats.value].sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0))
)

function backendLabel(b: BackendStat): string {
  return `${b.backend_id} · ${formatTokens(b.total_tokens)}`
}

const modelRows = computed(() => {
  if (!selectedBackend.value) return []
  return usageRecords.value
    .filter((r) => String(r.backend_id || '') === selectedBackend.value)
    .sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0))
})

const keyRows = computed(() => {
  if (!selectedBackend.value) return []
  return accountStats.value
    .filter(
      (r) =>
        String(r.account_id || '').trim() &&
        String(r.backend_id || '') === selectedBackend.value
    )
    .sort((a, b) => (b.total_tokens ?? 0) - (a.total_tokens ?? 0))
})

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

function horizontalBarOption(rows: Array<{ name: string; value: number }>, color: [string, string]) {
  const sorted = [...rows].sort((a, b) => a.value - b.value)
  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        return `${p.name}<br/>${t('tokenUsage.totalToken')}: ${formatTokens(p.value)}`
      }
    },
    grid: {
      left: '3%',
      right: '6%',
      bottom: '3%',
      top: 16,
      containLabel: true
    },
    xAxis: {
      type: 'value',
      name: t('tokenUsage.tokenCount'),
      axisLabel: { formatter: (v: number) => formatTokens(v) }
    },
    yAxis: {
      type: 'category',
      data: sorted.map((r) => r.name)
    },
    series: [
      {
        name: t('tokenUsage.totalToken'),
        type: 'bar',
        data: sorted.map((r) => r.value),
        barMaxWidth: 22,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
            { offset: 0, color: color[0] },
            { offset: 1, color: color[1] }
          ])
        }
      }
    ]
  }
}

const modelChartOption = computed(() =>
  horizontalBarOption(
    modelRows.value.map((r) => ({ name: r.model, value: r.total_tokens })),
    ['#83bff6', '#188df0']
  )
)

const keyChartOption = computed(() =>
  horizontalBarOption(
    keyRows.value.map((r) => ({ name: r.account_id, value: r.total_tokens })),
    ['#83e6c0', '#2fbf71']
  )
)

async function loadDailyUsage() {
  try {
    const res: any = await tokenApi.getDailyUsage({ days: parseInt(chartDays.value) })
    dailyStats.value = res.daily_stats ?? []
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadDailyTrendFailed') + '：' + error.message)
  }
}

async function loadBackendStats() {
  try {
    const res: any = await tokenApi.getBackendStats({ days: 30 })
    backendStats.value = res.backend_stats ?? []
    if (selectedBackend.value && !backendStats.value.some((b) => b.backend_id === selectedBackend.value)) {
      selectedBackend.value = ''
    }
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadBackendStatsFailed') + '：' + error.message)
  }
}

async function loadUsageRecords() {
  try {
    const res: any = await tokenApi.getUsageBreakdown()
    usageRecords.value = res.records ?? []
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadUsageBreakdownFailed') + '：' + error.message)
  }
}

async function loadAccountStats() {
  try {
    const res: any = await tokenApi.getAccountStats({ days: 30 })
    accountStats.value = res.account_stats ?? []
  } catch (error: any) {
    ElMessage.error(t('tokenUsage.loadAccountStatsFailed') + '：' + error.message)
  }
}

async function reload() {
  await Promise.all([loadDailyUsage(), loadBackendStats(), loadUsageRecords(), loadAccountStats()])
}

onMounted(reload)

defineExpose({ reload })
</script>

<style scoped>
.usage-analytics {
  width: 100%;
}

.analytics-toolbar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-bottom: 8px;
}

.backend-select {
  width: 220px;
}

.chart-block-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

@media (max-width: 768px) {
  .backend-select {
    width: 100%;
  }
}
</style>
