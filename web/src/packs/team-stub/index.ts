/**
 * Empty Team pack stub for personal/minimal builds (E3).
 * Team SKU sets CENTAG_TEAM_PACK to centag-pro/web/packs/team.
 */
import type { RouteRecordRaw } from 'vue-router'
import type { Component } from 'vue'

export const teamPackRoutes: RouteRecordRaw[] = []

/**
 * Team pack provides the /dashboard overview page for Team SKU builds;
 * the stub falls back to the open-core Dashboard for personal/minimal.
 */
export const teamPackDashboard: Component = () => import('@/views/Dashboard.vue')

/** No commercial locale strings in personal/minimal builds. */
export const teamPackLocaleMessages: Record<string, Record<string, unknown>> = {}

export default { teamPackRoutes, teamPackDashboard, teamPackLocaleMessages }
