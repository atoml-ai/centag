import api from './index'

// 获取用户 Token 使用统计（需 JWT，路径与 server 注册一致）
export function getUserUsage(params?: { from?: string; to?: string }) {
  return api({
    url: '/api/v1/user/token-usage',
    method: 'get',
    params,
  })
}

export function getDailyUsage(params?: { days?: number }) {
  return api({
    url: '/api/v1/user/token-usage/daily',
    method: 'get',
    params,
  })
}

export function getModelStats(params?: { days?: number }) {
  return api({
    url: '/api/v1/user/token-usage/models',
    method: 'get',
    params,
  })
}

export function getBackendStats(params?: { days?: number }) {
  return api({
    url: '/api/v1/user/token-usage/backends',
    method: 'get',
    params,
  })
}

// 用户计量计价明细：按 (backend_id, model) 聚合，含单价与成本
export function getUsageBreakdown(params?: { from?: string; to?: string }) {
  return api({
    url: '/api/v1/user/usage',
    method: 'get',
    params,
  })
}

// 会话计量计价汇总：批量返回多个会话的计量与计价摘要（键为 session_id）
export function getSessionsUsageBreakdown(ids: string[]) {
  return api({
    url: '/api/v1/user/usage/sessions',
    method: 'get',
    params: { ids: ids.join(',') }
  })
}

// 用户自我限额（已移除，返回 enabled=false）
export function getSelfLimit() {
  return api({
    url: '/api/v1/user/usage/self-limit',
    method: 'get',
  })
}

// Admin all-users / ranking / quotas APIs live in Team SKU (centag-pro pack);
// open-core Host only exposes user self-service helpers above.
