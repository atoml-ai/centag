<template>
  <div class="cost-dashboard">
    <div class="usage-header">
      <h2>成本看板</h2>
      <div class="header-actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          @change="loadSummary"
        />
        <el-select v-model="groupBy" style="width: 140px; margin-left: 12px" @change="loadSummary">
          <el-option label="按模型" value="model" />
          <el-option label="按后端" value="backend" />
          <el-option label="按租户" value="tenant" />
          <el-option label="按部门" value="dept" />
          <el-option label="按日期" value="date" />
        </el-select>
        <el-radio-group
          v-model="displayCurrency"
          size="default"
          style="margin-left: 12px"
          @change="onDisplayCurrencyChange"
        >
          <el-radio-button value="USD">美元</el-radio-button>
          <el-radio-button value="CNY">人民币</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <div class="stats-cards">
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-value">{{ currencySymbol }}{{ formatCost(summary.total_cost_usd) }}</div>
          <div class="stat-label">总成本 ({{ displayCurrency }})</div>
        </div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-value">{{ formatNumber(summary.total_tokens) }}</div>
          <div class="stat-label">总 Token</div>
        </div>
      </el-card>
      <el-card v-if="cacheSavedTracked" class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-value">{{ currencySymbol }}{{ formatCost(summary.cache_saved_usd) }}</div>
          <div class="stat-label">缓存节省 ({{ displayCurrency }}，估算)</div>
        </div>
      </el-card>
    </div>

    <el-card class="chart-card">
      <template #header>
        <span>成本分布</span>
      </template>
      <v-chart :option="chartOption" style="height: 380px" autoresize />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import * as costApi from '@/api/cost'
import {
  currencySymbol as symbolOf,
  formatDisplayCost,
  getDisplayCurrency,
  setDisplayCurrency,
  toDisplayAmount,
  type DisplayCurrency
} from '@/utils/billing-currency'

echarts.use([CanvasRenderer, BarChart, LineChart, GridComponent, TooltipComponent, LegendComponent])

const dateRange = ref<[string, string] | null>(null)
const groupBy = ref<'model' | 'backend' | 'tenant' | 'date' | 'dept'>('model')
const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
const summary = ref<costApi.CostSummary>({
  total_cost_usd: 0,
  total_tokens: 0,
  cache_saved_usd: 0,
  currency: 'USD',
  usd_to_cny: 7.2,
  groups: [],
  from: '',
  to: '',
  group_by: 'model',
})

const usdToCny = computed(() => summary.value.usd_to_cny || 7.2)
const cacheSavedTracked = computed(() => (summary.value.cache_saved_usd || 0) > 0)
const currencySymbol = computed(() => symbolOf(displayCurrency.value))
const costLegend = computed(() => `成本 (${displayCurrency.value})`)

const chartOption = computed(() => {
  const groups = summary.value.groups || []
  const cur = displayCurrency.value
  const rate = usdToCny.value
  return {
    tooltip: { trigger: 'axis' },
    legend: { data: [costLegend.value, 'Token'] },
    xAxis: {
      type: 'category',
      data: groups.map((g) => g.key),
      axisLabel: { rotate: groups.length > 8 ? 30 : 0 },
    },
    yAxis: [
      { type: 'value', name: cur },
      { type: 'value', name: 'Token' },
    ],
    series: [
      {
        name: costLegend.value,
        type: groupBy.value === 'date' ? 'line' : 'bar',
        data: groups.map((g) => toDisplayAmount(g.cost_usd, cur, rate)),
      },
      {
        name: 'Token',
        type: 'line',
        yAxisIndex: 1,
        data: groups.map((g) => g.tokens),
      },
    ],
  }
})

function formatNumber(n: number) {
  return new Intl.NumberFormat('zh-CN').format(n || 0)
}

function formatCost(n: number) {
  return formatDisplayCost(n, displayCurrency.value, usdToCny.value)
}

function onDisplayCurrencyChange(v: DisplayCurrency | string | number | boolean | undefined) {
  const c = v === 'CNY' ? 'CNY' : 'USD'
  displayCurrency.value = c
  setDisplayCurrency(c)
}

async function loadSummary() {
  try {
    const params: costApi.CostSummaryParams = { group_by: groupBy.value }
    if (dateRange.value) {
      params.from = dateRange.value[0]
      params.to = dateRange.value[1]
    }
    summary.value = await costApi.getCostSummary(params)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : '加载成本数据失败'
    ElMessage.error(msg)
  }
}

onMounted(loadSummary)
</script>

<style scoped>
.cost-dashboard {
  padding: 20px;
}
.usage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.header-actions {
  display: flex;
  align-items: center;
}
.stats-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}
.stat-card .stat-value {
  font-size: 28px;
  font-weight: 600;
}
.stat-card .stat-label {
  color: #909399;
  margin-top: 4px;
}
</style>