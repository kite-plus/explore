import { avatarColor, initial } from '@/lib/avatar'

export { avatarColor, initial }

const dateTime = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const dateOnly = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium' })

export function formatDate(iso: string | null | undefined) {
  return iso ? dateOnly.format(new Date(iso)) : '—'
}

export function formatDateTime(iso: string | null | undefined) {
  return iso ? dateTime.format(new Date(iso)) : '—'
}

export function formatRelative(iso: string | null | undefined, now = Date.now()) {
  if (!iso) return '—'
  const diff = now - new Date(iso).getTime()
  const abs = Math.abs(diff)
  const suffix = diff < 0 ? '后' : '前'
  if (abs < 60_000) return '刚刚'
  if (abs < 3_600_000) return `${Math.floor(abs / 60_000)} 分钟${suffix}`
  if (abs < 86_400_000) return `${Math.floor(abs / 3_600_000)} 小时${suffix}`
  if (abs < 2_592_000_000) return `${Math.floor(abs / 86_400_000)} 天${suffix}`
  return new Date(iso).toLocaleDateString('zh-CN')
}

export function formatInterval(seconds: number) {
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`
  if (seconds < 86400) return `${Math.round(seconds / 3600)} 小时`
  return `${Math.round(seconds / 86400)} 天`
}
