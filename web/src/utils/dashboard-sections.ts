import type { Edition } from '@/utils/edition'
import { getCapabilities, type Capabilities } from './capabilities'

/** 首页主区布局（CSS grid 变体） */
export type DashboardLayout = 'lite' | 'personal' | 'team'

/**
 * 概览页板块开关。由 getCapabilities 派生，避免与导航能力双写。
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
  /** 接入卡内「配置模型 / 开始对话」——目标态关闭，对话走流水线测试 */
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

export function sectionsFromCapabilities(caps: Capabilities): DashboardSections {
  if (caps.liteHome) {
    return {
      layout: 'lite',
      pageTitle: caps.role !== 'minimal',
      headerActions: caps.role === 'minimal',
      serviceStatus: false,
      serviceStatusCompact: false,
      teamAccessInStatus: false,
      pluginsStorage: false,
      proxyControls: false,
      accessPanel: true,
      accessQuickLinks: false,
      accessCompact: true,
      backends: caps.homeBackendsPanel,
      pipelines: caps.homePipelinesPanel,
      pipelineCreateButton: true,
      usageBilling: caps.usageBilling,
      usageEphemeralHint: caps.role === 'minimal',
      liteChatDrawer: caps.pipelineTestChat,
      opsStats: false
    }
  }

  return {
    layout: 'team',
    pageTitle: true,
    headerActions: false,
    serviceStatus: true,
    serviceStatusCompact: false,
    teamAccessInStatus: false,
    pluginsStorage: false,
    proxyControls: false,
    accessPanel: false,
    accessQuickLinks: false,
    accessCompact: false,
    backends: caps.homeBackendsPanel,
    pipelines: caps.homePipelinesPanel,
    pipelineCreateButton: true,
    usageBilling: false,
    usageEphemeralHint: false,
    liteChatDrawer: caps.pipelineTestChat,
    opsStats: false
  }
}

/**
 * @param roleAdmin team 下为 true 时使用运维概览；普通用户用 lite 干活首页
 */
export function getDashboardSections(edition: Edition, roleAdmin = false): DashboardSections {
  return sectionsFromCapabilities(getCapabilities(edition, roleAdmin))
}
