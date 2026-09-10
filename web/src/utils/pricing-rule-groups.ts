import type { PricingRule } from '@/api/billing'

/** Align with core/pkg/backend ModelHasFreeTier. */
export function modelHasFreeTier(modelName: string): boolean {
  let n = String(modelName || '')
    .toLowerCase()
    .trim()
  n = n.replace(/\s+/g, ' ')
  if (!n) return false
  for (const suf of [' free', '-free', '_free', '/free']) {
    if (n.endsWith(suf)) return true
  }
  if (n.endsWith('free') && n.length > 4) {
    const last = n[n.length - 5]
    return (last >= '0' && last <= '9') || last === '.' || last === '-' || last === '_'
  }
  return false
}

/**
 * Free tier is name-driven (align with backend.ModelHasFreeTier), e.g. *-free.
 * Zero unit prices alone are NOT treated as free: LiteLLM placeholder zeros
 * (github_copilot/*, some reseller stubs) otherwise land in the free bucket.
 */
export function isFreePricingRule(rule: Pick<PricingRule, 'model' | 'input_price_per_m' | 'output_price_per_m'>): boolean {
  return modelHasFreeTier(rule.model)
}

export type PriceTypeFilter = 'all' | 'cost' | 'revenue'
export type FreePaidFilter = 'all' | 'free' | 'paid'

export interface PricingRuleFilters {
  search?: string
  priceType?: PriceTypeFilter
  freePaid?: FreePaidFilter
  backendId?: string
}

export function filterPricingRules(rules: PricingRule[], filters: PricingRuleFilters = {}): PricingRule[] {
  const search = (filters.search || '').trim().toLowerCase()
  const priceType = filters.priceType || 'all'
  const freePaid = filters.freePaid || 'all'
  const backendId = (filters.backendId || '').trim()

  return rules.filter((r) => {
    if (backendId && r.backend_id !== backendId) return false
    const pt = r.price_type || 'cost'
    if (priceType !== 'all' && pt !== priceType) return false
    const free = isFreePricingRule(r)
    if (freePaid === 'free' && !free) return false
    if (freePaid === 'paid' && free) return false
    if (search) {
      const hay = `${r.name || ''} ${r.backend_id || ''} ${r.model || ''}`.toLowerCase()
      if (!hay.includes(search)) return false
    }
    return true
  })
}

export interface PricingRuleBackendGroup {
  backendId: string
  rules: PricingRule[]
  freeRules: PricingRule[]
  paidRules: PricingRule[]
  freeCount: number
  paidCount: number
}

function sortRules(a: PricingRule, b: PricingRule): number {
  const am = (a.model || '').localeCompare(b.model || '')
  if (am !== 0) return am
  const at = (a.price_type || 'cost').localeCompare(b.price_type || 'cost')
  if (at !== 0) return at
  return (b.priority || 0) - (a.priority || 0)
}

export function groupRulesByBackend(rules: PricingRule[]): PricingRuleBackendGroup[] {
  const map = new Map<string, PricingRule[]>()
  for (const r of rules) {
    const key = r.backend_id || '*'
    const list = map.get(key)
    if (list) list.push(r)
    else map.set(key, [r])
  }

  const groups: PricingRuleBackendGroup[] = []
  for (const backendId of [...map.keys()].sort((a, b) => a.localeCompare(b))) {
    const all = (map.get(backendId) || []).slice().sort(sortRules)
    const freeRules = all.filter(isFreePricingRule)
    const paidRules = all.filter((r) => !isFreePricingRule(r))
    groups.push({
      backendId,
      rules: all,
      freeRules,
      paidRules,
      freeCount: freeRules.length,
      paidCount: paidRules.length
    })
  }
  return groups
}

export function uniqueBackendIds(rules: PricingRule[]): string[] {
  return [...new Set(rules.map((r) => r.backend_id || '*'))].sort((a, b) => a.localeCompare(b))
}
