import api from './index'

export interface PricingRule {
  id?: number
  name: string
  backend_id: string
  model: string
  input_price_per_m: number
  output_price_per_m: number
  currency?: string
  priority: number
  enabled: boolean
  price_type?: string // 'cost' or 'revenue'
}

export function listPricingRules(priceType?: string) {
  const params: Record<string, string> = {}
  if (priceType) params.price_type = priceType
  return api({
    url: '/api/v1/admin/billing/rules',
    method: 'get',
    params
  }) as Promise<PricingRule[]>
}

export function createPricingRule(data: PricingRule) {
  return api({
    url: '/api/v1/admin/billing/rules',
    method: 'post',
    data
  }) as Promise<PricingRule>
}

export function updatePricingRule(id: number, data: PricingRule) {
  return api({
    url: `/api/v1/admin/billing/rules/${id}`,
    method: 'put',
    data
  }) as Promise<PricingRule>
}

export function deletePricingRule(id: number) {
  return api({
    url: `/api/v1/admin/billing/rules/${id}`,
    method: 'delete'
  })
}

export function importPricingRules(yamlText: string) {
  return api({
    url: '/api/v1/admin/billing/rules/import',
    method: 'post',
    data: yamlText,
    headers: { 'Content-Type': 'application/x-yaml' },
    transformRequest: [(data) => data]
  }) as Promise<{ imported: number }>
}

export function exportPricingRules() {
  return api({
    url: '/api/v1/admin/billing/rules/export',
    method: 'get',
    responseType: 'text'
  }) as Promise<string>
}

// Plan templates + assignments
export interface PlanTemplate {
  id?: number
  name: string
  description?: string
  enabled: boolean
  is_builtin?: boolean
  currency?: string
  price_type?: string
  budget_amount?: number
  budget_period?: string
  token_quota_total?: number
  token_quota_input?: number
  token_quota_output?: number
  token_quota_period?: string
  rate_limit_rpm?: number
  rate_limit_tpm?: number
}

export interface UserPlanAssignment {
  user_id: number
  template_id: number
  assigned_at?: string
  assigned_by?: string
  template_name?: string
  username?: string
}

export interface GroupPlanAssignment {
  group_id: string
  template_id: number
  metering_mode: 'per_member' | 'shared_pool'
  assigned_at?: string
  template_name?: string
  group_name?: string
}

export function listPlanTemplates() {
  return api({ url: '/api/v1/admin/billing/plan-templates', method: 'get' }) as Promise<PlanTemplate[]>
}

export function createPlanTemplate(data: PlanTemplate) {
  return api({ url: '/api/v1/admin/billing/plan-templates', method: 'post', data }) as Promise<PlanTemplate>
}

export function updatePlanTemplate(id: number, data: PlanTemplate) {
  return api({ url: `/api/v1/admin/billing/plan-templates/${id}`, method: 'put', data }) as Promise<PlanTemplate>
}

export function deletePlanTemplate(id: number) {
  return api({ url: `/api/v1/admin/billing/plan-templates/${id}`, method: 'delete' })
}

export function listUserPlanAssignments() {
  return api({ url: '/api/v1/admin/billing/user-plan-assignments', method: 'get' }) as Promise<UserPlanAssignment[]>
}

export function upsertUserPlanAssignment(userId: number, templateId: number, assignedBy?: string) {
  return api({
    url: `/api/v1/admin/billing/user-plan-assignments/${userId}`,
    method: 'put',
    data: { template_id: templateId, assigned_by: assignedBy || '' }
  })
}

export function deleteUserPlanAssignment(userId: number) {
  return api({ url: `/api/v1/admin/billing/user-plan-assignments/${userId}`, method: 'delete' })
}

export function listGroupPlanAssignments() {
  return api({ url: '/api/v1/admin/billing/group-plan-assignments', method: 'get' }) as Promise<GroupPlanAssignment[]>
}

