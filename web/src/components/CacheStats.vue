<template>
  <div class="cache-stats">
    <div class="stat-card" @click="$router.push('/cache')">
      <div class="stat-icon total-icon">
        <el-icon :size="20"><Collection /></el-icon>
      </div>
      <div class="stat-content">
        <div class="stat-label">缓存</div>
        <div class="stat-value">{{ formatNumber(cacheStats.total) }}</div>
      </div>
    </div>
    <div class="stat-card" @click="$router.push('/cache')">
      <div class="stat-icon hits-icon">
        <el-icon :size="20"><CircleCheck /></el-icon>
      </div>
      <div class="stat-content">
        <div class="stat-label">命中</div>
        <div class="stat-value">{{ formatNumber(cacheStats.hits) }}</div>
      </div>
    </div>
    <div class="stat-card" @click="$router.push('/cache')">
      <div class="stat-icon misses-icon">
        <el-icon :size="20"><CircleClose /></el-icon>
      </div>
      <div class="stat-content">
        <div class="stat-label">未中</div>
        <div class="stat-value">{{ formatNumber(cacheStats.misses) }}</div>
      </div>
    </div>
    <div class="stat-card" @click="$router.push('/cache')">
      <div class="stat-icon rate-icon">
        <el-icon :size="20"><Coin /></el-icon>
      </div>
      <div class="stat-content">
        <div class="stat-label">命中率</div>
        <div class="stat-value" :style="{ color: hitRateColor }">{{ hitRate }}%</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Collection, CircleCheck, CircleClose, Coin } from '@element-plus/icons-vue'
import { getCacheStats, getCacheList } from '@/api'
import { useAuthStore } from '@/stores/auth'

const cacheStats = ref({
  total: 0,
  hits: 0,
  misses: 0,
  enabled: false,
  type: '',
  ttl: 0,
  max_items: 0,
  max_size: 0
})

let intervalId: number | null = null
const authStore = useAuthStore()

// 计算命中率
const hitRate = computed(() => {
  const total = cacheStats.value.hits + cacheStats.value.misses
  if (total === 0) return '0.00'
  return ((cacheStats.value.hits / total) * 100).toFixed(2)
})

// 根据命中率返回颜色
const hitRateColor = computed(() => {
  const rate = parseFloat(hitRate.value)
  if (rate >= 80) return '#67c23a'
  if (rate >= 50) return '#e6a23c'
  return '#f56c6c'
})

// 格式化数字
function formatNumber(num: number): string {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  }
  return num.toString()
}

// 加载缓存统计
async function loadStats() {
  if (!authStore.isAuthenticated) return
  try {
    const data = await getCacheStats()
    const listData = await getCacheList({ page: 1, size: 1, type: 'all' })
    
    cacheStats.value = {
      ...cacheStats.value,
      ...data,
      total: listData?.total_count || 0  // 使用列表API返回的总数
    }
  } catch (error: any) {
    const message = String(error?.message || '')
    if (message.includes('no refresh token') || message.includes('401')) return
    console.error('Failed to load cache stats:', error)
  }
}

onMounted(() => {
  loadStats()
  // 每 5 秒自动刷新
  intervalId = window.setInterval(loadStats, 5000)
})

onUnmounted(() => {
  if (intervalId !== null) {
    clearInterval(intervalId)
  }
})
</script>

<style scoped>
.cache-stats {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: #ffffff;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid rgba(255, 255, 255, 0.4);
}

.stat-card:hover {
  background: #f0f7ff;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 6px;
}

.total-icon {
  background: rgba(103, 194, 58, 0.15);
  color: #67c23a;
}

.hits-icon {
  background: rgba(64, 158, 255, 0.15);
  color: #409eff;
}

.misses-icon {
  background: rgba(245, 108, 108, 0.15);
  color: #f56c6c;
}

.rate-icon {
  background: rgba(230, 162, 60, 0.15);
  color: #e6a23c;
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-label {
  font-size: 0.7rem;
  color: #909399;
  font-weight: 400;
  white-space: nowrap;
}

.stat-value {
  font-size: 0.95rem;
  font-weight: 600;
  color: #303133;
  white-space: nowrap;
}

@media (max-width: 1400px) {
  .cache-stats {
    gap: 6px;
  }
  
  .stat-card {
    padding: 4px 10px;
    gap: 6px;
  }
  
  .stat-icon {
    width: 28px;
    height: 28px;
  }
  
  .stat-label {
    font-size: 0.65rem;
  }
  
  .stat-value {
    font-size: 0.85rem;
  }
}

@media (max-width: 1200px) {
  .stat-label {
    display: none;
  }
  
  .stat-card {
    padding: 6px;
  }
}

@media (max-width: 768px) {
  .cache-stats {
    display: none;
  }
}
</style>
