import type { NavItem } from './types'
import { dashboardNav } from './shared'

/** Minimal 精简管理台导航（概览；计费入口在用量与会话面板内） */
export const NAV_MENU_MINIMAL: NavItem[] = [
  dashboardNav('概览')
]
