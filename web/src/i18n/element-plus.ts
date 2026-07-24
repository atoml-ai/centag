import en from 'element-plus/es/locale/lang/en'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import ja from 'element-plus/es/locale/lang/ja'
import ko from 'element-plus/es/locale/lang/ko'
import ru from 'element-plus/es/locale/lang/ru'
import es from 'element-plus/es/locale/lang/es'
import type { AppLocale } from '@/i18n'

export const epLocaleMap: Record<AppLocale, typeof en> = {
  'en': en,
  'zh-CN': zhCn,
  'ja': ja,
  'ko': ko,
  'ru': ru,
  'es': es
}

export function getEpLocale(locale: AppLocale) {
  return epLocaleMap[locale] || en
}
