export type DisplayCurrency = 'USD' | 'CNY'

const STORAGE_KEY = 'centag.displayCurrency'
export const DEFAULT_USD_TO_CNY = 7.2

export function getDisplayCurrency(): DisplayCurrency {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'CNY' || v === 'USD') return v
  } catch {
    /* ignore */
  }
  return 'USD'
}

export function setDisplayCurrency(c: DisplayCurrency) {
  try {
    localStorage.setItem(STORAGE_KEY, c)
  } catch {
    /* ignore */
  }
}

/** Convert a USD storage amount for display. */
export function toDisplayAmount(usd: number, display: DisplayCurrency, usdToCny: number): number {
  const rate = usdToCny > 0 ? usdToCny : DEFAULT_USD_TO_CNY
  if (display === 'CNY') return Number(usd || 0) * rate
  return Number(usd || 0)
}

export function currencySymbol(display: DisplayCurrency): string {
  return display === 'CNY' ? '¥' : '$'
}

export function formatDisplayCost(
  usd: number | undefined | null,
  display: DisplayCurrency,
  usdToCny: number
): string {
  const v = toDisplayAmount(Number(usd || 0), display, usdToCny)
  if (v === 0) return '0'
  if (v < 0.01) return v.toFixed(6)
  return v.toFixed(4)
}
