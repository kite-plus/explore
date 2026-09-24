import { CircleCheck } from 'lucide-react'
import { Link } from '@/admin/router'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import type { FetchQueueItem } from '@/lib/admin-types'

/** The blogs failing most in a row, with their last error. */
export function FailingBlogs({ items }: { items: FetchQueueItem[] }) {
  const failing = items
    .filter((item) => item.consecutive_failures > 0)
    .sort((a, b) => b.consecutive_failures - a.consecutive_failures)
    .slice(0, 6)
  if (failing.length === 0) {
    return (
      <p className='flex items-center gap-2 text-sm text-muted-foreground'>
        <CircleCheck className='size-4 text-success' />
        每个博客最近一次抓取都成功了。
      </p>
    )
  }
  return (
    <ul className='flex flex-col gap-1'>
      {failing.map((item) => (
        <li key={item.host}>
          <Link
            to={`/admin/queue?filter=${encodeURIComponent(item.host)}`}
            className='-mx-2 flex items-center gap-3 rounded-md px-2 py-2 hover:bg-accent/50'
          >
            <BlogAvatar host={item.host} name={item.name} />
            <div className='min-w-0 flex-1'>
              <p className='truncate text-sm font-medium'>{item.name || item.host}</p>
              <p className='truncate text-xs text-muted-foreground' title={item.last_error || undefined}>
                {item.last_error || item.host}
              </p>
            </div>
            <span className='shrink-0 text-xs font-medium text-destructive tabular-nums'>
              连续 {item.consecutive_failures} 次
            </span>
          </Link>
        </li>
      ))}
    </ul>
  )
}
