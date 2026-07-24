<template>
  <header class="app-header">
    <div class="header-left">
      <div class="logo" @click="$router.push('/')">
        <CentagMark :size="24" color="var(--shell-accent)" />
        <span class="logo-text">Centag</span>
      </div>

      <!-- 导航菜单 -->
      <nav class="header-nav">
        <template v-for="item in visibleNavItems" :key="item.id">
          <!-- 有子菜单的下拉菜单 -->
          <el-dropdown v-if="item.children" trigger="hover" @command="(cmd) => handleDropdownCommand(cmd, item)">
            <div class="nav-item" :class="{ active: currentNav === item.id || isChildActive(item) }" @click="handleDropdownClick(item)">
              <el-icon :size="16">
                <component :is="item.icon" />
              </el-icon>
              <span>{{ item.labelKey ? t(item.labelKey) : item.label }}</span>
              <el-icon :size="12" class="dropdown-arrow"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu class="nav-dropdown-menu">
                <template v-for="child in visibleChildren(item)" :key="child.id">
                  <!-- 嵌套分组：更多 → 接入 / 缓存 … -->
                  <el-dropdown-item
                    v-if="child.children?.length"
                    :class="{ 'is-active': currentNav === child.id || isChildActive(child) }"
                    @click.stop
                  >
                    <el-dropdown
                      trigger="hover"
                      placement="right-start"
                      teleported
                      popper-class="nav-submenu-popper"
                      @command="(cmd) => handleDropdownCommand(String(cmd), child)"
                    >
                      <div class="nav-submenu-trigger" @click.stop="handleDropdownClick(child)">
                        <el-icon><component :is="child.icon" /></el-icon>
                        <span>{{ child.labelKey ? t(child.labelKey) : child.label }}</span>
                        <el-icon class="submenu-arrow"><ArrowRight /></el-icon>
                      </div>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item
                            v-for="leaf in visibleChildren(child)"
                            :key="leaf.id"
                            :command="leaf.id"
                            :class="{ 'is-active': currentNav === leaf.id }"
                          >
                            <el-icon><component :is="leaf.icon" /></el-icon>
                            <span>{{ leaf.labelKey ? t(leaf.labelKey) : leaf.label }}</span>
                          </el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-else
                    :command="child.id"
                    :class="{ 'is-active': currentNav === child.id }"
                  >
                    <el-icon><component :is="child.icon" /></el-icon>
                    <span>{{ child.labelKey ? t(child.labelKey) : child.label }}</span>
                  </el-dropdown-item>
                </template>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <!-- 普通菜单项 -->
          <div
            v-else
            class="nav-item"
            :class="{ active: currentNav === item.id }"
            @click="handleNavClick(item)"
          >
            <el-icon :size="16">
              <component :is="item.icon" />
            </el-icon>
            <span>{{ item.labelKey ? t(item.labelKey) : item.label }}</span>
          </div>
        </template>
      </nav>
    </div>

    <div class="header-right">
      <el-button :loading="refreshing" @click="handleRefresh" circle>
        <el-icon><Refresh /></el-icon>
      </el-button>

      <el-dropdown trigger="click" @command="(cmd) => handleLocaleChange(cmd as AppLocale)">
        <el-button circle>
          <el-icon><Link /></el-icon>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="locale in supportedLocales"
              :key="locale"
              :command="locale"
              :class="{ 'is-active': localeStore.getLocale() === locale }"
            >
              {{ localeLabels[locale] }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <!-- Minimal：无用户菜单；改密/退出在概览页 -->
      <el-dropdown v-if="!isMinimal" trigger="click" @command="handleUserCommand">
        <div class="user-avatar">
          <el-avatar :size="32" :style="avatarStyle">
            {{ avatarText }}
          </el-avatar>
          <span class="username">{{ authStore.displayName }}</span>
          <el-icon :size="12" class="dropdown-arrow"><ArrowDown /></el-icon>
        </div>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item disabled>
              <div class="user-info">
                <div class="user-name">{{ authStore.displayName }}</div>
                <el-tag :type="authStore.isAdmin ? 'danger' : 'info'" size="small">
                  {{ authStore.isAdmin ? t('appHeader.admin') : t('appHeader.user') }}
                </el-tag>
              </div>
            </el-dropdown-item>
            <el-dropdown-item command="profile">
              <el-icon><User /></el-icon>{{ t('appHeader.profile') }}
            </el-dropdown-item>
            <el-dropdown-item v-if="showSystemConfig" command="config">
              <el-icon><Setting /></el-icon>{{ t('appHeader.systemConfig') }}
            </el-dropdown-item>
            <el-dropdown-item divided command="logout">
              <el-icon><SwitchButton /></el-icon>{{ t('appHeader.logout') }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useNavigation } from '@/composables/useNavigation'
import { useEdition } from '@/composables/useEdition'
import type { NavItem } from '@/utils/nav'
import { getCapabilities } from '@/utils/capabilities'
import { Refresh, ArrowDown, ArrowRight, User, SwitchButton, Setting, Link } from '@element-plus/icons-vue'
import CentagMark from '@/components/icons/CentagMark.vue'
import { ElMessageBox } from 'element-plus'
import { useLocaleStore } from '@/stores/locale'
import { supportedLocales, localeLabels, type AppLocale } from '@/i18n'

const { t } = useI18n()
const localeStore = useLocaleStore()
const router = useRouter()
const authStore = useAuthStore()
const { isMinimal, edition } = useEdition()
const showSystemConfig = computed(
  () => getCapabilities(edition.value, authStore.isAdmin).systemConfig
)
const {
  visibleNavItems,
  currentNav,
  isChildActive,
  navigateTo,
  navigateToChild,
  bindRouteSync
} = useNavigation()

const refreshing = ref(false)

function handleLocaleChange(locale: AppLocale) {
  localeStore.setLocale(locale)
}

bindRouteSync()

function visibleChildren(item: NavItem) {
  return item.children ?? []
}

function handleNavClick(item: NavItem) {
  navigateTo(item)
}

function handleDropdownClick(item: NavItem) {
  if (item.children && item.children.length > 0) {
    const first = visibleChildren(item)[0]
    if (first) navigateTo(first)
  }
}

function handleDropdownCommand(command: string, parentItem: NavItem) {
  navigateToChild(command, parentItem)
}

// ── Refresh ──────────────────────────────────────────────────────────────────

const handleRefresh = () => {
  refreshing.value = true
  window.location.reload()
}

// ── User menu ────────────────────────────────────────────────────────────────

async function handleUserCommand(cmd: string) {
  if (cmd === 'profile') {
    router.push('/profile')
  } else if (cmd === 'config') {
    router.push('/config')
  } else if (cmd === 'logout') {
    try {
      await ElMessageBox.confirm(t('appHeader.logoutConfirm'), t('appHeader.logoutTitle'), {
        confirmButtonText: t('appHeader.confirm'),
        cancelButtonText: t('appHeader.cancel'),
        type: 'warning'
      })
      await authStore.logout()
      router.push('/login')
    } catch {
      // user cancelled
    }
  }
}

// ── Avatar ───────────────────────────────────────────────────────────────────

const avatarText = computed(() => {
  const name = authStore.displayName
  return name ? name.charAt(0).toUpperCase() : 'U'
})

const avatarStyle = computed(() => ({
  background: authStore.isAdmin
    ? 'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)'
    : 'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)',
  color: '#fff',
  fontWeight: '600',
  fontSize: '14px',
  cursor: 'pointer'
}))

</script>

<style scoped>
.app-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: var(--header-height);
  background: var(--shell-header-bg);
  border-bottom: 1px solid var(--shell-header-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--spacing-xl);
  z-index: 1000;
  box-shadow: var(--shadow-sm);
}

