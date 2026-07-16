<template>
  <div class="ab-comparison">
    <div class="usage-header">
      <h2>A/B 对比报表</h2>
      <div class="header-actions">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          @change="loadAll"
        />
        <el-button :loading="loading" style="margin-left: 12px" @click="loadAll">刷新</el-button>
      </div>
    </div>

    <div class="stats-cards">
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-value">{{ summary.total_comparisons }}</div>
          <div class="stat-label">对比次数</div>
        </div>
      </el-card>
      <el-card class="stat-card" shadow="hover">
        <div class="stat-content">
          <div class="stat-value">{{ topWinModel }}</div>
          <div class="stat-label">胜率最高模型</div>
        </div>
      </el-card>
    </div>

    <el-card class="chart-card">
      <template #header><span>模型胜率</span></template>
      <v-chart :option="winChartOption" style="height: 320px" autoresize />
    </el-card>

    <el-card class="table-card" v-loading="loading">
      <template #header><span>历史对比</span></template>
      <el-table :data="results" stripe border>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="model_a" label="模型 A" min-width="120" />
        <el-table-column prop="score_a" label="分数 A" width="90" />
        <el-table-column prop="model_b" label="模型 B" min-width="120" />
        <el-table-column prop="score_b" label="分数 B" width="90" />
        <el-table-column label="胜者" min-width="120">
          <template #default="{ row }">
            {{ row.winner_node === row.candidate_a_node ? row.model_a : row.model_b }}
          </template>
        </el-table-column>
        <el-table-column prop="question" label="问题" min-width="200" show-overflow-tooltip />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import * as abEvalApi from '@/api/ab_eval'

echarts.use([CanvasRenderer, BarChart, GridComponent, TooltipComponent])

const loading = ref(false)
const dateRange = ref<[string, string] | null>(null)
const summary = ref<abEvalApi.ABEvalSummary>({
  total_comparisons: 0,
  from: '',
  to: '',
  model_wins: [],
  avg_score_by_model: [],
  avg_latency_by_model: [],
  avg_cost_by_model: [],
})
const results = ref<abEvalApi.ABEvalResult[]>([])

const topWinModel = computed(() => {
  const wins = summary.value.model_wins || []
  if (!wins.length) return '-'
  return wins.reduce((a, b) => (b.win_rate > a.win_rate ? b : a)).model
})

const winChartOption = computed(() => {
  const wins = summary.value.model_wins || []
  return {
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: wins.map((w) => w.model) },
    yAxis: { type: 'value', max: 1, axisLabel: { formatter: (v: number) => `${(v * 100).toFixed(0)}%` } },
    series: [
      {
        type: 'bar',
        data: wins.map((w) => w.win_rate),
        itemStyle: { color: '#409EFF' },
      },
    ],
  }
})

function queryParams() {
  const params: { from?: string; to?: string } = {}
  if (dateRange.value?.[0]) params.from = dateRange.value[0]
  if (dateRange.value?.[1]) params.to = dateRange.value[1]
  return params
}

async function loadAll() {
  loading.value = true
  try {
    const params = queryParams()
    const [sumPayload, listPayload] = await Promise.all([
      abEvalApi.getABEvalSummary(params),
      abEvalApi.listABEvalResults(params),
    ])
    summary.value = sumPayload
    results.value = listPayload?.results || []
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : '加载 A/B 报表失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)
</script>

<style scoped>
.ab-comparison {
  padding: 20px;
}
.usage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}
.stat-value {
  font-size: 28px;
  font-weight: 600;
}
.stat-label {
  color: #909399;
  margin-top: 4px;
}
.chart-card,
.table-card {
  margin-bottom: 20px;
}
</style>