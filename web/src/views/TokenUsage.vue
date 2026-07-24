<template>
  <div class="token-usage">
    <div class="usage-header">
      <h2>{{ pageTitle }}</h2>
    </div>

    <UsageMetricsSummary
      ref="metricsRef"
      mode="full"
      :hint="pageHint"
      show-billing-button
      @open-billing="billingVisible = true"
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
          <v-chart :option="backendChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>
    </el-row>

    <BillingRulesDialog v-model="billingVisible" @saved="onBillingSaved" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
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
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'

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

const { t } = useI18n()

const route = useRoute()
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
const billingVisible = ref(false)

const chartDays = ref('30')
const dailyStats = ref<any[]>([])
const modelStats = ref<any[]>([])
const backendStats = ref<any[]>([])

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

function onBillingSaved() {
  metricsRef.value?.reload()
}

function syncBillingQuery() {
  if (!canManageBilling.value) return
  if (route.query.billing === '1' || route.query.billing === 'true') {
    billingVisible.value = true
  }
}

watch(billingVisible, (open) => {
  if (!open && (route.query.billing === '1' || route.query.billing === 'true')) {
    const q = { ...route.query }
    delete q.billing
    router.replace({ path: '/token-usage', query: q })
  }
})

onMounted(() => {
  syncBillingQuery()
  metricsRef.value?.reload()
  loadDailyUsage()
  loadModelStats()
  loadBackendStats()
})
</script>

<style scoped>
.token-usage {
  padding: 20px;
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

@media (max-width: 768px) {
  .usage-header {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
