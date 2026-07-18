import type { NavItem } from './types'
import { dashboardNav } from './shared'

/** Minimal：导航仅概览；后端/流水线/用量在首页（与 personal 共用 lite 板块） */
export const NAV_MENU_MINIMAL: NavItem[] = [
  dashboardNav('概览')
]
