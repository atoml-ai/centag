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

    <!-- 趋势图表 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :span="24">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>每日使用趋势</span>
              <el-radio-group v-model="chartDays" size="small" @change="loadDailyUsage">
                <el-radio-button value="7">7 天</el-radio-button>
                <el-radio-button value="30">30 天</el-radio-button>
                <el-radio-button value="90">90 天</el-radio-button>
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
              <span>模型使用 TOP5</span>
              <el-button type="primary" link size="small" @click="loadModelStats">刷新</el-button>
            </div>
          </template>
          <v-chart :option="modelChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>后端使用 TOP5</span>
              <el-button type="primary" link size="small" @click="loadBackendStats">刷新</el-button>
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

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { isTeam } = useEdition()
// Admin ranking/quotas live in Team pack (centag-pro); this page is self-service + billing entry only.
const canManageBilling = computed(() => !(isTeam.value && !authStore.isAdmin))
const pageTitle = computed(() => (canManageBilling.value ? '用量与计费' : '用量'))
const pageHint = computed(() =>
  canManageBilling.value
    ? 'Token 计量、成本汇总与计费规则入口。首页「用量与会话」为简易查询。'
    : 'Token 计量与成本汇总。计费规则由管理员配置。'
)

const metricsRef = ref<InstanceType<typeof UsageMetricsSummary> | null>(null)
const billingVisible = ref(false)

const chartDays = ref('30')
const dailyStats = ref<any[]>([])
const modelStats = ref<any[]>([])
const backendStats = ref<any[]>([])

const dailyChartOption = computed(() => {
  // API 多为新→旧；反转后旧→新（左→右）
  const rows = [...dailyStats.value].reverse()
  const n = rows.length
  // 默认窗口：最多展示约 14 天，其余靠缩放查看
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
      data: ['输入 Token', '输出 Token', '总 Token']
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
      name: 'Token 数'
    },
    series: [
      {
        name: '输入 Token',
        type: 'bar',
        stack: 'total',
        data: rows.map((s) => s.prompt_tokens),
        itemStyle: { color: '#5470c6' },
        barMaxWidth: 28
      },
      {
        name: '输出 Token',
        type: 'bar',
        stack: 'total',
        data: rows.map((s) => s.comp_tokens),
        itemStyle: { color: '#91cc75' },
        barMaxWidth: 28
      },
      {
        name: '总 Token',
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
      name: '模型使用',
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
    name: 'Token 数'
  },
  yAxis: {
    type: 'category',
    data: backendStats.value.map((s) => s.backend_id).reverse()
  },
  series: [
    {
      name: '总 Token',
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
    ElMessage.error('加载每日趋势失败：' + error.message)
  }
}

const loadModelStats = async () => {
  try {
    const res = await tokenApi.getModelStats({ days: 30 })
    modelStats.value = (res.model_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error('加载模型统计失败：' + error.message)
  }
}

const loadBackendStats = async () => {
  try {
    const res = await tokenApi.getBackendStats({ days: 30 })
    backendStats.value = (res.backend_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error('加载后端统计失败：' + error.message)
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
