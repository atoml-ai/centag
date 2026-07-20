/**
 * Route intent checks (R01 deep-link vs list redirect).
 * Run: npm run test:edition-routes  (from web/)
 */
import { getCapabilities } from './capabilities'
import { resolveCapabilityRouteRedirect, resolveEditionRouteRedirect } from './edition'

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg)
}

function run() {
  const userCaps = getCapabilities('team', false)
  assert(
    resolveCapabilityRouteRedirect('/pipelines', userCaps, 'team') === '/dashboard',
    'user: pipeline list → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/pipelines/create', userCaps, 'team') === null,
    'user: pipeline create allowed'
  )
  assert(
    resolveCapabilityRouteRedirect('/pipelines/foo-id', userCaps, 'team') === null,
    'user: pipeline id allowed'
  )
  assert(
    resolveCapabilityRouteRedirect('/backends', userCaps, 'team') === '/dashboard',
    'user: backends list → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/chat', userCaps, 'team') === '/dashboard',
    'user: chat → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/storage', userCaps, 'team') === '/dashboard',
    'user: storage → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/memory', userCaps, 'team') === null,
    'user: memory allowed'
  )

  const personalCaps = getCapabilities('personal', true)
  assert(
    resolveCapabilityRouteRedirect('/storage', personalCaps, 'personal') === null,
    'personal: storage allowed'
  )
  assert(
    resolveCapabilityRouteRedirect('/pipelines', personalCaps, 'personal') === '/dashboard',
    'personal: pipeline list → dashboard'
  )

  assert(
    resolveEditionRouteRedirect('/pipelines/abc', 'team', false) === null,
    'edition: team user deep link'
  )
  assert(
    resolveEditionRouteRedirect('/pipelines', 'team', false) === '/dashboard',
    'edition: team user list'
  )
  assert(
    resolveEditionRouteRedirect('/backends', 'team', true) === null,
    'edition: team admin backends allowed'
  )
  assert(
    resolveEditionRouteRedirect('/chat', 'team', true) === '/dashboard',
    'edition: team admin chat redirect'
  )
  assert(
    resolveEditionRouteRedirect('/storage', 'team', true) === null,
    'edition: team admin storage allowed'
  )
  assert(
    resolveEditionRouteRedirect('/memory', 'team', true) === '/dashboard',
    'edition: team admin memory blocked'
  )

  // Edge: trailing slash + storage family + proxy block for admin
  assert(
    resolveCapabilityRouteRedirect('/pipelines/', userCaps, 'team') === '/dashboard',
    'user: pipeline list trailing slash → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/data-stores', userCaps, 'team') === '/dashboard',
    'user: data-stores → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/cache', userCaps, 'team') === '/dashboard',
    'user: cache → dashboard'
  )
  assert(
    resolveCapabilityRouteRedirect('/evaluation', userCaps, 'team') === '/dashboard',
    'user: evaluation → dashboard'
  )
  assert(
    resolveEditionRouteRedirect('/system-proxy', 'team', true) === '/dashboard',
    'edition: team admin local proxy blocked'
  )
  assert(
    resolveEditionRouteRedirect('/host-proxy', 'team', true) === '/dashboard',
    'edition: team admin host-proxy blocked'
  )
  assert(
    resolveEditionRouteRedirect('/backends', 'minimal', false) === '/dashboard',
    'edition: minimal backends → dashboard'
  )
  assert(
    resolveEditionRouteRedirect('/dashboard', 'minimal', false) === null,
    'edition: minimal dashboard allowed'
  )
  assert(
    resolveEditionRouteRedirect('/pipelines/create', 'personal', true) === null,
    'edition: personal create allowed'
  )

  console.log('edition.route.selftest: OK')
}

run()