export function upsertGroupPlanAssignment(
  groupId: string,
  templateId: number,
  meteringMode: 'per_member' | 'shared_pool'
) {
  return api({
    url: `/api/v1/admin/billing/group-plan-assignments/${encodeURIComponent(groupId)}`,
    method: 'put',
    data: { template_id: templateId, metering_mode: meteringMode }
  })
}

export function deleteGroupPlanAssignment(groupId: string) {
  return api({
    url: `/api/v1/admin/billing/group-plan-assignments/${encodeURIComponent(groupId)}`,
    method: 'delete'
  })
}

// UserPlan types and APIs (legacy flat rows)
export interface UserPlan {
  id?: number
  user_id: string
  tenant_id?: string
  plan_name: string
  currency?: string
  price_type?: string
  budget_amount?: number
  budget_period?: string
  budget_start_at?: string
  budget_end_at?: string
  token_quota_total?: number
  token_quota_input?: number
  token_quota_output?: number
  token_quota_period?: string
  token_quota_start_at?: string
  token_quota_end_at?: string
  rate_limit_rpm?: number
  rate_limit_tpm?: number
  available_backend_ids?: string[]
  available_models?: string[]
  available_pipeline_ids?: string[]
  /** @deprecated use available_backend_ids */
  allowed_backends?: string[]
  /** @deprecated use available_models */
  allowed_models?: string[]
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export function listUserPlans() {
  return api({
    url: '/api/v1/admin/billing/user-plans',
    method: 'get'
  }) as Promise<UserPlan[]>
}

export function getUserPlan(id: number) {
  return api({
    url: `/api/v1/admin/billing/user-plans/${id}`,
    method: 'get'
  }) as Promise<UserPlan>
}

export function createUserPlan(data: UserPlan) {
  return api({
    url: '/api/v1/admin/billing/user-plans',
    method: 'post',
    data
  }) as Promise<UserPlan>
}

export function updateUserPlan(id: number, data: UserPlan) {
  return api({
    url: `/api/v1/admin/billing/user-plans/${id}`,
    method: 'put',
    data
  }) as Promise<UserPlan>
}

export function deleteUserPlan(id: number) {
  return api({
    url: `/api/v1/admin/billing/user-plans/${id}`,
    method: 'delete'
  })
}

// UserPricingOverride types and APIs
export interface UserPricingOverride {
  id?: number
  user_id: string
  backend_id: string
  model: string
  input_price_per_m: number
  output_price_per_m: number
  currency?: string
  price_type?: string
  expires_at?: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export function listUserPricingOverrides() {
  return api({
    url: '/api/v1/admin/billing/user-pricing-overrides',
    method: 'get'
  }) as Promise<UserPricingOverride[]>
}

export function createUserPricingOverride(data: UserPricingOverride) {
  return api({
    url: '/api/v1/admin/billing/user-pricing-overrides',
    method: 'post',
    data
  }) as Promise<UserPricingOverride>
}

export function updateUserPricingOverride(id: number, data: UserPricingOverride) {
  return api({
    url: `/api/v1/admin/billing/user-pricing-overrides/${id}`,
    method: 'put',
    data
  }) as Promise<UserPricingOverride>
}

export function deleteUserPricingOverride(id: number) {
  return api({
    url: `/api/v1/admin/billing/user-pricing-overrides/${id}`,
    method: 'delete'
  })
}

// User billing APIs (for user self-service)
export interface UserBillingInfo {
  plan: UserPlan | null
  scope: {
    allowed_backends: string[]
    allowed_models: string[]
    rate_limit_rpm?: number
    rate_limit_tpm?: number
  }
  usage: {
    input_tokens: number
    output_tokens: number
    total_tokens: number
    billed_amount?: number
    total_cost: number
    budget_amount?: number
    budget_remaining?: number
    token_quota_total?: number
    token_remaining?: number
    token_quota_input?: number
    token_input_remaining?: number
    token_quota_output?: number
    token_output_remaining?: number
    period_start?: string
    period_end?: string
    price_type?: string
  }
}

export function getUserBillingInfo() {
  return api({
    url: '/api/v1/user/billing/info',
    method: 'get'
  }) as Promise<UserBillingInfo>
}

export function getUserBillingSummary(params?: { group_by?: string }) {
  return api({
    url: '/api/v1/user/billing/summary',
    method: 'get',
    params
  }) as Promise<import('./cost').CostSummary>
}

export function getUserUsageDetail(params?: { start_date?: string; end_date?: string }) {
  return api({
    url: '/api/v1/user/billing/usage',
    method: 'get',
    params
  }) as Promise<{
    records: Array<{
      date: string
      model: string
      backend_id: string
      input_tokens: number
      output_tokens: number
      cost_input_price: number
      cost_output_price: number
      total_cost: number
    }>
    summary: {
      total_input_tokens: number
      total_output_tokens: number
      total_cost: number
    }
  }>
}

// Pricing sync batch types and APIs (Team pricing-sync workflow)
export interface PricingSyncItem {
  id?: number
  batch_id?: number
  backend_id: string
  model: string
  litellm_key?: string
  price_type?: string
  input_price_per_m?: number
  output_price_per_m?: number
  resolution?: string
  source?: string
  selected: boolean
  status?: string
  created_at?: string
}

export interface PricingSyncBatch {
  id?: number
  source: string
  status: string
  bin_path?: string
  config_dir?: string
  error?: string
  created_at?: string
  applied_at?: string
  items?: PricingSyncItem[]
}

export function triggerPricingSync(data: { source?: string; bin_path?: string; config_dir?: string }) {
  return api({
    url: '/api/v1/admin/billing/sync/trigger',
    method: 'post',
    data
  }) as Promise<PricingSyncBatch>
}

export function createPricingSyncBatch(data: {
  source?: string
  bin_path?: string
  config_dir?: string
  items: PricingSyncItem[]
}) {
  return api({
    url: '/api/v1/admin/billing/sync',
    method: 'post',
    data
  }) as Promise<PricingSyncBatch>
}

export function listPricingSyncBatches(params?: { limit?: number }) {
  return api({
    url: '/api/v1/admin/billing/sync',
    method: 'get',
    params
  }) as Promise<PricingSyncBatch[]>
}

export function getPricingSyncBatch(id: number) {
  return api({
    url: `/api/v1/admin/billing/sync/${id}`,
    method: 'get'
  }) as Promise<PricingSyncBatch>
}

export function selectPricingSyncItems(id: number, itemIds: number[]) {
  return api({
    url: `/api/v1/admin/billing/sync/${id}/select`,
    method: 'post',
    data: { item_ids: itemIds }
  }) as Promise<PricingSyncItem[]>
}

export function applyPricingSyncBatch(id: number) {
  return api({
    url: `/api/v1/admin/billing/sync/${id}/apply`,
    method: 'post'
  }) as Promise<{ applied: number; skipped?: number }>
}

export function rejectPricingSyncBatch(id: number) {
  return api({
    url: `/api/v1/admin/billing/sync/${id}/reject`,
    method: 'post'
  })
}

export function deletePricingSyncBatch(id: number) {
  return api({
    url: `/api/v1/admin/billing/sync/${id}`,
    method: 'delete'
  })
}

// Configsync — Feishu Bitable price sync
export interface ConfigSyncStatus {
  enabled: boolean
  provider?: string
  last_sync?: string
  ok?: boolean
  error?: string
}

export function getConfigSyncStatus() {
  return api({
    url: '/api/v1/admin/configsync/status',
    method: 'get'
  }) as Promise<ConfigSyncStatus>
}

export function triggerConfigSyncNow() {
  return api({
    url: '/api/v1/admin/configsync/sync-now',
    method: 'post'
  }) as Promise<{ status: string; synced_at: string }>
}
