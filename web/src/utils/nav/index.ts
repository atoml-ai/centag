import type { Edition } from '@/utils/edition'
import { getCapabilities } from '@/utils/capabilities'
import { buildWorkerNav } from './shared'
import { NAV_MENU_TEAM_ADMIN } from './team'
import { NAV_MENU_MINIMAL } from './minimal'
import type { NavItem } from './types'

export type { NavItem } from './types'
export { NAV_MENU_PERSONAL } from './personal'
export { NAV_MENU_TEAM, NAV_MENU_TEAM_ADMIN, NAV_MENU_TEAM_USER } from './team'
export { NAV_MENU_MINIMAL } from './minimal'

/**
 * 导航真源：minimal 短导航；team admin 运维导航；
 * personal / team_user 同源 buildWorkerNav(capabilities)。
 */
export function getNavMenu(edition: Edition, isAdmin = false): NavItem[] {
  if (edition === 'minimal') return NAV_MENU_MINIMAL
  if (edition === 'team' && isAdmin) return NAV_MENU_TEAM_ADMIN
  return buildWorkerNav(getCapabilities(edition, isAdmin))
}

/** Flatten menu for route → nav id sync. */
export { canSeeNavItem, filterNavMenu } from './visibility'
export type { NavVisibilityContext } from './visibility'

/** Flatten nested nav（含「更多 → 分类 → 叶子」三级） */
export function flattenNavMenu(menu: NavItem[]): NavItem[] {
  const items: NavItem[] = []
  for (const item of menu) {
    items.push(item)
    if (item.children?.length) {
      items.push(...flattenNavMenu(item.children))
    }
  }
  return items
}

/** Deep-find a nav node by id. */
export function findNavItemById(menu: NavItem[], id: string): NavItem | undefined {
  for (const item of menu) {
    if (item.id === id) return item
    if (item.children?.length) {
      const hit = findNavItemById(item.children, id)
      if (hit) return hit
    }
  }
  return undefined
}
