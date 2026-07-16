import type { NavItem } from './types'
import {
  accessGroup,
  appGroup,
  cacheMemoryGroup,
  dashboardNav,
  proxyStrategyGroup,
  systemAdminGroup
} from './shared'

/**
 * 个人版 — 顶栏主菜单（含原桌面侧栏收纳的进阶分组）
 */
export const NAV_MENU_PERSONAL: NavItem[] = [
  dashboardNav('首页'),
  proxyStrategyGroup({ includeNodePlugins: true }),
  appGroup({ tokenUsageLabel: '用量' }),
  cacheMemoryGroup(),
  accessGroup(),
  systemAdminGroup({ relaxedAccess: true })
]
