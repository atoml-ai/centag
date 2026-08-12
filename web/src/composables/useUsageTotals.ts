import { ref, computed, watch } from 'vue'
import { getUserUsage, getUsageBreakdown } from '@/api/token-usage'
import { getCostSummary } from '@/api/cost'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { storeToRefs } from 'pinia'
import {
  currencySymbol,
  formatDisplayCost,
  getDisplayCurrency,
  type DisplayCurrency
} from '@/utils/billing-currency'
import { formatTokens } from '@/utils/format'

/** 状态栏：总费用 + 总 Token（鉴权就绪后加载） */
export function useUsageTotals(options?: { enabled?: boolean }) {
  const authStore = useAuthStore()
  const { isTeam } = useEdition()
  const { isAuthenticated } = storeToRefs(authStore)
  const enabled = options?.enabled ?? true

  const totalTokens = ref(0)
  const totalCostUsd = ref(0)
  const usdToCny = ref(7.2)
  const displayCurrency = ref<DisplayCurrency>(getDisplayCurrency())
  let loading = false

  const costText = computed(() => {
    const symbol = currencySymbol(displayCurrency.value)
    const amount = formatDisplayCost(totalCostUsd.value, displayCurrency.value, usdToCny.value)
    return `${symbol}${amount}`
  })

  function formatNumber(num: number): string {
    return formatTokens(num)
  }

  const tokensText = computed(() => formatNumber(totalTokens.value))

  async function loadUsageTotals() {
    if (!enabled || !isAuthenticated.value || loading) return
    loading = true
    try {
      const [usageRes, costRes] = await Promise.allSettled([getUserUsage(), loadCostSummary()])

      if (usageRes.status === 'fulfilled') {
        const res: any = usageRes.value
        const data = res?.stats ?? res?.data?.stats ?? res
        totalTokens.value = Number(data?.total_tokens || 0)
      }

      if (costRes.status === 'fulfilled' && costRes.value) {
        const cs: any = costRes.value
        totalCostUsd.value = Number(cs.total_cost_usd || 0)
        if (cs.usd_to_cny) {
          usdToCny.value = Number(cs.usd_to_cny)
        }
        if (!totalTokens.value && cs.total_tokens) {
          totalTokens.value = Number(cs.total_tokens)
        }
      }
    } catch (error) {
      console.error('Failed to load usage totals:', error)
    } finally {
      loading = false
    }
  }

  // Personal/minimal disable admin cost APIs; derive totals from the
  // user-scoped metering/pricing breakdown regardless of role.
  async function loadCostSummary(): Promise<any> {
    if (!isTeam.value) {
      const res: any = await getUsageBreakdown()
      const sum: any = res?.summary ?? {}
      return { total_cost_usd: Number(sum.total_cost || 0), total_tokens: Number(sum.total_tokens || 0) }
    }
    if (!authStore.isAdmin) {
      const { getUserBillingSummary } = await import('@/api/billing')
      return getUserBillingSummary({ group_by: 'model' })
    }
    return getCostSummary()
  }

  watch(
    isAuthenticated,
    (ok) => {
      if (ok) void loadUsageTotals()
    },
    { immediate: true }
  )

  return {
    totalTokens,
    totalCostUsd,
    costText,
    tokensText,
    loadUsageTotals
  }
}
