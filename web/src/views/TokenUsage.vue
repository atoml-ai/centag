<template>
  <div class="token-usage">
    <div class="usage-header">
      <h2>📊 Token 使用统计</h2>
      <div class="date-range">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          @change="loadUsage"
        />
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-cards">
      <el-card class="stat-card" shadow="hover">
        <div class="stat-icon total">
          <el-icon><DataLine /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ formatNumber(stats.total_tokens) }}</div>
          <div class="stat-label">总 Token 数</div>
        </div>
      </el-card>

      <el-card class="stat-card" shadow="hover">
        <div class="stat-icon prompt">
          <el-icon><Upload /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ formatNumber(stats.total_prompt_tokens) }}</div>
          <div class="stat-label">输入 Token</div>
        </div>
      </el-card>

      <el-card class="stat-card" shadow="hover">
        <div class="stat-icon completion">
          <el-icon><Download /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ formatNumber(stats.total_completion_tokens) }}</div>
          <div class="stat-label">输出 Token</div>
        </div>
      </el-card>

      <el-card class="stat-card" shadow="hover">
        <div class="stat-icon requests">
          <el-icon><List /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ formatNumber(stats.request_count) }}</div>
          <div class="stat-label">请求次数</div>
        </div>
      </el-card>
    </div>

    <!-- 趋势图表 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :span="24">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>📈 每日使用趋势</span>
              <el-radio-group v-model="chartDays" size="small" @change="loadDailyUsage">
                <el-radio-button value="7">7 天</el-radio-button>
                <el-radio-button value="30">30 天</el-radio-button>
                <el-radio-button value="90">90 天</el-radio-button>
              </el-radio-group>
            </div>
          </template>
          <v-chart :option="dailyChartOption" style="height: 400px" autoresize />
        </el-card>
      </el-col>
    </el-row>

    <!-- 模型和后端统计 -->
    <el-row :gutter="20" class="chart-row">
      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>🤖 模型使用 TOP5</span>
              <el-button type="text" size="small" @click="loadModelStats">刷新</el-button>
            </div>
          </template>
          <v-chart :option="modelChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>⚙️ 后端使用 TOP5</span>
              <el-button type="text" size="small" @click="loadBackendStats">刷新</el-button>
            </div>
          </template>
          <v-chart :option="backendChartOption" style="height: 300px" autoresize />
        </el-card>
      </el-col>
    </el-row>

    <!-- 团队版管理员：用户排行 -->
    <el-card v-if="showUserRanking" class="ranking-card">
      <template #header>
        <div class="card-header">
          <span>🏆 用户 Token 使用排行</span>
          <el-radio-group v-model="rankingDays" size="small" @change="loadRanking">
            <el-radio-button value="7">7 天</el-radio-button>
            <el-radio-button value="30">30 天</el-radio-button>
          </el-radio-group>
        </div>
      </template>
      <el-table :data="ranking" stripe style="width: 100%">
        <el-table-column type="index" label="排名" width="80" align="center" />
        <el-table-column prop="username" label="用户" />
        <el-table-column prop="total_tokens" label="总 Token 数" sortable>
          <template #default="{ row }">
            {{ formatNumber(row.total_tokens) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" align="center">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="showQuotaDialog(row)">设置配额</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 配额设置对话框 -->
    <el-dialog v-model="quotaDialogVisible" title="设置 Token 配额" width="500px">
      <el-form :model="quotaForm" label-width="100px">
        <el-form-item label="用户">
          <el-input v-model="quotaForm.username" disabled />
        </el-form-item>
        <el-form-item label="日限额" :error="quotaErrors.daily">
          <el-input-number v-model="quotaForm.daily_limit" :min="0" :step="1000" style="width: 100%" />
          <div class="form-tip">0 表示无限制</div>
        </el-form-item>
        <el-form-item label="月限额" :error="quotaErrors.monthly">
          <el-input-number v-model="quotaForm.monthly_limit" :min="0" :step="10000" style="width: 100%" />
          <div class="form-tip">0 表示无限制</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="quotaDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="quotaLoading" @click="saveQuota">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { ElMessage } from 'element-plus'
import { DataLine, Upload, Download, List } from '@element-plus/icons-vue'
import * as echarts from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import * as tokenApi from '@/api/token-usage'

echarts.use([CanvasRenderer, BarChart, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent])

const authStore = useAuthStore()
const { isPersonal } = useEdition()
const showUserRanking = computed(() => authStore.isAdmin && !isPersonal.value)

// 统计数据
const stats = ref({
  total_tokens: 0,
  total_prompt_tokens: 0,
  total_completion_tokens: 0,
  request_count: 0,
})

const dateRange = ref<[string, string] | null>(null)

// 图表数据
const chartDays = ref('30')
const dailyStats = ref<any[]>([])
const modelStats = ref<any[]>([])
const backendStats = ref<any[]>([])
const ranking = ref<any[]>([])
const rankingDays = ref('30')

// 配额对话框
const quotaDialogVisible = ref(false)
const quotaLoading = ref(false)
const quotaForm = ref({
  user_id: 0,
  username: '',
  daily_limit: 0,
  monthly_limit: 0,
})
const quotaErrors = ref({ daily: '', monthly: '' })

// 每日趋势图表
const dailyChartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' },
  },
  legend: {
    data: ['输入 Token', '输出 Token', '总 Token'],
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true,
  },
  xAxis: {
    type: 'category',
    data: dailyStats.value.map((s) => s.date).reverse(),
  },
  yAxis: {
    type: 'value',
    name: 'Token 数',
  },
  series: [
    {
      name: '输入 Token',
      type: 'bar',
      stack: 'total',
      data: dailyStats.value.map((s) => s.prompt_tokens).reverse(),
      itemStyle: { color: '#5470c6' },
    },
    {
      name: '输出 Token',
      type: 'bar',
      stack: 'total',
      data: dailyStats.value.map((s) => s.comp_tokens).reverse(),
      itemStyle: { color: '#91cc75' },
    },
    {
      name: '总 Token',
      type: 'line',
      data: dailyStats.value.map((s) => s.total_tokens).reverse(),
      itemStyle: { color: '#fac858' },
      lineStyle: { width: 3 },
    },
  ],
}))

