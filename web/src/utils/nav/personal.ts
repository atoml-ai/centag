import type { NavItem } from './types'
import {
  chatNav,
  dashboardNav,
  personalAppGroup,
  personalConfigGroup,
  personalMoreGroup
} from './shared'

/**
 * 个人桌面版导航：
 * 一级 = 首页 / 配置 / 对话 / 应用 / 更多
 * 更多下再分：接入 / 缓存与记忆 / Agent / 系统
 */
export const NAV_MENU_PERSONAL: NavItem[] = [
  dashboardNav('首页'),
  personalConfigGroup(),
  chatNav(),
  personalAppGroup(),
  personalMoreGroup()
]
