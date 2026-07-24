<template>
  <el-config-provider :locale="epLocale">
    <WebLayout />
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useAppStore } from '@/stores/app'
import { useLocaleStore } from '@/stores/locale'
import { syncEditionFromStatus } from '@/utils/edition'
import { getStatus } from '@/api'
import { getEpLocale } from '@/i18n/element-plus'
import { setDayjsLocale } from '@/i18n/dayjs'
import WebLayout from '@/components/layout/WebLayout.vue'

const appStore = useAppStore()
const localeStore = useLocaleStore()
const { currentLocale } = storeToRefs(localeStore)
const epLocale = computed(() => getEpLocale(currentLocale.value))

onMounted(async () => {
  appStore.initTheme()
  setDayjsLocale(currentLocale.value)
  try {
    const status = await getStatus()
    syncEditionFromStatus(status)
  } catch {
    // edition falls back to HTML data attribute or team default
  }
})
</script>
