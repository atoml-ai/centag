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

// Admin all-users / ranking / quotas APIs live in Team SKU (centag-pro pack);
// open-core Host only exposes user self-service helpers above.
