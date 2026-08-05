import { createI18n } from 'vue-i18n'
import en from '@/locales/en'
import zhCN from '@/locales/zh-CN'
import ja from '@/locales/ja'
import ko from '@/locales/ko'
import ru from '@/locales/ru'
import es from '@/locales/es'
import { teamPackLocaleMessages } from '@team-pack'

function deepMerge(base: Record<string, unknown>, extra: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = { ...base }
  for (const [k, v] of Object.entries(extra)) {
    const cur = out[k]
    if (
      v &&
      typeof v === 'object' &&
      !Array.isArray(v) &&
      cur &&
      typeof cur === 'object' &&
      !Array.isArray(cur)
    ) {
      out[k] = deepMerge(cur as Record<string, unknown>, v as Record<string, unknown>)
    } else {
      out[k] = v
    }
  }
  return out
}

function withTeamPack(localeKey: string, hostMessages: Record<string, unknown>): Record<string, unknown> {
  const pack = (teamPackLocaleMessages as Record<string, Record<string, unknown>> | undefined)?.[localeKey]
  if (!pack) return hostMessages
  return deepMerge(hostMessages, pack)
}

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
  'en': withTeamPack('en', en as unknown as Record<string, unknown>),
  'zh-CN': withTeamPack('zh-CN', zhCN as unknown as Record<string, unknown>),
  // fallback: use English team pack strings for other locales when pack keys missing
  'ja': withTeamPack('en', withTeamPack('ja', ja as unknown as Record<string, unknown>)),
  'ko': withTeamPack('en', withTeamPack('ko', ko as unknown as Record<string, unknown>)),
  'ru': withTeamPack('en', withTeamPack('ru', ru as unknown as Record<string, unknown>)),
  'es': withTeamPack('en', withTeamPack('es', es as unknown as Record<string, unknown>))
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
