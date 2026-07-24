import i18n from '@/i18n'

export function applyDocumentTitle(titleKey?: unknown) {
  if (typeof titleKey === 'string' && titleKey) {
    document.title = `${i18n.global.t(titleKey)} - Centag`
  } else {
    document.title = 'Centag'
  }
}
