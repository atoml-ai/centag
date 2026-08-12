/**
 * Table-driven checks for getCapabilities (no vitest in repo).
 * Run: npm run test:capabilities  (from web/)
 */
import { getCapabilities, resolveCapabilityRole, type Capabilities } from './capabilities'

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function expectFlags(
  label: string,
  got: Capabilities,
  flags: Partial<Capabilities>
) {
  for (const [k, v] of Object.entries(flags)) {
    const key = k as keyof Capabilities
    assert(got[key] === v, `${label}: ${k} expected ${String(v)} got ${String(got[key])}`)
  }
}

function run() {
  assert(resolveCapabilityRole('personal', true) === 'personal', 'personal role')
  assert(resolveCapabilityRole('personal', false) === 'personal', 'personal ignores isAdmin')
  assert(resolveCapabilityRole('team', false) === 'team_user', 'team user role')
  assert(resolveCapabilityRole('team', true) === 'team_admin', 'team admin role')
  assert(resolveCapabilityRole('minimal', false) === 'minimal', 'minimal role')
  assert(resolveCapabilityRole('minimal', true) === 'minimal', 'minimal ignores isAdmin')

  const matrix: Array<{ label: string; edition: 'personal' | 'team' | 'minimal'; isAdmin: boolean; flags: Partial<Capabilities> }> = [
    {
      label: 'personal',
      edition: 'personal',
      isAdmin: true,
      flags: {
        navChatPage: false,
        navBackendsPage: false,
        navPipelinesPage: false,
        pipelineTestChat: true,
        storageConfig: true,
        navMoreMenu: false,
        memoryFull: false,
        memoryQuery: false,
        navFallbackPolicy: false,
        systemConfig: true,
        localProxy: true,
        liteHome: true
      }
    },
    {
      label: 'team_user',
      edition: 'team',
      isAdmin: false,
      flags: {
        navChatPage: false,
        navBackendsPage: false,
        pipelineTestChat: true,
        storageConfig: false,
        navMoreMenu: true,
        memoryQuery: true,
        memoryFull: false,
        navHostProxyTools: false,
        navFallbackPolicy: false,
        systemConfig: false,
        localProxy: true,
        liteHome: true
      }
    },
    {
      label: 'team_admin',
      edition: 'team',
      isAdmin: true,
      flags: {
        navChatPage: false,
        navBackendsPage: true,
        navPipelinesPage: true,
        pipelineTestChat: true,
        storageConfig: true,
        localProxy: false,
        memoryQuery: false,
        navFallbackPolicy: false,
        usageBilling: true,
        userAdmin: true,
        liteHome: false,
        opsStats: true,
        homeBackendsPanel: false,
        homePipelinesPanel: false
      }
    },
    {
      label: 'minimal',
      edition: 'minimal',
      isAdmin: false,
      flags: {
        pipelineTestChat: true,
        navChatPage: false,
        storageConfig: false,
        navMoreMenu: false,
        localProxy: false,
        memoryQuery: false,
        liteHome: true,
        manageBackends: true,
        managePipelines: true,
        systemConfig: true
      }
    }
  ]

  for (const row of matrix) {
    expectFlags(row.label, getCapabilities(row.edition, row.isAdmin), row.flags)
  }

  // All roles: no independent chat nav; all keep pipeline test chat
  for (const [edition, admin] of [
    ['personal', true],
    ['team', false],
    ['team', true],
    ['minimal', false]
  ] as const) {
    const caps = getCapabilities(edition, admin)
    assert(caps.navChatPage === false, `${edition}/${admin}: navChatPage must be false`)
    assert(caps.pipelineTestChat === true, `${edition}/${admin}: pipelineTestChat must be true`)
  }

  console.log('capabilities.selftest: OK')
}

run()
