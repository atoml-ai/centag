<template>
  <div class="status-bar">
    <div class="status-left">
      <span class="status-item">
        <span class="status-dot" :class="statusClass"></span>
        {{ statusText }}
      </span>
      <span class="status-sep">·</span>
      <span class="status-item">版本 <span class="mono">{{ status.version || '--' }}</span></span>
      <span class="status-sep">·</span>
      <span class="status-item">运行时长 {{ formatUptime(status.uptime) }}</span>
      <span class="status-sep">·</span>
      <span class="status-item">启动于 {{ status.start_time || '--' }}</span>

      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-clickable"
        :title="backendId ? `后端 ID: ${backendId}` : '未设置默认后端'"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Cpu /></el-icon>
        <span>后端</span>
        <span class="mono truncate">{{ backendName || '未设置' }}</span>
      </span>
      <span
        class="status-item status-clickable"
        :title="model ? `默认模型: ${model}` : '未设置默认模型'"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Coin /></el-icon>
        <span>模型</span>
        <span class="mono truncate">{{ model || '未设置' }}</span>
      </span>
      <span
        class="status-item status-clickable"
        :title="pipelineId ? `流水线 ID: ${pipelineId}` : '未设置默认流水线'"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Share /></el-icon>
        <span>流水线</span>
        <span class="mono truncate">{{ pipelineName || '未设置' }}</span>
      </span>

      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-clickable"
        title="查看用量与计费"
        @click="goUsage"
      >
        <el-icon :size="12"><Money /></el-icon>
        <span>总费用</span>
        <span class="mono">{{ costText }}</span>
      </span>
      <span
        class="status-item status-clickable"
        title="查看 Token 用量"
        @click="goUsage"
      >
        <el-icon :size="12"><DataLine /></el-icon>
        <span>总 Token</span>
        <span class="mono">{{ tokensText }}</span>
      </span>
    </div>
    <div class="status-right">
      <span
        class="status-item status-log-toggle"
        :class="{ 'status-log-active': logPanelVisible }"
        title="实时日志控制台"
        @click="toggleLogPanel"
      >
        <el-icon :size="12"><Monitor /></el-icon>
        <span>日志</span>
      </span>
      <span class="status-sep status-sep-group">|</span>
      <span class="status-item">构建于 {{ status.build_time || '--' }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Cpu, Coin, Share, Money, DataLine, Monitor } from '@element-plus/icons-vue'
import { getStatus } from '@/api'
import { formatUptime } from '@/utils/format'
import { useActivePipeline } from '@/composables/useActivePipeline'
import { useDefaultProxySettings } from '@/composables/useDefaultProxySettings'
import { useUsageTotals } from '@/composables/useUsageTotals'
import { useLogPanel } from '@/composables/useLogPanel'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const { isMinimal, isTeam } = useEdition()
const status = ref<any>({})
const { visible: logPanelVisible, toggle: toggleLogPanel } = useLogPanel()

const { pipelineId, pipelineName } = useActivePipeline({ enabled: true })
const { backendId, backendName, model } = useDefaultProxySettings({ enabled: true })
const { costText, tokensText } = useUsageTotals({ enabled: true })

const statusClass = computed(() => {
  return status.value.status === 'healthy' ? 'status-healthy' : 'status-error'
})

const statusText = computed(() => {
  return status.value.status === 'healthy' ? '运行中' : '异常'
})

function goUsage() {
  if (isMinimal.value) {
    router.push('/dashboard')
    return
  }
  // team 超管无 /token-usage，走成本看板
  if (isTeam.value && authStore.isAdmin) {
    router.push('/cost')
    return
  }
  router.push('/token-usage')
}

async function loadStatus() {
  try {
    const res = await getStatus()
    if (res) {
      status.value = res
    }
  } catch (e) {
    console.error('Failed to load status:', e)
  }
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped>
.status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 16px;
  background: var(--shell-sidebar-bg, #f5f7fa);
  border-top: 1px solid var(--shell-sidebar-border, #e4e7ed);
  font-size: 0.75rem;
  color: #909399;
  flex-shrink: 0;
  gap: 8px;
}

.status-left,
.status-right {
  display: flex;
  align-items: center;
  gap: 4px 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.status-left {
  flex: 1;
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.status-sep {
  color: #dcdfe6;
}

.status-sep-group {
  color: #c0c4cc;
  margin: 0 2px;
}

.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 2px;
}

.status-healthy {
  background: #67c23a;
}

.status-error {
  background: #f56c6c;
}

.status-clickable {
  cursor: pointer;
  transition: color 0.15s ease;
}

.status-clickable:hover {
  color: #606266;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.truncate {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-log-toggle {
  cursor: pointer;
  transition: color 0.15s ease;
  color: #909399;
}

.status-log-toggle:hover {
  color: #606266;
}

.status-log-active {
  color: #409eff;
  font-weight: 500;
}
</style>
