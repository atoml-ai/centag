import type { Edition } from '@/utils/edition'

/** 产品面角色（由 edition × isAdmin 解析，非平行页面） */
export type CapabilityRole = 'minimal' | 'personal' | 'team_user' | 'team_admin'

/**
 * UI 能力位：导航 / 首页 / 路由意图的唯一差异真源。
 * backends/pipelines API 全角色可用；下列位只控制「从哪进 UI」与写运维面。
 */
export interface Capabilities {
  role: CapabilityRole
  manageBackends: boolean
  managePipelines: boolean
  homeBackendsPanel: boolean
  homePipelinesPanel: boolean
  /** 侧栏独立后端列表页（team admin） */
  navBackendsPage: boolean
  /** 侧栏独立策略列表页（team admin） */
  navPipelinesPage: boolean
  /** 流水线「测试」→ MinimalChat */
  pipelineTestChat: boolean
  /** 独立「对话」导航（目标态恒 false） */
  navChatPage: boolean
  localProxy: boolean
  storageConfig: boolean
  /** 顶栏是否展示「更多」分组（personal 暂整体隐藏） */
  navMoreMenu: boolean
  /** 更多：主机代理 + Clash（高级；主推仍是接入→系统代理） */
  navHostProxyTools: boolean
  /** 存储配置内：数据存储管理 */
  navDataStores: boolean
  /** 存储配置内：缓存评估 */
  navEvaluation: boolean
  /** 独立「降级策略」导航（已并入系统配置韧性页，默认 false） */
  navFallbackPolicy: boolean
  /** 运行统计板块（team admin 概览） */
  opsStats: boolean
  memoryQuery: boolean
  /** 记忆同步/重建等写运维（personal；非 team_user） */
  memoryFull: boolean
  usageBilling: boolean
  agentSetup: boolean
  /** 系统配置入口（personal：右上角用户菜单，非顶栏） */
  systemConfig: boolean
  myTenant: boolean
  userAdmin: boolean
  /** lite 干活首页 vs team 运维概览 */
  liteHome: boolean
}

export function resolveCapabilityRole(edition: Edition, isAdmin = false): CapabilityRole {
  if (edition === 'minimal') return 'minimal'
  if (edition === 'personal') return 'personal'
  return isAdmin ? 'team_admin' : 'team_user'
}

const WORKER_CAPS: Omit<Capabilities, 'role'> = {
  manageBackends: true,
  managePipelines: true,
  homeBackendsPanel: true,
  homePipelinesPanel: true,
  navBackendsPage: false,
  navPipelinesPage: false,
  pipelineTestChat: true,
  navChatPage: false,
  localProxy: true,
  storageConfig: true,
  navMoreMenu: true,
  navHostProxyTools: true,
  navDataStores: true,
  navEvaluation: true,
  navFallbackPolicy: false,
  memoryQuery: true,
  memoryFull: true,
  usageBilling: true,
  agentSetup: true,
  systemConfig: true,
  myTenant: false,
  userAdmin: false,
  liteHome: true,
  opsStats: false
}

/**
 * personal 发布面：
 * - 顶栏不展示「更多」（实验入口已收纳在 more 结构中，待整体开放）
 * - 系统配置改走右上角用户菜单
 * - 降级策略并入系统配置韧性页，无独立导航
 */
const PERSONAL_NAV_PREVIEW_OFF = {
  navMoreMenu: false,
  navFallbackPolicy: false,
  // 路由层仍挡住未成熟页；结构上已挂在 more 内供后续整体开放
  memoryQuery: false,
  memoryFull: false
} as const

export function getCapabilities(edition: Edition, isAdmin = false): Capabilities {
  const role = resolveCapabilityRole(edition, isAdmin)

  if (role === 'minimal') {
    return {
      role,
      ...WORKER_CAPS,
      localProxy: false,
      storageConfig: false,
      navMoreMenu: false,
      navHostProxyTools: false,
      navDataStores: false,
      navEvaluation: false,
      navFallbackPolicy: false,
      memoryQuery: false,
      memoryFull: false,
      agentSetup: false,
      systemConfig: false
    }
  }

  if (role === 'personal') {
    return { role, ...WORKER_CAPS, ...PERSONAL_NAV_PREVIEW_OFF }
  }

  if (role === 'team_user') {
    return {
      role,
      ...WORKER_CAPS,
      storageConfig: false,
      navHostProxyTools: false,
      navDataStores: false,
      navEvaluation: false,
      navFallbackPolicy: false,
      memoryFull: false,
      systemConfig: false,
      myTenant: true
    }
  }

  // team_admin：运维面 + 共用资源；测试对话仍开；无本机代理/记忆查询
  // 概览页：隐藏后端/流水线配置（有独立页面），显示运行统计和计量计费
  return {
    role,
    manageBackends: true,
    managePipelines: true,
    homeBackendsPanel: false,
    homePipelinesPanel: false,
    navBackendsPage: true,
    navPipelinesPage: true,
    pipelineTestChat: true,
    navChatPage: false,
    localProxy: false,
    storageConfig: true,
    navMoreMenu: false,
    navHostProxyTools: false,
    navDataStores: true,
    navEvaluation: true,
    navFallbackPolicy: false,
    memoryQuery: false,
    memoryFull: false,
    usageBilling: true,
    agentSetup: false,
    systemConfig: true,
    myTenant: false,
    userAdmin: true,
    liteHome: false,
    opsStats: true
  }
}
