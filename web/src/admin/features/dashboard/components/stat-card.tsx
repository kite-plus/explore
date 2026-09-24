import { ChevronRight, type LucideIcon } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import { Link } from '@/admin/router'
import { Skeleton } from '@/admin/components/ui/skeleton'

export type Tone = 'success' | 'warning' | 'error' | 'neutral'

// The toasts' state colors.
const toneClass: Record<Tone, string> = {
  success: 'text-success',
  warning: 'text-warning',
  error: 'text-destructive',
  neutral: 'text-muted-foreground',
}

type StatCardProps = {
  title: string
  /** Undefined while loading. */
  value: number | string | undefined
  hint: React.ReactNode
  icon: LucideIcon
  tone: Tone
  /** Where the figure is handled; without it the card is not a link. */
  to?: string
}

/** A figure with its state. As in the toasts, only the icon carries the state. */
export function StatCard({ title, value, hint, icon: Icon, tone, to }: StatCardProps) {
  const className = 'flex flex-col gap-3 rounded-xl border bg-card p-4 text-card-foreground shadow-xs'
  const body = (
    <>
      <div className='flex items-center gap-2 text-sm font-medium'>
        <Icon className={cn('size-4 shrink-0', value === undefined ? toneClass.neutral : toneClass[tone])} />
        {title}
        {to && (
          <ChevronRight className='ms-auto size-4 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100' />
        )}
      </div>
      <div>
        {value === undefined ? (
          <Skeleton className='h-8 w-16' />
        ) : (
          <div className='text-2xl font-semibold tabular-nums'>
            {typeof value === 'number' ? value.toLocaleString('zh-CN') : value}
          </div>
        )}
        <p className='mt-1 text-xs text-muted-foreground'>{hint}</p>
      </div>
    </>
  )
  return to ? (
    <Link to={to} className={cn(className, 'group transition-colors hover:bg-accent/50')}>
      {body}
    </Link>
  ) : (
    <div className={className}>{body}</div>
  )
}
