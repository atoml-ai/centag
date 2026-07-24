import { createI18n } from 'vue-i18n'
import en from '@/locales/en'
import zhCN from '@/locales/zh-CN'
import ja from '@/locales/ja'
import ko from '@/locales/ko'
import ru from '@/locales/ru'
import es from '@/locales/es'

export type AppLocale = 'en' | 'zh-CN' | 'ja' | 'ko' | 'ru' | 'es'

export const supportedLocales: AppLocale[] = ['en', 'zh-CN', 'ja', 'ko', 'ru', 'es']

export const localeLabels: Record<AppLocale, string> = {
  'en': 'English',
  'zh-CN': '简体中文',
  'ja': '日本語',
  'ko': '한국어',
  'ru': 'Русский',
  'es': 'Español'
}

const messages = {
  'en': en,
  'zh-CN': zhCN,
  'ja': ja,
  'ko': ko,
  'ru': ru,
  'es': es
}

function detectBrowserLocale(): AppLocale {
  const browserLang = navigator.language
  const lang = browserLang.toLowerCase()

  if (lang.startsWith('zh')) return 'zh-CN'
  if (lang.startsWith('ja')) return 'ja'
  if (lang.startsWith('ko')) return 'ko'
  if (lang.startsWith('ru')) return 'ru'
  if (lang.startsWith('es')) return 'es'
  return 'en'
}

function getInitialLocale(): AppLocale {
  const savedLocale = localStorage.getItem('centag.locale') as AppLocale | null
  if (savedLocale && supportedLocales.includes(savedLocale)) {
    return savedLocale
  }
  return detectBrowserLocale()
}

const i18n = createI18n({
  legacy: false,
  locale: getInitialLocale(),
  fallbackLocale: 'en',
  messages,
  missingWarn: import.meta.env.DEV,
  fallbackWarn: import.meta.env.DEV
})

export default i18n

export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  localStorage.setItem('centag.locale', locale)
  document.documentElement.lang = locale
}
