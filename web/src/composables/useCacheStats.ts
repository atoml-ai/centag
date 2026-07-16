import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getCacheStats, getCacheList } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { isMinimalEdition } from '@/utils/edition'

export function useCacheStats(options?: { pollMs?: number; enabled?: boolean }) {
  const authStore = useAuthStore()
  const pollMs = options?.pollMs ?? 5000
  const enabled = options?.enabled ?? true

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

  const hitRate = computed(() => {
    const total = cacheStats.value.hits + cacheStats.value.misses
    if (total === 0) return '0.00'
    return ((cacheStats.value.hits / total) * 100).toFixed(2)
  })

  const hitRateColor = computed(() => {
    const rate = parseFloat(hitRate.value)
    if (rate >= 80) return '#67c23a'
    if (rate >= 50) return '#e6a23c'
    return '#f56c6c'
  })

  let intervalId: number | null = null

  function formatNumber(num: number): string {
    if (num >= 1_000_000) return (num / 1_000_000).toFixed(1) + 'M'
    if (num >= 1_000) return (num / 1_000).toFixed(1) + 'K'
    return num.toString()
  }

  async function loadStats() {
    if (!authStore.isAuthenticated || isMinimalEdition()) return
    try {
      const data = await getCacheStats()
      const listData = await getCacheList({ page: 1, size: 1, type: 'all' })
      cacheStats.value = {
        ...cacheStats.value,
        ...data,
        total: listData?.total_count || 0
      }
    } catch (error: any) {
      const message = String(error?.message || '')
      if (message.includes('no refresh token') || message.includes('401')) return
      console.error('Failed to load cache stats:', error)
    }
  }

  function startPolling() {
    loadStats()
    intervalId = window.setInterval(loadStats, pollMs)
  }

  function stopPolling() {
    if (intervalId !== null) {
      clearInterval(intervalId)
      intervalId = null
    }
  }

  onMounted(() => {
    if (enabled && !isMinimalEdition()) startPolling()
  })
  onUnmounted(stopPolling)

  return {
    cacheStats,
    hitRate,
    hitRateColor,
    formatNumber,
    loadStats
  }
}