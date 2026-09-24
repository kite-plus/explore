import { CalendarClock, CircleAlert, CirclePause, Clock, LoaderCircle, RotateCcw } from 'lucide-react'
import type { FetchAttempt } from '@/lib/admin-types'

export const queueStatuses = [
  { label: '待执行', value: 'queued' as const, icon: Clock },
  { label: '抓取中', value: 'running' as const, icon: LoaderCircle },
  { label: '等待重试', value: 'retry' as const, icon: RotateCcw },
  { label: '执行中断', value: 'stalled' as const, icon: CircleAlert },
  { label: '定时等待', value: 'scheduled' as const, icon: CalendarClock },
  { label: '已暂停', value: 'paused' as const, icon: CirclePause },
]

export const outcomes: Record<FetchAttempt['outcome'], { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  running: { label: '抓取中', variant: 'outline' },
  changed: { label: '内容已更新', variant: 'default' },
  unchanged: { label: '内容无变化', variant: 'secondary' },
  failed: { label: '失败', variant: 'destructive' },
}
