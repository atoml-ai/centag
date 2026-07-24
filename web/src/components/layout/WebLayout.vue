<template>
  <div :class="['layout', { 'layout-auth': isLoginPage }]">
    <AppHeader v-if="!isLoginPage" />
    <div class="layout-body">
      <main :class="['main', { 'main-auth': isLoginPage }]">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
    <LiveLogSidebar v-if="!isLoginPage" :visible="showLiveLogs" @close="closeLogPanel" />
    <StatusBar v-if="!isLoginPage" />
    <LanguageSelector />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { useLogPanel } from '@/composables/useLogPanel'
import AppHeader from '@/components/layout/AppHeader.vue'
import StatusBar from '@/components/layout/StatusBar.vue'
import LiveLogSidebar from '@/components/layout/LiveLogSidebar.vue'
import LanguageSelector from '@/components/LanguageSelector.vue'

const route = useRoute()
const isLoginPage = computed(() => route.path === '/login')
const { visible: showLiveLogs, toggle: toggleLiveLogs, close: closeLogPanel } = useLogPanel()

onMounted(() => {
  window.addEventListener('centag:toggle-log-sidebar', toggleLiveLogs)
})

onBeforeUnmount(() => {
  window.removeEventListener('centag:toggle-log-sidebar', toggleLiveLogs)
})
</script>

<style scoped>
.layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.layout-auth {
  height: 100vh;
  overflow: hidden;
}

.layout-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.main {
  flex: 1;
  min-width: 0;
  padding: calc(var(--header-height) + var(--spacing-md)) var(--spacing-md) var(--spacing-md);
  margin-top: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.main-auth {
  padding: 0;
}

@media (max-width: 768px) {
  .main {
    padding: calc(var(--header-height) + var(--spacing-sm)) var(--spacing-sm) var(--spacing-sm);
  }
}
</style>