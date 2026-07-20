/**
 * Dashboard sections derived from capabilities (T1/T3/T5).
 * Run: npm run test:dashboard-sections  (from web/)
 */
import { getDashboardSections } from './dashboard-sections'

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function run() {
  const cases: Array<{
    name: string
    edition: 'personal' | 'team' | 'minimal'
    isAdmin: boolean
    expect: Partial<ReturnType<typeof getDashboardSections>>
  }> = [
    {
      name: 'personal lite',
      edition: 'personal',
      isAdmin: true,
      expect: {
        layout: 'lite',
        backends: true,
        pipelines: true,
        liteChatDrawer: true,
        accessQuickLinks: false,
        usageBilling: true,
        accessPanel: true
      }
    },
    {
      name: 'team_user lite',
      edition: 'team',
      isAdmin: false,
      expect: {
        layout: 'lite',
        liteChatDrawer: true,
        accessQuickLinks: false,
        usageBilling: true
      }
    },
    {
      name: 'team_admin ops + test drawer',
      edition: 'team',
      isAdmin: true,
      expect: {
        layout: 'team',
        liteChatDrawer: true,
        accessQuickLinks: false,
        backends: true,
        pipelines: true,
        usageBilling: false
      }
    },
    {
      name: 'minimal lite',
      edition: 'minimal',
      isAdmin: false,
      expect: {
        layout: 'lite',
        headerActions: true,
        liteChatDrawer: true,
        usageEphemeralHint: true,
        accessQuickLinks: false
      }
    }
  ]

  for (const tc of cases) {
    const got = getDashboardSections(tc.edition, tc.isAdmin)
    for (const [k, v] of Object.entries(tc.expect)) {
      const key = k as keyof typeof got
      assert(got[key] === v, `${tc.name}: ${k} expected ${String(v)} got ${String(got[key])}`)
    }
  }

  console.log('dashboard-sections.selftest: OK')
}

run()
