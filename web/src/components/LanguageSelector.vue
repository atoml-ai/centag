<template>
  <div class="language-selector">
    <el-popover placement="top" :width="200" trigger="click">
      <template #reference>
        <el-button class="language-button" circle :title="localeLabels[currentLocale]">
          <span class="language-code">{{ shortCode }}</span>
        </el-button>
      </template>
      <div class="language-list">
        <div
          v-for="locale in supportedLocales"
          :key="locale"
          class="language-item"
          :class="{ active: currentLocale === locale }"
          @click="handleLocaleChange(locale)"
        >
          {{ localeLabels[locale] }}
        </div>
      </div>
    </el-popover>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useLocaleStore } from '@/stores/locale'
import { supportedLocales, localeLabels, type AppLocale } from '@/i18n'

const localeStore = useLocaleStore()
const { currentLocale } = storeToRefs(localeStore)

const shortCode = computed(() => {
  const map: Record<AppLocale, string> = {
    en: 'EN',
    'zh-CN': '中',
    ja: 'あ',
    ko: '한',
    ru: 'RU',
    es: 'ES'
  }
  return map[currentLocale.value] || 'EN'
})

function handleLocaleChange(locale: AppLocale) {
  localeStore.setLocale(locale)
}
</script>

<style scoped>
.language-selector {
  position: fixed;
  bottom: 48px;
  right: 20px;
  z-index: 1000;
}

.language-button {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: white;
  border: none;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.language-button:hover {
  background: var(--el-color-primary-light-3);
}

.language-code {
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
}

.language-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.language-item {
  padding: 8px 12px;
  border-radius: 4px;
  cursor: pointer;
  transition: background-color 0.2s;
}

.language-item:hover {
  background-color: var(--el-fill-color-light);
}

.language-item.active {
  background-color: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}
</style>
