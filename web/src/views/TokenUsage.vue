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
          <v-chart :option="dailyChartOption" style="height: 400px" autoresize />
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

    <el-card v-if="showUserRanking" class="ranking-card">
      <template #header>
        <div class="card-header">
          <span>用户 Token 使用排行</span>
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
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import * as tokenApi from '@/api/token-usage'
import UsageMetricsSummary from '@/components/usage/UsageMetricsSummary.vue'
import BillingRulesDialog from '@/components/dashboard/BillingRulesDialog.vue'

echarts.use([CanvasRenderer, BarChart, LineChart, PieChart, GridComponent, TooltipComponent, LegendComponent])

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { isPersonal, isTeam } = useEdition()
const showUserRanking = computed(() => authStore.isAdmin && !isPersonal.value)
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
const ranking = ref<any[]>([])
const rankingDays = ref('30')

const quotaDialogVisible = ref(false)
const quotaLoading = ref(false)
const quotaForm = ref({
  user_id: 0,
  username: '',
  daily_limit: 0,
  monthly_limit: 0
})
const quotaErrors = ref({ daily: '', monthly: '' })

const dailyChartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    axisPointer: { type: 'shadow' }
  },
  legend: {
    data: ['输入 Token', '输出 Token', '总 Token']
  },
  grid: {
    left: '3%',
    right: '4%',
    bottom: '3%',
    containLabel: true
  },
  xAxis: {
    type: 'category',
    data: dailyStats.value.map((s) => s.date).reverse()
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
      data: dailyStats.value.map((s) => s.prompt_tokens).reverse(),
      itemStyle: { color: '#5470c6' }
    },
    {
      name: '输出 Token',
      type: 'bar',
      stack: 'total',
      data: dailyStats.value.map((s) => s.comp_tokens).reverse(),
      itemStyle: { color: '#91cc75' }
    },
    {
      name: '总 Token',
      type: 'line',
      data: dailyStats.value.map((s) => s.total_tokens).reverse(),
      itemStyle: { color: '#fac858' },
      lineStyle: { width: 3 }
    }
  ]
}))

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

const formatNumber = (num: number) => num.toLocaleString()

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

const loadRanking = async () => {
  try {
    const res = await tokenApi.getUserRanking({ days: parseInt(rankingDays.value) })
    ranking.value = res.ranking ?? []
  } catch (error: any) {
    ElMessage.error('加载排行失败：' + error.message)
  }
}

const showQuotaDialog = (row: any) => {
  quotaForm.value = {
    user_id: row.user_id,
    username: row.username,
    daily_limit: 0,
    monthly_limit: 0
  }
  quotaErrors.value = { daily: '', monthly: '' }
  quotaDialogVisible.value = true
}

const saveQuota = async () => {
  quotaLoading.value = true
  quotaErrors.value = { daily: '', monthly: '' }

  try {
    await tokenApi.setUserQuota(quotaForm.value.user_id, {
      daily_limit: quotaForm.value.daily_limit,
      monthly_limit: quotaForm.value.monthly_limit
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
  if (showUserRanking.value) {
    loadRanking()
  }
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

.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 5px;
}

@media (max-width: 768px) {
  .usage-header {
    flex-direction: column;
    gap: 15px;
  }
}
</style>
