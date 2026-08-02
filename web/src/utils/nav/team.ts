import type { NavItem } from './types'
import {
  backendsNav,
  pipelinesNav,
  costDashboardNav,
  dashboardNav,
  cacheManagementNavGroup,
  navGroup,
  configNav,
  userTenantGroup,
  proxyStrategyGroup,
  appGroup,
  cacheMemoryGroup,
  accessGroup,
  systemAdminGroup
} from './shared'

/**
 * 团队超管：人 + 共用资源 + 存储配置；流水线测试对话在首页抽屉（非侧栏）。
 */
export const NAV_MENU_TEAM_ADMIN: NavItem[] = [
  dashboardNav('nav.overview'),
  backendsNav({ labelKey: 'nav.backends', requiresAdmin: true }),
  pipelinesNav({ labelKey: 'nav.pipelines', requiresAdmin: true }),
  cacheManagementNavGroup(),
  userTenantGroup(),
  navGroup(
    'system-admin',
    'nav.system',
    'Setting',
    [
      configNav({ requiresAdmin: true }),
      {
        id: 'ab-comparison',
        labelKey: 'nav.abComparison',
        icon: 'DataAnalysis',
        path: '/ab-comparison',
        requiresAdmin: true,
        requiresTeam: true
      }
    ],
    '/config'
  )
]

/** @deprecated 使用 getNavMenu(edition, isAdmin)；保留导出以免外部引用断裂 */
export const NAV_MENU_TEAM_USER: NavItem[] = []

/** @deprecated 使用 getNavMenu(edition, isAdmin) */
export const NAV_MENU_TEAM: NavItem[] = [
  dashboardNav('nav.overview'),
  proxyStrategyGroup({ includeNodePlugins: true, adminOnly: true }),
  appGroup(),
  cacheMemoryGroup({ adminTools: true }),
  accessGroup({ adminOnly: true }),
  systemAdminGroup({ teamExtras: true })
]
