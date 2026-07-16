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

export function getAllUsersUsage(params?: { from?: string; to?: string }) {
  return api({
    url: '/api/v1/admin/token-usage/all',
    method: 'get',
    params,
  })
}

export function getUserRanking(params?: { limit?: number; days?: number }) {
  return api({
    url: '/api/v1/admin/token-usage/ranking',
    method: 'get',
    params,
  })
}

/** 管理员设置配额：POST /api/v1/admin/quotas，body 含 user_id */
export function setUserQuota(userId: number, data: { daily_limit: number; monthly_limit: number }) {
  return api({
    url: '/api/v1/admin/quotas',
    method: 'post',
    data: {
      user_id: userId,
      daily_limit: data.daily_limit,
      monthly_limit: data.monthly_limit,
    },
  })
}
