import type { NavItem } from './types'
import {
  accessGroup,
  appGroup,
  cacheMemoryGroup,
  dashboardNav,
  proxyStrategyGroup,
  systemAdminGroup
} from './shared'

/** 团队 Web 版 — 顶栏主菜单（与桌面版分类一致） */
export const NAV_MENU_TEAM: NavItem[] = [
  dashboardNav('概览'),
  proxyStrategyGroup({ includeNodePlugins: true, adminOnly: true }),
  appGroup(),
  cacheMemoryGroup({ adminTools: true }),
  accessGroup({ adminOnly: true }),
  systemAdminGroup({ teamExtras: true })
]