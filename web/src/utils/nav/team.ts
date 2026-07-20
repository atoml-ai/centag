import type { NavItem } from './types'
import {
  accessGroup,
  appGroup,
  backendsNav,
  cacheMemoryGroup,
  costDashboardNav,
  dashboardNav,
  personalAppGroup,
  personalConfigGroup,
  personalMoreGroup,
  pipelinesNav,
  proxyStrategyGroup,
  chatNav,
  navGroup,
  systemAdminGroup,
  configNav
} from './shared'

/**
 * 团队超管：只管「人 + 共享资源」，不管业务干活页（对话/接入/缓存/应用等）。
 *
 * - 用户 CRUD / 限额 / 租户（API Key 在用户管理内操作）
 * - 共用后端与共用策略（系统预设）
 * - 系统配置与更新
 * - 成本看板（限额与用量总览）
 */
export const NAV_MENU_TEAM_ADMIN: NavItem[] = [
  dashboardNav('概览'),
  navGroup(
    'shared-resources',
    '共享资源',
    'Connection',
    [
      backendsNav({ label: '共用后端', requiresAdmin: true }),
      pipelinesNav({ label: '共用策略', requiresAdmin: true })
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

/**
 * 团队普通用户：与 personal 导航一致，另在「更多 → 系统」提供「我的租户」。
 */
export const NAV_MENU_TEAM_USER: NavItem[] = [
  dashboardNav('首页'),
  personalConfigGroup(),
  chatNav(),
  personalAppGroup(),
  personalMoreGroup({ teamUser: true })
]

/** @deprecated 使用 getNavMenu(edition, isAdmin) */
export const NAV_MENU_TEAM: NavItem[] = [
  dashboardNav('概览'),
  proxyStrategyGroup({ includeNodePlugins: true, adminOnly: true }),
  appGroup(),
  cacheMemoryGroup({ adminTools: true }),
  accessGroup({ adminOnly: true }),
  systemAdminGroup({ teamExtras: true })
]
