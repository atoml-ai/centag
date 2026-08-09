/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

/** E3: resolved by vite alias to team-stub or staged team pack */
declare module '@team-pack' {
  import type { RouteRecordRaw } from 'vue-router'
  import type { Component } from 'vue'
  export const teamPackRoutes: RouteRecordRaw[]
  export const teamPackDashboard: Component
  export const teamPackLocaleMessages: Record<string, Record<string, unknown>>
  const pack: {
    teamPackRoutes: RouteRecordRaw[]
    teamPackDashboard: Component
    teamPackLocaleMessages: Record<string, Record<string, unknown>>
  }
  export default pack
}
