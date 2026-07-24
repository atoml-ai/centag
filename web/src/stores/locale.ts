import { defineStore } from 'pinia'
import { ref } from 'vue'
import i18n, { setLocale as i18nSetLocale, type AppLocale, supportedLocales } from '@/i18n'
import { setDayjsLocale } from '@/i18n/dayjs'
import { applyDocumentTitle } from '@/i18n/document-title'

export const useLocaleStore = defineStore('locale', () => {
  const currentLocale = ref<AppLocale>(i18n.global.locale.value as AppLocale)

  function setLocale(locale: AppLocale) {
    if (!supportedLocales.includes(locale)) {
      console.warn(`Unsupported locale: ${locale}`)
      return
    }
    currentLocale.value = locale
    i18nSetLocale(locale)
    setDayjsLocale(locale)
    document.documentElement.lang = locale

    // Refresh browser tab title for the current route (lazy to avoid init cycles).
    void import('@/router')
      .then(({ default: router }) => {
        applyDocumentTitle(router.currentRoute.value.meta.titleKey)
      })
      .catch(() => {
        // ignore circular-init races during first paint
      })
  }

  function getLocale(): AppLocale {
    return currentLocale.value
  }

  return {
    currentLocale,
    setLocale,
    getLocale
  }
})
