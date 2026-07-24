import type { NavItem } from './types'
import {
  backendsNav,
  pipelinesNav,
  fallbackPolicyNav,
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
  dashboardNav('nav.overview'),
  navGroup(
    'shared-resources',
    'nav.proxyStrategy',
    'Connection',
    [
      backendsNav({ labelKey: 'nav.backends', requiresAdmin: true }),
      pipelinesNav({ labelKey: 'nav.pipelines', requiresAdmin: true }),
      fallbackPolicyNav({ requiresAdmin: true }),
      storageConfigNavGroup()
    ],
    '/backends'
  ),
  navGroup(
    'user-admin',
    'nav.systemAdmin',
    'UserFilled',
    [
      {
        id: 'system-users',
        labelKey: 'nav.users',
        icon: 'UserFilled',
        path: '/system/users',
        requiresAdmin: true,
        requiresTeam: true
      },
      {
        id: 'tenants',
        labelKey: 'nav.tenants',
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
    'nav.system',
    'Setting',
    [
      configNav({ requiresAdmin: true }),
      {
        id: 'billing-rules',
        labelKey: 'nav.billingRules',
        icon: 'Coin',
        path: '/billing',
        requiresAdmin: true
      },
      {
        id: 'system-update',
        labelKey: 'nav.systemUpdate',
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
  dashboardNav('nav.overview'),
  proxyStrategyGroup({ includeNodePlugins: true, adminOnly: true }),
  appGroup(),
  cacheMemoryGroup({ adminTools: true }),
  accessGroup({ adminOnly: true }),
  systemAdminGroup({ teamExtras: true })
]
