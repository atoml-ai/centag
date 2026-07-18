import { ref } from 'vue'

export type Edition = 'personal' | 'team' | 'minimal'

const EDITION_ATTR = 'data-edition'

/**
 * Edition capability matrix:
 * - personal: single-user surfaces (legacy desktop-oriented)
 * - team: full server deployment
 * - minimal: lite WebUI (dashboard / backends / pipelines + password auth)
 */

function readEditionFromDom(): Edition {
  const edition = document.documentElement.getAttribute(EDITION_ATTR)
  if (edition === 'personal' || edition === 'minimal') {
    return edition
  }
  return 'team'
}

/** Reactive edition for Vue; updated by HTML injection and /api/v1/status. */
export const editionRef = ref<Edition>(typeof document !== 'undefined' ? readEditionFromDom() : 'team')

export function currentEdition(): Edition {
  return editionRef.value
}

export function isPersonalEdition(): boolean {
  return currentEdition() === 'personal'
}

export function isMinimalEdition(): boolean {
  return currentEdition() === 'minimal'
}

/** Apply edition from /api/v1/status (authoritative when HTML injection is absent). */
export function syncEditionFromStatus(status: { edition?: string } | null | undefined) {
  const edition = status?.edition
  if (edition === 'personal' || edition === 'team' || edition === 'minimal') {
    document.documentElement.setAttribute(EDITION_ATTR, edition)
    editionRef.value = edition
  }
}

export const TEAM_ONLY_ROUTE_PREFIXES = [
  '/cost',
  '/billing',
  '/ab-comparison',
  '/tenants',
  '/system/users',
  '/system/update'
] as const

/** Routes allowed in minimal lite WebUI (plus /login). */
export const MINIMAL_ALLOWED_ROUTE_PREFIXES = [
  '/dashboard'
] as const

export function isTeamOnlyRoute(path: string): boolean {
  return TEAM_ONLY_ROUTE_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`))
}

export function isMinimalAllowedRoute(path: string): boolean {
  if (path === '/login' || path.startsWith('/login')) return true
  return MINIMAL_ALLOWED_ROUTE_PREFIXES.some(
    (prefix) => path === prefix || path.startsWith(`${prefix}/`)
  )
}
