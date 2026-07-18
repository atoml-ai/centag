import type { Edition } from '@/utils/edition'
import type { NavItem } from './types'

export interface NavVisibilityContext {
  isAdmin: boolean
  edition: Edition
}

/** Whether a nav item should appear for the current user and product edition. */
export function canSeeNavItem(item: NavItem, ctx: NavVisibilityContext): boolean {
  if (item.requiresAdmin && !ctx.isAdmin) return false
  if (item.requiresTeam && ctx.edition !== 'team') return false
  return true
}

/** Filter a nav tree by admin role and edition（递归保留嵌套分组）. */
export function filterNavMenu(menu: NavItem[], ctx: NavVisibilityContext): NavItem[] {
  return menu
    .filter((item) => canSeeNavItem(item, ctx))
    .map((item) => {
      if (!item.children?.length) return { ...item }
      const children = filterNavMenu(item.children, ctx)
      return { ...item, children }
    })
    .filter((item) => !item.children || item.children.length > 0)
}
