import { ref } from 'vue'

export type Edition = 'personal' | 'team' | 'minimal'

const EDITION_ATTR = 'data-edition'

/**
 * Edition capability matrix:
 * - personal: desktop-oriented single-user (lite home + short nav + advanced under「更多」)
 * - team: full server deployment
 * - minimal: lite WebUI (dashboard only in nav; backends/pipelines on home)
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

export function isTeamEdition(): boolean {
  return currentEdition() === 'team'
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
  '/ab-comparison',
  '/tenants',
  '/system/users',
  '/system/update'
] as const

/**
 * Team 超管允许访问的路径白名单（人 + 共享资源 + 系统）。
 * 未列出的业务页（对话/接入/缓存/应用等）一律踢回概览。
 */
export const TEAM_ADMIN_ALLOWED_ROUTE_PREFIXES = [
  '/dashboard',
  '/backends',
  '/pipelines',
  '/system/users',
  '/system/update',
  '/tenants',
  '/config',
  '/cost',
  '/billing',
  '/profile',
  '/settings'
] as const

/** @deprecated 改用 isTeamAdminAllowedRoute 白名单 */
export const TEAM_ADMIN_BUSINESS_ROUTE_PREFIXES = [
  '/chat',
  '/conversations',
  '/token-usage',
  '/billing'
] as const

/** Routes allowed in minimal lite WebUI (plus /login). */
export const MINIMAL_ALLOWED_ROUTE_PREFIXES = [
  '/dashboard'
] as const

export function isTeamOnlyRoute(path: string): boolean {
  return TEAM_ONLY_ROUTE_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`))
}

export function isTeamAdminAllowedRoute(path: string): boolean {
  if (path === '/' || path === '/login' || path.startsWith('/login')) return true
  return TEAM_ADMIN_ALLOWED_ROUTE_PREFIXES.some(
    (prefix) => path === prefix || path.startsWith(`${prefix}/`)
  )
}

/** @deprecated 改用 isTeamAdminAllowedRoute */
export function isTeamAdminBusinessRoute(path: string): boolean {
  return !isTeamAdminAllowedRoute(path)
}

export function isMinimalAllowedRoute(path: string): boolean {
  if (path === '/login' || path.startsWith('/login')) return true
  return MINIMAL_ALLOWED_ROUTE_PREFIXES.some(
    (prefix) => path === prefix || path.startsWith(`${prefix}/`)
  )
}
