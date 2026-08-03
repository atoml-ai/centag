/**
 * Nav menu shape by capability role (T2).
 * Run: npm run test:nav  (from web/)
 *
 * Uses relative imports so tsx can run without Vite path aliases.
 */
import { getCapabilities } from '../capabilities'
import { buildMoreNavChildren, buildWorkerNav } from './shared'
import { NAV_MENU_TEAM_ADMIN } from './team'
import { NAV_MENU_MINIMAL } from './minimal'
import { filterNavMenu } from './visibility'
import type { NavItem } from './types'
import type { Edition } from '../edition'

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function getNavMenu(edition: Edition, isAdmin = false): NavItem[] {
  if (edition === 'minimal') return NAV_MENU_MINIMAL
  if (edition === 'team' && isAdmin) return NAV_MENU_TEAM_ADMIN
  // 与运行时 useNavigation 一致：按 edition/isAdmin 做可见性过滤
  return filterNavMenu(buildWorkerNav(getCapabilities(edition, isAdmin)), {
    isAdmin,
    edition
  })
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
      // 顶栏仅首页/用量/接入；更多整体隐藏；系统配置走用户菜单
      mustHave: ['dashboard', 'usage', 'access'],
      mustNot: [
        'chat',
        'backends',
        'pipelines',
        'personal-config',
        'my-tenant',
        'my-billing',
        'local-proxy',
        'more',
        'config-basic',
        'fallback-policies',
        'memory',
        'host-proxy',
        'clash-rules',
        'data-stores',
        'evaluation',
        'logs',
        'storage-config'
      ]
    },
    {
      name: 'team_user worker',
      edition: 'team',
      isAdmin: false,
      mustHave: ['dashboard', 'usage', 'access', 'memory', 'more', 'my-billing', 'logs'],
      mustNot: [
        'chat',
        'backends',
        'pipelines',
        'storage-config',
        'config-basic',
        'shared-resources',
        'local-proxy',
        'host-proxy',
        'clash-rules',
        'fallback-policies'
      ]
    },
    {
      name: 'team_admin ops',
      edition: 'team',
      isAdmin: true,
      mustHave: [
        'dashboard',
        'backends',
        'pipelines',
        'cache-management',
        'user-tenant',
        'system-users',
        'cost-dashboard',
        'pricing-sync',
        'config-basic',
        'ab-comparison'
      ],
      mustNot: [
        'chat',
        'access',
        'memory',
        'usage',
        'my-billing',
        'local-proxy',
        'more',
        'shared-resources',
        'storage-config'
      ]
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

  // personal 顶栏不再含 more；与 team_user 顶栏不必完全一致
  const personalTop = getNavMenu('personal', true).map((n) => n.id)
  assert(
    JSON.stringify(personalTop) === JSON.stringify(['dashboard', 'usage', 'access']),
    'personal top-level nav should be dashboard/usage/access'
  )

  // personal 更多结构仍收纳实验入口（虽不挂顶栏）
  const personalMoreIds = new Set(
    flattenNavMenu(buildMoreNavChildren(getCapabilities('personal', true))).map((n) => n.id)
  )
  for (const id of ['storage-config', 'memory', 'host-proxy', 'clash-rules', 'logs', 'data-stores', 'evaluation']) {
    assert(personalMoreIds.has(id), `personal more stash missing ${id}`)
  }
  assert(!personalMoreIds.has('config-basic'), 'personal more stash must not include system config')
  assert(!personalMoreIds.has('fallback-policies'), 'personal more stash must not include fallback nav')

  assert(
    !getNavMenu('team', false).some((n) => n.id === 'memory'),
    'team_user: memory not top-level (lives under more)'
  )
  assert(idsOf('team', false).has('memory'), 'team_user: memory available under more')

  console.log('nav.selftest: OK')
}

run()
