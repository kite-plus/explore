import { cn } from '@/admin/lib/utils'
import { Link } from '@/admin/router'
import { queueStatuses } from '@/admin/features/queue/data/data'
import type { FetchQueueItem } from '@/lib/admin-types'

/** Fetch jobs by state, colored as in the queue table while a state has jobs. */
export function QueueStates({ items }: { items: FetchQueueItem[] }) {
  return (
    <div className='@container'>
      <ul className='grid gap-x-4 gap-y-1 @[18rem]:grid-cols-2'>
        {queueStatuses.map(({ value, label, icon: Icon, className }) => {
          const count = items.filter((item) => item.queue_status === value).length
          return (
            <li key={value}>
              <Link
                to={`/admin/queue?status=${encodeURIComponent(JSON.stringify([value]))}`}
                className='-mx-2 flex items-center gap-3 rounded-md px-2 py-2 text-sm hover:bg-accent/50'
              >
                <Icon className={cn('size-4 shrink-0', count > 0 ? className : 'text-muted-foreground')} />
                <span className={cn('truncate', count === 0 && 'text-muted-foreground')}>{label}</span>
                <span className={cn('ms-auto tabular-nums', count === 0 && 'text-muted-foreground')}>{count}</span>
              </Link>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
