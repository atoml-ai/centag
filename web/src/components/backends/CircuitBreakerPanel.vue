<template>
  <div class="cb-panel" v-loading="loading">
    <div class="cb-panel-head">
      <div class="cb-title">
        <el-icon><WarningFilled v-if="summary.open > 0" class="cb-alert" /><Cpu v-else /></el-icon>
        <span>{{ t('circuitBreakerPanel.title') }}</span>
        <el-tag v-if="summary.open > 0" type="danger" size="small" effect="dark">
          {{ t('circuitBreakerPanel.openCount', { n: summary.open }) }}
        </el-tag>
        <el-tag v-else type="success" size="small" effect="light">
          {{ t('circuitBreakerPanel.allHealthy') }}
        </el-tag>
      </div>
      <div class="cb-actions">
        <span class="cb-updated">{{ t('circuitBreakerPanel.updatedAt', { t: updatedAt }) }}</span>
        <el-switch
          v-model="autoRefresh"
          :active-text="t('circuitBreakerPanel.autoRefresh')"
          size="small"
          @change="onAutoRefreshChange"
        />
        <el-button size="small" :icon="Refresh" :loading="loading" @click="fetchStatus">
          {{ t('circuitBreakerPanel.refresh') }}
        </el-button>
      </div>
    </div>

    <div v-if="statusList.length === 0" class="cb-empty">
      {{ t('circuitBreakerPanel.noData') }}
    </div>

    <div v-else class="cb-list">
      <div
        v-for="item in statusList"
        :key="item.backend_id"
        class="cb-row"
        :class="'cb-' + item.state"
      >
        <span class="cb-dot" :class="'dot-' + item.state" />
        <div class="cb-main">
          <div class="cb-name">
            {{ item.backend_id }}
            <el-tag size="small" :type="stateTagType(item.state)" effect="light">
              {{ stateLabel(item.state) }}
            </el-tag>
          </div>
          <div class="cb-meta">
            <span>{{ t('circuitBreakerPanel.failures') }}: {{ item.consecutive_failures }}/{{ item.failure_threshold }}</span>
            <span v-if="item.state === 'open' && item.open_since">
              {{ t('circuitBreakerPanel.openSince') }}: {{ fmtTime(item.open_since) }}
            </span>
            <span v-if="item.last_failure_at">
              {{ t('circuitBreakerPanel.lastFailure') }}: {{ fmtTime(item.last_failure_at) }}
            </span>
          </div>
        </div>
        <el-button
          v-if="item.state !== 'closed'"
          size="small"
          type="warning"
          plain
          :loading="resetting[item.backend_id]"
          @click="handleReset(item.backend_id)"
        >
          {{ t('circuitBreakerPanel.reset') }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh, Cpu, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getCircuitBreakerStatus, resetCircuitBreaker, type CircuitBreakerSnapshot } from '@/api/backend'

const { t } = useI18n()

const statusList = ref<CircuitBreakerSnapshot[]>([])
const summary = reactive({ total: 0, open: 0, half_open: 0, closed: 0 })
const loading = ref(false)
const autoRefresh = ref(true)
const updatedAt = ref('')
const resetting = reactive<Record<string, boolean>>({})
let timer: ReturnType<typeof setInterval> | null = null

function stateLabel(state: string): string {
  if (state === 'open') return t('circuitBreakerPanel.stateOpen')
  if (state === 'half-open') return t('circuitBreakerPanel.stateHalfOpen')
  return t('circuitBreakerPanel.stateClosed')
}

function stateTagType(state: string): 'danger' | 'warning' | 'success' {
  if (state === 'open') return 'danger'
  if (state === 'half-open') return 'warning'
  return 'success'
}

function fmtTime(s: string): string {
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleTimeString()
}

async function fetchStatus() {
  loading.value = true
  try {
    const res: any = await getCircuitBreakerStatus()
    const data = res?.data ?? res
    statusList.value = data?.circuit_breakers ?? []
    const sm = data?.summary ?? {}
    summary.total = sm.total ?? statusList.value.length
    summary.open = sm.open ?? 0
    summary.half_open = sm.half_open ?? 0
    summary.closed = sm.closed ?? 0
    updatedAt.value = new Date().toLocaleTimeString()
  } catch (err: any) {
    // 静默失败，避免轮询刷屏；仅首次给出提示
    if (statusList.value.length === 0) {
      ElMessage.error(err?.response?.data?.message || t('circuitBreakerPanel.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

function startTimer() {
  stopTimer()
  timer = setInterval(() => {
    if (!loading.value) fetchStatus()
  }, 5000)
}

function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function onAutoRefreshChange(val: boolean) {
  if (val) startTimer()
  else stopTimer()
}

async function handleReset(id: string) {
  resetting[id] = true
  try {
    await resetCircuitBreaker(id)
    ElMessage.success(t('circuitBreakerPanel.resetSuccess', { id }))
    await fetchStatus()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || t('circuitBreakerPanel.resetFailed'))
  } finally {
    resetting[id] = false
  }
}

onMounted(() => {
  fetchStatus()
  if (autoRefresh.value) startTimer()
})

onBeforeUnmount(() => {
  stopTimer()
})
</script>

<style scoped>
.cb-panel {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  padding: 14px 16px;
  background: var(--el-bg-color);
  margin-bottom: 16px;
}
.cb-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.cb-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
}
.cb-alert {
  color: var(--el-color-danger);
}
.cb-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.cb-updated {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.cb-empty {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  padding: 8px 0;
}
.cb-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.cb-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
  border-left: 3px solid var(--el-border-color);
}
.cb-row.cb-open {
  border-left-color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
}
.cb-row.cb-half-open {
  border-left-color: var(--el-color-warning);
  background: var(--el-color-warning-light-9);
}
.cb-row.cb-closed {
  border-left-color: var(--el-color-success);
}
.cb-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot-open {
  background: var(--el-color-danger);
}
.dot-half-open {
  background: var(--el-color-warning);
}
.dot-closed {
  background: var(--el-color-success);
}
.cb-main {
  flex: 1;
  min-width: 0;
}
.cb-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 13px;
}
.cb-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
