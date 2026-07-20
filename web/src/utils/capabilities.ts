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
  memoryQuery: boolean
  /** 记忆同步/重建等写运维（personal；非 team_user） */
  memoryFull: boolean
  usageBilling: boolean
  agentSetup: boolean
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
  memoryQuery: true,
  memoryFull: true,
  usageBilling: true,
  agentSetup: true,
  systemConfig: true,
  myTenant: false,
  userAdmin: false,
  liteHome: true
}

export function getCapabilities(edition: Edition, isAdmin = false): Capabilities {
  const role = resolveCapabilityRole(edition, isAdmin)

  if (role === 'minimal') {
    return {
      role,
      ...WORKER_CAPS,
      localProxy: false,
      storageConfig: false,
      memoryQuery: false,
      memoryFull: false,
      agentSetup: false,
      systemConfig: false
    }
  }

  if (role === 'personal') {
    return { role, ...WORKER_CAPS }
  }

  if (role === 'team_user') {
    return {
      role,
      ...WORKER_CAPS,
      storageConfig: false,
      memoryFull: false,
      systemConfig: false,
      myTenant: true
    }
  }

  // team_admin：运维面 + 共用资源；测试对话仍开；无本机代理/记忆查询/业务用量
  return {
    role,
    manageBackends: true,
    managePipelines: true,
    homeBackendsPanel: true,
    homePipelinesPanel: true,
    navBackendsPage: true,
    navPipelinesPage: true,
    pipelineTestChat: true,
    navChatPage: false,
    localProxy: false,
    storageConfig: true,
    memoryQuery: false,
    memoryFull: false,
    usageBilling: false,
    agentSetup: false,
    systemConfig: true,
    myTenant: false,
    userAdmin: true,
    liteHome: false
  }
}
