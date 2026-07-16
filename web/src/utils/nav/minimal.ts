import type { NavItem } from './types'
import { dashboardNav } from './shared'

/** Minimal 精简管理台导航（无设置页） */
export const NAV_MENU_MINIMAL: NavItem[] = [
  dashboardNav('概览'),
  {
    id: 'backends',
    label: '后端管理',
    icon: 'Connection',
    path: '/backends'
  },
  {
    id: 'pipelines',
    label: '策略管理',
    icon: 'Share',
    path: '/pipelines'
  }
]
