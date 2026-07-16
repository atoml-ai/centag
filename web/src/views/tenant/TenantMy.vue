<template>
  <div class="tenant-my">
    <el-card v-loading="loading">
      <template #header>
        <div class="card-header">
          <span>我的租户</span>
          <el-button :loading="loading" @click="loadData">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>

      <el-alert
        v-if="!loading && !tenant"
        type="warning"
        :closable="false"
        show-icon
        title="未找到租户信息。请联系管理员创建租户。"
      />

      <template v-if="tenant">
        <el-descriptions :column="2" border style="margin-bottom: 24px">
          <el-descriptions-item label="租户 ID" width="120">{{ tenant.id }}</el-descriptions-item>
          <el-descriptions-item label="名称">{{ tenant.name }}</el-descriptions-item>
          <el-descriptions-item label="描述">
            {{ tenant.description || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusType(tenant.status)">{{ getStatusLabel(tenant.status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ formatDate(tenant.created_at) }}
          </el-descriptions-item>
          <el-descriptions-item label="更新时间">
            {{ formatDate(tenant.updated_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <!-- 日配额 -->
        <el-divider content-position="left">日配额</el-divider>
        <div class="quota-grid">
          <div class="quota-item">
            <div class="quota-label">今日请求</div>
            <el-progress
              :percentage="dailyRequestPercent"
              :status="dailyRequestStatus"
              :stroke-width="20"
              :text-inside="true"
            >
              <span>{{ quota?.used_today_requests ?? 0 }} / {{ quota?.daily_request_limit ?? '∞' }}</span>
            </el-progress>
          </div>
          <div class="quota-item">
            <div class="quota-label">今日 Token</div>
            <el-progress
              :percentage="dailyTokenPercent"
              :status="dailyTokenStatus"
              :stroke-width="20"
              :text-inside="true"
            >
              <span>{{ formatNumber(quota?.used_today_tokens ?? 0) }} / {{ quota?.daily_token_limit ? formatNumber(quota.daily_token_limit) : '∞' }}</span>
            </el-progress>
          </div>
        </div>

        <!-- 月配额 -->
        <el-divider content-position="left">月配额</el-divider>
        <div class="quota-grid">
          <div class="quota-item">
            <div class="quota-label">本月请求</div>
            <el-progress
              :percentage="monthlyRequestPercent"
              :status="monthlyRequestStatus"
              :stroke-width="20"
              :text-inside="true"
            >
              <span>{{ quota?.used_month_requests ?? 0 }} / {{ quota?.monthly_request_limit ? quota.monthly_request_limit : '∞' }}</span>
            </el-progress>
          </div>
          <div class="quota-item">
            <div class="quota-label">本月 Token</div>
            <el-progress
              :percentage="monthlyTokenPercent"
              :status="monthlyTokenStatus"
              :stroke-width="20"
              :text-inside="true"
            >
              <span>{{ formatNumber(quota?.used_month_tokens ?? 0) }} / {{ quota?.monthly_token_limit ? formatNumber(quota.monthly_token_limit) : '∞' }}</span>
            </el-progress>
          </div>
        </div>

        <!-- 资源限制 -->
        <el-divider content-position="left">资源限制</el-divider>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="最大后端数">
            {{ quota?.max_backends ?? '∞' }}
          </el-descriptions-item>
          <el-descriptions-item label="最大 API Key 数">
            {{ quota?.max_api_keys ?? '∞' }}
          </el-descriptions-item>
        </el-descriptions>
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { getCurrentTenant, getCurrentQuota, type Tenant, type TenantQuota } from '@/api/tenant'

const loading = ref(false)
const tenant = ref<Tenant | null>(null)
const quota = ref<TenantQuota | null>(null)

const loadData = async () => {
  loading.value = true
  try {
    const [t, q] = await Promise.allSettled([
      getCurrentTenant(),
      getCurrentQuota()
    ])
    if (t.status === 'fulfilled') {
      const td = t.value
      tenant.value = td?.tenant ?? td ?? null
    }
    if (q.status === 'fulfilled') {
      quota.value = q.value ?? null
    }
  } catch (error) {
    console.error('加载租户数据失败', error)
  } finally {
    loading.value = false
  }
}

// ── 进度百分比计算 ──

function calcPercent(used: number | undefined, limit: number | undefined): number {
  if (!limit || limit <= 0) return 0
  return Math.min(Math.round(((used ?? 0) / limit) * 100), 100)
}

function progressStatus(pct: number): 'success' | 'warning' | 'exception' {
  if (pct >= 90) return 'exception'
  if (pct >= 70) return 'warning'
  return 'success'
}

const dailyRequestPercent = computed(() => calcPercent(quota.value?.used_today_requests, quota.value?.daily_request_limit))
const dailyTokenPercent = computed(() => calcPercent(quota.value?.used_today_tokens, quota.value?.daily_token_limit))
const monthlyRequestPercent = computed(() => calcPercent(quota.value?.used_month_requests, quota.value?.monthly_request_limit))
const monthlyTokenPercent = computed(() => calcPercent(quota.value?.used_month_tokens, quota.value?.monthly_token_limit))

const dailyRequestStatus = computed(() => progressStatus(dailyRequestPercent.value))
const dailyTokenStatus = computed(() => progressStatus(dailyTokenPercent.value))
const monthlyRequestStatus = computed(() => progressStatus(monthlyRequestPercent.value))
const monthlyTokenStatus = computed(() => progressStatus(monthlyTokenPercent.value))

// ── 工具函数 ──

const getStatusType = (status?: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'suspended': return 'warning'
    case 'deleted': return 'danger'
    default: return 'info'
  }
}

const getStatusLabel = (status?: string) => {
  switch (status) {
    case 'active': return '活跃'
    case 'suspended': return '暂停'
    case 'deleted': return '已删除'
    default: return status ?? '-'
  }
}

const formatDate = (date?: string) => {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

const formatNumber = (n: number) => {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

onMounted(loadData)
</script>

<style scoped>
.tenant-my {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.quota-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 0 4px;
}

.quota-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.quota-label {
  font-size: 0.9rem;
  color: var(--desktop-sidebar-text, #606266);
}
</style>
