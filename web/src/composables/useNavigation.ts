import { computed, watch, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'
import { findNavItemById, flattenNavMenu, getNavMenu, type NavItem } from '@/utils/nav'
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

  function isChildActive(item: NavItem): boolean {
    if (!item.children?.length) return false
    return item.children.some(
      (child) => child.id === currentNav.value || isChildActive(child)
    )
  }

  function navigateTo(item: NavItem) {
    // 分组节点：跳到第一个可导航叶子
    if (item.children?.length && !isLeafNavigable(item)) {
      const leaf = firstNavigableLeaf(item)
      if (leaf) {
        appStore.setCurrentNav(leaf.id)
        if (leaf.path) router.push(leaf.path)
        return
      }
    }
    appStore.setCurrentNav(item.id)
    if (item.path) router.push(item.path)
  }

  function navigateToChild(childId: string, parentItem: NavItem) {
    const child =
      findNavItemById(parentItem.children ?? [], childId) ||
      findNavItemById([parentItem], childId)
    if (!child) return
    navigateTo(child)
  }

  function isLeafNavigable(item: NavItem) {
    return !!item.path && !item.children?.length
  }

  function firstNavigableLeaf(item: NavItem): NavItem | undefined {
    if (isLeafNavigable(item)) return item
    for (const child of item.children ?? []) {
      const leaf = firstNavigableLeaf(child)
      if (leaf) return leaf
    }
    return undefined
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