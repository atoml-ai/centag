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

/** Filter a nav tree by admin role and edition. */
export function filterNavMenu(menu: NavItem[], ctx: NavVisibilityContext): NavItem[] {
  return menu
    .filter((item) => canSeeNavItem(item, ctx))
    .map((item) => ({
      ...item,
      children: item.children ? item.children.filter((child) => canSeeNavItem(child, ctx)) : undefined
    }))
    .filter((item) => !item.children || item.children.length > 0)
}