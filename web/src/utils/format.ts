import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

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
