import { CalendarClock, CircleAlert, CirclePause, Clock, LoaderCircle, RotateCcw } from 'lucide-react'
import type { FetchAttempt } from '@/lib/admin-types'

export const queueStatuses = [
  { label: '待执行', value: 'queued' as const, icon: Clock, className: 'text-info' },
  { label: '抓取中', value: 'running' as const, icon: LoaderCircle, className: 'text-info' },
  { label: '等待重试', value: 'retry' as const, icon: RotateCcw, className: 'text-warning' },
  { label: '执行中断', value: 'stalled' as const, icon: CircleAlert, className: 'text-destructive' },
  { label: '定时等待', value: 'scheduled' as const, icon: CalendarClock, className: 'text-success' },
  { label: '已暂停', value: 'paused' as const, icon: CirclePause, className: 'text-muted-foreground' },
]

/** Each outcome's text color and timeline dot. */
export const outcomes: Record<FetchAttempt['outcome'], { label: string; className: string; dot: string }> = {
  running: { label: '抓取中', className: 'text-info', dot: 'bg-info' },
  changed: { label: '内容已更新', className: 'text-success', dot: 'bg-success' },
  unchanged: { label: '内容无变化', className: 'text-muted-foreground', dot: 'bg-muted-foreground' },
  failed: { label: '失败', className: 'text-destructive', dot: 'bg-destructive' },
}
