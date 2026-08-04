/**
 * Run: npx tsx src/utils/pricing-rule-groups.selftest.ts  (from web/)
 */
import {
  filterPricingRules,
  filterRulesToConfigured,
  groupRulesByBackend,
  isFreePricingRule,
  modelHasFreeTier,
  orphanBackendRules,
  type PricingRuleFilters
} from './pricing-rule-groups'

type Rule = {
  id?: number
  name: string
  backend_id: string
  model: string
  input_price_per_m: number
  output_price_per_m: number
  priority: number
  enabled: boolean
  price_type?: string
}

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

const rules: Rule[] = [
  {
    id: 1,
    name: 'free model',
    backend_id: 'zen',
    model: 'mimo-v2.5-free',
    input_price_per_m: 0.1,
    output_price_per_m: 0.2,
    priority: 100,
    enabled: true,
    price_type: 'cost'
  },
  {
    id: 2,
    name: 'zero price',
    backend_id: 'zen',
    model: 'local-zero',
    input_price_per_m: 0,
    output_price_per_m: 0,
    priority: 50,
    enabled: true,
    price_type: 'cost'
  },
  {
    id: 3,
    name: 'paid',
    backend_id: 'ppinfra',
    model: 'deepseek-v3.2',
    input_price_per_m: 1,
    output_price_per_m: 2,
    priority: 100,
    enabled: true,
    price_type: 'cost'
  }
]

function run() {
  assert(modelHasFreeTier('mimo-v2.5-free'), 'model free suffix')
  assert(isFreePricingRule(rules[0]), 'name free tier counts as free')
  assert(isFreePricingRule(rules[1]), 'zero price counts as free')
  assert(!isFreePricingRule(rules[2]), 'paid model')

  const groups = groupRulesByBackend(rules as any)
  assert(groups.length === 2, 'two backends')
  assert(groups[0].backendId === 'ppinfra', 'sorted backends')
  assert(groups[1].freeCount === 2 && groups[1].paidCount === 0, 'zen free/paid split')

  const filters: PricingRuleFilters = { freePaid: 'free' }
  const onlyFree = filterPricingRules(rules as any, filters)
  assert(onlyFree.length === 2, 'filter free')
  const onlyZen = filterPricingRules(rules as any, { backendId: 'zen', search: 'mimo' })
  assert(onlyZen.length === 1 && onlyZen[0].id === 1, 'filter backend+search')

  const configured = new Set(['zen'])
  const models = new Map<string, Set<string>>([['zen', new Set(['mimo-v2.5-free'])]])
  const scoped = filterRulesToConfigured(rules as any, configured, models)
  assert(scoped.length === 1 && scoped[0].id === 1, 'scope to configured backend+model')
  const orphans = orphanBackendRules(rules as any, configured)
  assert(orphans.length === 1 && orphans[0].backend_id === 'ppinfra', 'orphan backend rules')

  console.log('pricing-rule-groups.selftest: OK')
}

run()
