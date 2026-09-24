import { CircleCheck, CirclePause, TriangleAlert } from 'lucide-react'
import type { AdminBlog } from '@/lib/admin-types'

export type BlogState = 'active' | 'unhealthy' | 'paused'

export const blogStates = [
  { label: '运行中', value: 'active' as const, icon: CircleCheck, className: 'text-success' },
  { label: '抓取异常', value: 'unhealthy' as const, icon: TriangleAlert, className: 'text-destructive' },
  { label: '已暂停', value: 'paused' as const, icon: CirclePause, className: 'text-muted-foreground' },
]

export function blogState(blog: AdminBlog): BlogState {
  if (blog.status === 'paused') return 'paused'
  return !blog.visible || blog.consecutive_failures > 0 ? 'unhealthy' : 'active'
}

/** Distinct values of a field, most common first, for a faceted filter. */
export function facetOptions(values: string[]) {
  const counts = new Map<string, number>()
  for (const value of values) counts.set(value, (counts.get(value) ?? 0) + 1)
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([value]) => ({ label: value || '未知', value }))
}
