import type { Edition } from '@/utils/edition'

/** 首页主区布局（CSS grid 变体） */
export type DashboardLayout = 'lite' | 'personal' | 'team'

/**
 * 概览页板块开关。改发行版差异时优先改这里，避免在 Dashboard.vue 堆三条分支。
 */
export interface DashboardSections {
  layout: DashboardLayout
  /** 页头标题/说明（minimal 仅操作按钮） */
  pageTitle: boolean
  /** minimal：AI 对话 / 安全 / 退出 */
  headerActions: boolean
  /** 服务状态卡片 */
  serviceStatus: boolean
  /** 服务状态用紧凑四格（personal） */
  serviceStatusCompact: boolean
  /** 团队：外部地址 + 多协议接入写在状态卡内 */
  teamAccessInStatus: boolean
  /** 团队：插件 / 存储写在状态卡内 */
  pluginsStorage: boolean
  /** 系统代理 / Host 代理开关 */
  proxyControls: boolean
  /** 独立 API 接入卡片（ApiAccessPanel） */
  accessPanel: boolean
  /** 接入卡内「配置模型 / 开始对话」 */
  accessQuickLinks: boolean
  /** 接入卡紧凑样式 */
  accessCompact: boolean
  backends: boolean
  pipelines: boolean
  /** 流水线卡头「创建」按钮 */
  pipelineCreateButton: boolean
  /** 用量与会话（含计费规则入口） */
  usageBilling: boolean
  /** 用量提示：进程内 ephemeral */
  usageEphemeralHint: boolean
  /** 流水线「测试」用的精简 AI 对话抽屉（与 minimal 共用 MinimalChat） */
  liteChatDrawer: boolean
  /** 运行统计 / 模型统计 / 趋势 / 实时请求 */
  opsStats: boolean
}

const SECTION_MATRIX: Record<Edition, DashboardSections> = {
  minimal: {
    layout: 'lite',
    pageTitle: false,
    headerActions: true,
    serviceStatus: false,
    serviceStatusCompact: false,
    teamAccessInStatus: false,
    pluginsStorage: false,
    proxyControls: false,
    accessPanel: true,
    accessQuickLinks: false,
    accessCompact: true,
    backends: true,
    pipelines: true,
    pipelineCreateButton: true,
    usageBilling: true,
    usageEphemeralHint: true,
    liteChatDrawer: true,
    opsStats: false
  },
  // 桌面个人版：与 minimal 同结构干活首页；持久化/进阶页走导航「更多」
  personal: {
    layout: 'lite',
    pageTitle: true,
    headerActions: false,
    serviceStatus: false,
    serviceStatusCompact: false,
    teamAccessInStatus: false,
    pluginsStorage: false,
    proxyControls: false,
    accessPanel: true,
    accessQuickLinks: false,
    accessCompact: true,
    backends: true,
    pipelines: true,
    pipelineCreateButton: true,
    usageBilling: true,
    usageEphemeralHint: false,
    liteChatDrawer: true,
    opsStats: false
  },
  team: {
    layout: 'team',
    pageTitle: true,
    headerActions: false,
    serviceStatus: true,
    serviceStatusCompact: false,
    teamAccessInStatus: true,
    pluginsStorage: true,
    proxyControls: true,
    accessPanel: false,
    accessQuickLinks: false,
    accessCompact: false,
    backends: true,
    pipelines: true,
    pipelineCreateButton: false,
    usageBilling: true,
    usageEphemeralHint: false,
    liteChatDrawer: false,
    opsStats: true
  }
}

export function getDashboardSections(edition: Edition): DashboardSections {
  return SECTION_MATRIX[edition] ?? SECTION_MATRIX.team
}
