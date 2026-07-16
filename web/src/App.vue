<template>
  <WebLayout />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { syncEditionFromStatus } from '@/utils/edition'
import { getStatus } from '@/api'
import WebLayout from '@/components/layout/WebLayout.vue'

const appStore = useAppStore()

onMounted(async () => {
  appStore.initTheme()
  try {
    const status = await getStatus()
    syncEditionFromStatus(status)
  } catch {
    // edition falls back to HTML data attribute or team default
  }
})
</script>
