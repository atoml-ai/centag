import type { NavItem } from './types'
import { dashboardNav } from './shared'

/** Minimal 精简管理台导航（概览 + AI 对话） */
export const NAV_MENU_MINIMAL: NavItem[] = [
  dashboardNav('概览'),
  {
    id: 'chat',
    label: 'AI 对话',
    icon: 'ChatDotRound',
    path: '/chat'
  }
]
