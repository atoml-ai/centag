/**
 * Empty Team pack stub for personal/minimal builds (E3).
 * Team SKU sets CENTAG_TEAM_PACK to centag-pro/web/packs/team.
 */
import type { RouteRecordRaw } from 'vue-router'

export const teamPackRoutes: RouteRecordRaw[] = []

/** No commercial locale strings in personal/minimal builds. */
export const teamPackLocaleMessages: Record<string, Record<string, unknown>> = {}

export default { teamPackRoutes, teamPackLocaleMessages }