.header-left {
  display: flex;
  align-items: center;
  gap: var(--spacing-xl);
}

.logo {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  cursor: pointer;
  padding: var(--spacing-sm) 0;
}

.logo-text {
  font-size: 1.0625rem;
  font-weight: 600;
  color: var(--shell-header-text);
  letter-spacing: -0.2px;
  white-space: nowrap;
}

.header-nav {
  display: flex;
  align-items: center;
  gap: 4px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 0.8125rem;
  color: var(--shell-sidebar-text);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  white-space: nowrap;
  font-weight: 400;
  border-radius: var(--shell-nav-radius);
  letter-spacing: 0.1px;
}

.nav-item:hover {
  background: var(--shell-sidebar-hover);
  color: var(--shell-sidebar-text);
}

.nav-item.active {
  background: var(--shell-sidebar-active-bg);
  color: var(--shell-sidebar-active-text);
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

/* User avatar area */
.user-avatar {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
  transition: background 0.2s;
}

.user-avatar:hover {
  background: var(--shell-sidebar-hover);
}

.username {
  font-size: 0.8125rem;
  color: var(--shell-sidebar-text);
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-arrow {
  color: var(--shell-sidebar-muted);
}

.nav-submenu-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 120px;
}

.submenu-arrow {
  margin-left: auto;
  color: var(--shell-sidebar-muted);
  font-size: 12px;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
}

.user-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

@media (max-width: 1200px) {
  .header-left { gap: var(--spacing-md); }
  .nav-item { padding: 8px 12px; font-size: 0.8125rem; }
}

@media (max-width: 768px) {
  .app-header { padding: 0 var(--spacing-md); }
  .logo-text { display: none; }
  .header-nav { gap: 2px; }
  .nav-item span { display: none; }
  .nav-item { padding: 8px; }
  .username { display: none; }
}
</style>
