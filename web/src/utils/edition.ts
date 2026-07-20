import { ref } from 'vue'
import { getCapabilities, type Capabilities } from './capabilities'

export type Edition = 'personal' | 'team' | 'minimal'

const EDITION_ATTR = 'data-edition'

/**
 * Edition capability matrix:
 * - personal: desktop-oriented single-user (lite home + short nav)
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
 * Team 超管允许访问的路径白名单（人 + 共享资源 + 存储配置 + 系统）。
 * 未列出的业务页（对话/接入/记忆/本机代理等）一律踢回概览。
 */
export const TEAM_ADMIN_ALLOWED_ROUTE_PREFIXES = [
  '/dashboard',
  '/backends',
  '/pipelines',
  '/storage',
  '/data-stores',
  '/cache',
  '/evaluation',
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

function normalizePath(path: string): string {
  if (!path) return '/'
  const bare = path.split('?')[0].split('#')[0]
  if (bare.length > 1 && bare.endsWith('/')) return bare.slice(0, -1)
  return bare
}

/** Exact list routes that redirect to dashboard when nav.*Page is false */
function isBackendListPath(path: string): boolean {
  const p = normalizePath(path)
  return p === '/backends'
}

function isPipelineListPath(path: string): boolean {
  const p = normalizePath(path)
  return p === '/pipelines'
}

function isChatProductPath(path: string): boolean {
  const p = normalizePath(path)
  return p === '/chat' || p.startsWith('/chat/')
}

function isStorageConfigPath(path: string): boolean {
  const p = normalizePath(path)
  return (
    p === '/storage' ||
    p.startsWith('/storage/') ||
    p === '/data-stores' ||
    p.startsWith('/data-stores/') ||
    p === '/cache' ||
    p.startsWith('/cache/') ||
    p === '/evaluation' ||
    p.startsWith('/evaluation/')
  )
}

/**
 * 普通用户 / personal 路由意图：列表页可 redirect；深链编辑放行。
 * 返回应 redirect 的目标，或 null 表示放行。
 * 不删除 backends/pipelines API——仅调整 UI 入口。
 */
export function resolveCapabilityRouteRedirect(
  path: string,
  caps: Capabilities,
  edition: Edition
): string | null {
  if (edition === 'minimal') {
    return isMinimalAllowedRoute(path) ? null : '/dashboard'
  }

  if (isChatProductPath(path)) {
    return '/dashboard'
  }

  if (!caps.navBackendsPage && isBackendListPath(path)) {
    return '/dashboard'
  }

  if (!caps.navPipelinesPage && isPipelineListPath(path)) {
    return '/dashboard'
  }

  if (!caps.storageConfig && isStorageConfigPath(path)) {
    return '/dashboard'
  }

  if (!caps.localProxy) {
    const p = normalizePath(path)
    if (
      p === '/host-proxy' ||
      p.startsWith('/host-proxy/') ||
      p === '/system-proxy' ||
      p.startsWith('/system-proxy/') ||
      p === '/clash-rules' ||
      p.startsWith('/clash-rules/')
    ) {
      return '/dashboard'
    }
  }

  if (!caps.memoryQuery) {
    const p = normalizePath(path)
    if (p === '/memory' || p.startsWith('/memory/')) {
      return '/dashboard'
    }
  }

  return null
}

/** 供路由守卫：结合 edition + isAdmin */
export function resolveEditionRouteRedirect(
  path: string,
  edition: Edition,
  isAdmin: boolean
): string | null {
  if (path === '/login' || path.startsWith('/login')) return null

  if (edition === 'personal' && isTeamOnlyRoute(path)) {
    return '/dashboard'
  }

  if (edition === 'team' && isAdmin) {
    if (isChatProductPath(path)) return '/dashboard'
    return isTeamAdminAllowedRoute(path) ? null : '/dashboard'
  }

  if (edition === 'minimal') {
    return isMinimalAllowedRoute(path) ? null : '/dashboard'
  }

  // personal / team_user
  const caps = getCapabilities(edition, isAdmin)
  return resolveCapabilityRouteRedirect(path, caps, edition)
}
