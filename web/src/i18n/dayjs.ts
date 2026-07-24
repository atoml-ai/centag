import dayjs from 'dayjs'
import 'dayjs/locale/en'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/ja'
import 'dayjs/locale/ko'
import 'dayjs/locale/ru'
import 'dayjs/locale/es'
import type { AppLocale } from '@/i18n'

export const dayjsLocaleMap: Record<AppLocale, string> = {
  'en': 'en',
  'zh-CN': 'zh-cn',
  'ja': 'ja',
  'ko': 'ko',
  'ru': 'ru',
  'es': 'es'
}

export function setDayjsLocale(locale: AppLocale) {
  const dayjsLocale = dayjsLocaleMap[locale] || 'en'
  dayjs.locale(dayjsLocale)
}
