import type { Edition } from '@/utils/edition'
import { NAV_MENU_PERSONAL } from './personal'
import { NAV_MENU_TEAM } from './team'
import { NAV_MENU_MINIMAL } from './minimal'
import type { NavItem } from './types'

export type { NavItem } from './types'
export { NAV_MENU_PERSONAL } from './personal'
export { NAV_MENU_TEAM } from './team'
export { NAV_MENU_MINIMAL } from './minimal'

export function getNavMenu(edition: Edition): NavItem[] {
  if (edition === 'minimal') return NAV_MENU_MINIMAL
  if (edition === 'personal') return NAV_MENU_PERSONAL
  return NAV_MENU_TEAM
}

/** Flatten menu for route → nav id sync. */
export { canSeeNavItem, filterNavMenu } from './visibility'
export type { NavVisibilityContext } from './visibility'

export function flattenNavMenu(menu: NavItem[]): NavItem[] {
  const items: NavItem[] = []
  for (const item of menu) {
    items.push(item)
    if (item.children) {
      items.push(...item.children)
    }
  }
  return items
}
