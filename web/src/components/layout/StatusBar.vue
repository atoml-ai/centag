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
      <span class="status-item">运行时长 {{ status.uptime || '--' }}</span>
      <span class="status-sep">·</span>
      <span class="status-item">启动于 {{ status.start_time || '--' }}</span>

      <span v-if="cacheItems.length" class="status-sep status-sep-group">|</span>
      <span
        v-for="item in cacheItems"
        :key="item.key"
        class="status-item status-cache-item"
        :title="`查看${item.label}`"
        @click="router.push('/cache')"
      >
        <el-icon :size="12" :class="item.iconClass"><component :is="item.icon" /></el-icon>
        <span>{{ item.label }}</span>
        <span class="mono" :style="item.valueStyle">{{ item.value }}</span>
      </span>
      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-pipeline-item"
        :title="pipelineId ? `流水线 ID: ${pipelineId}` : '未设置默认流水线'"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Share /></el-icon>
        <span>流水线</span>
        <span class="mono pipeline-name">{{ pipelineName }}</span>
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
import { Collection, CircleCheck, CircleClose, Coin, Share, Monitor } from '@element-plus/icons-vue'
import { getStatus } from '@/api'
import { useCacheStats } from '@/composables/useCacheStats'
import { useActivePipeline } from '@/composables/useActivePipeline'
import { useLogPanel } from '@/composables/useLogPanel'
import { useEdition } from '@/composables/useEdition'

const router = useRouter()
const status = ref<any>({})
const { visible: logPanelVisible, toggle: toggleLogPanel } = useLogPanel()
const { isMinimal } = useEdition()

const { cacheStats, hitRate, hitRateColor, formatNumber } = useCacheStats({
  enabled: !isMinimal.value
})
const { pipelineId, pipelineName } = useActivePipeline({
  enabled: true
})

const cacheItems = computed(() => {
  if (isMinimal.value) return []
  return [
    {
      key: 'total',
      label: '缓存',
      value: formatNumber(cacheStats.value.total),
      icon: Collection,
      iconClass: 'cache-icon-total'
    },
    {
      key: 'hits',
      label: '命中',
      value: formatNumber(cacheStats.value.hits),
      icon: CircleCheck,
      iconClass: 'cache-icon-hits'
    },
    {
      key: 'misses',
      label: '未中',
      value: formatNumber(cacheStats.value.misses),
      icon: CircleClose,
      iconClass: 'cache-icon-misses'
    },
    {
      key: 'rate',
      label: '命中率',
      value: `${hitRate.value}%`,
      icon: Coin,
      iconClass: 'cache-icon-rate',
      valueStyle: { color: hitRateColor.value }
    }
  ]
})

const statusClass = computed(() => {
  return status.value.status === 'healthy' ? 'status-healthy' : 'status-error'
})

const statusText = computed(() => {
  return status.value.status === 'healthy' ? '运行中' : '异常'
})

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

.status-cache-item,
.status-pipeline-item {
  cursor: pointer;
  transition: color 0.15s ease;
}

.status-cache-item:hover,
.status-pipeline-item:hover {
  color: #606266;
}

.cache-icon-total {
  color: #67c23a;
}

.cache-icon-hits {
  color: #409eff;
}

.cache-icon-misses {
  color: #f56c6c;
}

.cache-icon-rate {
  color: #e6a23c;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.pipeline-name {
  max-width: 160px;
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