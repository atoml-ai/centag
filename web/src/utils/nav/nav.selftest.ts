/**
 * Nav menu shape by capability role (T2).
 * Run: npm run test:nav  (from web/)
 *
 * Uses relative imports so tsx can run without Vite path aliases.
 */
import { getCapabilities } from '../capabilities'
import { buildWorkerNav } from './shared'
import { NAV_MENU_TEAM_ADMIN } from './team'
import { NAV_MENU_MINIMAL } from './minimal'
import type { NavItem } from './types'
import type { Edition } from '../edition'

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function getNavMenu(edition: Edition, isAdmin = false): NavItem[] {
  if (edition === 'minimal') return NAV_MENU_MINIMAL
  if (edition === 'team' && isAdmin) return NAV_MENU_TEAM_ADMIN
  return buildWorkerNav(getCapabilities(edition, isAdmin))
}

function flattenNavMenu(menu: NavItem[]): NavItem[] {
  const items: NavItem[] = []
  for (const item of menu) {
    items.push(item)
    if (item.children?.length) {
      items.push(...flattenNavMenu(item.children))
    }
  }
  return items
}

function idsOf(edition: Edition, isAdmin: boolean): Set<string> {
  return new Set(flattenNavMenu(getNavMenu(edition, isAdmin)).map((n) => n.id))
}

function run() {
  const cases: Array<{
    name: string
    edition: Edition
    isAdmin: boolean
    mustHave: string[]
    mustNot: string[]
  }> = [
    {
      name: 'personal worker',
      edition: 'personal',
      isAdmin: true,
      mustHave: ['dashboard', 'usage', 'local-proxy', 'memory', 'more', 'storage-config', 'config-basic'],
      mustNot: ['chat', 'backends', 'pipelines', 'personal-config', 'my-tenant']
    },
    {
      name: 'team_user worker',
      edition: 'team',
      isAdmin: false,
      mustHave: ['dashboard', 'usage', 'local-proxy', 'memory', 'more', 'my-tenant'],
      mustNot: ['chat', 'backends', 'pipelines', 'storage-config', 'config-basic', 'shared-resources']
    },
    {
      name: 'team_admin ops',
      edition: 'team',
      isAdmin: true,
      mustHave: ['dashboard', 'shared-resources', 'backends', 'pipelines', 'storage-config', 'system-users'],
      mustNot: ['chat', 'local-proxy', 'memory', 'usage']
    },
    {
      name: 'minimal short',
      edition: 'minimal',
      isAdmin: false,
      mustHave: ['dashboard'],
      mustNot: ['chat', 'local-proxy', 'memory', 'more', 'backends', 'pipelines', 'storage-config']
    }
  ]

  for (const tc of cases) {
    const ids = idsOf(tc.edition, tc.isAdmin)
    for (const id of tc.mustHave) {
      assert(ids.has(id), `${tc.name}: missing nav id ${id}`)
    }
    for (const id of tc.mustNot) {
      assert(!ids.has(id), `${tc.name}: unexpected nav id ${id}`)
    }
  }

  const personalTop = getNavMenu('personal', true).map((n) => n.id)
  const userTop = getNavMenu('team', false).map((n) => n.id)
  assert(
    JSON.stringify(personalTop) === JSON.stringify(userTop),
    'personal and team_user top-level nav ids must match'
  )

  console.log('nav.selftest: OK')
}

run()
