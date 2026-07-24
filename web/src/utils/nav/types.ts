export interface NavItem {
  id: string
  label?: string
  labelKey?: string
  icon: string
  path?: string
  /** Hidden for non-admin users. */
  requiresAdmin?: boolean
  /** Hidden in personal (desktop) edition — must match backend teamEditionOnly routes. */
  requiresTeam?: boolean
  children?: NavItem[]
}