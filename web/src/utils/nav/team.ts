import type { NavItem } from './types'
import {
  backendsNav,
  pipelinesNav,
  costDashboardNav,
  dashboardNav,
  storageConfigNavGroup,
  navGroup,
  configNav,
  appGroup,
  cacheMemoryGroup,
  accessGroup,
  systemAdminGroup,
  proxyStrategyGroup
} from './shared'

/**
 * 团队超管：人 + 共享资源 + 存储配置；流水线测试对话在首页抽屉（非侧栏）。
 */
export const NAV_MENU_TEAM_ADMIN: NavItem[] = [
  dashboardNav('概览'),
  navGroup(
    'shared-resources',
    '共享资源',
    'Connection',
    [
      backendsNav({ label: '共用后端', requiresAdmin: true }),
      pipelinesNav({ label: '共用策略', requiresAdmin: true }),
      storageConfigNavGroup()
    ],
    '/backends'
  ),
  navGroup(
    'user-admin',
    '用户与租户',
    'UserFilled',
    [
      {
        id: 'system-users',
        label: '用户管理',
        icon: 'UserFilled',
        path: '/system/users',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'tenants',
        label: '租户管理',
        icon: 'OfficeBuilding',
        path: '/tenants',
        requiresAdmin: true,
        requiresTeam: true
      },
      costDashboardNav()
    ],
    '/system/users'
  ),
  navGroup(
    'system-admin',
    '系统',
    'Setting',
    [
      configNav({ requiresAdmin: true }),
      {
        id: 'billing-rules',
        label: '计费规则',
        icon: 'Coin',
        path: '/billing',
        requiresAdmin: true
      },
      {
        id: 'system-update',
        label: '系统更新',
        icon: 'Upload',
        path: '/system/update',
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
  dashboardNav('概览'),
  proxyStrategyGroup({ includeNodePlugins: true, adminOnly: true }),
  appGroup(),
  cacheMemoryGroup({ adminTools: true }),
  accessGroup({ adminOnly: true }),
  systemAdminGroup({ teamExtras: true })
]
