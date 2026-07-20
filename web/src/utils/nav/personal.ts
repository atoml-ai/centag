import type { NavItem } from './types'
import { buildWorkerNav } from './shared'
import { getCapabilities } from '@/utils/capabilities'

/** @deprecated 使用 getNavMenu('personal')；静态快照仅兼容旧导入 */
export const NAV_MENU_PERSONAL: NavItem[] = buildWorkerNav(getCapabilities('personal', true))
