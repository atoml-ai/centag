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

/** Free if both unit prices are 0, or model name looks like a free tier. */
export function isFreePricingRule(rule: Pick<PricingRule, 'model' | 'input_price_per_m' | 'output_price_per_m'>): boolean {
  const inP = Number(rule.input_price_per_m) || 0
  const outP = Number(rule.output_price_per_m) || 0
  if (inP === 0 && outP === 0) return true
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

/** Collect model name aliases from a backend's supported_models entries. */
export function collectBackendModelNames(supportedModels: unknown): Set<string> {
  const out = new Set<string>()
  if (!Array.isArray(supportedModels)) return out
  for (const m of supportedModels) {
    if (typeof m === 'string') {
      const t = m.trim().toLowerCase()
      if (t) out.add(t)
      continue
    }
    if (m && typeof m === 'object') {
      const o = m as Record<string, unknown>
      for (const key of ['actual_model', 'requested_model', 'name', 'id', 'model']) {
        const v = o[key]
        if (typeof v === 'string') {
          const t = v.trim().toLowerCase()
          if (t) out.add(t)
        }
      }
    }
  }
  return out
}

export function isRuleForConfiguredBackend(rule: PricingRule, configuredBackendIds: Set<string>): boolean {
  const be = (rule.backend_id || '').trim()
  if (!be || be === '*') return true
  return configuredBackendIds.has(be)
}

/** Model `*` always matches; otherwise require membership in that backend's model set. */
export function isRuleForConfiguredModel(
  rule: PricingRule,
  modelsByBackend: Map<string, Set<string>>
): boolean {
  const model = (rule.model || '').trim()
  if (!model || model === '*') return true
  const be = (rule.backend_id || '').trim()
  if (!be || be === '*') return true
  const models = modelsByBackend.get(be)
  if (!models || models.size === 0) {
    // Backend has no model list yet — keep rules so admin can still edit prices.
    return true
  }
  return models.has(model.toLowerCase())
}

export function filterRulesToConfigured(
  rules: PricingRule[],
  configuredBackendIds: Set<string>,
  modelsByBackend: Map<string, Set<string>>
): PricingRule[] {
  return rules.filter(
    (r) => isRuleForConfiguredBackend(r, configuredBackendIds) && isRuleForConfiguredModel(r, modelsByBackend)
  )
}

/** Rules whose backend_id is a concrete ID not present in configured backends (safe to prune). */
export function orphanBackendRules(rules: PricingRule[], configuredBackendIds: Set<string>): PricingRule[] {
  return rules.filter((r) => {
    const be = (r.backend_id || '').trim()
    if (!be || be === '*') return false
    return !configuredBackendIds.has(be)
  })
}