// 模型使用图表
const modelChartOption = computed(() => ({
  tooltip: {
    trigger: 'item',
    formatter: '{b}: {c} ({d}%)',
  },
  legend: {
    orient: 'vertical',
    left: 'left',
  },
  series: [
    {
      name: '模型使用',
      type: 'pie',
      radius: '50%',
      data: modelStats.value.map((s) => ({
        name: s.model,
        value: s.total_tokens,
      })),
      emphasis: {
        itemStyle: {
          shadowBlur: 10,
          shadowOffsetX: 0,
          shadowColor: 'rgba(0, 0, 0, 0.5)',
        },
      },
    },
  ],
}))

// 后端使用图表
const backendChartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' },
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true,
  },
  xAxis: {
    type: 'value',
    name: 'Token 数',
  },
  yAxis: {
    type: 'category',
    data: backendStats.value.map((s) => s.backend_id).reverse(),
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
          { offset: 1, color: '#188df0' },
        ]),
      },
    },
  ],
}))

// 格式化数字
const formatNumber = (num: number) => {
  return num.toLocaleString()
}

// 加载使用统计
const loadUsage = async () => {
  try {
    const params: any = {}
    if (dateRange.value) {
      params.from = dateRange.value[0]
      params.to = dateRange.value[1]
    }
    const res = await tokenApi.getUserUsage(params)
    // api 拦截器已解包 { success, data }，此处 res 即为 data
    stats.value = res.stats ?? stats.value
  } catch (error: any) {
    ElMessage.error('加载使用统计失败：' + error.message)
  }
}

// 加载每日趋势
const loadDailyUsage = async () => {
  try {
    const res = await tokenApi.getDailyUsage({ days: parseInt(chartDays.value) })
    dailyStats.value = res.daily_stats ?? []
  } catch (error: any) {
    ElMessage.error('加载每日趋势失败：' + error.message)
  }
}

// 加载模型统计
const loadModelStats = async () => {
  try {
    const res = await tokenApi.getModelStats({ days: 30 })
    modelStats.value = (res.model_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error('加载模型统计失败：' + error.message)
  }
}

// 加载后端统计
const loadBackendStats = async () => {
  try {
    const res = await tokenApi.getBackendStats({ days: 30 })
    backendStats.value = (res.backend_stats ?? []).slice(0, 5)
  } catch (error: any) {
    ElMessage.error('加载后端统计失败：' + error.message)
  }
}

// 加载用户排行
const loadRanking = async () => {
  try {
    const res = await tokenApi.getUserRanking({ days: parseInt(rankingDays.value) })
    ranking.value = res.ranking ?? []
  } catch (error: any) {
    ElMessage.error('加载排行失败：' + error.message)
  }
}

// 显示配额对话框
const showQuotaDialog = (row: any) => {
  quotaForm.value = {
    user_id: row.user_id,
    username: row.username,
    daily_limit: 0,
    monthly_limit: 0,
  }
  quotaErrors.value = { daily: '', monthly: '' }
  quotaDialogVisible.value = true
}

// 保存配额
const saveQuota = async () => {
  quotaLoading.value = true
  quotaErrors.value = { daily: '', monthly: '' }

  try {
    await tokenApi.setUserQuota(quotaForm.value.user_id, {
      daily_limit: quotaForm.value.daily_limit,
      monthly_limit: quotaForm.value.monthly_limit,
    })
    ElMessage.success('配额设置成功')
    quotaDialogVisible.value = false
    loadRanking()
  } catch (error: any) {
    ElMessage.error('设置配额失败：' + error.message)
  } finally {
    quotaLoading.value = false
  }
}

onMounted(() => {
  loadUsage()
  loadDailyUsage()
  loadModelStats()
  loadBackendStats()
  if (showUserRanking.value) {
    loadRanking()
  }
})
</script>

<style scoped>
.token-usage {
  padding: 20px;
}

.usage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.usage-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  margin-bottom: 20px;
}

.stat-card {
  display: flex;
  align-items: center;
  padding: 20px;
}

.stat-icon {
  width: 60px;
  height: 60px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 15px;
  font-size: 28px;
  color: white;
}

.stat-icon.total {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.stat-icon.prompt {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
}

.stat-icon.completion {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);
}

.stat-icon.requests {
  background: linear-gradient(135deg, #43e97b 0%, #38f9d7 100%);
}

.stat-content {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: bold;
  color: #333;
  margin-bottom: 5px;
}

.stat-label {
  font-size: 14px;
  color: #666;
}

.chart-row {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 5px;
}

@media (max-width: 1200px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }

  .usage-header {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
