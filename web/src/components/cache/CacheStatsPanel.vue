<template>
  <div class="stats-panel">
    <el-card class="stats-card">
      <template #header>
        <span class="card-title">{{ t('cache.hitRate') }}</span>
      </template>
      <div class="stats-content">
        <el-progress type="dashboard" :percentage="parseFloat(hitRate)" :color="hitRateColor" :width="120">
          <template #default="{ percentage }">
            <span class="percentage-value">{{ percentage }}%</span>
          </template>
        </el-progress>
        <div class="stats-numbers">
          <div class="stat-item">
            <span class="stat-label">{{ t('cache.hits') }}</span>
            <span class="stat-value hit">{{ formatNumber(stats.hits) }}</span>
          </div>
          <div class="stat-item">
            <span class="stat-label">{{ t('cache.misses') }}</span>
            <span class="stat-value miss">{{ formatNumber(stats.misses) }}</span>
          </div>
        </div>
      </div>
    </el-card>

    <el-card class="info-card">
      <template #header>
        <span class="card-title">{{ t('cache.cacheStatus') }}</span>
      </template>
      <div class="status-list">
        <div class="status-item">
          <span class="status-label">{{ t('cache.enabledStatus') }}</span>
          <el-tag :type="stats.enabled ? 'success' : 'info'" size="small">
            {{ stats.enabled ? t('cache.enabled') : t('cache.disabled') }}
          </el-tag>
        </div>
        <div class="status-item">
          <span class="status-label">{{ t('cache.backendLabel') }}</span>
          <span>{{ backendLabel }}</span>
        </div>
        <div class="status-item">
          <span class="status-label">{{ t('cache.expiry') }}</span>
          <span>{{ stats.ttl ? `${stats.ttl}s` : t('cache.permanent') }}</span>
        </div>
        <div class="status-item">
          <span class="status-label">{{ t('cache.maxEntries') }}</span>
          <span>{{ stats.max_items || t('cache.unlimited') }}</span>
        </div>
      </div>
    </el-card>

    <el-card class="actions-card">
      <el-button :loading="loading" style="width: 100%" @click="$emit('refresh')">
        <el-icon><Refresh /></el-icon>
        {{ t('cache.refreshData') }}
      </el-button>
      <el-button
        type="danger"
        :loading="clearing"
        style="width: 100%; margin-left: 0; margin-top: 8px"
        @click="$emit('clear')"
      >
        <el-icon><Delete /></el-icon>
        {{ t('cache.clearCache') }}
      </el-button>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh, Delete } from '@element-plus/icons-vue'
import { formatNumber } from '@/utils/format'

const props = defineProps<{
  stats: Record<string, any>
  backend?: string
  loading?: boolean
  clearing?: boolean
}>()

defineEmits<{ refresh: []; clear: [] }>()

const { t } = useI18n()

const hitRate = computed(() => {
  if (props.stats.hit_rate !== undefined) return Number(props.stats.hit_rate).toFixed(2)
  const total = (props.stats.hits || 0) + (props.stats.misses || 0)
  if (!total) return '0.00'
  return (((props.stats.hits || 0) / total) * 100).toFixed(2)
})

const hitRateColor = computed(() => {
  const rate = parseFloat(hitRate.value)
  if (rate >= 80) return '#10b981'
  if (rate >= 50) return '#f59e0b'
  return '#ef4444'
})

const backendLabel = computed(() => {
  const b = props.backend || props.stats.backend || 'exact'
  if (b === 'semantic') return t('cache.filterOptions.semantic')
  if (b === 'external') return t('cache.filterOptions.external')
  return t('cache.filterOptions.exact')
})
</script>

<style scoped>
.stats-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.stats-card,
.info-card,
.actions-card {
  border: 1px solid var(--color-gray-200);
  box-shadow: var(--shadow-sm);
}
.stats-content {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.stats-numbers {
  display: flex;
  justify-content: space-around;
  width: 100%;
  margin-top: 12px;
}
.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.stat-label {
  font-size: 12px;
  color: var(--color-gray-600);
}
.stat-value {
  font-size: 16px;
  font-weight: 600;
}
.stat-value.hit {
  color: var(--color-success);
}
.stat-value.miss {
  color: var(--color-error);
}
.status-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}
.status-label {
  color: var(--color-gray-600);
}
.card-title {
  font-weight: 600;
}
.percentage-value {
  font-size: 1rem;
  font-weight: 600;
}
</style>
