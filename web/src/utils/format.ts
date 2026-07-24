import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
// Locale is applied via setDayjsLocale() in App.vue / locale store (i18n/dayjs.ts).

dayjs.extend(relativeTime)

/**
 * 格式化日期时间
 */
export function formatDateTime(date: string | Date | null | undefined): string {
  if (!date) return '—'
  return dayjs(date).format('YYYY-MM-DD HH:mm:ss')
}

/**
 * 格式化相对时间
 */
export function formatRelativeTime(date: string | Date | null | undefined): string {
  if (!date) return '—'
  return dayjs(date).fromNow()
}

/**
 * 格式化数字
 */
export function formatNumber(num: number | null | undefined): string {
  if (num === null || num === undefined) return '—'
  return num.toLocaleString()
}

/**
 * 格式化字节大小
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i]
}

/**
 * 格式化百分比
 */
export function formatPercent(value: number | null | undefined, total: number): string {
  if (value === null || value === undefined || total === 0) return '0%'
  return ((value / total) * 100).toFixed(2) + '%'
}

/**
 * 格式化运行时长。
 * 兼容后端 Go duration（如 `8.644190584s`、`1h2m3.5s`）与已友好格式（如 `8s`、`1h2m`）。
 */
export function formatUptime(raw: string | null | undefined): string {
  if (!raw) return '--'
  const s = String(raw).trim()
  if (!s) return '--'

  // 已是简洁格式（无小数）则原样返回
  if (/^(\d+d)?(\d+h)?(\d+m)?(\d+s)?$/.test(s) && /[dhms]/.test(s)) {
    return s
  }

  let totalSec = 0
  const re = /([\d.]+)(ns|us|µs|ms|s|m|h)/g
  let matched = false
  let m: RegExpExecArray | null
  while ((m = re.exec(s)) !== null) {
    matched = true
    const n = parseFloat(m[1])
    if (Number.isNaN(n)) continue
    switch (m[2]) {
      case 'h':
        totalSec += n * 3600
        break
      case 'm':
        totalSec += n * 60
        break
      case 's':
        totalSec += n
        break
      case 'ms':
        totalSec += n / 1000
        break
      default:
        // ns/us：忽略
        break
    }
  }
  if (!matched) return s

  totalSec = Math.max(0, Math.floor(totalSec))
  const days = Math.floor(totalSec / 86400)
  const hours = Math.floor((totalSec % 86400) / 3600)
  const mins = Math.floor((totalSec % 3600) / 60)
  const secs = totalSec % 60
  if (days > 0) return `${days}d${hours}h${mins}m`
  if (hours > 0) return `${hours}h${mins}m${secs}s`
  if (mins > 0) return `${mins}m${secs}s`
  return `${secs}s`
}
