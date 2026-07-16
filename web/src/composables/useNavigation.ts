import { computed, watch, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { flattenNavMenu, getNavMenu, type NavItem } from '@/utils/nav'
import { filterNavMenu } from '@/utils/nav/visibility'

export function useNavigation() {
  const router = useRouter()
  const route = useRoute()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const { edition } = useEdition()

  const navMenu = computed(() => getNavMenu(edition.value))

  const visibleNavItems = computed(() => {
    return filterNavMenu(navMenu.value, {
      isAdmin: authStore.isAdmin,
      edition: edition.value
    })
  })

  const currentNav = computed(() => appStore.currentNav)

  function isChildActive(item: NavItem) {
    if (!item.children) return false
    return item.children.some((child) => child.id === currentNav.value)
  }

  function navigateTo(item: NavItem) {
    appStore.setCurrentNav(item.id)
    if (item.path) router.push(item.path)
  }

  function navigateToChild(childId: string, parentItem: NavItem) {
    appStore.setCurrentNav(childId)
    const child = parentItem.children?.find((c) => c.id === childId)
    if (child?.path) router.push(child.path)
  }

  function syncNavFromRoute(path: string) {
    if (path.startsWith('/pipelines')) {
      appStore.setCurrentNav('pipelines')
      return
    }
    if (path.startsWith('/pipeline/')) {
      const segment = path.slice('/pipeline/'.length).split('/')[0]
      if (segment === 'node-plugins') {
        appStore.setCurrentNav('node-plugins')
        return
      }
    }

    const flat = flattenNavMenu(navMenu.value)
    for (const item of flat) {
      if (item.path === path) {
        appStore.setCurrentNav(item.id)
        return
      }
    }
  }

  function bindRouteSync(routeRef: Ref<string> = computed(() => route.path)) {
    watch(routeRef, syncNavFromRoute, { immediate: true })
    watch(edition, () => syncNavFromRoute(route.path))
  }

  return {
    visibleNavItems,
    currentNav,
    isChildActive,
    navigateTo,
    navigateToChild,
    syncNavFromRoute,
    bindRouteSync
  }
}